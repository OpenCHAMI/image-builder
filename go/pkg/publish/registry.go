package publish

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
)

// RegistryPublisher implements Publisher interface for container registry publishing
type RegistryPublisher struct {
	logger *log.Logger
	config RegistryPublishConfig
}

// NewRegistryPublisher creates a new registry publisher
func NewRegistryPublisher(config RegistryPublishConfig) (*RegistryPublisher, error) {
	if config.RegistryEndpoint == "" {
		return nil, fmt.Errorf("RegistryEndpoint must be specified")
	}

	return &RegistryPublisher{
		logger: log.New(os.Stdout, "[RegistryPublisher] ", log.LstdFlags),
		config: config,
	}, nil
}

// Publish publishes the image to a container registry (implements Publisher interface)
func (r *RegistryPublisher) Publish(ctx context.Context, imageID, containerName string, imageConfig ImageConfig, store storage.Store) error {
	r.logger.Printf("Starting registry publishing for image %s", imageID)

	tags := imageConfig.PublishTags
	if len(tags) == 0 {
		tags = []string{"latest"}
	}

	// Create system context with registry options
	systemCtx := &types.SystemContext{}
	r.setupSystemContext(systemCtx, r.config.RegistryOpts)

	// Publish to registry with all tags
	for _, tag := range tags {
		if err := r.pushToRegistry(ctx, imageID, imageConfig, tag, store, systemCtx); err != nil {
			return fmt.Errorf("failed to push tag %s: %w", tag, err)
		}
	}

	return nil
}

// getRegistryEndpoint determines the registry endpoint to use
func (r *RegistryPublisher) getRegistryEndpoint() string {
	return r.config.RegistryEndpoint
}

// pushToRegistry pushes a single tagged image to the registry
func (r *RegistryPublisher) pushToRegistry(ctx context.Context, imageID string, imageConfig ImageConfig, tag string, store storage.Store, systemCtx *types.SystemContext) error {
	registryEndpoint := r.getRegistryEndpoint()
	registryImage := fmt.Sprintf("%s/%s:%s", registryEndpoint, imageConfig.Name, tag)

	r.logger.Printf("Pushing image %s as %s", imageID, registryImage)

	// Parse the destination reference
	destRef, err := alltransports.ParseImageName("docker://" + registryImage)
	if err != nil {
		return fmt.Errorf("failed to parse registry image name %s: %w", registryImage, err)
	}

	// Setup push options
	pushOptions := buildah.PushOptions{
		Store:         store,
		SystemContext: systemCtx,
		ReportWriter:  r.logger.Writer(),
	}

	// Push the image using the explicit imageID
	_, digest, err := buildah.Push(ctx, imageID, destRef, pushOptions)
	if err != nil {
		return fmt.Errorf("failed to push %s: %w", registryImage, err)
	}

	r.logger.Printf("Successfully pushed image: %s (digest: %s)", registryImage, digest)
	return nil
}

// setupSystemContext configures the system context based on registry options
func (r *RegistryPublisher) setupSystemContext(systemCtx *types.SystemContext, registryOpts []string) {
	// Handle registry options
	for _, opt := range registryOpts {
		if strings.Contains(opt, "--tls-verify=false") {
			systemCtx.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
		}
	}
}
