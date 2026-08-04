#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

IMAGE="techfunways/reminders"
BUILDER="techfunway-reminders-multiarch"
VERSION="$(cat VERSION | tr -d '\n')"
BUILD_TIME="$(date +%Y-%m-%dT%H:%M:%S)"
GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
OUTPUT_DIR="release/${VERSION}"
OUTPUT_FILE="${OUTPUT_DIR}/techfunway-reminders-${VERSION}-multiarch.oci.tar"

if ! command -v docker >/dev/null 2>&1; then
  echo "Error: Docker is required to build the multi-platform image."
  exit 1
fi

mkdir -p "$OUTPUT_DIR"
rm -f "$OUTPUT_FILE"

# A docker-container builder is required for a single OCI archive containing
# both architectures. The default Docker driver cannot reliably export that
# merged manifest without pushing it to a registry.
if ! docker buildx inspect "$BUILDER" >/dev/null 2>&1; then
  docker buildx create --name "$BUILDER" --driver docker-container --use >/dev/null
fi

echo "Building ${IMAGE}:${VERSION} for linux/amd64,linux/arm64..."
echo "Writing local OCI image archive (no registry push): ${OUTPUT_FILE}"
docker buildx build \
  --builder "$BUILDER" \
  --platform linux/amd64,linux/arm64 \
  --build-arg "VERSION=${VERSION}" \
  --build-arg "BUILD_TIME=${BUILD_TIME}" \
  --build-arg "GIT_COMMIT=${GIT_COMMIT}" \
  -t "${IMAGE}:${VERSION}" \
  -t "${IMAGE}:latest" \
  --output "type=oci,dest=${OUTPUT_FILE}" \
  .

echo "Multi-platform OCI image archive completed: ${OUTPUT_FILE}"
echo "Load it on a target with: docker load -i ${OUTPUT_FILE}"
