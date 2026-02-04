package installer

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/OpenCHAMI/image-builder/go/pkg/config"
	"github.com/OpenCHAMI/image-builder/go/pkg/utils"
	"github.com/containers/buildah"
	"github.com/containers/storage"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	TARGET_MOUNT_DIR = "/mnt/target"
)

func NewInstaller(targetBuilder *buildah.Builder, pkgMan string, gpgCheck bool, proxy string) (Installer, error) {
	scratch := targetBuilder.FromImage == ""
	packageManager, err := NewPackageManager(pkgMan, scratch)
	if err != nil {
		return nil, err
	}

	if scratch {
		return NewScratchInstaller(targetBuilder, packageManager, gpgCheck, proxy)
	} else {
		return NewTargetInstaller(targetBuilder, packageManager, gpgCheck, proxy)
	}

}

type Installer interface {
	InstallRepos(repos []config.Repository) error
	InstallPackages(packages []string) error
	InstallGroups(groups []string) error
	InstallModules(modules map[string][]string) error
	RemovePackages(packages []string) error
	Cleanup() error
}

// Installer handles package installation and repository management
type TargetInstaller struct {
	targetBuilder *buildah.Builder
	pkgMan        PackageManager
	logger        *log.Logger
	tempDir       string
	gpgCheck      bool
	proxy         string
}

type ScratchInstaller struct {
	store storage.Store
	TargetInstaller
	helperBuilder    *buildah.Builder
	targetMountPoint string
}

// NewTargetInstaller creates a new installer for the specified package manager
func NewTargetInstaller(builder *buildah.Builder, pkgMan PackageManager, gpgCheck bool, proxy string) (*TargetInstaller, error) {
	logger := log.New(os.Stdout, "[INSTALLER]", log.LstdFlags)

	// Create temp directory for package manager logs, cache, etc.
	tmpDir, err := os.MkdirTemp("", "image-build-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	logger.Printf("Created temp directory: %s", tmpDir)

	// // Ensure /tmp exists in the mount
	// if err := utils.CreateDir(filepath.Join(mountPoint, "tmp")); err != nil {
	// 	return nil, fmt.Errorf("failed to create /tmp in container mount: %w", err)
	// }

	//logger.Printf("Using mount point: %s", mountPoint)

	logger.Printf("Temporary directory created at %s", tmpDir)

	return &TargetInstaller{
		targetBuilder: builder,
		pkgMan:        pkgMan,
		logger:        logger,
		tempDir:       tmpDir,
		gpgCheck:      gpgCheck,
		proxy:         proxy,
	}, nil
}

func (i *TargetInstaller) installPackagesWithBuilder(builder *buildah.Builder, packages []string, mounts []specs.Mount) error {
	if len(packages) == 0 {
		i.logger.Println("No packages specified to install")
		return nil
	}

	i.logger.Printf("Installing %d packages", len(packages))

	args, err := i.pkgMan.InstallPackagesCommand(packages, i.gpgCheck, i.proxy)
	if err != nil {
		return fmt.Errorf("failed to generate install command: %w", err)
	}

	i.logger.Printf("Running command in container: %v", args)
	err = utils.RunCommandInBuilder(builder, args, mounts)
	if err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}
	return nil
}

func (i *TargetInstaller) InstallPackages(packages []string) error {
	return i.installPackagesWithBuilder(i.targetBuilder, packages, nil)
}

// createTargetMounts creates the standard mount configuration for scratch installer operations
func (i *ScratchInstaller) createTargetMounts() []specs.Mount {
	return []specs.Mount{
		{
			Destination: TARGET_MOUNT_DIR,
			Type:        "bind",
			Source:      i.targetMountPoint,
			Options:     []string{"rbind", "rw"},
		},
	}
}

func (i *ScratchInstaller) InstallPackages(packages []string) error {
	return i.installPackagesWithBuilder(i.helperBuilder, packages, i.createTargetMounts())
}

// InstallGroups installs the specified package groups
func (i *TargetInstaller) installGroups(builder *buildah.Builder, groups []string, mounts []specs.Mount) error {
	if len(groups) == 0 {
		i.logger.Println("No package groups specified to install")
		return nil
	}

	i.logger.Printf("Installing %d package groups", len(groups))

	args, err := i.pkgMan.InstallGroupsCommand(groups, i.gpgCheck, i.proxy)
	if err != nil {
		return fmt.Errorf("failed to generate install command: %w", err)
	}

	i.logger.Printf("Running command in container: %v", args)
	err = utils.RunCommandInBuilder(builder, args, mounts)
	if err != nil {
		return fmt.Errorf("failed to install package groups: %w", err)
	}

	return nil
}

func (i *TargetInstaller) InstallGroups(groups []string) error {
	return i.installGroups(i.targetBuilder, groups, nil)
}

// InstallModules installs the specified modules
func (i *TargetInstaller) installModules(builder *buildah.Builder, modules map[string][]string, mounts []specs.Mount) error {
	if len(modules) == 0 {
		i.logger.Println("No modules specified to install")
		return nil
	}

	i.logger.Printf("Installing modules with %d commands", len(modules))

	for command, moduleList := range modules {
		if len(moduleList) == 0 {
			continue
		}

		i.logger.Printf("Running module command '%s' for modules: %v", command, moduleList)

		args, err := i.pkgMan.InstallModulesCommand(command, moduleList, i.gpgCheck, i.proxy)
		if err != nil {
			return fmt.Errorf("failed to generate module command: %w", err)
		}

		i.logger.Printf("Running command in container: %v", args)
		err = utils.RunCommandInBuilder(builder, args, mounts)
		if err != nil {
			return fmt.Errorf("failed to run module command '%s': %w", command, err)
		}
	}

	return nil
}

func (i *TargetInstaller) InstallModules(modules map[string][]string) error {
	return i.installModules(i.targetBuilder, modules, nil)
}

func (i *ScratchInstaller) InstallGroups(groups []string) error {
	return i.installGroups(i.helperBuilder, groups, i.createTargetMounts())
}

func (i *ScratchInstaller) InstallModules(modules map[string][]string) error {
	return i.installModules(i.helperBuilder, modules, i.createTargetMounts())
}

func (i *TargetInstaller) installRepos(builder *buildah.Builder, repos []config.Repository, mounts []specs.Mount) error {
	if len(repos) == 0 {
		i.logger.Printf("No repositories specified to install")
		return nil
	}
	for _, repo := range repos {
		i.logger.Printf("Installing repo %s: %s", repo.Alias, repo.URL)

		args, err := i.pkgMan.InstallRepoCommand(repo, i.proxy)
		if err != nil {
			return fmt.Errorf("failed to generate install repo command: %w", err)
		}

		i.logger.Printf("Running command in container: %v", args)
		err = utils.RunCommandInBuilder(builder, args, mounts)
		if err != nil {
			return fmt.Errorf("failed to install repo %s: %w", repo.Alias, err)
		}

		// Install GPG key if specified and gpgcheck is enabled
		if repo.GPG != "" && i.gpgCheck {
			err = i.installGPGKey(builder, repo.GPG, i.proxy, mounts)
			if err != nil {
				return fmt.Errorf("failed to install GPG key for repo %s: %w", repo.Alias, err)
			}
		}
	}

	return nil
}

func (i *TargetInstaller) InstallRepos(repos []config.Repository) error {

	return i.installRepos(i.targetBuilder, repos, nil)
}

func (i *ScratchInstaller) InstallRepos(repos []config.Repository) error {
	return i.installRepos(i.helperBuilder, repos, i.createTargetMounts())
}

func (i *TargetInstaller) installGPGKey(builder *buildah.Builder, gpgURL string, proxy string, mounts []specs.Mount) error {
	i.logger.Printf("Installing GPG key %s", gpgURL)

	args, err := i.pkgMan.InstallGpgKeyCommand(gpgURL)
	if err != nil {
		return fmt.Errorf("failed to generate GPG key install command: %w", err)
	}

	env := []string{}
	if proxy != "" {
		env = append(env, fmt.Sprintf("http_proxy=%s", proxy))
		env = append(env, fmt.Sprintf("https_proxy=%s", proxy))
	}

	// Run the RPM import command in helper container
	err = utils.RunCommandInBuilderWithEnv(builder, args, mounts, env)
	if err != nil {
		return fmt.Errorf("failed to import GPG key: %w", err)
	}

	i.logger.Printf("Successfully imported GPG key from %s", gpgURL)
	return nil
}

func (i *TargetInstaller) Cleanup() error {
	if i.tempDir != "" {
		i.logger.Printf("Cleaning up temporary directory %s", i.tempDir)
		err := os.RemoveAll(i.tempDir)
		if err != nil {
			return fmt.Errorf("failed to remove temp directory: %v", err)
		}
	}

	return nil
}

func (i *ScratchInstaller) Cleanup() error {
	if err := i.helperBuilder.Delete(); err != nil {
		return fmt.Errorf("failed to delete container: %v", err)
	}

	err := i.TargetInstaller.targetBuilder.Unmount()
	if err != nil {
		return fmt.Errorf("failed to unmount target container: %v", err)
	}

	return i.TargetInstaller.Cleanup()
}

func NewScratchInstaller(targetBuilder *buildah.Builder, pkgMan PackageManager, gpgCheck bool, proxy string) (*ScratchInstaller, error) {
	var helperImage string
	// TODO can probaly use type switch here
	switch pkgMan.Command() {
	case "dnf", "yum", "microdnf":
		helperImage = "docker.io/cjh1/scratch-image-helper"
	case "zypper":
		helperImage = "registry.opensuse.org/opensuse/leap:15"
	default:
		return nil, fmt.Errorf("unsupported package manager for helper container: %s", pkgMan)
	}

	store, err := utils.GetStore()
	if err != nil {
		return nil, fmt.Errorf("failed to get buildah store")
	}

	scratchHelperBuilder, err := utils.CreateBuilder(context.Background(), store, helperImage, "scratch-helper")
	if err != nil {
		return nil, fmt.Errorf("failed to create helper container: %w", err)
	}

	// Mount the target container
	mountPoint, err := targetBuilder.Mount("")
	if err != nil {
		return nil, fmt.Errorf("failed to mount container: %w", err)
	}

	log.Printf("Container mounted at: %s", mountPoint)

	targetInstaller, err := NewTargetInstaller(targetBuilder, pkgMan, gpgCheck, proxy)
	if err != nil {
		return nil, fmt.Errorf("failed to create target installer: %w", err)
	}

	return &ScratchInstaller{
		store:            store,
		TargetInstaller:  *targetInstaller,
		helperBuilder:    scratchHelperBuilder,
		targetMountPoint: mountPoint,
	}, nil
}

func (i *TargetInstaller) removePackages(builder *buildah.Builder, packages []string, mounts []specs.Mount) error {
	if len(packages) == 0 {
		i.logger.Println("No packages specified to remove")
		return nil
	}

	i.logger.Printf("Removing %d packages", len(packages))

	args, err := i.pkgMan.RemovePackagesCommand(packages)
	if err != nil {
		return fmt.Errorf("failed to generate remove command: %w", err)
	}

	i.logger.Printf("Running command in container: %v", args)
	err = utils.RunCommandInBuilder(builder, args, nil)
	if err != nil {
		return fmt.Errorf("failed to remove packages: %w", err)
	}
	return nil
}

func (i *TargetInstaller) RemovePackages(packages []string) error {
	return i.removePackages(i.targetBuilder, packages, nil)
}

func (i *ScratchInstaller) RemovePackages(packages []string) error {
	return i.removePackages(i.helperBuilder, packages, i.createTargetMounts())
}
