# Examples

This example covers how to build your first image with the `image-build` tool. Make sure you change into the root directory of this repository.

```bash
image_builder=path/to/image-builder
cd ${image_builder}
```

## Dependencies

This example requires the following tools to be installed on the host system.

 - `buildah`
 - `podman`

## Building the Image Builder

First, we will create a container image that will be use to run the `image-build` tool. The dockerfile we will be using for this is specified under the path `${image_builder}/dockerfiles/dnf/Dockerfile`. Run the following in the root directory of the repository.

```bash
buildah bud -t image-builder-test -f ${image_builder}/dockerfiles/dnf/Dockerfile .
```

Once the command above finishes without errors, you should see an `localhost/image-builder-test:latest` image listed when running the following command.

```bash
buildah images
```

Now, you can run the container image using podman by executing the following command.

```bash
podman run \
        --name image-build-test \
        --rm \
        --device /dev/fuse \
        -it \
        -v $PWD:/data/configs:ro \
	image-builder-test
	bash
```

This will bind the current directory to the container and drop you into a shell where we can build images.

> [!NOTE]
> Once you exit the container, the images will be deleted and you will have to rebuild them again.

## Building the Base Layer

Use the `base.yaml` config file to build a "base" layer that we can use with other images. We will be running the `image-build` tool with `buildah unshare`.

1. Run the `podman run` command mentioned in the ["Building the Image Builder"](<README#Building the Image Builder>) section to shell into the container.

``` bash
podman run \
        --name image-build-test \
        --rm \
        --device /dev/fuse \
        -it \
        -v $PWD:/data/configs:ro \
	image-builder-test bash
```

You should see a prompt as `root` and no images when you run `buildah images`.

2. In the container, run `buildah` to build the "base" layer.

```bash
	buildah unshare bash -c "image-build --config /data/configs/base.yaml"
```

3. In the container, run `buildah images` again to confirm that the image was built successfully. You should see an `localhost/almalinux:v0.0.1` image that was just created.

You can then build on top of this base image with a new config file by setting the `--parent` flag to the base image.

> [!TIP]
> Append the `--log-level=DEBUG` flag to see the output of the above command. Otherwise, no output will be shown.

## Building the Ansible Type Layer

You can also run an ansible playbook against a `buildah` container.  Using the `buildah` connection plugin in ansible, the parent container as a host. Use the `compute-ansible.yaml` config file to build an "ansible" layer:

```bash
podman run \
        --name image-build-test \
        --rm \
        --device /dev/fuse \
        -it \
        -v $PWD:/data/configs:ro \
	image-builder-test \
	buildah unshare bash -c "image-build --config /data/configs/compute-ansible.yaml"
```

This requires the parent to be setup to run ansible tasks. In our example, the "base" layer created by `base.yaml` is.


### Publish images
The `image-build` tool can publish the image layers to a few kinds of endpoints. In the examples above, both the "base" layer and the "ansible" layer are published local. Lets look at the 3 publishing options `image-build` contains:

### S3
using the `--publish-s3 <URL>` option will push to an s3 endpoint defined in `publish_dest`
You can also set the access and secret values with `S3_ACCESS` and `S3_SECRET` respectively

### Registry
Using the `--publish-registry <URL>` option will push to a docker registry defined in `publish_dest`. You can point to a certs directory by setting `REGISTRY_CERTS_DIR`.

### Local
Using the `--publish-local` option will squash the layer and copy it to buildah's local image destination .
