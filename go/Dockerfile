# Build stage
FROM docker.io/library/golang:1.23-bookworm AS builder

# Install build dependencies for CGO
RUN apt-get update && apt-get install -y gcc libc6-dev libgpgme-dev libseccomp-dev libsqlite3-dev libbtrfs-dev libdevmapper-dev && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy source code
COPY . ./

# Download dependencies
RUN go mod download

# Build the application with CGO enabled
RUN go build -o image-builder ./cmd/image-builder

# Runtime stage
FROM docker.io/library/almalinux:9.6

RUN dnf clean all && \
    dnf update --nogpgcheck -y && \
    dnf install -y epel-release

RUN dnf install -y \
        bash \
        buildah \
        squashfs-tools \
	    fuse-overlayfs

# Create local user for rootless image builds
RUN useradd --uid 1002 builder && \
    chown -R builder /home/builder

# Set up capabilities for newuidmap/newgidmap
RUN setcap cap_setuid=ep "$(command -v newuidmap)" && \
    setcap cap_setgid=ep "$(command -v newgidmap)" &&\
    chmod 0755 "$(command -v newuidmap)" && \
    chmod 0755 "$(command -v newgidmap)" && \
    rpm --restore shadow-utils && \
    echo "builder:2000:50000" > /etc/subuid && \
    echo "builder:2000:50000" > /etc/subgid

# Make builder the default user when running container
USER builder
WORKDIR /home/builder

ENV BUILDAH_ISOLATION=chroot

# Configure container storage for builder user
# Using 'vfs' storage driver to work around whiteout file handling issues in nested containers
RUN mkdir -p /home/builder/.config/containers && \
    { \
        echo '[storage]'; \
        echo 'driver = "vfs"'; \
        echo 'graphroot = "/home/builder/.local/share/containers/storage"'; \
    } > /home/builder/.config/containers/storage.conf

# Copy binary from builder
COPY --from=builder /build/image-builder /app/image-builder

ENTRYPOINT ["/app/image-builder"]
