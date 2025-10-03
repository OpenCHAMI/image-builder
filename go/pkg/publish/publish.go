package publish

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/OpenCHAMI/image-builder/go/pkg/utils"
)

// PublishLocal publishes the image to local storage only
func PublishLocal(ctx context.Context, imageID, containerName string, imageConfig ImageConfig, logger *log.Logger) error {
	logger.Printf("Publishing image ID: %s (container: %s) to local storage", imageID, containerName)

	store, err := utils.GetStore()
	if err != nil {
		return fmt.Errorf("failed to get buildah store: %w", err)
	}

	localPub := NewLocalPublisher()
	if err := localPub.Publish(ctx, imageID, containerName, imageConfig, store); err != nil {
		return fmt.Errorf("local publishing failed: %w", err)
	}

	logger.Printf("Successfully published to local storage")
	return nil
}

// PublishToS3 publishes the image to S3 storage
func PublishToS3(ctx context.Context, imageID, containerName string, imageConfig ImageConfig, s3Config S3PublishConfig, logger *log.Logger) error {
	logger.Printf("Publishing image ID: %s (container: %s) to S3: %s", imageID, containerName, s3Config.S3Endpoint)

	store, err := utils.GetStore()
	if err != nil {
		return fmt.Errorf("failed to get buildah store: %w", err)
	}

	s3Pub, err := NewS3Publisher(s3Config)
	if err != nil {
		return fmt.Errorf("failed to create S3 publisher: %w", err)
	}

	if err := s3Pub.Publish(ctx, imageID, containerName, imageConfig, store); err != nil {
		return fmt.Errorf("S3 publishing failed: %w", err)
	}

	logger.Printf("Successfully published to S3")
	return nil
}

// PublishToRegistry publishes the image to a container registry
func PublishToRegistry(ctx context.Context, imageID, containerName string, imageConfig ImageConfig, registryConfig RegistryPublishConfig, logger *log.Logger) error {
	logger.Printf("Publishing image ID: %s (container: %s) to registry", imageID, containerName)

	store, err := utils.GetStore()
	if err != nil {
		return fmt.Errorf("failed to get buildah store: %w", err)
	}

	registryPub, err := NewRegistryPublisher(registryConfig)
	if err != nil {
		return fmt.Errorf("failed to create registry publisher: %w", err)
	}

	if err := registryPub.Publish(ctx, imageID, containerName, imageConfig, store); err != nil {
		return fmt.Errorf("registry publishing failed: %w", err)
	}

	logger.Printf("Successfully published to registry")
	return nil
}

// GenerateLabels creates standard labels for container images
func GenerateLabels(config ImageConfig, packages, packageGroups, repositories []string) map[string]string {
	labels := make(map[string]string)

	// Copy existing labels
	for k, v := range config.Labels {
		labels[k] = v
	}

	// Basic metadata
	labels["org.openchami.image.name"] = config.Name
	labels["org.openchami.image.type"] = config.LayerType
	labels["org.openchami.image.parent"] = config.Parent

	// Tags information
	if len(config.PublishTags) > 0 {
		labels["org.openchami.image.tags"] = joinStrings(config.PublishTags, ",")
	}

	// Build information
	labels["org.openchami.image.build-date"] = time.Now().Format(time.RFC3339)

	// Package information
	if len(packages) > 0 {
		labels["org.openchami.image.packages"] = joinStrings(packages, ",")
	}

	if len(packageGroups) > 0 {
		labels["org.openchami.image.package-groups"] = joinStrings(packageGroups, ",")
	}

	// Repository information
	if len(repositories) > 0 {
		labels["org.openchami.image.repositories"] = joinStrings(repositories, ",")
	}

	return labels
}

// Helper functions
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
