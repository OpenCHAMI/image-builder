package installer

import (
	"path/filepath"

	"github.com/OpenCHAMI/image-builder/go/pkg/config"
)

type Dnf struct {
}

func (i *Dnf) InstallModulesCommand(command string, modules []string, gpgCheck bool, proxy string) ([]string, error) {
	args := []string{"dnf", "-y"}
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

// Command returns the DNF command name
func (i *Dnf) Command() string {
	return "dnf"
}

type DnfScratch struct {
	Dnf
}

// Command returns the DNF command name
func (i *DnfScratch) Command() string {
	return i.Dnf.Command()
}

func NewDnf(scratch bool) (PackageManager, error) {
	if scratch {
		return &DnfScratch{}, nil
	} else {
		return &Dnf{}, nil
	}
}

func (i *Dnf) InstallRepoCommand(repo config.Repository, proxy string) ([]string, error) {
	args := []string{"dnf", "-y"}
	if proxy != "" {
		args = append(args, "--setopt=proxy="+proxy)
	}
	args = append(args, "config-manager", "--save", "--add-repo")
	args = append(args, repo.URL)

	return args, nil
}

func (i *DnfScratch) InstallRepoCommand(repo config.Repository, proxy string) ([]string, error) {
	repoDest := "etc/yum.repos.d"

	args, err := i.Dnf.InstallRepoCommand(repo, proxy)
	if err != nil {
		return nil, err
	}
	args = append(args, "--setopt=reposdir="+filepath.Join(TARGET_MOUNT_DIR, repoDest))

	return args, nil
}

func (i *Dnf) InstallPackagesCommand(packages []string, gpgCheck bool, proxy string) ([]string, error) {
	args := []string{"dnf", "-y"}
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

func (i *DnfScratch) InstallPackagesCommand(packages []string, gpgCheck bool, proxy string) ([]string, error) {
	repoDest := "etc/yum.repos.d"
	args, err := i.Dnf.InstallPackagesCommand(packages, gpgCheck, proxy)
	if err != nil {
		return nil, err
	}

	args = append(args, "--setopt=reposdir="+filepath.Join(TARGET_MOUNT_DIR, repoDest))
	args = append(args, "--installroot="+TARGET_MOUNT_DIR)
	// Force RPM to use sqlite backend instead of bdb_ro to avoid database permission issues
	//args = append(args, "--setopt=_db_backend=sqlite")

	return args, nil
}

func (i *Dnf) InstallGroupsCommand(package_groups []string, gpgCheck bool, proxy string) ([]string, error) {
	args := []string{"dnf", "-y"}
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

func (i *DnfScratch) InstallGroupsCommand(package_groups []string, gpgCheck bool, proxy string) ([]string, error) {
	repoDest := "etc/yum.repos.d"
	args, err := i.Dnf.InstallGroupsCommand(package_groups, gpgCheck, proxy)
	if err != nil {
		return nil, err
	}

	args = append(args, "--installroot="+TARGET_MOUNT_DIR)
	args = append(args, "--setopt=reposdir="+filepath.Join(TARGET_MOUNT_DIR, repoDest))
	// Force RPM to use sqlite backend instead of bdb_ro to avoid database permission issues
	args = append(args, "--setopt=_db_backend=sqlite")

	return args, nil
}

func (i *Dnf) RemovePackagesCommand(packages []string) ([]string, error) {
	args := []string{"dnf", "-y", "remove"}
	args = append(args, packages...)

	// TODO gpg check option

	return args, nil
}

func (i *DnfScratch) InstallModulesCommand(command string, modules []string, gpgCheck bool, proxy string) ([]string, error) {
	repoDest := "etc/yum.repos.d"
	args, err := i.Dnf.InstallModulesCommand(command, modules, gpgCheck, proxy)
	if err != nil {
		return nil, err
	}

	args = append(args, "--installroot="+TARGET_MOUNT_DIR)
	args = append(args, "--setopt=reposdir="+filepath.Join(TARGET_MOUNT_DIR, repoDest))
	// Force RPM to use sqlite backend instead of bdb_ro to avoid database permission issues
	args = append(args, "--setopt=_db_backend=sqlite")

	return args, nil
}

func (i *DnfScratch) RemovePackagesCommand(packages []string) ([]string, error) {
	repoDest := "etc/yum.repos.d"
	args, err := i.Dnf.RemovePackagesCommand(packages)
	if err != nil {
		return nil, err
	}
	args = append(args, "--setopt=reposdir="+filepath.Join(TARGET_MOUNT_DIR, repoDest))
	args = append(args, "--installroot="+TARGET_MOUNT_DIR)
	// Force RPM to use sqlite backend instead of bdb_ro to avoid database permission issues
	args = append(args, "--setopt=_db_backend=sqlite")

	return args, nil
}

func (i *Dnf) InstallGpgKeyCommand(gpgURL string) ([]string, error) {
	args := []string{"rpm", "--import", gpgURL}
	return args, nil
}

func (i *DnfScratch) InstallGpgKeyCommand(gpgURL string) ([]string, error) {
	args, err := i.Dnf.InstallGpgKeyCommand(gpgURL)
	if err != nil {
		return nil, err
	}
	args = append(args, "--root", TARGET_MOUNT_DIR)

	return args, nil
}
