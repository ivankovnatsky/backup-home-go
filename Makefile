.PHONY: build clean test lint run preview install fmt fmt-check build-all deps dist all dry-run run-verbose version bump-version release re-release release-dry-run

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
RCLONE_PATH ?= Machines/$(shell hostname)/$(shell basename $(shell dirname $(HOME)))/$(shell whoami)

# Get version from flake.nix
VERSION ?= $(shell grep 'version = ' flake.nix | sed 's/.*version = "\(.*\)".*/\1/')

# If there's a GitHub release, use it; otherwise use VERSION from flake.nix
LATEST_RELEASE := $(shell gh release list -L 1 | awk '{print $$1}' | sed 's/v//' )
ifeq ($(strip $(LATEST_RELEASE)),)
    LATEST_RELEASE := $(VERSION)
endif

# Split version and increment patch
MAJOR := $(shell echo "$(LATEST_RELEASE)" | cut -d. -f1)
MINOR := $(shell echo "$(LATEST_RELEASE)" | cut -d. -f2)
PATCH := $(shell echo "$(LATEST_RELEASE)" | cut -d. -f3)
NEXT_RELEASE_VERSION := $(MAJOR).$(MINOR).$(shell echo $$(($(PATCH) + 1)))

# Add these variables at the top with other variables
GIT_COMMIT = $(shell git rev-parse --short HEAD)
BUILD_TIME = $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS = -ldflags="-s -w -X main.version=${VERSION} -X main.gitCommit=${GIT_COMMIT} -X main.buildTime=${BUILD_TIME}"

# Default target
all: clean dist build

# Build the project
build:
	$(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME) $(MAIN_PATH)

# Build with optimizations
build-release:
	$(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME) $(MAIN_PATH)

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

# Build for Unix platforms
build-all:
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)_darwin_arm64 $(MAIN_PATH)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)_linux_amd64 $(MAIN_PATH)

build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)_linux_amd64 $(MAIN_PATH)

build-windows:
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)_windows_amd64.exe $(MAIN_PATH)

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

# Dry run to see what would be backed up
dry-run: build
	./dist/$(BINARY_NAME) \
		--source $(HOME) \
		--destination "$(RCLONE_REMOTE):$(RCLONE_PATH)" \
		--preview \
		--compression 6

# Run the program with all CLI options
run: build
	./dist/$(BINARY_NAME) \
		--source $(HOME) \
		--destination "$(RCLONE_REMOTE):$(RCLONE_PATH)"

# Run with verbose output
run-verbose: build
	./dist/$(BINARY_NAME) \
		--source $(HOME) \
		--destination "$(RCLONE_REMOTE):$(RCLONE_PATH)" \
		--compression 6 \
		-v

# Add new targets
version:
	@echo "Version: ${VERSION}"
	@echo "Git commit: ${GIT_COMMIT}"
	@echo "Build time: ${BUILD_TIME}"

bump-version:
	@if [ "$(NEW_VERSION)" = "" ]; then \
		CURRENT_MAJOR=$$(echo "${VERSION}" | cut -d. -f1); \
		CURRENT_MINOR=$$(echo "${VERSION}" | cut -d. -f2); \
		CURRENT_PATCH=$$(echo "${VERSION}" | cut -d. -f3); \
		NEW_PATCH=$$((CURRENT_PATCH + 1)); \
		NEW_VERSION="$$CURRENT_MAJOR.$$CURRENT_MINOR.$$NEW_PATCH"; \
		echo "Auto-incrementing patch version from ${VERSION} to $$NEW_VERSION"; \
		make bump-version NEW_VERSION=$$NEW_VERSION; \
	else \
		echo "Bumping version from ${VERSION} to $(NEW_VERSION)"; \
		sed -i.bak 's/version = "${VERSION}"/version = "${NEW_VERSION}"/' flake.nix; \
		rm -f flake.nix.bak; \
		git add flake.nix; \
		git commit -m "build: bump version to ${NEW_VERSION}"; \
		git tag -a "v${NEW_VERSION}" -m "Version ${NEW_VERSION}"; \
	fi

release: bump-version
	@echo "Creating release v${VERSION}"
	@git push --follow-tags
	@gh release create "v${VERSION}" --generate-notes

re-release:
	@echo "Re-creating release v${VERSION}"
	@gh release delete "v${VERSION}" --cleanup-tag --yes || true
	@git push --follow-tags
	@gh release create "v${VERSION}" --generate-notes

release-dry-run:
	@echo "=== Release Dry Run ==="
	@echo "Current version (flake.nix): ${VERSION}"
	@echo "Latest GitHub release: ${LATEST_RELEASE}"
	@echo "Next release version: ${NEXT_RELEASE_VERSION}"
	@echo "\nThis would:"
	@echo "1. Bump version in flake.nix from ${VERSION} to ${NEXT_RELEASE_VERSION}"
	@echo "2. Create git commit with message: 'build: bump version to ${NEXT_RELEASE_VERSION}'"
	@echo "3. Create git tag: v${NEXT_RELEASE_VERSION}"
	@echo "4. Push changes and tags to GitHub"
	@echo "5. Create GitHub release v${NEXT_RELEASE_VERSION} with auto-generated notes"
	@echo "\nTo proceed with the actual release, run: make release"
