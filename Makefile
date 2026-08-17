APP_NAME := reminder
DOCKER_IMAGE := techfunways/reminders
PACKAGE_TARGET ?= fnos
PORT ?= 8906
DEV_DATA_DIR ?= ./data
VERSION := $(shell cat VERSION | tr -d '\n')
BUILD_TIME := $(shell date +%Y-%m-%dT%H:%M:%S)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X smallgo/server/version.Version=$(VERSION) -X smallgo/server/version.BuildTime=$(BUILD_TIME) -X smallgo/server/version.GitCommit=$(GIT_COMMIT) -X smallgo/server/version.AppName=提醒事项

.PHONY: help dev start build frontend-deps build-frontend build-backend build-linux build-docker build-docker-multi build-all package package-fnos fnpack clean

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  help            Show this help"
	@echo "  dev             Build frontend/backend and start one server on port $(PORT)"
	@echo "  start           Start the built backend with bundled frontend"
	@echo "  build           Build frontend + backend"
	@echo "  build-frontend  Build frontend only"
	@echo "  build-backend   Build backend only"
	@echo "  build-linux     Build for linux/amd64"
	@echo "  build-docker    Build an offline local image for this Docker architecture"
	@echo "  build-docker-multi  Build an amd64/arm64 OCI image archive (requires local multiarch base images)"
	@echo "  build-all       Build for all platforms"
	@echo "  package         Package one target; default is fnOS (PACKAGE_TARGET=fnos|apps|docker)"
	@echo "  package-fnos    Build fnOS packages only; does not build a Docker image"
	@echo "  fnpack          Alias for package-fnos"
	@echo "  clean           Clean build artifacts"

dev: build
	@echo "Starting 提醒事项 at http://localhost:$(PORT)"
	./build/$(APP_NAME) -port=$(PORT) -data-dir="$(DEV_DATA_DIR)" -web-dir=./server/static/dist

start: build-backend
	@test -f server/static/dist/index.html || (echo "Frontend is missing. Run 'make build-frontend' first." && exit 1)
	./build/$(APP_NAME) -port=$(PORT) -data-dir="$(DEV_DATA_DIR)" -web-dir=./server/static/dist

build: build-frontend build-backend

frontend-deps:
	@if [ ! -x web/node_modules/.bin/vite ] || [ ! -x web/node_modules/.bin/vue-tsc ]; then \
		echo "Installing frontend dependencies..."; \
		cd web && npm ci; \
	fi

build-frontend: frontend-deps
	cd web && npm run build
	rm -rf server/static/dist
	mkdir -p server/static/dist
	cp -R web/dist/. server/static/dist/

build-backend:
	mkdir -p build
	cd server && go build -ldflags "$(LDFLAGS)" -o ../build/$(APP_NAME) .

build-linux:
	docker run --rm \
		-v "$(CURDIR)/server:/src" \
		-v "go-build-cache:/root/.cache/go-build" \
		-v "go-mod-cache:/go/pkg/mod" \
		-w /src \
		--platform linux/amd64 \
		-e "LDFLAGS=$(LDFLAGS)" \
		golang:1.26-alpine \
		sh -c 'apk add --no-cache gcc musl-dev && CGO_ENABLED=1 go build -ldflags "$$LDFLAGS -extldflags -static" -o reminder-linux-amd64 .'

build-docker:
	bash scripts/build-docker.sh

build-docker-multi:
	bash scripts/build-docker-multi.sh

build-all:
	bash scripts/build-all.sh

package:
	@case "$(PACKAGE_TARGET)" in \
		fnos) $(MAKE) --no-print-directory package-fnos ;; \
		apps) $(MAKE) --no-print-directory build-all ;; \
		docker) $(MAKE) --no-print-directory build-docker ;; \
		*) echo "Unsupported PACKAGE_TARGET: $(PACKAGE_TARGET) (use fnos, apps, or docker)"; exit 2 ;; \
	esac

package-fnos:
	bash scripts/build-fnpack.sh

fnpack: package-fnos

clean:
	rm -f server/$(APP_NAME) server/$(APP_NAME)-*
	rm -rf server/static/dist
	rm -rf web/dist
	rm -rf build/
