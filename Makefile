BINARY      := openclaw-agentd
MODULE      := github.com/burmaster/openclaw-agentd
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS     := -ldflags "-X main.version=$(VERSION) -s -w"
BUILD_DIR   := dist
GOOS_DARWIN := darwin
GOARCH_AMD  := amd64
GOARCH_ARM  := arm64

.PHONY: all build build-darwin test lint clean install fmt vet

all: build

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/$(BINARY)/

build-darwin:
	mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS_DARWIN) GOARCH=$(GOARCH_AMD) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 ./cmd/$(BINARY)/
	GOOS=$(GOOS_DARWIN) GOARCH=$(GOARCH_ARM) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 ./cmd/$(BINARY)/

release: build-darwin
	cd $(BUILD_DIR) && \
	  tar czf $(BINARY)-darwin-amd64.tar.gz $(BINARY)-darwin-amd64 && \
	  tar czf $(BINARY)-darwin-arm64.tar.gz $(BINARY)-darwin-arm64
	sha256sum $(BUILD_DIR)/*.tar.gz > $(BUILD_DIR)/checksums.txt
	@echo "Update Formula/openclaw-agentd.rb with checksums from $(BUILD_DIR)/checksums.txt"

test:
	go test ./... -v -race -timeout 60s

lint:
	@which golangci-lint > /dev/null || (echo "install golangci-lint first: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

fmt:
	gofmt -w -s .

vet:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR)

install: build
	cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)
	@echo "Installed to /usr/local/bin/$(BINARY)"

uninstall:
	rm -f /usr/local/bin/$(BINARY)
