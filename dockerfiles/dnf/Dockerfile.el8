FROM docker.io/library/almalinux:8.8

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

COPY requirements.txt /
RUN pip3.11 install -r /requirements.txt

COPY src/ /usr/local/bin/
RUN chmod -R 0755 /usr/local/bin/

COPY entrypoint.sh /entrypoint.sh
RUN chmod 0755 /entrypoint.sh

# Allow non-root to run buildah commands
RUN setcap cap_setuid=ep "$(command -v newuidmap)" && \
    setcap cap_setgid=ep "$(command -v newgidmap)" &&\
    chmod 0755 "$(command -v newuidmap)" && \
    chmod 0755 "$(command -v newgidmap)" && \
    rpm --restore shadow-utils

# Create local user for rootless image builds
RUN useradd --uid 1000 builder && \
    chown -R builder /home/builder

ENV BUILDAH_ISOLATION=chroot

ENTRYPOINT ["/entrypoint.sh"]
