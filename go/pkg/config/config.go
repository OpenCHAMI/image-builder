package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RuntimeArgs represents runtime and CLI-specific configuration
type RuntimeArgs struct {
	LogLevel     string
	ConfigFile   string
	VerboseLevel int
	Debug        bool
}

// ImageArgs represents core image configuration options
type ImageArgs struct {
	Name           string
	Parent         string
	LayerType      string
	PackageManager string
}

// AnsibleArgs represents Ansible configuration options
type AnsibleArgs struct {
	AnsibleGroups    []string
	AnsiblePlaybook  []string
	AnsibleInv       string
	AnsibleVars      map[string]string
	AnsibleVerbosity int
}

// RegistryArgs represents container registry configuration options
type RegistryArgs struct {
	RegistryOptsPull []string
	RegistryOptsPush []string
}

// PackageManagerArgs represents package manager configuration options
type PackageManagerArgs struct {
	Proxy    string
	GPGCheck bool
}

// S3Args represents S3 storage configuration options
type S3Args struct {
	S3Prefix string
	S3Bucket string
}

// PublishingArgs represents publishing and distribution options
type PublishingArgs struct {
	PublishS3       string
	PublishRegistry string
	PublishLocal    bool
	PublishTags     []string
}

// SecurityArgs represents security scanning options
type SecurityArgs struct {
	ScapBenchmark bool
	OvalEval      bool
	InstallScap   bool
}

// CLIArgs represents command-line arguments that can override YAML configuration
type CLIArgs struct {
	RuntimeArgs
	ImageArgs
	AnsibleArgs
	RegistryArgs
	PackageManagerArgs
	S3Args
	PublishingArgs
	SecurityArgs
}

// Options represents the legacy options section in YAML files
type Options struct {
	Name           string `yaml:"name"`
	Parent         string `yaml:"parent"`
	LayerType      string `yaml:"layer_type"`
	PackageManager string `yaml:"pkg_manager"`

	// Publishing options
	PublishLocal    bool   `yaml:"publish_local"`
	PublishS3       string `yaml:"publish_s3"`
	PublishRegistry string `yaml:"publish_registry"`
	S3Bucket        string `yaml:"s3_bucket"`
	S3Prefix        string `yaml:"s3_prefix"`
	PublishTags     string `yaml:"publish_tags"`

	// Repository options
	Proxy    string `yaml:"proxy"`
	GPGCheck bool   `yaml:"gpgcheck"`

	// Ansible options
	Groups    []string          `yaml:"groups"`
	Playbooks string            `yaml:"playbooks"`
	Inventory string            `yaml:"inventory"`
	Vars      map[string]string `yaml:"vars"`

	// Security scanning options
	ScapBenchmark bool `yaml:"scap_benchmark"`
	OvalEval      bool `yaml:"oval_eval"`
	InstallScap   bool `yaml:"install_scap"`
}

// Config represents the unified configuration combining YAML file data and command-line arguments
type Config struct {
	// Runtime/CLI options
	LogLevel     string
	ConfigFile   string
	VerboseLevel int
	Debug        bool

	// Core image configuration (from YAML, can be overridden by CLI)
	Name           string `yaml:"name"`
	Parent         string `yaml:"parent"`
	LayerType      string `yaml:"layer_type"`
	PackageManager string `yaml:"package_manager"`

	// Image build configuration (from YAML)
	Options        Options             `yaml:"options"`
	Modules        map[string][]string `yaml:"modules"`
	Packages       []string            `yaml:"packages"`
	PackageGroups  []string            `yaml:"package_groups"`
	RemovePackages []string            `yaml:"remove_packages"`
	Commands       []Command           `yaml:"cmds"`
	CopyFiles      []CopyFile          `yaml:"copyfiles"`
	Repositories   []Repository        `yaml:"repos"`

	// Ansible configuration (from CLI/YAML)
	AnsibleGroups    []string          `yaml:"groups"`
	AnsiblePlaybook  []string          `yaml:"playbooks"`
	AnsibleInv       string            `yaml:"inventory"`
	AnsibleVars      map[string]string `yaml:"vars"`
	AnsibleVerbosity int               `yaml:"ansible_verbosity"`

	// Registry configuration (from CLI)
	RegistryOptsPull []string
	RegistryOptsPush []string

	// Publishing options (from CLI/YAML)
	Proxy           string   `yaml:"proxy"`
	PublishS3       string   `yaml:"publish_s3"`
	PublishRegistry string   `yaml:"publish_registry"`
	PublishLocal    bool     `yaml:"publish_local"`
	PublishTags     []string `yaml:"publish_tags"`
	S3Prefix        string   `yaml:"s3_prefix"`
	S3Bucket        string   `yaml:"s3_bucket"`

	// Repository options (from CLI/YAML)
	GPGCheck bool `yaml:"gpgcheck"`

	// Security scanning options (from CLI/YAML)
	ScapBenchmark bool `yaml:"scap_benchmark"`
	OvalEval      bool `yaml:"oval_eval"`
	InstallScap   bool `yaml:"install_scap"`
}

// Command represents a command to run during image build
type Command struct {
	Cmd string `yaml:"cmd"`
}

// CopyFile represents a file copy operation
type CopyFile struct {
	Src  string `yaml:"src"`
	Dest string `yaml:"dest"`
}

// Repository represents a package repository configuration
type Repository struct {
	Alias    string `yaml:"alias"`
	URL      string `yaml:"url"`
	Priority int    `yaml:"priority,omitempty"`
	GPG      string `yaml:"gpg,omitempty"`
}

// LoadConfig loads and parses a YAML configuration file into a Config struct
func LoadConfig(yamlFile string) (*Config, error) {
	if !filepath.IsAbs(yamlFile) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
		yamlFile = filepath.Join(cwd, yamlFile)
	}

	if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", yamlFile)
	}

	data, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Extract values from options section to top-level fields (if top-level fields are empty)
	if config.Name == "" && config.Options.Name != "" {
		config.Name = config.Options.Name
	}
	if config.Parent == "" && config.Options.Parent != "" {
		config.Parent = config.Options.Parent
	}
	if config.LayerType == "" && config.Options.LayerType != "" {
		config.LayerType = config.Options.LayerType
	}
	if config.PackageManager == "" && config.Options.PackageManager != "" {
		config.PackageManager = config.Options.PackageManager
	}

	// Publishing options from options section
	if config.PublishS3 == "" && config.Options.PublishS3 != "" {
		config.PublishS3 = config.Options.PublishS3
	}
	if config.PublishRegistry == "" && config.Options.PublishRegistry != "" {
		config.PublishRegistry = config.Options.PublishRegistry
	}
	if config.S3Bucket == "" && config.Options.S3Bucket != "" {
		config.S3Bucket = config.Options.S3Bucket
	}
	if config.S3Prefix == "" && config.Options.S3Prefix != "" {
		config.S3Prefix = config.Options.S3Prefix
	}
	if len(config.PublishTags) == 0 && config.Options.PublishTags != "" {
		for _, tag := range strings.Split(config.Options.PublishTags, ",") {
			config.PublishTags = append(config.PublishTags, strings.TrimSpace(tag))
		}
	}
	// Always use options value for boolean fields (they have proper defaults)
	config.PublishLocal = config.Options.PublishLocal

	// Repository options from options section
	if config.Proxy == "" && config.Options.Proxy != "" {
		config.Proxy = config.Options.Proxy
	}
	config.GPGCheck = config.Options.GPGCheck

	// Ansible options from options section - force extraction even if already set
	// This handles the reexec case where values might get lost
	if len(config.Options.Groups) > 0 {
		config.AnsibleGroups = make([]string, len(config.Options.Groups))
		copy(config.AnsibleGroups, config.Options.Groups)
	}
	if config.Options.Playbooks != "" {
		config.AnsiblePlaybook = []string{config.Options.Playbooks}
	}
	if config.Options.Inventory != "" {
		config.AnsibleInv = config.Options.Inventory
	}
	if len(config.Options.Vars) > 0 {
		config.AnsibleVars = make(map[string]string)
		for k, v := range config.Options.Vars {
			config.AnsibleVars[k] = v
		}
	}

	// Security scanning options from options section
	config.ScapBenchmark = config.Options.ScapBenchmark
	config.OvalEval = config.Options.OvalEval
	config.InstallScap = config.Options.InstallScap
	if config.Modules == nil {
		config.Modules = make(map[string][]string)
	}
	if config.Packages == nil {
		config.Packages = []string{}
	}
	if config.PackageGroups == nil {
		config.PackageGroups = []string{}
	}
	if config.RemovePackages == nil {
		config.RemovePackages = []string{}
	}
	if config.Commands == nil {
		config.Commands = []Command{}
	}
	if config.CopyFiles == nil {
		config.CopyFiles = []CopyFile{}
	}
	if config.Repositories == nil {
		config.Repositories = []Repository{}
	}
	if config.AnsibleVars == nil {
		config.AnsibleVars = make(map[string]string)
	}

	return &config, nil
}

// MergeCommandLineArgs merges command-line arguments into the config,
// with CLI args taking precedence over YAML values
func (c *Config) MergeCommandLineArgs(cliArgs *CLIArgs) {
	// Runtime options (always from CLI)
	c.LogLevel = cliArgs.RuntimeArgs.LogLevel
	c.ConfigFile = cliArgs.RuntimeArgs.ConfigFile
	c.VerboseLevel = cliArgs.RuntimeArgs.VerboseLevel
	c.Debug = cliArgs.RuntimeArgs.Debug

	// Core fields - CLI overrides YAML if provided
	if cliArgs.ImageArgs.Name != "" && cliArgs.ImageArgs.Name != "image" { // "image" is the default
		c.Name = cliArgs.ImageArgs.Name
	}
	if cliArgs.ImageArgs.Parent != "" {
		c.Parent = cliArgs.ImageArgs.Parent
	}
	if cliArgs.ImageArgs.LayerType != "" {
		c.LayerType = cliArgs.ImageArgs.LayerType
	}
	if cliArgs.ImageArgs.PackageManager != "" {
		c.PackageManager = cliArgs.ImageArgs.PackageManager
	}

	// Ansible options - CLI overrides YAML if provided
	if len(cliArgs.AnsibleArgs.AnsibleGroups) > 0 {
		c.AnsibleGroups = cliArgs.AnsibleArgs.AnsibleGroups
	}
	if len(cliArgs.AnsibleArgs.AnsiblePlaybook) > 0 {
		c.AnsiblePlaybook = cliArgs.AnsibleArgs.AnsiblePlaybook
	}
	if cliArgs.AnsibleArgs.AnsibleInv != "" {
		c.AnsibleInv = cliArgs.AnsibleArgs.AnsibleInv
	}
	if len(cliArgs.AnsibleArgs.AnsibleVars) > 0 {
		c.AnsibleVars = cliArgs.AnsibleArgs.AnsibleVars
	}
	if cliArgs.AnsibleArgs.AnsibleVerbosity > 0 {
		c.AnsibleVerbosity = cliArgs.AnsibleArgs.AnsibleVerbosity
	}

	// Registry options (always from CLI)
	c.RegistryOptsPull = cliArgs.RegistryArgs.RegistryOptsPull
	c.RegistryOptsPush = cliArgs.RegistryArgs.RegistryOptsPush

	// Package manager options - CLI overrides YAML if provided
	if cliArgs.PackageManagerArgs.Proxy != "" {
		c.Proxy = cliArgs.PackageManagerArgs.Proxy
	}
	c.GPGCheck = cliArgs.PackageManagerArgs.GPGCheck

	// S3 options - CLI overrides YAML if provided
	if cliArgs.S3Args.S3Prefix != "" {
		c.S3Prefix = cliArgs.S3Args.S3Prefix
	}
	if cliArgs.S3Args.S3Bucket != "" {
		c.S3Bucket = cliArgs.S3Args.S3Bucket
	}

	// Publishing options - CLI overrides YAML if provided
	if cliArgs.PublishingArgs.PublishS3 != "" {
		c.PublishS3 = cliArgs.PublishingArgs.PublishS3
	}
	if cliArgs.PublishingArgs.PublishRegistry != "" {
		c.PublishRegistry = cliArgs.PublishingArgs.PublishRegistry
	}
	// PublishLocal defaults to false, so we always use CLI value
	c.PublishLocal = cliArgs.PublishingArgs.PublishLocal
	if len(cliArgs.PublishingArgs.PublishTags) > 0 {
		c.PublishTags = cliArgs.PublishingArgs.PublishTags
	}

	// Security scanning options - CLI overrides YAML if provided
	// These default to false, so we always use CLI values
	c.ScapBenchmark = cliArgs.SecurityArgs.ScapBenchmark
	c.OvalEval = cliArgs.SecurityArgs.OvalEval
	c.InstallScap = cliArgs.SecurityArgs.InstallScap
}
