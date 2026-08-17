#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <image-name> <platform>" >&2
  exit 2
fi

IMAGE_NAME="$1"
PLATFORM="$2"

if [ -n "${BUILDER:-}" ]; then
  docker buildx build \
    --builder "$BUILDER" \
    --platform "$PLATFORM" \
    -f benzhi.Dockerfile \
    -t "$IMAGE_NAME" \
    --load \
    .
else
  docker buildx build \
    --platform "$PLATFORM" \
    -f benzhi.Dockerfile \
    -t "$IMAGE_NAME" \
    --load \
    .
fi
