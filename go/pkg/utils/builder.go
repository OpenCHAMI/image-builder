package utils

import (
	"context"
	"fmt"

	"github.com/containers/buildah"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/storage"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// GetStore returns a buildah store using default configuration with rootless-friendly settings
func GetStore() (storage.Store, error) {
	// Get buildah store with default options
	storeOptions, err := storage.DefaultStoreOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to get default store options: %w", err)
	}

	store, err := storage.GetStore(storeOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get buildah store: %w", err)
	}

	return store, nil
}

func CreateBuilder(ctx context.Context, store storage.Store, image string, name string) (*buildah.Builder, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if store == nil {
		return nil, fmt.Errorf("store cannot be nil")
	}
	if image == "" {
		return nil, fmt.Errorf("image name cannot be empty")
	}
	if name == "" {
		return nil, fmt.Errorf("container name cannot be empty")
	}

	// TODO figure out capabilities needed for non-root users
	defaultCaps := []string{
		"CAP_CHOWN",
		"CAP_DAC_OVERRIDE",
		"CAP_FSETID",
		"CAP_FOWNER",
		"CAP_MKNOD",
		"CAP_NET_RAW",
		"CAP_SETGID",
		"CAP_SETUID",
		"CAP_SETFCAP",
		"CAP_SETPCAP",
		"CAP_NET_BIND_SERVICE",
		"CAP_SYS_CHROOT",
		"CAP_KILL",
		"CAP_AUDIT_WRITE",
	}

	helperBuilder, err := buildah.NewBuilder(ctx, store, buildah.BuilderOptions{
		FromImage:    image,
		Capabilities: defaultCaps,
		PullPolicy:   buildah.PullIfMissing,
		Container:    name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create builder: %w", err)
	}

	return helperBuilder, nil

}

func runCommandInBuilder(builder *buildah.Builder, cmd []string, mounts []specs.Mount, env []string) error {
	if builder == nil {
		return fmt.Errorf("builder cannot be nil")
	}
	if len(cmd) == 0 {
		return fmt.Errorf("command cannot be empty")
	}

	isolation, err := parse.IsolationOption("")
	if err != nil {
		return fmt.Errorf("failed to parse isolation option: %w", err)
	}

	options := buildah.RunOptions{
		Isolation: isolation,
		Terminal:  buildah.WithoutTerminal,
	}

	if env != nil {
		options.Env = append(builder.Env(), env...)
	}

	if mounts != nil {
		options.Mounts = mounts
	}

	// Execute the command in the container
	if err := builder.Run(cmd, options); err != nil {
		return fmt.Errorf("failed to run command in container: %w", err)
	}

	return nil
}

func RunCommandInBuilder(builder *buildah.Builder, cmd []string, mounts []specs.Mount) error {
	return runCommandInBuilder(builder, cmd, mounts, nil)
}

func RunCommandInBuilderWithEnv(builder *buildah.Builder, cmd []string, mounts []specs.Mount, env []string) error {
	return runCommandInBuilder(builder, cmd, mounts, env)
}

func RunCommandInContainer(container string, cmd []string) error {
	// Get buildah store
	store, err := GetStore()
	if err != nil {
		return err
	}

	defer func() {
		if _, err := store.Shutdown(false); err != nil {
			// Log the error but don't fail the operation
			fmt.Printf("Warning: failed to shutdown store: %v\n", err)
		}
	}()

	// Get the builder for this container
	builder, err := buildah.OpenBuilder(store, container)
	if err != nil {
		return fmt.Errorf("failed to open builder for container %s: %w", container, err)
	}

	return RunCommandInBuilder(builder, cmd, nil)
}
