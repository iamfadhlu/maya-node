# Maya ZEC Lib

This library provides the necessary functionality for Bifrost to interact with the Zcash blockchain. It uses Rust for core Zcash operations (via zcash_primitives and related crates) and exposes a Go API via UniFFI bindings.

## Architecture

This is a hybrid Rust/Go implementation:

- **Go Frontend**: Implements the `UTXOClient` interface to integrate with Bifrost
- **Rust Backend**: Handles core Zcash transaction building and signing logic
- **UniFFI Bridge**: Provides cross-language interoperability between Go and Rust

## Features

- Support for transparent Zcash addresses
- Transaction building and signing with proper Zcash consensus rules
- ZIP-317 fee calculation
- Output Viewing Key (OVK) generation for transaction verification
- Compatible with Maya Protocol's Threshold Signature Scheme (TSS)

## Current Limitations

- Only transparent Zcash addresses are currently supported
- Shielded transactions (Sapling/Orchard) are not yet supported, though some code infrastructure exists

## Requirements

### Common Requirements

1. Install [cargo/rust](https://doc.rust-lang.org/cargo/getting-started/installation.html)

### For Go Bindings

1. Install [go-1.22.1](https://go.dev/doc/manage-install) or later
2. Install [`uniffi-bindgen-go`](https://github.com/NordSecurity/uniffi-bindgen-go?tab=readme-ov-file#how-to-install)

```bash
cargo install uniffi-bindgen-go --git https://github.com/NordSecurity/uniffi-bindgen-go --tag v0.2.0+v0.25.0
```

### For Node.js Bindings

1. Install [Node.js](https://nodejs.org/) (v18 or later recommended)
2. Optionally install [cargo-napi](https://github.com/napi-rs/napi-rs) for advanced builds:

```bash
npm install -g @napi-rs/cli
```

## Building

The library supports multiple build targets and can be built using either Make or the build script.

### Using Make (Recommended)

```bash
# Build all targets (Go and Node.js)
make all

# Build only Go bindings
make go

# Build only Node.js bindings
make node

# Clean build artifacts
make clean

# Run tests
make test

# See all available targets
make help
```

### Using build.sh

```bash
# Build all targets
./build.sh

# Build only Go bindings
./build.sh --target go

# Build only Node.js bindings
./build.sh --target node

# Build all and delete target directory
./build.sh --target all --delete-target
```

Both methods will:

- Compile the Rust library with appropriate features
- Generate language-specific bindings
- Copy artifacts to the correct locations
- Run basic tests to verify the build

## Key Components

- **interface.udl**: UniFFI interface definition
- **tx.rs**: Zcash transaction handling logic
- **addr.rs**: Address validation and encoding
- **network.rs**: Network parameter definitions
- **error.rs**: Error type definitions
- **config.rs**: Configuration for provers and loggers

## Language Bindings

This library provides bindings for multiple languages:

### Go Bindings

The primary integration with Bifrost uses Go bindings generated via UniFFI.

### TypeScript/Node.js Bindings

TypeScript bindings are available via NAPI-RS for Node.js applications.

#### Installation

```bash
cd ts/
npm install
npm run build
```

#### Usage

```typescript
import {
  initZecLib,
  validateZecAddress,
  buildPartialTx,
  Network,
} from "@mayaprotocol/zec-napi";

// Initialize library
initZecLib();

// Validate address
validateZecAddress("tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU", Network.Test);
```

## End-to-End Testing

The TypeScript bindings include comprehensive E2E tests that run against a real Zcash regtest network.

### Quick Start E2E Testing

```bash
cd ts/

# Install dependencies
npm install

# Build bindings
npm run build

# Start regtest network
npm run regtest:up

# Wait for setup (check logs)
npm run regtest:logs

# Run E2E tests
npm run test:e2e

# Cleanup
npm run regtest:down
```

### E2E Test Coverage

The E2E tests validate:

- ✅ **Network Connectivity** - Connect to Zcash regtest node
- ✅ **Address Management** - Validate addresses, check balances, fetch UTXOs
- ✅ **Transaction Building** - Create transactions from real UTXO data
- ✅ **NAPI Integration** - Rust bindings work with real blockchain data
- ✅ **Error Handling** - Proper error responses for invalid operations
- ✅ **Blockchain Operations** - Generate blocks, fetch transaction details

### Test Environment

The E2E tests use Docker to run a Zcash regtest node with:

- Pre-funded test addresses
- RPC interface on `localhost:18232`
- Automated setup with 150+ blocks
- Test transactions with real UTXOs

For detailed E2E testing instructions, see [`ts/E2E_TESTING.md`](ts/E2E_TESTING.md).

### Development Workflow

```bash
# Unit tests (fast, mocked)
cd ts/ && npm test -- src/__tests__/mock.test.ts

# Integration tests
cd ts/ && npm test -- --testPathIgnorePatterns=regtest-e2e

# Full E2E validation
cd ts/ && npm run regtest:up && npm run test:e2e && npm run regtest:down
```
