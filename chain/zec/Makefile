# Makefile for building Zcash bindings for multiple languages
# Supports: Go (via UniFFI) and Node.js (via NAPI)

# Variables
CARGO = cargo
CARGO_NAPI = cargo-napi
TARGET = x86_64-unknown-linux-gnu
BUILD_DIR = target/$(TARGET)/release
NODE_MODULE_NAME = zec

# Detect OS for library extensions
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
    LIB_EXT = so
    NODE_EXT = linux-x64-gnu.node
endif
ifeq ($(UNAME_S),Darwin)
    LIB_EXT = dylib
    NODE_EXT = darwin-x64.node
endif

# Default target
.PHONY: all
all: go node

# Build for Go (UniFFI)
.PHONY: go
go: check-deps
	@echo "Building Go bindings..."
	$(CARGO) build --no-default-features --features uniffi --release --target $(TARGET)
	@echo "Copying library for Go..."
	cp $(BUILD_DIR)/libzec.$(LIB_EXT) ../../lib/libzec.$(LIB_EXT)
	@echo "Go bindings built successfully!"

# Build for Node.js (NAPI)
.PHONY: node
node: check-node-deps
	@echo "Building Node.js NAPI bindings..."
	$(CARGO) build --no-default-features --features napi --release --target $(TARGET)
	@echo "Copying NAPI module..."
	cp $(BUILD_DIR)/libzec.$(LIB_EXT) ts/$(NODE_MODULE_NAME).$(NODE_EXT)
	@echo "Node.js bindings built successfully!"

# Build for multiple Node.js platforms
.PHONY: node-all
node-all: node-linux node-macos node-windows
	@echo "All Node.js platform bindings built successfully!"

.PHONY: node-linux
node-linux: check-node-deps
	@echo "Building Node.js NAPI bindings for Linux x64..."
	$(CARGO) build --no-default-features --features napi --release --target x86_64-unknown-linux-gnu
	cp target/x86_64-unknown-linux-gnu/release/libzec.so ts/$(NODE_MODULE_NAME).linux-x64-gnu.node
	@echo "Linux x64 bindings built!"

.PHONY: node-macos
node-macos: check-node-deps
	@echo "Building Node.js NAPI bindings for macOS x64..."
	@if [ "$$(uname)" = "Darwin" ]; then \
		rustup target add x86_64-apple-darwin || true; \
		$(CARGO) build --no-default-features --features napi --release --target x86_64-apple-darwin; \
		cp target/x86_64-apple-darwin/release/libzec.dylib ts/$(NODE_MODULE_NAME).darwin-x64.node; \
		echo "macOS x64 bindings built!"; \
	else \
		echo "Error: macOS binaries must be built on macOS due to cross-compilation limitations."; \
		echo "Run this command on a macOS system or use CI/CD."; \
		exit 1; \
	fi

.PHONY: node-macos-arm
node-macos-arm: check-node-deps
	@echo "Building Node.js NAPI bindings for macOS ARM64..."
	@if [ "$$(uname)" = "Darwin" ]; then \
		rustup target add aarch64-apple-darwin || true; \
		$(CARGO) build --no-default-features --features napi --release --target aarch64-apple-darwin; \
		cp target/aarch64-apple-darwin/release/libzec.dylib ts/$(NODE_MODULE_NAME).darwin-arm64.node; \
		echo "macOS ARM64 bindings built!"; \
	else \
		echo "Error: macOS binaries must be built on macOS due to cross-compilation limitations."; \
		echo "Run this command on a macOS system or use CI/CD."; \
		exit 1; \
	fi

.PHONY: node-windows
node-windows: check-node-deps
	@echo "Building Node.js NAPI bindings for Windows x64..."
	@if [ "$$(uname -o 2>/dev/null)" = "Msys" ] || [ "$$(uname -s)" = "MINGW64_NT"* ] || command -v cmd.exe >/dev/null 2>&1; then \
		rustup target add x86_64-pc-windows-msvc || true; \
		$(CARGO) build --no-default-features --features napi --release --target x86_64-pc-windows-msvc; \
		cp target/x86_64-pc-windows-msvc/release/zec.dll ts/$(NODE_MODULE_NAME).win32-x64-msvc.node; \
		echo "Windows x64 bindings built!"; \
	else \
		echo "Error: Windows binaries must be built on Windows due to cross-compilation limitations."; \
		echo "Run this command on a Windows system or use CI/CD."; \
		exit 1; \
	fi

# Build using cargo-napi (alternative method for Node.js)
.PHONY: node-napi
node-napi: check-cargo-napi
	@echo "Building Node.js NAPI bindings with cargo-napi..."
	cd ts && $(CARGO_NAPI) build --platform --release
	@echo "Node.js bindings built successfully with cargo-napi!"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	$(CARGO) clean
	rm -f ../../lib/libzec.$(LIB_EXT)
	rm -f ts/$(NODE_MODULE_NAME).$(NODE_EXT)
	rm -rf ts/target
	@echo "Clean complete!"

# Check dependencies
.PHONY: check-deps
check-deps:
	@which $(CARGO) >/dev/null 2>&1 || { echo "Error: cargo is not installed"; exit 1; }
	@echo "Dependencies checked!"

# Check Node.js specific dependencies
.PHONY: check-node-deps
check-node-deps: check-deps
	@echo "Checking Node.js dependencies..."
	@node --version >/dev/null 2>&1 || { echo "Error: Node.js is not installed"; exit 1; }

# Check cargo-napi installation
.PHONY: check-cargo-napi
check-cargo-napi: check-node-deps
	@which $(CARGO_NAPI) >/dev/null 2>&1 || { echo "Error: cargo-napi is not installed. Run: npm install -g @napi-rs/cli"; exit 1; }

# Install development dependencies
.PHONY: install-deps
install-deps:
	@echo "Installing development dependencies..."
	npm install -g @napi-rs/cli
	@echo "Dependencies installed!"

# Run tests
.PHONY: test
test: test-go test-node

.PHONY: test-go
test-go:
	@echo "Running Go tests..."
	cd go && ./test.sh

.PHONY: test-node
test-node: node
	@echo "Running Node.js tests..."
	cd ts && npm test

# Development builds (without optimization)
.PHONY: dev-go
dev-go: check-deps
	@echo "Building Go bindings (debug)..."
	$(CARGO) build --features uniffi --target $(TARGET)
	cp target/$(TARGET)/debug/libzec.$(LIB_EXT) ../../lib/libzec.$(LIB_EXT)

.PHONY: dev-node
dev-node: check-node-deps
	@echo "Building Node.js NAPI bindings (debug)..."
	$(CARGO) build --features napi --target $(TARGET)
	cp target/$(TARGET)/debug/libzec.$(LIB_EXT) ts/$(NODE_MODULE_NAME).$(NODE_EXT)

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all          - Build both Go and Node.js bindings (default)"
	@echo "  go           - Build Go bindings only"
	@echo "  node         - Build Node.js NAPI bindings"
	@echo "  node-napi    - Build Node.js bindings using cargo-napi"
	@echo "  clean        - Remove all build artifacts"
	@echo "  test         - Run all tests"
	@echo "  test-go      - Run Go tests only"
	@echo "  test-node    - Run Node.js tests only"
	@echo "  dev-go       - Build Go bindings in debug mode"
	@echo "  dev-node     - Build Node.js bindings in debug mode"
	@echo "  install-deps - Install development dependencies"
	@echo "  help         - Show this help message"
	@echo "  node-all     - Build Node.js bindings for all platforms"
	@echo "  node-linux   - Build Node.js bindings for Linux x64"
	@echo "  node-macos   - Build Node.js bindings for macOS x64"
	@echo "  node-macos-arm - Build Node.js bindings for macOS ARM64"
	@echo "  node-windows - Build Node.js bindings for Windows x64"