package layer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenCHAMI/image-builder/go/pkg/utils"
	"github.com/containers/buildah"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// AnsibleRunner manages containerized Ansible execution
type AnsibleRunner struct {
	store         storage.Store
	helperBuilder *buildah.Builder
	logger        *log.Logger
}

// NewAnsibleRunner creates a new containerized Ansible runner
func NewAnsibleRunner(ctx context.Context) (*AnsibleRunner, error) {
	// Create Ansible helper container with Python and Ansible pre-installed
	// Using a base image that has Ansible and common collections available
	ansibleImage := "docker.io/cytopia/ansible:latest"
	containerName := "ansible-helper-" + generateRandomString(8)

	store, err := utils.GetStore()
	if err != nil {
		return nil, fmt.Errorf("failed to get buildah store")
	}

	// Use our existing utility to create the builder - this handles storage correctly
	helperBuilder, err := utils.CreateBuilder(ctx, store, ansibleImage, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ansible helper container: %w", err)
	}

	return &AnsibleRunner{
		store:         store,
		helperBuilder: helperBuilder,
		logger:        log.New(os.Stdout, "[ANSIBLE] ", log.LstdFlags),
	}, nil
}

// Cleanup cleans up the Ansible runner resources
func (ar *AnsibleRunner) Cleanup() {
	_, err := ar.store.Shutdown(false)
	if err != nil {
		ar.logger.Printf("Warning: failed to delete storage: %v", err)
	}

	if ar.helperBuilder != nil {
		ar.helperBuilder.Delete()
	}
	// Store cleanup is handled by the CreateBuilder utility
}

// RunPlaybooks executes Ansible playbooks against target containers using containerized approach
func (ar *AnsibleRunner) RunPlaybooks(targetBuilder *buildah.Builder, playbooks []string, groups []string, vars map[string]string, inventoryPath string, verbosity int) error {
	// Mount the target container to get access to its filesystem
	targetMountPoint, err := targetBuilder.Mount("")
	if err != nil {
		return fmt.Errorf("failed to mount target container: %w", err)
	}
	defer targetBuilder.Unmount()

	// Create temporary directory in helper container for Ansible work
	tempDir := "/tmp/ansible-work"
	if err := utils.RunCommandInBuilder(ar.helperBuilder, []string{"mkdir", "-p", tempDir}, nil); err != nil {
		return fmt.Errorf("failed to create work directory in Ansible helper: %w", err)
	}

	// Get container name for use in playbook execution
	containerName := targetBuilder.Container

	// Determine Ansible directory to bind mount (common parent of playbooks)
	var ansibleDir string
	if len(playbooks) > 0 {
		// Use the parent directory of the first playbook's directory (e.g., /path/to/ansible/playbooks -> /path/to/ansible)
		playbookDir := filepath.Dir(playbooks[0])
		ansibleDir = filepath.Dir(playbookDir)
	}

	// Execute playbooks with bind mounts
	for _, playbook := range playbooks {
		if err := ar.executePlaybook(containerName, playbook, ansibleDir, inventoryPath, targetMountPoint, vars, verbosity); err != nil {
			return fmt.Errorf("failed to execute playbook %s: %w", playbook, err)
		}
	}

	return nil
}

// generateRandomString creates a random string for container names
func generateRandomString(length int) string {
	bytes := make([]byte, length/2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// createTempResolvConf creates a temporary resolv.conf file on the host
func createTempResolvConf() (string, error) {
	// Create temporary file on host
	tmpFile, err := os.CreateTemp("", "resolv-*.conf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp resolv.conf: %w", err)
	}

	// Write DNS content
	dnsContent := `
nameserver 8.8.8.8
nameserver 8.8.4.4
`
	if _, err := tmpFile.WriteString(dnsContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write DNS content: %w", err)
	}

	tmpFile.Close()
	return tmpFile.Name(), nil
}

// executePlaybook runs an Ansible playbook with bind-mounted Ansible directory
func (ar *AnsibleRunner) executePlaybook(containerName, playbookPath, ansibleDir, inventoryPath, targetMount string, vars map[string]string, verbosity int) error {
	// Build ansible-playbook command with paths relative to bind mount
	ansibleMountPoint := "/ansible"
	chrootPath := "/chroot"

	// Clean up trailing slash from inventory path
	cleanInventoryPath := strings.TrimSuffix(inventoryPath, "/")

	relativePath, err := filepath.Rel(ansibleDir, playbookPath)
	if err != nil {
		return fmt.Errorf("failed to get relative path for playbook: %w", err)
	}

	// Get the relative path from ansible dir to inventory
	inventoryRelPath, err := filepath.Rel(ansibleDir, cleanInventoryPath)
	if err != nil {
		return fmt.Errorf("failed to get relative inventory path: %w", err)
	}

	// Create a minimal hosts file with our containerized target
	hostsFile := "/tmp/hosts"
	hostsContent := fmt.Sprintf("[compute]\n%s ansible_connection=chroot ansible_host=%s ansible_python_interpreter=/usr/bin/python3\n", containerName, chrootPath)

	// Prepare ansible-playbook command - create hosts file and use the inventory directory (which will load group_vars automatically)
	setupCmd := fmt.Sprintf("echo '%s' > %s", hostsContent, hostsFile)

	// Build extra vars string from the vars map
	extraVars := "ansible_host=" + chrootPath
	for key, value := range vars {
		extraVars += fmt.Sprintf(" %s=%s", key, value)
	}

	args := []string{
		"sh", "-c",
		fmt.Sprintf("%s && ansible-playbook -i %s -i %s/%s --connection=chroot --limit %s --extra-vars='%s' %s",
			setupCmd, hostsFile, ansibleMountPoint, inventoryRelPath, containerName, extraVars, ansibleMountPoint+"/"+relativePath),
	}

	// Add verbosity flags (force at least -vv to see chroot operations)
	minVerbosity := 2
	if verbosity < minVerbosity {
		verbosity = minVerbosity
	}
	verbosityFlags := ""
	for i := 0; i < verbosity; i++ {
		verbosityFlags += " -v"
	}

	// Update the ansible-playbook command with verbosity
	args[2] = fmt.Sprintf("%s && ansible-playbook -i %s -i %s/%s --connection=chroot --limit %s --extra-vars='%s'%s %s",
		setupCmd, hostsFile, ansibleMountPoint, inventoryRelPath, containerName, extraVars, verbosityFlags, ansibleMountPoint+"/"+relativePath)

	// Create temporary resolv.conf for DNS resolution in chroot
	resolvConfPath, err := createTempResolvConf()
	if err != nil {
		return fmt.Errorf("failed to create temporary resolv.conf: %w", err)
	}
	defer os.Remove(resolvConfPath) // Clean up after execution

	// Create bind mount specifications - mount Ansible data and target filesystem
	// The ansible mount includes the inventory directory
	mounts := []specs.Mount{
		{
			Source:      ansibleDir,
			Destination: ansibleMountPoint,
			Type:        "bind",
			Options:     []string{"bind", "ro"},
		},
		{
			Source:      targetMount,
			Destination: chrootPath,
			Type:        "bind",
			Options:     []string{"bind", "rw"},
		},
		// Mount /proc, /sys, /dev for network access in chroot
		{
			Source:      "/proc",
			Destination: chrootPath + "/proc",
			Type:        "bind",
			Options:     []string{"bind"},
		},
		{
			Source:      "/sys",
			Destination: chrootPath + "/sys",
			Type:        "bind",
			Options:     []string{"bind"},
		},
		{
			Source:      "/dev",
			Destination: chrootPath + "/dev",
			Type:        "bind",
			Options:     []string{"bind"},
		},
		// Mount temporary resolv.conf for DNS resolution
		{
			Source:      resolvConfPath,
			Destination: chrootPath + "/etc/resolv.conf",
			Type:        "bind",
			Options:     []string{"bind"},
		},
	}

	// Execute command in helper container with bind mount
	err = utils.RunCommandInBuilder(ar.helperBuilder, args, mounts)
	if err != nil {
		return fmt.Errorf("failed to run ansible-playbook: %w", err)
	}

	fmt.Printf("Playbook %s executed successfully for container %s\n", filepath.Base(playbookPath), containerName)
	return nil
}

// RunAnsiblePlaybooks executes Ansible playbooks in helper container
func RunAnsiblePlaybooks(ctx context.Context, targetBuilder *buildah.Builder, playbooks []string, groups []string, vars map[string]string, inventoryPath string, verbosity int) error {
	// Create new Ansible runner
	runner, err := NewAnsibleRunner(ctx)
	if err != nil {
		return fmt.Errorf("failed to create Ansible runner: %w", err)
	}
	defer runner.Cleanup()

	// Run playbooks using containerized approach
	return runner.RunPlaybooks(targetBuilder, playbooks, groups, vars, inventoryPath, verbosity)
}

// BuildAnsible builds a layer using Ansible
func (l *Layer) BuildAnsible() error {
	containerName := l.Config.Name

	l.Logger.Printf("Building Ansible layer: %s from parent %s", containerName, l.Config.Parent)

	// Create buildah context
	ctx := context.Background()

	store, err := utils.GetStore()
	if err != nil {
		return fmt.Errorf("failed to get buildah store: %w", err)
	}

	// Create target builder
	builder, err := utils.CreateBuilder(ctx, store, l.Config.Parent, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Set up cleanup in case of errors
	defer func() {
		if _, err := store.Shutdown(false); err != nil {
			l.Logger.Printf("Warning: failed to delete storage: %v", err)
		}

		if deleteErr := builder.Delete(); deleteErr != nil {
			l.Logger.Printf("Warning: failed to clean up target container on error: %v", deleteErr)
		}

	}()

	// Run Ansible playbooks
	l.Logger.Printf("Running Ansible playbooks: %v", l.Config.AnsiblePlaybook)
	if err := RunAnsiblePlaybooks(
		ctx,
		builder,
		l.Config.AnsiblePlaybook,
		l.Config.AnsibleGroups,
		l.Config.AnsibleVars,
		l.Config.AnsibleInv,
		l.Config.AnsibleVerbosity,
	); err != nil {
		return fmt.Errorf("failed to run Ansible playbooks: %w", err)
	}

	// Commit the changes to a new image
	l.Logger.Printf("Committing changes to image: %s", containerName)

	// Setup system context
	systemContext := &types.SystemContext{}

	// Create the commit options
	commitOptions := buildah.CommitOptions{
		Squash:        true,
		OmitTimestamp: false,
		SystemContext: systemContext,
		ReportWriter:  os.Stdout,
		// Add additional tags to the image
		AdditionalTags: []string{containerName},
	}

	// Use nil as ImageReference to let buildah generate a reference
	imageID, _, _, err := builder.Commit(ctx, nil, commitOptions)
	if err != nil {
		return fmt.Errorf("failed to commit container: %w", err)
	}
	l.Logger.Printf("Successfully built Ansible image: %s", imageID)

	// Clean up the target container after successful commit
	if err := builder.Delete(); err != nil {
		l.Logger.Printf("Warning: failed to clean up target container: %v", err)
	}

	return nil
}
