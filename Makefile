BINARY_NAME=verifisci
BUILD_DIR=bin
DIST_DIR=dist
VERSION?=$(shell git describe --tags --always 2>/dev/null || echo "v1.0.0")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS=-s -w -X github.com/hossam1522/VerifiSci/cmd.Version=$(VERSION) -X github.com/hossam1522/VerifiSci/cmd.Commit=$(COMMIT) -X github.com/hossam1522/VerifiSci/cmd.BuildDate=$(DATE)

.PHONY: all build clean test install release

all: build

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) .

install:
	go install -ldflags="$(LDFLAGS)" .

clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR)

test:
	go test -v ./...

release: clean
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe .
