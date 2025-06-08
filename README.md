# image-build

A wrapper around various `buildah` commands that makes creating images in layers easier.
There are two supported modes at the moment, a "base" type layer and an "ansible" type layer

# Running

The recommended and official way to run `image-build` is using the `ghcr.io/openchami/image-build` container (specifically using [Podman](https://podman.io)) as it avoids Python versioning/dependency troubles. Running bare-metal is not officially supported, though it is possible to do at one's own risk. Using Docker has caused issues and is not officially supported, though it is probably possible (again, at one's own risk) with some tweaking.

To build an image using the container, the config file needs to be mapped into the container, as well as the FUSE filesystem device:

```
podman run --rm \
  --device /dev/fuse \
  --network host \
  --cap-add=SYS_ADMIN \
  --cap-add=SETUID \
  --cap-add=SETGID \
  --security-opt seccomp=unconfined \
  --security-opt label=disable \
  -v /opt/workdir/images/test-rocky-9.5.yaml:/home/builder/config.yaml \
  ghcr.io/openchami/image-build:latest \
  image-build --config config.yaml --log-level DEBUG
```

If the config.yaml pushes to S3, specify the credentials by adding `-e S3_ACCESS=<s3-user>` and `-e S3_SECRET=<s3-password>` to the command above. See [S3](#s3) below.

# Building Container

From the root of the repository:
```
buildah bud -t ghcr.io/openchami/image-build:latest -f src/dockerfiles/Dockerfile .
```

# Configuration

## Base Type Layer

The premise here is very simple. The `image-build` tool builds a base layer by starting a container, then using the provided package manager to install repos and packages. There is limited support for running basic commands inside the container. These settings are provided in a config file and command line options

An example config file that builds a base OS image based on Rocky 8.10:

```yaml
# Example image-build config for a base-type image.

# Global image-build options for this image
options:
  # Build a "normal" layer (as opposed to an Ansible-type layer)
  layer_type: 'base'

  # Name and tag for this image, used in publishing to OCI registries
  # and S3 for identification.
  name: 'rocky-base'
  # One or more tags to publish image with. If one, value is a string.
  # If multiple, the value is a YAML array of strings.
  publish_tags: '8.10'

  # Distribution flavor of image.
  pkg_manager: 'dnf'

  # Starting filesystem of image. 'scratch' means to start with a blank
  # filesystem. Currently, only OCI images can be used as parents. In
  # this example, the image is pushed to:
  #
  #  registry.mysite.tld/openchami/rocky-base:8.10
  #
  # This value can be used as the value to 'parent' if one wished to use
  # the 'rocky-base:8.10' image as a parent.
  parent: 'scratch'

  # Publish OCI image to local podman registry. Note that if running
  # the image-build container, this option will not be a benefit if
  # the container is removed after running, since the container gets
  # deleted after the build process exits.
  #publish_local: true

  # Publish OCI image to container registry. This image can be used
  # as a parent for child images. Use this when this image should
  # be used as a parent for subsequent images.
  #
  # The below config, combined with 'name' and 'publish_tags', will
  # publish this OCI image to:
  #
  #  registry.mysite.tld/openchami/rocky-base:8.10
  #
  publish_registry: 'registry.mysite.tld/openchami'
  registry_opts_push:
    - '--tls-verify=false'

  # Publish to S3 instance. This image be used for booting. Use this
  # if an image is to be used for booting.
  #
  # The below config, combined with 'name' and 'publish_tags', will
  # publish this SquashFS image to:
  #
  #  http://s3.mysite.tld/boot-images/compute/base/rocky8.10-rocky-base-8.10
  #
  publish_s3: 'http://s3.mysite.tld'
  s3_prefix: 'compute/base/'
  s3_bucket: 'boot-images'

# Package repositories to add. This example uses YUM/DNF repositories.
repos:
  - alias: 'rocky-baseos'
    url: 'http://dl.rockylinux.org/pub/rocky/8/BaseOS/x86_64/os'
  - alias: 'rock_appstream'
    url: 'http://dl.rockylinux.org/pub/rocky/8/AppStream/x86_64/os'
  - alias: 'rock_powertools'
    url: 'http://dl.rockylinux.org/pub/rocky/8/PowerTools/x86_64/os'
  - alias: 'epel'
    url: 'http://dl.fedoraproject.org/pub/epel/8/Everything/x86_64/'

# Package groups to install, in this example YUM/DNF package groups.
package_groups:
  - 'Minimal Install'
  - 'Development Tools'

# List of packages to install after repos get added. These names get passed
# straight to the package manager.
packages:
  - kernel
  - wget

# List of commands to run after package management steps get run. Each
# command gets passed to the shell, so redirection can be used. Besides
# 'cmd', an optional 'loglevel` can be passed (e.g. 'INFO', 'DEBUG') to
# control command verbosity. By default, it is 'INFO'.
cmds:
  - cmd: 'echo hello'
```

Then you can use this config file to build a "base" layer (make sure the `S3_ACCESS` and `S3_SECRET` environment variables are set to the S3 credentials if being used):

```
podman run \
  --rm \
  --device /dev/fuse \
  -v /path/to/config.yaml:/home/builder/config.yaml:Z \
  -e "S3_ACCESS=${S3_ACCESS}" \
  -e "S3_SECRET=${S3_SECRET}" \
  ghcr.io/openchami/image-build \
  image-build --config config.yaml --log-level DEBUG
```

See [Publishing Images](#publishing-images) below for more explanation on how `image-build` publishes images.

You can then build on top of this base os with a new config file, just point the `parent` key at the base os container image, in the above example, `registry.mysite.tld/openchami/rocky-base:8.10`.


## Ansible Type Layer

You can also run an Ansible playbook against a buildah container. This type of layer uses the Buildah connection plugin in Ansible to treat the container as a host.

Configuration for an Ansible-type layer is largely the same as a base-type layer configuration with a few differences.

```yaml
# An Ansible-type layer only needs the global options block.
options:
  # Layer type us 'ansible' instead of 'base'
  layer_type: 'ansible'

  # Ansible-specific options.
  #
  # 'groups' defines the Ansible groups in the passed inventory to run the
  # playbook(s) on.
  groups:
    - 'img_ochami_compute'
    - 'img_ochami'
  #
  # The playbook(s) to run against the image.
  playbooks: 'playbooks/images/compute.yaml'
  #
  # The Ansible inventory to pass corresponding with the playbook(s).
  inventory: 'inventory/'
  #
  # The numerical Ansible verbosity level:
  #   0 (default): Displays only critical information.
  #   1 (-v): Shows basic information like task names and results.
  #   2 (-vv): Includes more detailed output, such as variable values.
  #   3 (-vvv): Displays additional debugging data, such as task-level operations.
  #   4 (-vvvv): Enables connection debugging, providing a deep dive into network communication.
  ansible_verbosity: '3'

  # Everything else is the same format as base layer.
  name: 'ansible-layer'
  publish_tags: '8.10'
  parent: 'registry.mysite.tld/openchami/rocky-base:8.10'
  publish_registry: 'registry.mysite.tld/openchami'
  registry_opts_push:
    - '--tls-verify=false'
  publish_s3: 'http://s3.mysite.tld'
  s3_prefix: 'compute/ansible/'
  s3_bucket: 'boot-images'
```

Build the image with:

```
podman run \
  --rm \
  --device /dev/fuse \
  -v /path/to/config.yaml:/home/builder/config.yaml:Z \
  -v /path/to/ansible/inventory/:/home/builder/inventory/:Z \
  -v /path/to/ansible/playbooks/:/home/builder/playbooks/:Z \
  -e "S3_ACCESS=${S3_ACCESS}" \
  -e "S3_SECRET=${S3_SECRET}" \
  ghcr.io/openchami/image-build \
  image-build --config config.yaml --log-level DEBUG
```

> [!NOTE]
> In order to be able to use Ansible on the image, the parent must be set up to
> use Ansible (e.g. Ansible must be installed, etc.).

# Publishing Images

The `image-build` tool can publish the image layers to a few kinds of endpoints

## S3

Using the `--publish-s3 <URL>` flag or `publish-s3` config key will push to an S3 endpoint.

Credentials for S3 can be set via environment variables. Use `S3_ACCESS` for the username and `S3_SECRET` for the password.

## Registry

Using the `--publish-registry <URL>` flag or `publish-registry` config key will push to the passed registry base URL (not including image tag). Use `--registry-opts-push`/`registry-opts-push` to specify flags/args to pass to the `buildah push` command to push.

There is an equivalent flag/config option `--registry-opts-pull`/`registry-opts-pull` whose value is passed to the `buildah push` command to pull the parent OCI image.

## Local

Using the `--publish-local` flag or `publish-local` config key will push the resulting OCI image to the local podman registry using `buildah commit`.
