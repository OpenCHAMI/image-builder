package publish

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/containers/storage"
)

// S3Publisher implements Publisher interface for AWS S3 publishing
type S3Publisher struct {
	logger *log.Logger
	config S3PublishConfig
}

// NewS3Publisher creates a new S3 publisher
func NewS3Publisher(config S3PublishConfig) (*S3Publisher, error) {
	if config.S3Endpoint == "" {
		return nil, fmt.Errorf("S3 endpoint is required")
	}
	if config.S3Bucket == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}

	return &S3Publisher{
		logger: log.New(os.Stdout, "[S3Publisher] ", log.LstdFlags),
		config: config,
	}, nil
}

// validate checks if S3 configuration is valid
func (s *S3Publisher) validate(config S3PublishConfig) error {

	return nil
}

// Publish publishes the image to S3 (implements Publisher interface)
func (s *S3Publisher) Publish(ctx context.Context, imageID, containerName string, imageConfig ImageConfig, store storage.Store) error {
	// Validate configuration
	if err := s.validate(s.config); err != nil {
		return fmt.Errorf("S3 configuration validation failed: %w", err)
	}

	// Mount the container to access its filesystem
	mountPoint, err := s.mountContainer(containerName, store)
	if err != nil {
		return fmt.Errorf("failed to mount container: %w", err)
	}
	defer s.unmountContainer(containerName, store)

	return s.uploadToS3(ctx, mountPoint, imageConfig)
}

// mountContainer mounts the container and returns the mount point
func (s *S3Publisher) mountContainer(containerName string, store storage.Store) (string, error) {
	// Use buildah mount command to get the mount point
	cmd := exec.Command("buildah", "mount", containerName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to mount container %s: %w", containerName, err)
	}

	mountPoint := strings.TrimSpace(string(output))
	s.logger.Printf("Container mounted at: %s", mountPoint)
	return mountPoint, nil
}

// unmountContainer unmounts the container
func (s *S3Publisher) unmountContainer(containerName string, store storage.Store) {
	cmd := exec.Command("buildah", "umount", containerName)
	if err := cmd.Run(); err != nil {
		s.logger.Printf("Warning: failed to unmount container %s: %v", containerName, err)
	}
}

// uploadToS3 handles the S3 upload process similar to the Python implementation
func (s *S3Publisher) uploadToS3(ctx context.Context, mountPoint string, imageConfig ImageConfig) error {
	s.logger.Printf("Starting S3 upload process for mount point: %s", mountPoint)

	// Create S3 client
	s3Client, err := s.createS3Client(ctx)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Create temporary directory for squashed image
	tempDir, err := os.MkdirTemp("", "image-builder-s3-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Find kernel version and boot files
	kernelVersion, initrd, vmlinuz, err := s.findBootFiles(mountPoint)
	if err != nil {
		return fmt.Errorf("failed to find boot files: %w", err)
	}

	s.logger.Printf("Found kernel version: %s, initrd: %s, vmlinuz: %s", kernelVersion, initrd, vmlinuz)

	// Squash the image
	squashFile := filepath.Join(tempDir, "rootfs")
	if err := s.squashImage(mountPoint, squashFile); err != nil {
		return fmt.Errorf("failed to squash image: %w", err)
	}

	// Get OS information and build image name
	osVersion, err := s.getOSVersion(mountPoint)
	if err != nil {
		s.logger.Printf("Warning: could not determine OS version: %v", err)
		osVersion = "unknown"
	}

	// Build S3 keys
	s3Prefix := s.config.S3Prefix
	if s3Prefix == "" {
		s3Prefix = "image"
	}

	tags := imageConfig.PublishTags
	if len(tags) == 0 {
		tags = []string{"latest"}
	}
	imageName := fmt.Sprintf("%s-%s-%s-%s", s3Prefix, osVersion, imageConfig.Name, tags[0])

	s.logger.Printf("Uploading files to S3 bucket: %s", s.config.S3Bucket)

	// Upload vmlinuz if it exists
	if vmlinuz != "" {
		vmlinuzPath := filepath.Join(mountPoint, "boot", vmlinuz)
		if err := s.uploadFileToS3(ctx, s3Client, vmlinuzPath, "efi-images/"+s3Prefix+vmlinuz, s.config.S3Bucket); err != nil {
			return fmt.Errorf("failed to upload vmlinuz: %w", err)
		}
	} else {
		s.logger.Printf("No vmlinuz to upload - skipping")
	}

	// Upload initrd if it exists
	if initrd != "" {
		initrdPath := filepath.Join(mountPoint, "boot", initrd)
		if err := s.uploadFileToS3(ctx, s3Client, initrdPath, "efi-images/"+s3Prefix+initrd, s.config.S3Bucket); err != nil {
			return fmt.Errorf("failed to upload initrd: %w", err)
		}
	} else {
		s.logger.Printf("No initramfs to upload - skipping")
	}

	// Upload squashed rootfs
	if err := s.uploadFileToS3(ctx, s3Client, squashFile, imageName, s.config.S3Bucket); err != nil {
		return fmt.Errorf("failed to upload rootfs: %w", err)
	}

	s.logger.Printf("Successfully uploaded image to S3: %s", imageName)
	return nil
}

// createS3Client creates an S3 client with proper configuration
func (s *S3Publisher) createS3Client(ctx context.Context) (*s3.Client, error) {
	// For now, use default AWS configuration
	// TODO: Add support for custom endpoints and credentials from config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// If custom credentials are needed, they could be added here
	// For example, if config had S3 credential fields:
	// cfg.Credentials = credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")

	return s3.NewFromConfig(cfg), nil
}

// findBootFiles finds the kernel version and corresponding boot files
func (s *S3Publisher) findBootFiles(mountPoint string) (kernelVersion, initrd, vmlinuz string, err error) {
	modulesDir := filepath.Join(mountPoint, "lib", "modules")

	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read modules directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		kver := entry.Name()
		s.logger.Printf("Checking kernel version: %s", kver)

		kernelVersion = kver

		// Look for vmlinuz
		vmlinuzPath := filepath.Join(mountPoint, "boot", "vmlinuz-"+kver)
		if _, err := os.Stat(vmlinuzPath); err == nil {
			vmlinuz = "vmlinuz-" + kver
			s.logger.Printf("Found vmlinuz: %s", vmlinuz)
		} else {
			s.logger.Printf("No vmlinuz found for %s", kver)
			vmlinuz = ""
		}

		// Look for initramfs
		initramfsPath := filepath.Join(mountPoint, "boot", "initramfs-"+kver+".img")
		initrdPath := filepath.Join(mountPoint, "boot", "initrd-"+kver)

		if _, err := os.Stat(initramfsPath); err == nil {
			initrd = "initramfs-" + kver + ".img"
			s.logger.Printf("Found initramfs: %s", initrd)
		} else if _, err := os.Stat(initrdPath); err == nil {
			initrd = "initrd-" + kver
			s.logger.Printf("Found initrd: %s", initrd)
		} else {
			s.logger.Printf("No initramfs found for %s", kver)
			initrd = ""
		}

		// If we found at least one boot file, we can proceed
		if vmlinuz != "" || initrd != "" {
			return kernelVersion, initrd, vmlinuz, nil
		}

		s.logger.Printf("No usable boot files found for %s, trying next kernel", kver)
	}

	return "", "", "", fmt.Errorf("no valid kernel/initramfs combination found")
}

// squashImage creates a squashfs image from the mounted filesystem
func (s *S3Publisher) squashImage(mountPoint, outputFile string) error {
	s.logger.Printf("Creating squashfs image: %s -> %s", mountPoint, outputFile)

	cmd := exec.Command("mksquashfs", mountPoint, outputFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mksquashfs failed: %w, output: %s", err, string(output))
	}

	s.logger.Printf("Successfully created squashfs image")
	return nil
}

// getOSVersion attempts to determine the OS version from the mounted filesystem
// This matches the Python implementation logic exactly
func (s *S3Publisher) getOSVersion(mountPoint string) (string, error) {
	// Try to read /etc/os-release
	osReleasePath := filepath.Join(mountPoint, "etc", "os-release")
	if data, err := os.ReadFile(osReleasePath); err == nil {
		osDict := make(map[string]string)
		content := string(data)

		// Parse key=value pairs like Python implementation
		for _, line := range strings.Split(content, "\n") {
			if strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					osDict[key] = value
				}
			}
		}

		// Follow Python logic: ID + VERSION_ID, or fallback to ID_LIKE + NAME
		if id, hasID := osDict["ID"]; hasID {
			if versionID, hasVersion := osDict["VERSION_ID"]; hasVersion {
				// Remove quotes from both
				id = strings.Trim(id, `"`)
				versionID = strings.Trim(versionID, `"`)
				return strings.ToLower(id + versionID), nil
			}
		}

		if idLike, hasIDLike := osDict["ID_LIKE"]; hasIDLike {
			if name, hasName := osDict["NAME"]; hasName {
				// Remove quotes from both
				idLike = strings.Trim(idLike, `"`)
				name = strings.Trim(name, `"`)
				return strings.ToLower(idLike + "-" + name), nil
			}
		}
	}

	// Fallback: try other methods
	redhatReleasePath := filepath.Join(mountPoint, "etc", "redhat-release")
	if data, err := os.ReadFile(redhatReleasePath); err == nil {
		content := strings.TrimSpace(string(data))
		content = strings.ReplaceAll(content, " ", "-")
		content = strings.ToLower(content)
		return content, nil
	}

	return "unknown", fmt.Errorf("could not determine OS version")
}

// uploadFileToS3 uploads a single file to S3
func (s *S3Publisher) uploadFileToS3(ctx context.Context, client *s3.Client, filePath, s3Key, bucket string) error {
	s.logger.Printf("Uploading %s as %s to bucket %s", filePath, s3Key, bucket)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(s3Key),
		Body:   file,
	})

	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}

	s.logger.Printf("Successfully uploaded: %s", s3Key)
	return nil
}
