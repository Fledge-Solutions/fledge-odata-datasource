# Makefile for Grafana OData Plugin
# This ensures the plugin is built with the correct binary name and location

# Plugin configuration (must match plugin.json)
PLUGIN_NAME = gpx_odata-datasource
PLUGIN_DIR = /opt/homebrew/var/lib/grafana/plugins/fledge-odata-datasource

# Binary names for different platforms
BINARY_DARWIN_ARM64 = $(PLUGIN_NAME)_darwin_arm64
BINARY_DARWIN_AMD64 = $(PLUGIN_NAME)_darwin_amd64
BINARY_LINUX_AMD64 = $(PLUGIN_NAME)_linux_amd64
BINARY_LINUX_ARM64 = $(PLUGIN_NAME)_linux_arm64
BINARY_WINDOWS_AMD64 = $(PLUGIN_NAME)_windows_amd64.exe

# Detect current platform
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Darwin)
	ifeq ($(UNAME_M),arm64)
		CURRENT_BINARY = $(BINARY_DARWIN_ARM64)
	else
		CURRENT_BINARY = $(BINARY_DARWIN_AMD64)
	endif
else ifeq ($(UNAME_S),Linux)
	ifeq ($(UNAME_M),aarch64)
		CURRENT_BINARY = $(BINARY_LINUX_ARM64)
	else
		CURRENT_BINARY = $(BINARY_LINUX_AMD64)
	endif
endif

.PHONY: all build-frontend build-backend build install dev clean help restart-grafana package

# Default target
all: build

# Build everything (frontend + backend)
build: build-frontend build-backend

# Build only the frontend
build-frontend:
	@echo "Building frontend..."
	npm run build

# Build only the backend for current platform
build-backend:
	@echo "Building backend for $(CURRENT_BINARY)..."
	go build -o ./dist/$(CURRENT_BINARY) ./pkg

# Build backend for all platforms
build-backend-all:
	@echo "Building backend for all platforms..."
	GOOS=darwin GOARCH=arm64 go build -o ./dist/$(BINARY_DARWIN_ARM64) ./pkg
	GOOS=darwin GOARCH=amd64 go build -o ./dist/$(BINARY_DARWIN_AMD64) ./pkg
	GOOS=linux GOARCH=amd64 go build -o ./dist/$(BINARY_LINUX_AMD64) ./pkg
	GOOS=linux GOARCH=arm64 go build -o ./dist/$(BINARY_LINUX_ARM64) ./pkg
	GOOS=windows GOARCH=amd64 go build -o ./dist/$(BINARY_WINDOWS_AMD64) ./pkg

# Install the plugin to Grafana's plugin directory
install: build
	@echo "Installing plugin to $(PLUGIN_DIR)..."
	@mkdir -p $(PLUGIN_DIR)/dist
	@cp -r dist/* $(PLUGIN_DIR)/dist/
	@echo "Plugin installed successfully!"

# Development build and install with restart
dev: build install restart-grafana
	@echo "Development build complete and Grafana restarted!"

# Restart Grafana (macOS with Homebrew)
restart-grafana:
	@echo "Restarting Grafana..."
	brew services restart grafana
	@sleep 3
	@echo "Grafana restarted!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf dist/
	rm -rf coverage/
	rm -f fledge-odata-datasource-*.zip fledge-odata-datasource-*.zip.sha1
	@echo "Clean complete!"

# Package the plugin for distribution
package:
	@echo "Packaging plugin..."
	./package-plugin.sh

# Show help
help:
	@echo "Grafana Fledge OData Plugin - Build Commands"
	@echo "============================================="
	@echo "make build           - Build both frontend and backend"
	@echo "make build-frontend  - Build only the frontend"
	@echo "make build-backend   - Build only the backend for current platform"
	@echo "make build-backend-all - Build backend for all platforms"
	@echo "make package         - Build and package plugin for distribution"
	@echo "make install         - Build and install plugin to Grafana"
	@echo "make dev             - Build, install, and restart Grafana"
	@echo "make restart-grafana - Restart Grafana service"
	@echo "make clean           - Remove build artifacts and packages"
	@echo "make help            - Show this help message"
	@echo ""
	@echo "Current platform: $(CURRENT_BINARY)"
	@echo "Plugin directory: $(PLUGIN_DIR)"
