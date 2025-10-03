This version of image-build uses the buildah API directly instead of shelling out to the buildah command line tool. See the [buildah API documentation](https://pkg.go.dev/github.com/containers/buildah) for more details. The idea is that it will give us tighter integration with container APIs and is a cleaner approach than using subprocesses.

Its designed to be a drop-in replacement for the python implementation, so the CLI is very similar, appart from following the Go conventions, such as using a single dash for flags. It operates on the same configuration files as the python version.

It supports:

- scratch parents using a helper container to provide dnf, yum etc. without them having to be installed on the host.
- ansible layers again using a helper container and the chroot connection plugin, again not need to ansible to be installed on the host.
- publishing to S3, registries and local

It currently doesn't support the following flags ( although they could be added if needed):
- -scap-benchmark
- -oval-eval
- -install-scap

Support for `--buildah_extra_args` such as when running command is not implemented, as it probably makes more sense to explictly support the options with
additions to the configation file format, for example supporting mounting volumes when running commands.