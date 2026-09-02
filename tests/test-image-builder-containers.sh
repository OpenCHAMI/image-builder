#!/bin/bash
#
# Smoke tests for ImageBuilder container images.
# Usage:
#   ./tests/test-image-builder-containers.sh

set -euo pipefail

PODMAN="${PODMAN:-podman}"

IMAGES=(
    "localhost/image-builder:test"
    "localhost/image-builder:test.el9"
    "localhost/image-builder:test.minimal"
)

PASS=0
FAIL=0

run_test()
{
    local name="$1"
    shift

    printf '  %-55s' "$name"

    if "$@" >/dev/null 2>&1; then
        echo "PASS"
        PASS=$((PASS + 1))
    else
        echo "FAIL"
        FAIL=$((FAIL + 1))
    fi
}

run_test_output()
{
    local name="$1"
    shift

    printf '  %-55s' "$name"

    if output=$("$@" 2>&1); then
        echo "PASS"
        PASS=$((PASS + 1))
        return 0
    else
        echo "FAIL"
        echo
        echo "$output"
        FAIL=$((FAIL + 1))
        return 1
    fi
}

test_image()
{
    local image="$1"

    echo
    echo "============================================================"
    echo "Testing: $image"
    echo "============================================================"

    # ------------------------------------------------------------
    # Container startup / ImageBuilder entrypoint
    # ------------------------------------------------------------

    run_test \
        "Container starts and --help works" \
        "$PODMAN" run --rm "$image" --help

    # ------------------------------------------------------------
    # User configuration
    # ------------------------------------------------------------

    run_test_output \
        "Container runs as builder user" \
        "$PODMAN" run --rm "$image" id

    run_test \
        "builder user exists" \
        "$PODMAN" run --rm "$image" \
        getent passwd builder

    run_test \
        "builder home directory exists" \
        "$PODMAN" run --rm "$image" \
        test -d /home/builder

    # ------------------------------------------------------------
    # Python
    # ------------------------------------------------------------

    run_test \
        "Python 3.11 is installed" \
        "$PODMAN" run --rm "$image" \
        python3.11 --version

    run_test \
        "pip is installed" \
        "$PODMAN" run --rm "$image" \
        python3.11 -m pip --version

    run_test \
        "Python can import installed dependencies" \
        "$PODMAN" run --rm "$image" \
        python3.11 -c 'import ansible'

    # ------------------------------------------------------------
    # Buildah
    # ------------------------------------------------------------

    run_test \
        "Buildah is installed" \
        "$PODMAN" run --rm "$image" \
        buildah --version

    run_test \
        "Buildah info succeeds" \
        "$PODMAN" run --rm "$image" \
        buildah info

    # ------------------------------------------------------------
    # Rootless user namespace configuration
    # ------------------------------------------------------------

    run_test \
        "newuidmap exists" \
        "$PODMAN" run --rm "$image" \
        command -v newuidmap

    run_test \
        "newgidmap exists" \
        "$PODMAN" run --rm "$image" \
        command -v newgidmap

    run_test \
        "newuidmap has cap_setuid" \
        "$PODMAN" run --rm "$image" \
        sh -c 'getcap "$(command -v newuidmap)" | grep -q cap_setuid'

    run_test \
        "newgidmap has cap_setgid" \
        "$PODMAN" run --rm "$image" \
        sh -c 'getcap "$(command -v newgidmap)" | grep -q cap_setgid'

    run_test \
        "/etc/subuid contains builder mapping" \
        "$PODMAN" run --rm "$image" \
        sh -c 'grep -q "^builder:2000:50000$" /etc/subuid'

    run_test \
        "/etc/subgid contains builder mapping" \
        "$PODMAN" run --rm "$image" \
        sh -c 'grep -q "^builder:2000:50000$" /etc/subgid'

    # ------------------------------------------------------------
    # Rootless Buildah namespace test
    # ------------------------------------------------------------

    run_test_output \
        "Buildah unshare works" \
        "$PODMAN" run --rm "$image" \
        buildah unshare id

    run_test_output \
        "Buildah unshare has user namespace" \
        "$PODMAN" run --rm "$image" \
        buildah unshare cat /proc/self/uid_map
}

echo
echo "ImageBuilder container smoke tests"
echo

for image in "${IMAGES[@]}"; do
    if ! "$PODMAN" image exists "$image"; then
        echo "ERROR: image does not exist: $image"
        echo "Build the image before running the tests."
        exit 2
    fi

    test_image "$image"
done

echo
echo "============================================================"
echo "Test summary"
echo "============================================================"
echo "Passed: $PASS"
echo "Failed: $FAIL"
echo

if [[ "$FAIL" -ne 0 ]]; then
    echo "RESULT: FAIL"
    exit 1
fi

echo "RESULT: PASS"
