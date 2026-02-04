package layer

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/OpenCHAMI/image-builder/go/pkg/config"
	"github.com/OpenCHAMI/image-builder/go/pkg/installer"

	"github.com/containers/buildah"
	"github.com/containers/storage/pkg/unshare"

	"github.com/OpenCHAMI/image-builder/go/pkg/utils"
)

// Layer handles image layer building operations
type Layer struct {
	Config *config.Config
	Logger *log.Logger
}

// NewLayer creates a new layer builder
func NewLayer(cfg *config.Config) (*Layer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if cfg.Name == "" {
		return nil, fmt.Errorf("config.Name is required")
	}
	if cfg.LayerType == "" {
		return nil, fmt.Errorf("config.LayerType is required")
	}
	if cfg.Parent == "" {
		return nil, fmt.Errorf("config.Parent is required")
	}

	return &Layer{
		Config: cfg,
		Logger: log.New(os.Stdout, "[LAYER] ", log.LstdFlags),
	}, nil
}

// BuildLayer builds a layer based on the layer type and publishes it if needed
func (l *Layer) BuildLayer() error {
	if l.Config == nil {
		return fmt.Errorf("layer config is nil")
	}
	if l.Config.Name == "" {
		return fmt.Errorf("layer name cannot be empty")
	}
	if l.Config.LayerType == "" {
		return fmt.Errorf("layer type cannot be empty")
	}

	l.Logger.Printf("[INFO] Starting layer build - Name: %s, Type: %s, Parent: %s", l.Config.Name, l.Config.LayerType, l.Config.Parent)

	var err error

	// Build the layer based on the type
	switch l.Config.LayerType {
	case "base":
		err = l.BuildBase()
	case "ansible":
		err = l.BuildAnsible()
	default:
		return fmt.Errorf("unsupported layer type: %s", l.Config.LayerType)
	}

	if err != nil {
		return fmt.Errorf("failed to build layer: %w", err)
	}

	l.Logger.Printf("[INFO] Layer build pipeline completed successfully - %s (%s)", l.Config.Name, l.Config.LayerType)

	return nil
}

// BuildBase builds a base layer
func (l *Layer) BuildBase() error {
	if l.Config.Parent == "" {
		return fmt.Errorf("parent image cannot be empty for base layer")
	}

	dtString := time.Now().Format("20060102150405")
	containerName := l.Config.Name + "-" + dtString

	l.Logger.Printf("[INFO] Building base layer - Container: %s, Parent: %s, Package Manager: %s", containerName, l.Config.Parent, l.Config.PackageManager)

	// Create buildah context
	ctx := context.Background()

	// Add debugging information
	l.Logger.Printf("[DEBUG] Runtime info - UID: %d, Rootless: %v", os.Geteuid(), unshare.IsRootless())

	// Create a container from the parent image
	l.Logger.Printf("[INFO] Creating buildah container from base image: %s", l.Config.Parent)

	store, err := utils.GetStore()
	if err != nil {
		return fmt.Errorf("failed to get buildah store: %w", err)
	}

	targetBuilder, err := utils.CreateBuilder(ctx, store, l.Config.Parent, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container %s from image %s: %w", containerName, l.Config.Parent, err)
	}

	defer func() {
		if _, err := store.Shutdown(false); err != nil {
			l.Logger.Printf("Warning: failed to delete storage: %v", err)
		}

		if deleteErr := targetBuilder.Delete(); deleteErr != nil {
			l.Logger.Printf("Warning: failed to clean up target container on error: %v", deleteErr)
		}
	}()

	// Create installer for package management
	inst, err := installer.NewInstaller(targetBuilder, l.Config.PackageManager, l.Config.GPGCheck, l.Config.Proxy)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}
	defer inst.Cleanup()

	repositories := l.Config.Repositories

	if err := inst.InstallRepos(repositories); err != nil {
		return fmt.Errorf("failed to install repositories: %w", err)
	}

	// Install modules
	if err := inst.InstallModules(l.Config.Modules); err != nil {
		return fmt.Errorf("failed to install modules: %w", err)
	}

	// Install package groups
	if err := inst.InstallGroups(l.Config.PackageGroups); err != nil {
		return fmt.Errorf("failed to install package groups: %w", err)
	}

	// Install packages
	if err := inst.InstallPackages(l.Config.Packages); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}

	// Remove specified packages
	if err := inst.RemovePackages(l.Config.RemovePackages); err != nil {
		return fmt.Errorf("failed to remove packages: %w", err)
	}

	// Run commands
	for _, cmd := range l.Config.Commands {
		l.Logger.Printf("[INFO] Executing command: %s", cmd.Cmd)

		err := utils.RunCommandInBuilder(targetBuilder, []string{"sh", "-c", cmd.Cmd}, nil)
		if err != nil {
			return fmt.Errorf("failed to run command '%s': %w", cmd.Cmd, err)
		}
		l.Logger.Printf("[INFO] Command completed successfully: %s", cmd.Cmd)
	}

	// Copy files
	for _, copyFile := range l.Config.CopyFiles {
		src := copyFile.Src
		dest := copyFile.Dest

		if src == "" || dest == "" {
			continue
		}

		if _, err := os.Stat(src); os.IsNotExist(err) {
			return fmt.Errorf("source file does not exist: %s", src)
		}

		l.Logger.Printf("[INFO] Copying file: %s -> %s (owner: root:root)", src, dest)

		// Build builder to copy files
		if err := targetBuilder.Add(dest, false, buildah.AddAndCopyOptions{
			Chown: "root:root",
		}, src); err != nil {
			return fmt.Errorf("failed to copy file to container: %w", err)
		}
		l.Logger.Printf("[INFO] Successfully copied file: %s", src)

	}

	// Commit the changes to a new image
	l.Logger.Printf("[INFO] Committing container changes - Name: %s, Squash: %t", containerName, true)

	// Create the commit options
	commitOptions := buildah.CommitOptions{
		Squash:        true,
		OmitTimestamp: false,
		ReportWriter:  os.Stdout,
		// Add additional tags to the image
		AdditionalTags: []string{containerName},
	}

	// Use nil as ImageReference to let buildah generate a reference
	imageID, _, _, err := targetBuilder.Commit(ctx, nil, commitOptions)
	if err != nil {
		return fmt.Errorf("failed to commit container: %w", err)
	}

	l.Logger.Printf("[INFO] Image build completed successfully - ID: %s, Name: %s", imageID, containerName)

	// Publish the image if publishing options are specified
	if err := l.Publish(imageID, containerName); err != nil {
		return fmt.Errorf("failed to publish image: %w", err)
	}

	return nil
}
