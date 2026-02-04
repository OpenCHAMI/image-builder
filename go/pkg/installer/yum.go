package installer

import (
	"fmt"
	"path/filepath"

	"github.com/OpenCHAMI/image-builder/go/pkg/config"
)

type Yum struct {
}

// Command returns the Yum command name
func (i *Yum) Command() string {
	return "yum"
}

type YumScratch struct {
	Yum
}

// Command returns the Yum command name
func (i *YumScratch) Command() string {
	return i.Yum.Command()
}

func NewYum(scratch bool) (PackageManager, error) {
	if scratch {
		return &YumScratch{}, nil
	} else {
		return &Yum{}, nil
	}
}

func (i *Yum) InstallRepoCommand(repo config.Repository, proxy string) ([]string, error) {
	args := []string{"yum", "-y"}
	if proxy != "" {
		args = append(args, "--setopt=proxy="+proxy)
	}
	args = append(args, "config-manager", "--save", "--add-repo")
	args = append(args, repo.URL)

	return args, nil
}

func (i *YumScratch) InstallRepoCommand(repo config.Repository, proxy string) ([]string, error) {
	repoDest := "etc/yum.repos.d"

	args, err := i.Yum.InstallRepoCommand(repo, proxy)
	if err != nil {
		return nil, err
	}
	args = append(args, "--setopt=reposdir="+filepath.Join(TARGET_MOUNT_DIR, repoDest))

	return args, nil
}

func (i *Yum) InstallPackagesCommand(packages []string, gpgCheck bool, proxy string) ([]string, error) {
	args := []string{"yum", "-y"}
	if proxy != "" {
		args = append(args, "--setopt=proxy="+proxy)
	}
	args = append(args, "install")
	if !gpgCheck {
		args = append(args, "--nogpgcheck")
	}
	args = append(args, packages...)

	return args, nil
}

func (i *YumScratch) InstallPackagesCommand(packages []string, gpgCheck bool, proxy string) ([]string, error) {
	args, err := i.Yum.InstallPackagesCommand(packages, gpgCheck, proxy)
	if err != nil {
		return nil, err
	}

	repoDest := "etc/yum.repos.d"
	args = append(args, "--installroot="+TARGET_MOUNT_DIR)
	args = append(args, "--setopt=reposdir="+filepath.Join(TARGET_MOUNT_DIR, repoDest))

	return args, nil
}

func (i *Yum) InstallGroupsCommand(package_groups []string, gpgCheck bool, proxy string) ([]string, error) {
	args := []string{"yum", "-y"}
	if proxy != "" {
		args = append(args, "--setopt=proxy="+proxy)
	}
	args = append(args, "groupinstall")
	if !gpgCheck {
		args = append(args, "--nogpgcheck")
	}
	args = append(args, package_groups...)

	return args, nil
}

func (i *YumScratch) InstallGroupsCommand(package_groups []string, gpgCheck bool, proxy string) ([]string, error) {
	repoDest := "etc/yum.repos.d"
	args, err := i.Yum.InstallGroupsCommand(package_groups, gpgCheck, proxy)
	if err != nil {
		return nil, err
	}

	args = append(args, "--installroot="+TARGET_MOUNT_DIR)
	args = append(args, "--setopt=reposdir="+filepath.Join(TARGET_MOUNT_DIR, repoDest))

	return args, nil
}

func (i *Yum) InstallModulesCommand(command string, modules []string, gpgCheck bool, proxy string) ([]string, error) {
	// Yum typically doesn't support modules - this is primarily a DNF feature
	// but some newer versions of Yum may support it
	args := []string{"yum", "-y"}
	if proxy != "" {
		args = append(args, "--setopt=proxy="+proxy)
	}
	args = append(args, "module", command)
	if !gpgCheck {
		args = append(args, "--nogpgcheck")
	}
	args = append(args, modules...)

	return args, nil
}

func (i *YumScratch) InstallModulesCommand(command string, modules []string, gpgCheck bool, proxy string) ([]string, error) {
	repoDest := "etc/yum.repos.d"
	args, err := i.Yum.InstallModulesCommand(command, modules, gpgCheck, proxy)
	if err != nil {
		return nil, err
	}

	args = append(args, "--installroot="+TARGET_MOUNT_DIR)
	args = append(args, "--setopt=reposdir="+filepath.Join(TARGET_MOUNT_DIR, repoDest))

	return args, nil
}

func (i *Yum) RemovePackagesCommand(packages []string) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func (i *YumScratch) RemovePackagesCommand(packages []string) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func (i *Yum) InstallGpgKeyCommand(gpgURL string) ([]string, error) {
	return []string{"rpm", "--import", gpgURL}, nil
}

func (i *YumScratch) InstallGpgKeyCommand(gpgURL string) ([]string, error) {
	args, err := i.Yum.InstallGpgKeyCommand(gpgURL)
	if err != nil {
		return nil, err
	}
	args = append(args, "--root", TARGET_MOUNT_DIR)

	return args, nil
}
