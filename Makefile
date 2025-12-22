
# =============================================================================
# 7. MAKEFILE
# =============================================================================

# File: Makefile
.PHONY: all build test clean proto docker-build certs install

VERSION ?= 0.1.0
GOFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"

all: build

proto:
	@echo "Generating proto files..."
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/v1/*.proto

build: proto
	@echo "Building Mandau Core..."
	@go build $(GOFLAGS) -o bin/mandau-core ./cmd/mandau-core
	@echo "Building Mandau Agent..."
	@go build $(GOFLAGS) -o bin/mandau-agent ./cmd/mandau-agent
	@echo "Building Mandau CLI..."
	@go build $(GOFLAGS) -o bin/mandau ./cmd/mandau-cli

build-static: proto
	@echo "Building static Mandau Core..."
	@CGO_ENABLED=0 go build $(GOFLAGS) -a -installsuffix cgo -o bin/mandau-core ./cmd/mandau-core
	@echo "Building static Mandau Agent..."
	@CGO_ENABLED=0 go build $(GOFLAGS) -a -installsuffix cgo -o bin/mandau-agent ./cmd/mandau-agent
	@echo "Building static Mandau CLI..."
	@CGO_ENABLED=0 go build $(GOFLAGS) -a -installsuffix cgo -o bin/mandau ./cmd/mandau-cli

test:
	@go test -v -race -coverprofile=coverage.out ./...

clean:
	@rm -rf bin/ coverage.out

docker-build:
	@docker build -t mandau/core:$(VERSION) -f Dockerfile.core .
	@docker build -t mandau/agent:$(VERSION) -f Dockerfile.agent .

certs:
	@./scripts/generate-certs.sh ./certs

install:
	@install -m 755 bin/mandau-core /usr/local/bin/
	@install -m 755 bin/mandau-agent /usr/local/bin/
	@install -m 755 bin/mandau /usr/local/bin/