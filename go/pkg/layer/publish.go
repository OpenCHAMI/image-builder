package layer

import (
	"context"
	"fmt"

	"github.com/OpenCHAMI/image-builder/go/pkg/publish"
)

// Publish handles publishing the built container image by delegating to the publish package
func (l *Layer) Publish(imageID, containerName string) error {
	ctx := context.Background()
	imageConfig := l.createImageConfig()

	// Publish to local storage if requested
	if l.Config.PublishLocal {
		if err := publish.PublishLocal(ctx, imageID, containerName, imageConfig, l.Logger); err != nil {
			return fmt.Errorf("local publishing failed: %w", err)
		}
	}

	// Publish to S3 if configured
	if l.Config.PublishS3 != "" {

		s3Config := l.createS3Config()
		if err := publish.PublishToS3(ctx, imageID, containerName, imageConfig, s3Config, l.Logger); err != nil {
			return fmt.Errorf("S3 publishing failed: %w", err)
		}
	}

	// Publish to registry if configured
	if l.Config.PublishRegistry != "" {
		registryConfig := l.createRegistryConfig()
		if err := publish.PublishToRegistry(ctx, imageID, containerName, imageConfig, registryConfig, l.Logger); err != nil {
			return fmt.Errorf("registry publishing failed: %w", err)
		}
	}

	return nil
}

// createImageConfig creates the image configuration with labels
func (l *Layer) createImageConfig() publish.ImageConfig {
	// Get publish tags, default to "latest" if none specified
	tags := l.Config.PublishTags
	if len(tags) == 0 {
		tags = []string{"latest"}
	}

	// Extract repository aliases
	var repositories []string
	for _, repo := range l.Config.Repositories {
		repositories = append(repositories, repo.Alias)
	}

	imageConfig := publish.ImageConfig{
		Name:        l.Config.Name,
		LayerType:   l.Config.LayerType,
		Parent:      l.Config.Parent,
		PublishTags: tags,
		Labels:      make(map[string]string),
	}

	// Generate standard labels
	imageConfig.Labels = publish.GenerateLabels(imageConfig, l.Config.Packages, l.Config.PackageGroups, repositories)

	return imageConfig
}

// createS3Config creates S3 config if enabled, returns nil if disabled
func (l *Layer) createS3Config() publish.S3PublishConfig {
	return publish.S3PublishConfig{
		S3Endpoint: l.Config.PublishS3,
		S3Bucket:   l.Config.S3Bucket,
		S3Prefix:   l.Config.S3Prefix,
	}
}

// createRegistryConfig creates registry config if enabled, returns nil if disabled
func (l *Layer) createRegistryConfig() publish.RegistryPublishConfig {
	return publish.RegistryPublishConfig{
		RegistryEndpoint: l.Config.PublishRegistry,
		RegistryOpts:     l.Config.RegistryOptsPush,
	}
}
