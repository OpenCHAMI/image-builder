package publish

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/containers/storage"
)

// LocalPublisher implements Publisher interface for local podman/buildah publishing
type LocalPublisher struct {
	logger *log.Logger
}

// NewLocalPublisher creates a new local publisher
func NewLocalPublisher() *LocalPublisher {
	return &LocalPublisher{
		logger: log.New(os.Stdout, "[LocalPublisher] ", log.LstdFlags),
	}
}

// validate checks if local configuration is valid
func (l *LocalPublisher) validate(imageConfig ImageConfig) error {
	// Local publishing doesn't need much validation
	// Just ensure we have basic config
	if imageConfig.Name == "" {
		return fmt.Errorf("image name is required")
	}
	return nil
}

// Publish publishes the image to local storage
func (l *LocalPublisher) Publish(ctx context.Context, imageID, containerName string, imageConfig ImageConfig, store storage.Store) error {
	// Validate configuration first
	if err := l.validate(imageConfig); err != nil {
		return fmt.Errorf("local configuration validation failed: %w", err)
	}

	l.logger.Printf("Publishing to local storage")

	return l.commitToLocal(ctx, imageID, imageConfig, store)
}

// commitToLocal tags the already committed image with the specified publish tags using explicit imageID
func (l *LocalPublisher) commitToLocal(ctx context.Context, imageID string, imageConfig ImageConfig, store storage.Store) error {
	tags := imageConfig.PublishTags
	if len(tags) == 0 {
		tags = []string{"latest"}
	}

	if len(tags) == 0 {
		l.logger.Printf("No publish tags specified, using image ID: %s", imageID)
		return nil
	}

	// Tag the image with each publish tag using buildah API
	for _, tag := range tags {
		l.logger.Printf("Tagging image with tag: %s", tag)

		// Create target image name for local storage
		targetImage := fmt.Sprintf("localhost/%s:%s", imageConfig.Name, tag)

		l.logger.Printf("Tagging image ID %s as %s", imageID, targetImage)

		// Get the existing image and add the new tag
		img, err := store.Image(imageID)
		if err != nil {
			return fmt.Errorf("failed to get image %s: %w", imageID, err)
		}

		// Add the new name to the existing names
		newNames := append(img.Names, targetImage)

		// Update the image with new names
		if err := store.SetNames(imageID, newNames); err != nil {
			return fmt.Errorf("failed to set names for image %s: %w", imageID, err)
		}

		l.logger.Printf("Successfully tagged image as %s", targetImage)
	}

	return nil
}
