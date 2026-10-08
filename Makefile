VERSION ?= 0.1.0
BUILD_TIME ?= $(shell date -u +"%Y%m%d.%H%M%S")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "")
LDFLAGS = -s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)

.PHONY: all web build dev release test clean

all: web build

web:
	cd web && npm run build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/doo ./cmd/doo

dev:
	@echo "Starting backend and frontend dev servers..."
	@(go run ./cmd/doo & P1=$$!; cd web && npm run dev & P2=$$!; trap "kill $$P1 $$P2" EXIT; wait)

release: web
	mkdir -p dist
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/doo-$(VERSION)-linux-amd64 ./cmd/doo
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/doo-$(VERSION)-linux-arm64 ./cmd/doo
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/doo-$(VERSION)-darwin-amd64 ./cmd/doo
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/doo-$(VERSION)-darwin-arm64 ./cmd/doo
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/doo-$(VERSION)-windows-amd64.exe ./cmd/doo

test:
	go vet ./... && go test -v -count=1 ./...

clean:
	rm -rf bin dist web/dist
