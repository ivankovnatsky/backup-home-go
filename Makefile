.PHONY: build clean test lint run preview install fmt fmt-check build-all deps dist all dry-run run-verbose

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=backup-home
BINARY_UNIX=$(BINARY_NAME)
MAIN_PATH=./cmd/backup-home

# Default rclone remote and path
RCLONE_REMOTE ?= drive_Crypt

# Detect OS for path construction
ifdef COMSPEC
    # Windows path (COMSPEC is set on Windows)
    RCLONE_PATH ?= Machines/$(shell hostname)/Users/$(shell echo %USERNAME%)
else
    # Unix-like systems (Linux/macOS)
    RCLONE_PATH ?= Machines/$(shell hostname)/$(shell basename $(shell dirname $(HOME)))/$(shell whoami)
endif

# Default target
all: clean dist build

# Build the project
build:
	$(GOBUILD) -o dist/$(BINARY_NAME) $(MAIN_PATH)

# Build with optimizations
build-release:
	$(GOBUILD) -ldflags="-s -w" -o dist/$(BINARY_NAME) $(MAIN_PATH)

# Clean build files
clean:
	$(GOCLEAN)
	rm -rf dist/

# Run tests
test:
	$(GOTEST) -v ./...

# Run linting
lint:
	golangci-lint run

# Format code
fmt:
	gofmt -s -w .

# Check code formatting
fmt-check:
	test -z $$(gofmt -l .)

# Build for all platforms
build-all: clean
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o dist/$(BINARY_NAME)_darwin_amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o dist/$(BINARY_NAME)_darwin_arm64 $(MAIN_PATH)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o dist/$(BINARY_NAME)_linux_amd64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o dist/$(BINARY_NAME)_windows_amd64.exe $(MAIN_PATH)

# Install dependencies
deps:
	$(GOGET) -v ./...
	go mod tidy

# Create dist directory if it doesn't exist
dist:
	mkdir -p dist

# Install release version locally
install: build-release
	cp dist/$(BINARY_NAME) $(GOPATH)/bin/

# Run all checks (format, lint, test)
check: fmt-check lint test

# Run the program with all CLI options
run: build
	./dist/$(BINARY_NAME) \
		--source $(HOME) \
		--destination "$(RCLONE_REMOTE):$(RCLONE_PATH)"

# Preview what would be done without actually doing it
preview: build
	./dist/$(BINARY_NAME) \
		--source $(HOME) \
		--destination "$(RCLONE_REMOTE):$(RCLONE_PATH)" \
		--preview 

# Dry run to see what would be backed up
dry-run: build
	./dist/$(BINARY_NAME) \
		--source $(HOME) \
		--destination "$(RCLONE_REMOTE):$(RCLONE_PATH)" \
		--preview \
		--compression 6

# Run with verbose output
run-verbose: build
	./dist/$(BINARY_NAME) \
		--source $(HOME) \
		--destination "$(RCLONE_REMOTE):$(RCLONE_PATH)" \
		--compression 6 \
		-v
