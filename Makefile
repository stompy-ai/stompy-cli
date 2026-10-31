VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/stompy-ai/stompy-cli/cmd.Version=$(VERSION)"
BINARY := stompy

.PHONY: build install test test-coverage lint build-all clean

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/stompy

install:
	go install $(LDFLAGS) ./cmd/stompy

test:
	go test ./... -v -count=1

test-coverage:
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run ./...

build-all:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-arm64 ./cmd/stompy
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-amd64 ./cmd/stompy
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64 ./cmd/stompy
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-arm64 ./cmd/stompy
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-windows-amd64.exe ./cmd/stompy

clean:
	rm -rf bin/ coverage.out coverage.html
