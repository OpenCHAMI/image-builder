package publish

import (
	"context"

	"github.com/containers/storage"
)

// ImageConfig contains image metadata and configuration
type ImageConfig struct {
	Name        string
	LayerType   string
	Parent      string
	PublishTags []string
	Labels      map[string]string
}

// S3PublishConfig contains configuration specific to S3 publishing
type S3PublishConfig struct {
	S3Endpoint string
	S3Bucket   string
	S3Prefix   string
}

// RegistryPublishConfig contains configuration specific to registry publishing
type RegistryPublishConfig struct {
	RegistryEndpoint string
	RegistryOpts     []string
}

// Publisher defines the interface for publishing container images
type Publisher interface {
	// Publish publishes the container image
	Publish(ctx context.Context, imageID, containerName string, imageConfig ImageConfig, store storage.Store) error
}
