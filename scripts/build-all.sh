#!/bin/bash
# 全平台编译打包——产物格式对齐 brick-game 项目 release/<VERSION>/ 发行目录规范：
#   techfunway-reminders-<VERSION>-{linux,darwin}-{amd64,arm64}.tar.gz
#   techfunway-reminders-<VERSION>-windows-amd64.zip
#   docker-compose.yml（镜像 tag 钉死为当前版本）/ docker-compose.latest.yml
#   CHANGELOG.md / screenshots/ + screenshots-<VERSION>.zip
# 发行包必备文件清单见 AGENTS.md「打包与发行」一节，两个打包脚本共同保证。
#
# 压缩包内为 <包名>/ 目录：reminder 二进制 + static/dist 前端资源
# （启动示例：./reminder -web-dir ./static/dist -data-dir ./data）。
# reminders 使用 CGO（mattn/go-sqlite3），各平台编译方式：
#   linux        → golang:1.26-alpine 容器内 CGO_ENABLED=1 静态编译
#   darwin       → 本机 clang（amd64 目标加 -arch x86_64）
#   windows      → mingw-w64（x86_64-w64-mingw32-gcc，brew install mingw-w64）
set -e

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

APP_NAME="reminder"
PACKAGE_PREFIX="techfunway-reminders"
VERSION=$(cat VERSION | tr -d '\n')
BUILD_TIME=$(date +%Y-%m-%dT%H:%M:%S)
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS="-X smallgo/server/version.Version=${VERSION} -X smallgo/server/version.BuildTime=${BUILD_TIME} -X smallgo/server/version.GitCommit=${GIT_COMMIT} -X smallgo/server/version.AppName=提醒事项"

echo "Building frontend..."
if [ ! -d web/node_modules ]; then
  npm --prefix web ci
fi
npm --prefix web run build

echo "Copying frontend..."
rm -rf server/static/dist
mkdir -p server/static/dist
cp -R web/dist/. server/static/dist/

BUILD_DIR="release/${VERSION}"
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
  IFS="/" read -r GOOS GOARCH <<< "$PLATFORM"
  OUTPUT_NAME="${PACKAGE_PREFIX}-${VERSION}-${GOOS}-${GOARCH}"

  echo "Building ${OUTPUT_NAME}..."

  if [ "$GOOS" = "linux" ]; then
    # Linux 目标用 golang:alpine 容器编译（musl + 静态链接），容器缓存复用
    docker run --rm \
      -v "${ROOT_DIR}/server:/src" \
      -v "go-build-cache:/root/.cache/go-build" \
      -v "go-mod-cache:/go/pkg/mod" \
      -w /src \
      --platform "linux/${GOARCH}" \
      -e "LDFLAGS=${LDFLAGS}" \
      -e "GOARCH=${GOARCH}" \
      golang:1.26-alpine \
      sh -c 'for i in 1 2 3; do apk add --no-cache gcc musl-dev && break; echo "apk retry $i"; sleep 5; done && CGO_ENABLED=1 go build -ldflags "$LDFLAGS -extldflags -static" -o reminder-linux-${GOARCH} .'
    mv "server/reminder-linux-${GOARCH}" "server/${OUTPUT_NAME}"
  elif [ "$GOOS" = "windows" ]; then
    (cd server && \
      CGO_ENABLED=1 \
      CC=x86_64-w64-mingw32-gcc \
      GOOS=windows GOARCH=amd64 \
      go build -ldflags "${LDFLAGS} -extldflags -static" -o "../server/${OUTPUT_NAME}.exe" .)
  elif [ "$GOARCH" = "amd64" ]; then
    # Apple Silicon 上交叉编译 x86_64：给 clang 指定 -arch
    (cd server && \
      CGO_ENABLED=1 CC="clang -arch x86_64" \
      GOOS=darwin GOARCH=amd64 \
      go build -ldflags "${LDFLAGS}" -o "../server/${OUTPUT_NAME}" .)
  else
    (cd server && \
      CGO_ENABLED=1 \
      GOOS=$GOOS GOARCH=$GOARCH \
      go build -ldflags "${LDFLAGS}" -o "../server/${OUTPUT_NAME}" .)
  fi

  mkdir -p "${BUILD_DIR}/${OUTPUT_NAME}/static"
  if [ "$GOOS" = "windows" ]; then
    cp "server/${OUTPUT_NAME}.exe" "${BUILD_DIR}/${OUTPUT_NAME}/${APP_NAME}.exe"
    rm "server/${OUTPUT_NAME}.exe"
  else
    cp "server/${OUTPUT_NAME}" "${BUILD_DIR}/${OUTPUT_NAME}/${APP_NAME}"
    rm "server/${OUTPUT_NAME}"
  fi
  cp -R server/static/dist "${BUILD_DIR}/${OUTPUT_NAME}/static/dist"

  find "${BUILD_DIR}/${OUTPUT_NAME}" -name '.DS_Store' -delete

  cd "${ROOT_DIR}/${BUILD_DIR}"
  if [ "$GOOS" = "windows" ]; then
    zip -qr "${OUTPUT_NAME}.zip" "${OUTPUT_NAME}"
  else
    COPYFILE_DISABLE=1 tar czf "${OUTPUT_NAME}.tar.gz" "${OUTPUT_NAME}"
  fi
  cd "$ROOT_DIR"

  rm -rf "${BUILD_DIR}/${OUTPUT_NAME}"

  echo "Built ${OUTPUT_NAME}"
done

# 截图与更新日志随发行目录分发，版本目录自包含
if [ -d images/screenshots ]; then
  mkdir -p "${BUILD_DIR}/screenshots"
  cp images/screenshots/* "${BUILD_DIR}/screenshots/"
  (cd "${BUILD_DIR}" && zip -qr "screenshots-${VERSION}.zip" screenshots)
  echo "Copied screenshots and screenshots-${VERSION}.zip"
fi
[ -f CHANGELOG.md ] && cp CHANGELOG.md "${BUILD_DIR}/CHANGELOG.md"

# 部署 compose 文件随发行产物分发；版本文件中的镜像 tag 替换为当前版本，
# 拷贝到目标主机后 docker compose up -d 即可运行；latest 文件原样分发。
sed "s|techfunways/reminders:latest|techfunways/reminders:${VERSION}|" deploy/docker-compose.yml > "${BUILD_DIR}/docker-compose.yml"
cp deploy/docker-compose.latest.yml "${BUILD_DIR}/docker-compose.latest.yml"
echo "Copied docker-compose.yml and docker-compose.latest.yml"

echo "All builds completed in ${BUILD_DIR}/"
