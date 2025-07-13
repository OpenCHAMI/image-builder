#!/bin/bash
set -euo pipefail

# Only su if running as root
if [[ "$(id -u)" == "0" ]]; then
  exec su - builder -c "buildah unshare $*"
else
  exec buildah unshare "$@"
fi