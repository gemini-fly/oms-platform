APP := oms-platform
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/gemini-fly/oms-platform/internal/buildinfo.Version=$(VERSION) \
	-X github.com/gemini-fly/oms-platform/internal/buildinfo.Commit=$(COMMIT) \
	-X github.com/gemini-fly/oms-platform/internal/buildinfo.Date=$(BUILD_DATE)

.PHONY: build test vet sync-frontend release-snapshot

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/sy-platform-api

test:
	go test ./...

vet:
	go vet ./...

sync-frontend:
	npm --prefix frontend run build
	cp frontend/dist/index.html internal/web/static/index.html

release-snapshot:
	./scripts/package-release.sh $(VERSION)
