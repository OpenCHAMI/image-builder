FROM almalinux:8.8

RUN dnf clean all && \
    dnf update --nogpgcheck -y && \
    dnf install -y epel-release && \
    dnf config-manager -y --set-enabled powertools

RUN dnf install -y \
        bash \
        buildah \
        python3.11 \
        python3.11-pip \
        fuse-overlayfs \
        tar \
        squashfs-tools \
	fuse-overlayfs

RUN pip3.11 install ansible ansible-base ansible-bender boto3 dnspython requests jinja2_ansible_filters

COPY src/ /usr/local/bin/
RUN chmod -R 0755 /usr/local/bin/

# Allow non-root to run buildah commands
RUN setcap cap_setuid=ep "$(command -v newuidmap)" && \
    setcap cap_setgid=ep "$(command -v newgidmap)" &&\
    chmod 0755 "$(command -v newuidmap)" && \
    chmod 0755 "$(command -v newgidmap)" && \
    rpm --restore shadow-utils && \
    echo "builder:2000:50000" > /etc/subuid && \
    echo "builder:2000:50000" > /etc/subgid

# Create local user for rootless image builds
RUN useradd --uid 1002 builder && \
    chown -R builder /home/builder

# Make builder the default user when running container
USER builder
WORKDIR /home/builder

ENV BUILDAH_ISOLATION=chroot

ENTRYPOINT ["/usr/bin/buildah", "unshare"]
