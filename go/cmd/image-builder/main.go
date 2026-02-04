package main

import (
	"log"
	"os"

	"github.com/OpenCHAMI/image-builder/go/pkg/arguments"
	"github.com/OpenCHAMI/image-builder/go/pkg/config"
	"github.com/OpenCHAMI/image-builder/go/pkg/layer"
	"github.com/containers/buildah"
	"github.com/containers/storage/pkg/reexec"
	"github.com/containers/storage/pkg/unshare"
)

func needsRootlessSetup(layerType, parent string) bool {
	// Only need rootless setup if we're not running as root
	if os.Getuid() == 0 {
		return false
	}

	// Need rootless setup for layers that require mounting:
	// - ansible layers (need to mount for chroot operations)
	// - scratch parent (need to mount to build from scratch)
	return layerType == "ansible" || parent == "scratch"
}

func main() {
	// Initialize reexec for buildah/containers libraries
	if reexec.Init() {
		return
	}

	// Parse command-line arguments first to determine if we need rootless setup
	args, err := arguments.ParseCommandLine()
	if err != nil {
		log.Fatalf("Error parsing arguments: %v", err)
	}

	// Load configuration file to check layer type and parent
	log.Printf("DEBUG: Loading config file: %s", args.ConfigFile)
	cfg, err := config.LoadConfig(args.ConfigFile)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Merge command-line arguments into the loaded config
	cfg.MergeCommandLineArgs(args)

	// Only initialize rootless setup if needed
	if needsRootlessSetup(cfg.LayerType, cfg.Parent) {
		log.Printf("DEBUG: Initializing rootless setup for layer_type=%s, parent=%s", cfg.LayerType, cfg.Parent)
		// Initialize buildah reexec for rootless operation
		if buildah.InitReexec() {
			return
		}
		// Handle user namespace setup for rootless containers
		unshare.MaybeReexecUsingUserNamespace(false)
	}

	// Create layer builder with unified config
	layerBuilder, err := layer.NewLayer(cfg)
	if err != nil {
		log.Fatalf("Error creating layer builder: %v", err)
	}

	// Build the layer
	if err := layerBuilder.BuildLayer(); err != nil {
		log.Fatalf("Error building layer: %v", err)
	}

	log.Printf("Image build completed successfully")
}
