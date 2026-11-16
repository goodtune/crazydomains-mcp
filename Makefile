BINARY  := crazydomains-mcp
PKG     := ./cmd/crazydomains-mcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test race lint fmt vet tidy clean

all: lint test build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# lint runs gofmt, go vet, and staticcheck / golangci-lint when installed:
#   go install honnef.co/go/tools/cmd/staticcheck@latest
#   go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
lint: vet
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	@if command -v staticcheck >/dev/null; then staticcheck ./...; else echo "staticcheck not installed, skipping"; fi
	@if command -v golangci-lint >/dev/null; then golangci-lint run ./...; else echo "golangci-lint not installed, skipping"; fi

tidy:
	go mod tidy

clean:
	rm -rf bin
