package arguments

import (
	"flag"
	"fmt"
	"strings"

	"github.com/OpenCHAMI/image-builder/go/pkg/config"
)

// ParseCommandLine parses command line arguments
func ParseCommandLine() (*config.CLIArgs, error) {
	var args config.CLIArgs

	// Define command line flags
	logLevel := flag.String("log-level", "info", "Logging level")
	config := flag.String("config", "config.yaml", "Configuration file")
	layerType := flag.String("layer-type", "", "Layer type (base, ansible)")
	pkgMan := flag.String("pkg-manager", "", "Package manager (dnf, zypper)")
	groupList := flag.String("groups", "", "Ansible group list (comma separated)")
	playbooks := flag.String("pb", "", "Ansible playbooks (comma separated)")
	inventory := flag.String("inventory", "", "Ansible inventory")
	vars := flag.String("vars", "", "Ansible variables (key=value,key2=value2)")
	ansibleVerbosity := flag.Int("ansible-verbosity", 0, "Ansible verbosity (0-4)")
	name := flag.String("name", "image", "Image name")
	parent := flag.String("parent", "", "Parent image")
	registryOptsPull := flag.String("registry-opts-pull", "", "Registry options for pull (comma separated)")
	registryOptsPush := flag.String("registry-opts-push", "", "Registry options for push (comma separated)")

	// Publishing options
	proxy := flag.String("proxy", "", "HTTP proxy for network operations")
	publishS3 := flag.String("publish-s3", "", "S3 publishing destination")
	publishRegistry := flag.String("publish-registry", "", "Registry publishing destination")
	publishLocal := flag.Bool("publish-local", false, "Keep image in local storage only, don't push to registry")
	publishTags := flag.String("publish-tags", "", "Tags for published images (comma separated)")
	s3Prefix := flag.String("s3-prefix", "", "S3 key prefix")
	s3Bucket := flag.String("s3-bucket", "", "S3 bucket name")

	// Repository options
	gpgCheck := flag.Bool("gpgcheck", true, "Enable GPG signature checking")

	// Security scanning options
	scapBenchmark := flag.Bool("scap-benchmark", false, "Enable SCAP security benchmarking")
	ovalEval := flag.Bool("oval-eval", false, "Enable OVAL security evaluation")
	installScap := flag.Bool("install-scap", false, "Install SCAP tools")

	flag.Parse()

	// Handle positional arguments - if a config file is passed as first positional arg, use it
	if flag.NArg() > 0 {
		*config = flag.Arg(0)
	}

	// Process arguments
	args.LogLevel = *logLevel
	args.ConfigFile = *config
	args.LayerType = *layerType
	args.PackageManager = *pkgMan
	args.Name = *name
	args.Parent = *parent
	args.AnsibleVerbosity = *ansibleVerbosity

	// Process lists
	if *groupList != "" {
		args.AnsibleGroups = strings.Split(*groupList, ",")
	}

	if *playbooks != "" {
		args.AnsiblePlaybook = strings.Split(*playbooks, ",")
	}

	if *registryOptsPull != "" {
		args.RegistryOptsPull = strings.Split(*registryOptsPull, ",")
	}

	if *registryOptsPush != "" {
		args.RegistryOptsPush = strings.Split(*registryOptsPush, ",")
	}

	args.AnsibleInv = *inventory

	// Process vars
	args.AnsibleVars = make(map[string]string)
	if *vars != "" {
		for _, pair := range strings.Split(*vars, ",") {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 {
				args.AnsibleVars[kv[0]] = kv[1]
			}
		}
	}

	// Process publishing options
	args.Proxy = *proxy
	args.PublishS3 = *publishS3
	args.PublishRegistry = *publishRegistry
	args.PublishLocal = *publishLocal
	args.S3Prefix = *s3Prefix
	args.S3Bucket = *s3Bucket

	if *publishTags != "" {
		args.PublishTags = strings.Split(*publishTags, ",")
	}

	// Process repository options
	args.GPGCheck = *gpgCheck

	// Process security scanning options
	args.ScapBenchmark = *scapBenchmark
	args.OvalEval = *ovalEval
	args.InstallScap = *installScap

	return &args, validateArgs(&args)
}

// validateArgs performs basic validation on the provided arguments
func validateArgs(args *config.CLIArgs) error {
	// Check for unimplemented SCAP security scanning options
	if args.ScapBenchmark {
		return fmt.Errorf("--scap-benchmark is not currently implemented")
	}
	if args.OvalEval {
		return fmt.Errorf("--oval-eval is not currently implemented")
	}
	if args.InstallScap {
		return fmt.Errorf("--install-scap is not currently implemented")
	}

	// Add other validation logic as needed
	return nil
}
