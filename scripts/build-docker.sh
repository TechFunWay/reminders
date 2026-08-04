#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

IMAGE="techfunways/reminders"
VERSION="$(cat VERSION | tr -d '\n')"
DOCKER_ARCH="$(docker version --format '{{.Server.Arch}}')"
case "$DOCKER_ARCH" in
  amd64|x86_64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported local Docker architecture: ${DOCKER_ARCH}"; exit 1 ;;
esac

BUNDLE="release/${VERSION}/techfunway-reminders-linux-${ARCH}.tar.gz"
CONTEXT_DIR="build/docker-local-context"

if [ ! -f "$BUNDLE" ]; then
  echo "Missing ${BUNDLE}. Build the matching release archive first: make build-all"
  exit 1
fi
if [ ! -f /etc/ssl/cert.pem ]; then
  echo "Missing host CA bundle at /etc/ssl/cert.pem; cannot make an offline TLS-capable image."
  exit 1
fi
if ! docker image inspect alpine:3.21 >/dev/null 2>&1; then
  echo "Missing local base image alpine:3.21. Load it locally before offline build."
  exit 1
fi

rm -rf "$CONTEXT_DIR"
mkdir -p "$CONTEXT_DIR"
trap 'rm -rf "$CONTEXT_DIR"' EXIT

tar xzf "$BUNDLE" --strip-components=1 -C "$CONTEXT_DIR"
mv "$CONTEXT_DIR/techfunway-reminders-linux-${ARCH}" "$CONTEXT_DIR/reminder"
chmod +x "$CONTEXT_DIR/reminder"
cp /etc/ssl/cert.pem "$CONTEXT_DIR/ca-certificates.crt"

echo "Building local ${ARCH} image without network access or registry push..."
docker build --pull=false --network=none \
  -f "$ROOT_DIR/Dockerfile.local" \
  -t "${IMAGE}:${VERSION}" \
  -t "${IMAGE}:latest" \
  "$CONTEXT_DIR"

echo "Local image built: ${IMAGE}:${VERSION}"
