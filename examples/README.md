# Image-Build Examples

In each directory here, you will find example files to explore how you may want to use the image-build scritps and container.

If you are new to this tooling, we recommend starting with the `mini-bootcamp` example which is detailed below.

## 1. Create your image-build config file 

The yaml file that describes how to create the system image is fairly straightforward.

1. At the top are a set of options which are defined in [arguments.py](/src/arguments.py).
2. Next are a set of repositories that will be used for package installations
3. Package groups and a list of packages follow
4. Our example below skips a `files:` section for copying files directly into the image.
5. The `cmd:` section includes commands that will be run at the end of the image build.  __Note:__ The example below includes a command to create the initrd which is necessary to make it bootable.

```yaml
# rocky-9-base.yaml
options:
  layer_type: 'base'
  name: 'rocky9-base'
  publish_tags: '9'
  pkg_manager: 'dnf'
  parent: 'scratch'
  publish_registry: 'private_registry.yoursite.com'
  #registry_opts_push:
  #  - '--tls-verify=false'

repos:
  - alias: 'Rock_BaseOS'
    config: |
      baseurl=https://download.rockylinux.org/pub/rocky/9/BaseOS/x86_64/os/
      enabled=1
      gpgcheck=1
    gpg_key: 'https://dl.rockylinux.org/pub/rocky/RPM-GPG-KEY-Rocky-9'
  - alias: 'Rock_AppStream'
    config: |
      baseurl=https://download.rockylinux.org/pub/rocky/9/AppStream/x86_64/os/
      enabled=1
      gpgcheck=1
    gpg_key: 'https://dl.rockylinux.org/pub/rocky/RPM-GPG-KEY-Rocky-9'
  - alias: 'Epel'
    config: |
      baseurl=https://dl.fedoraproject.org/pub/epel/9/Everything/x86_64/
      enabled=1
      gpgcheck=1
    gpg_key: 'https://dl.fedoraproject.org/pub/epel/RPM-GPG-KEY-EPEL-9'


package_groups:
  - 'Minimal Install'

packages:
  - kernel
  - wget
  - dracut-live
  - dmidecode
  - tpm2-tools
  - tpm2-abrmd
  - tpm2-tss
  - vim
  - curl
  - tmux
  - cloud-init
  - wireguard-tools
  - jq

cmds:
  - cmd: 'dracut --add "dmsquash-live livenet network-manager" --kver $(basename /lib/modules/*) -N -f --logfile /tmp/dracut.log 2>/dev/null'
  - cmd: 'echo DRACUT LOG:; cat /tmp/dracut.log'
```

### Repository Configuration

Each repository requires:
- **`alias`**: Short name for the repo (used for the `.repo` filename)
- **`config`**: Repository configuration content (will be written to `/etc/imgbuild/yum.repos.d/<alias>.repo`)
  - Do not include the `[alias]` header - it's added automatically by the code
  - Must include `baseurl`, typically includes `enabled=1` and `gpgcheck=1`
- **`gpg_key`** (optional): URL to the GPG key for this repository
  - If all repos have `gpg_key` defined, keys will be downloaded and imported automatically
  - Packages will be installed without `--nogpgcheck`
  - If any repo is missing `gpg_key` or download fails, `--nogpgcheck` will be used with a warning
  - Works for both scratch and non-scratch installs

We have an example that activates our wireguard tunnel for cloud-init in [rocky-9-base.yaml](/examples/mini-bootcamp/rocky-9-base.yaml).  This is part of the larger tutorial for running an OpenCHAMI system.  You can use the contents of the file above or the included example file in the command below.

## 2. Use the published container to build the image

This is intended to be run in a checkout of this repository from the `examples/` directory which is mapped into the container as the `/data/` directory.

```bash
podman run \
  --name ochami-build-base-9 \
  --rm --replace -it \
  --device /dev/fuse \
  -t -v $PWD:/data:ro \
    ghcr.io/openchami/image-build:latest \
  "image-build --log-level=INFO --config /data/mini-bootcamp/rocky-9-base.yaml"
```

## 3. Import the image locally for serving over nfs

The included [import-image.sh](/scripts/image-import.sh) script imports from your repository and makes it available for you.

```bash
# This script is used to convert a system image stored in an OCI registry for use to boot HPC nodes.
# 1. Pull the container image using podman
# 2. Mount the container image
# 3. Copy the kernel and initramfs
# 4. Create a squashfs image of the root filesystem
# 5. Unmount the container image

scripts/import-image.sh registry.openchami.dev/system-images/rocky9-base:9 rocky9-image
```

