#!/bin/bash

set -euo pipefail

# Run this script from:
#   image-builder/scripts/

BUILDAH="${BUILDAH:-buildah}"

IMAGE_NAME="localhost/image-builder"
BUILD_CONTEXT=".."
DOCKERFILE_DIR="../dockerfiles/dnf"

echo "============================================================"
echo "Building ImageBuilder container images"
echo "============================================================"
echo

# Verify Buildah is installed.
if ! command -v "${BUILDAH}" >/dev/null 2>&1;
then
    echo "ERROR: buildah is not installed or not in PATH."
    exit 1
fi

build_image()
{
    local dockerfile="$1"
    local tag="$2"

    echo
    echo "------------------------------------------------------------"
    echo "Building ${IMAGE_NAME}:${tag}"
    echo "Dockerfile: ${dockerfile}"
    echo "Context:    ${BUILD_CONTEXT}"
    echo "------------------------------------------------------------"
    echo

    "${BUILDAH}" bud \
        -t ghcr.io/openchami/image-build:latest \
        --file "${dockerfile}" \
        --tag "${IMAGE_NAME}:${tag}" \
        "${BUILD_CONTEXT}"

    echo
    echo "SUCCESS: ${IMAGE_NAME}:${tag}"
}

# Standard DNF image
build_image \
    "${DOCKERFILE_DIR}/Dockerfile" \
    "test"

# DNF EL9 image
build_image \
    "${DOCKERFILE_DIR}/Dockerfile.el9" \
    "test.el9"

# Minimal DNF image
build_image \
    "${DOCKERFILE_DIR}/Dockerfile.minimal" \
    "test.minimal"

echo
echo "============================================================"
echo "All ImageBuilder dnf images built successfully"
echo "============================================================"
echo

