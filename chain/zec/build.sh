#!/bin/bash
set -euo pipefail

clear

echo "--- Starting Build Process ---"

# --- Configuration ---
ROOT_LIB=$(realpath ../../lib)
DELETE_TARGET="${DELETE_TARGET:-false}"
BUILD_TARGET="${BUILD_TARGET:-all}" # Options: all, go, node

# --- Helper Functions ---
print_usage() {
  echo "Usage: $0 [options]"
  echo "Options:"
  echo "  -t, --target <target>  Build target: all, go, node (default: all)"
  echo "  -d, --delete-target    Delete target directory after build"
  echo "  -h, --help            Show this help message"
  echo ""
  echo "Examples:"
  echo "  $0                    # Build all targets"
  echo "  $0 --target go        # Build only Go bindings"
  echo "  $0 --target node      # Build only Node.js bindings"
  echo "  $0 -t all -d          # Build all and delete target dir"
}

# --- Parse Arguments ---
while [[ $# -gt 0 ]]; do
  case $1 in
  -t | --target)
    BUILD_TARGET="$2"
    shift 2
    ;;
  -d | --delete-target)
    DELETE_TARGET="true"
    shift
    ;;
  -h | --help)
    print_usage
    exit 0
    ;;
  *)
    echo "Unknown option: $1"
    print_usage
    exit 1
    ;;
  esac
done

# --- Detect Platform ---
KERNEL_NAME=$(uname)
if [ "$KERNEL_NAME" = "Linux" ]; then
  LIBZEC_FILE="libzec.so"
  NODE_MODULE_EXT="linux-x64-gnu.node"
elif [ "$KERNEL_NAME" = "Darwin" ]; then
  LIBZEC_FILE="libzec.dylib"
  NODE_MODULE_EXT="darwin-x64.node"
else
  echo "ERROR: Unsupported kernel: $KERNEL_NAME"
  exit 1
fi

echo "INFO: Building for platform: $KERNEL_NAME"
echo "INFO: Build target: $BUILD_TARGET"

# --- Build Go Bindings ---
build_go() {
  echo ""
  echo "=== Building Go Bindings ==="

  # Check for uniffi-bindgen-go
  echo "INFO: Checking for uniffi-bindgen-go..."
  if ! command -v uniffi-bindgen-go &>/dev/null; then
    if [[ -d "$HOME/.cargo/bin" && ":$PATH:" != *":$HOME/.cargo/bin:"* ]]; then
      export PATH="$HOME/.cargo/bin:$PATH"
    fi
    if ! command -v uniffi-bindgen-go &>/dev/null; then
      echo "ERROR: uniffi-bindgen-go command not found." >&2
      echo "       Install with: cargo install uniffi-bindgen-go"
      exit 1
    fi
  fi
  echo "INFO: Found uniffi-bindgen-go."

  # Build Rust library with UniFFI feature
  echo "INFO: Building Rust library for Go (cargo build --release --no-default-features --features uniffi)..."
  if ! cargo build --release --no-default-features --features uniffi; then
    echo "ERROR: Rust build failed." >&2
    exit 1
  fi

  if [ ! -f "target/release/${LIBZEC_FILE}" ]; then
    echo "ERROR: Compiled Rust library 'target/release/${LIBZEC_FILE}' not found." >&2
    exit 1
  fi
  echo "INFO: Rust library built successfully."

  # Copy library to lib directory
  echo "INFO: Copying library to ${ROOT_LIB}/"
  rm -f "${ROOT_LIB}/${LIBZEC_FILE}"
  cp "target/release/${LIBZEC_FILE}" "${ROOT_LIB}/" || {
    echo "ERROR: Failed to copy library." >&2
    exit 1
  }

  # Generate Go bindings
  mkdir -p go/zec
  echo "INFO: Cleaning previous Go bindings..."
  rm -f go/zec/zec.go go/zec/*.c go/zec/*.h

  echo "INFO: Generating Go bindings..."
  if ! uniffi-bindgen-go "src/interface.udl" --out-dir go; then
    echo "ERROR: uniffi-bindgen-go failed." >&2
    exit 1
  fi

  if [ ! -f go/zec/zec.go ]; then
    echo "ERROR: Go binding generation failed." >&2
    exit 1
  fi
  echo "INFO: Go bindings generated successfully."

  # Run Go tests
  echo "INFO: Running Go tests..."
  LD_LIBRARY_PATH="${ROOT_LIB}" CGO_LDFLAGS="-L${ROOT_LIB} -lzec" go test -v ./go/zec/...
}

# --- Build Node.js Bindings ---
build_node() {
  echo ""
  echo "=== Building Node.js NAPI Bindings ==="

  # Check for Node.js
  if ! command -v node &>/dev/null; then
    echo "ERROR: Node.js is not installed." >&2
    exit 1
  fi
  echo "INFO: Found Node.js $(node --version)"

  # Build Rust library with NAPI feature
  echo "INFO: Building Rust library for Node.js (cargo build --release --no-default-features --features napi)..."
  if ! cargo build --release --no-default-features --features napi; then
    echo "ERROR: Rust NAPI build failed." >&2
    exit 1
  fi

  if [ ! -f "target/release/${LIBZEC_FILE}" ]; then
    echo "ERROR: Compiled NAPI library 'target/release/${LIBZEC_FILE}' not found." >&2
    exit 1
  fi
  echo "INFO: NAPI library built successfully."

  # Copy library as .node module
  echo "INFO: Copying NAPI module to ts/zec.${NODE_MODULE_EXT}"
  mkdir -p ts
  rm -f "ts/zec.${NODE_MODULE_EXT}"
  cp "target/release/${LIBZEC_FILE}" "ts/zec.${NODE_MODULE_EXT}" || {
    echo "ERROR: Failed to copy NAPI module." >&2
    exit 1
  }

  echo "INFO: Node.js NAPI bindings built successfully."

  # Run Node.js tests if package.json exists
  if [ -f "ts/package.json" ]; then
    echo "INFO: Running Node.js tests..."
    cd ts && npm test && cd ..
  else
    echo "INFO: No ts/package.json found, skipping Node.js tests."
  fi
}

# --- Main Build Logic ---
case $BUILD_TARGET in
all)
  build_go
  build_node
  ;;
go)
  build_go
  ;;
node)
  build_node
  ;;
*)
  echo "ERROR: Invalid build target: $BUILD_TARGET"
  print_usage
  exit 1
  ;;
esac

# --- Optionally Delete Target Folder ---
if [ "$DELETE_TARGET" = "true" ]; then
  echo ""
  echo "INFO: Deleting Rust target directory..."
  rm -rf target
else
  echo ""
  echo "INFO: Keeping Rust target directory."
fi

echo ""
echo "=== Build Complete ==="
echo "Build artifacts:"
if [[ $BUILD_TARGET == "all" || $BUILD_TARGET == "go" ]]; then
  echo "  - Go library: ${ROOT_LIB}/${LIBZEC_FILE}"
  echo "  - Go bindings: go/zec/zec.go"
fi
if [[ $BUILD_TARGET == "all" || $BUILD_TARGET == "node" ]]; then
  echo "  - Node.js module: ts/zec.${NODE_MODULE_EXT}"
fi

exit 0
