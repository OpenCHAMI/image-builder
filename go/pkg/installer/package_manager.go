package installer

import (
	"fmt"

	"github.com/OpenCHAMI/image-builder/go/pkg/config"
)

// PackageManager interface defines the contract for all package managers
type PackageManager interface {
	Command() string
	InstallRepoCommand(repo config.Repository, proxy string) ([]string, error)
	InstallPackagesCommand(packages []string, gpgCheck bool, proxy string) ([]string, error)
	InstallGroupsCommand(groups []string, gpgCheck bool, proxy string) ([]string, error)
	InstallModulesCommand(command string, modules []string, gpgCheck bool, proxy string) ([]string, error)
	InstallGpgKeyCommand(gpgURL string) ([]string, error)
	RemovePackagesCommand([]string) ([]string, error)
}

// NewPackageManager creates a new package manager instance based on the specified type
func NewPackageManager(pkgMan string, scratch bool) (PackageManager, error) {
	if pkgMan == "" {
		return nil, fmt.Errorf("package manager name cannot be empty")
	}

	switch pkgMan {
	case "dnf":
		return NewDnf(scratch)
	case "yum":
		return NewYum(scratch)
	case "zypper":
		return NewZypper(scratch)
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", pkgMan)
	}
}
