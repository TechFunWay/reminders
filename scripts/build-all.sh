#!/bin/bash
set -e

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

APP_NAME="reminder"
PACKAGE_NAME="techfunway-reminders"
APP_DISPLAY_NAME="提醒事项"
VERSION=$(cat VERSION | tr -d '\n')
BUILD_TIME=$(date +%Y-%m-%dT%H:%M:%S)
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS="-X smallgo/server/version.Version=${VERSION} -X smallgo/server/version.BuildTime=${BUILD_TIME} -X smallgo/server/version.GitCommit=${GIT_COMMIT} -X smallgo/server/version.AppName=${APP_DISPLAY_NAME}"

echo "Building frontend..."
cd web && npm ci && npm run build && cd ..

echo "Copying frontend..."
rm -rf server/static/dist
cp -r web/dist server/static/dist

BUILD_DIR="release/${VERSION}"
rm -rf ${BUILD_DIR}
mkdir -p ${BUILD_DIR}

PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
  IFS="/" read -r GOOS GOARCH <<< "$PLATFORM"
  OUTPUT_NAME="${PACKAGE_NAME}-${GOOS}-${GOARCH}"

  echo "Building ${OUTPUT_NAME}..."

  if [ "$GOOS" = "linux" ]; then
    # Use Docker for Linux targets (CGO cross-compilation)
    docker run --rm \
      -v "${ROOT_DIR}/server:/src" \
      -v "go-build-cache:/root/.cache/go-build" \
      -v "go-mod-cache:/go/pkg/mod" \
      -w /src \
      --platform "linux/${GOARCH}" \
      -e "LDFLAGS=${LDFLAGS}" \
      -e "GOARCH=${GOARCH}" \
      -e "OUTPUT_NAME=${OUTPUT_NAME}" \
      golang:1.26-alpine \
      sh -c 'apk add --no-cache gcc musl-dev && CGO_ENABLED=1 go build -ldflags "$LDFLAGS -extldflags -static" -o "$OUTPUT_NAME" .'
  elif [ "$GOOS" = "windows" ]; then
    cd server
    GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "${LDFLAGS}" -o ${OUTPUT_NAME}.exe .
    cd "$ROOT_DIR"
  else
    cd server
    CGO_ENABLED=1 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "${LDFLAGS}" -o ${OUTPUT_NAME} .
    cd "$ROOT_DIR"
  fi

  mkdir -p ${BUILD_DIR}/${OUTPUT_NAME}

  if [ "$GOOS" = "windows" ]; then
    cp server/${OUTPUT_NAME}.exe ${BUILD_DIR}/${OUTPUT_NAME}/
    rm server/${OUTPUT_NAME}.exe
  else
    cp server/${OUTPUT_NAME} ${BUILD_DIR}/${OUTPUT_NAME}/
    rm server/${OUTPUT_NAME}
  fi

  cp -r server/static/dist ${BUILD_DIR}/${OUTPUT_NAME}/www

  cd "${ROOT_DIR}/${BUILD_DIR}"
  if [ "$GOOS" = "windows" ]; then
    zip -r ${OUTPUT_NAME}.zip ${OUTPUT_NAME}
  else
    tar czf ${OUTPUT_NAME}.tar.gz ${OUTPUT_NAME}
  fi
  cd "$ROOT_DIR"

  rm -rf ${BUILD_DIR}/${OUTPUT_NAME}

  echo "Built ${OUTPUT_NAME}"
done

echo "All builds completed in ${BUILD_DIR}/"
