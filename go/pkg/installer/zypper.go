package installer

import (
	"fmt"
	"path/filepath"

	"github.com/OpenCHAMI/image-builder/go/pkg/config"
)

type Zypper struct {
}

// Command returns the Zypper command name
func (i *Zypper) Command() string {
	return "zypper"
}

type ZypperScratch struct {
	Zypper
}

// Command returns the Zypper command name
func (i *ZypperScratch) Command() string {
	return i.Zypper.Command()
}

func NewZypper(scratch bool) (PackageManager, error) {
	if scratch {
		return &ZypperScratch{}, nil
	} else {
		return &Zypper{}, nil
	}
}

func (i *Zypper) InstallRepoCommand(repo config.Repository, proxy string) ([]string, error) {
	args := []string{"zypper", "addrepo", "-f", "-p"}
	priority := repo.Priority
	if priority == 0 {
		priority = 99
	}
	args = append(args, fmt.Sprintf("%d", priority))
	args = append(args, repo.URL, repo.Alias)

	return args, nil
}

func (i *ZypperScratch) InstallRepoCommand(repo config.Repository, proxy string) ([]string, error) {
	repoDest := "etc/zypp/repos.d"

	args, err := i.Zypper.InstallRepoCommand(repo, proxy)
	if err != nil {
		return nil, err
	}

	args = append(args, "-D", repoDest, "addrepo")

	return args, nil
}

func (i *Zypper) InstallPackagesCommand(packages []string, gpgCheck bool, proxy string) ([]string, error) {
	args := []string{"zypper", "-n", "install", "--no-recommends"}
	if !gpgCheck {
		args = append(args, "--no-gpg-checks")
	}
	args = append(args, "-l")
	args = append(args, packages...)

	return args, nil
}

func (i *ZypperScratch) InstallPackagesCommand(packages []string, gpgCheck bool, proxy string) ([]string, error) {
	repoDest := "etc/zypp/repos.d"
	args, err := i.Zypper.InstallPackagesCommand(packages, gpgCheck, proxy)
	if err != nil {
		return nil, err
	}

	args = append(args, "-D")
	args = append(args, filepath.Join(TARGET_MOUNT_DIR, repoDest))
	args = append(args, "-C")
	args = append(args, filepath.Join(TARGET_MOUNT_DIR, "tmp"))
	args = append(args, "--installroot")
	args = append(args, TARGET_MOUNT_DIR)

	return args, nil
}

func (i *Zypper) InstallGroupsCommand(package_groups []string, gpgCheck bool, proxy string) ([]string, error) {
	return nil, fmt.Errorf("zypper does not support package groups")
}

func (i *Zypper) InstallModulesCommand(command string, modules []string, gpgCheck bool, proxy string) ([]string, error) {
	return nil, fmt.Errorf("zypper does not support modules")
}

func (i *ZypperScratch) InstallGroupsCommand(package_groups []string, gpgCheck bool, proxy string) ([]string, error) {
	return nil, fmt.Errorf("zypper does not support package groups")
}

func (i *ZypperScratch) InstallModulesCommand(command string, modules []string, gpgCheck bool, proxy string) ([]string, error) {
	return nil, fmt.Errorf("zypper does not support modules")
}

func (i *Zypper) RemovePackagesCommand(packages []string) ([]string, error) {
	args := []string{"zypper", "-n", "remove"}
	// Note: Keep --no-gpg-checks for remove operations as GPG checks don't apply to removal
	args = append(args, "--no-gpg-checks")
	args = append(args, "-l")
	args = append(args, packages...)

	return args, nil
}

func (i *ZypperScratch) RemovePackagesCommand(packages []string) ([]string, error) {
	args, err := i.Zypper.RemovePackagesCommand(packages)
	if err != nil {
		return nil, err
	}

	repoDest := "etc/zypp/repos.d"

	args = append(args, "-D")
	args = append(args, filepath.Join(TARGET_MOUNT_DIR, repoDest))
	args = append(args, "-C")
	args = append(args, filepath.Join(TARGET_MOUNT_DIR, "tmp"))

	args = append(args, "--installroot")
	args = append(args, TARGET_MOUNT_DIR)

	return args, nil
}

func (i *Zypper) InstallGpgKeyCommand(gpgURL string) ([]string, error) {
	args := []string{"rpm", "--import", gpgURL}
	return args, nil
}

func (i *ZypperScratch) InstallGpgKeyCommand(gpgURL string) ([]string, error) {
	args, err := i.Zypper.InstallGpgKeyCommand(gpgURL)
	if err != nil {
		return nil, err
	}
	args = append(args, "--root", TARGET_MOUNT_DIR)

	return args, nil
}
