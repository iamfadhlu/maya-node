# @mayaprotocol/zcash-ts

Low-level Zcash library for TypeScript/JavaScript, designed for integration with xchainjs-lib and other blockchain libraries.

## Overview

This package provides a comprehensive set of Zcash primitives and utilities, leveraging Rust's librustzcash through NAPI bindings for optimal performance and security. It's designed as a low-level library that higher-level packages like `@xchainjs/xchain-zcash` can build upon.

### TypeScript Support

This library includes comprehensive TypeScript declarations for all functions and types, including:

- Full type definitions for the NAPI module interface
- Typed transaction structures (`NapiPartialTx`, `BuiltPartialTx`)
- Network enums and validation
- Extended transaction data documentation

### Transaction Expiry

Zcash transactions support expiry heights to prevent transactions from being included in blocks after a certain point. This library provides full control over transaction expiry:

- **`blockHeight`** - The current blockchain height where the transaction is being created
- **`expiryHeight`** - The block height after which the transaction expires and cannot be mined
  - `0` = Transaction never expires
  - `> blockHeight` = Transaction expires at that specific block height
  - Default: `blockHeight + 40` (standard 40-block expiry window)

```typescript
// Transaction that never expires
const neverExpiresTx = buildTransaction({
  inputs,
  outputs,
  network: "mainnet",
  blockHeight: 1000000,
  pubkey,
  expiryHeight: 0, // Never expires
});

// Transaction that expires in 100 blocks
const customExpiryTx = buildTransaction({
  inputs,
  outputs,
  network: "mainnet",
  blockHeight: 1000000,
  pubkey,
  expiryHeight: 1000100, // Expires at block 1000100
});
```

## Installation

```bash
npm install @mayaprotocol/zcash-ts
```

### Platform Support

This package includes pre-built native binaries for:

- Linux x64 (`zec.linux-x64-gnu.node`)
- macOS x64 (`zec.darwin-x64.node`) - _Build on macOS required_
- macOS ARM64 (`zec.darwin-arm64.node`) - _Build on macOS required_
- Windows x64 (`zec.win32-x64-msvc.node`) - _Build on Windows required_

#### Building Native Binaries

To build the native NAPI binaries:

```bash
# Build for current platform
npm run build:native

# Build for specific platforms (requires appropriate toolchain)
npm run build:native:linux    # Linux x64
npm run build:native:macos    # macOS x64 (requires macOS)
npm run build:native:macos-arm # macOS ARM64 (requires macOS)
npm run build:native:windows  # Windows x64 (requires Windows)

# Attempt to build for all platforms (cross-compilation limited)
npm run build:native:all
```

**Note**: Cross-compilation from Linux to macOS/Windows requires additional toolchain setup and may not work for all dependencies (like secp256k1). For production builds, use platform-specific build environments.

## Complete API Reference

### Core Functions

#### Address Management

```typescript
// Validate a Zcash address for a specific network
validateAddress(address: string, network: string): boolean

// Generate address from private key (WIF or hex format)
addressFromPrivateKey(privateKey: string, network: string): string

// Generate address from public key buffer
addressFromPublicKey(publicKey: Buffer, network: string): string
```

#### Transaction Building

```typescript
// Build an unsigned transaction with NAPI bindings support
buildTransaction(params: {
  inputs: UTXO[]
  outputs: TxOutput[]
  network: string
  blockHeight: number
  pubkey: Buffer        // Required for NAPI features
  expiryHeight?: number // Block height after which tx expires (0 = never expires, default = blockHeight + 40)
}): UnsignedTransaction

// Sign a transaction with provided signatures
signTransaction(
  unsignedTx: UnsignedTransaction,
  signatures: Buffer[],
  network: string,
  vaultPubkey?: Buffer
): Buffer

// Calculate transaction ID from raw transaction bytes
calculateTransactionId(transactionBytes: Buffer): string
```

#### UTXO Selection (like coinselect)

```typescript
// Select UTXOs using accumulative strategy (default)
selectUTXOs(params: {
  utxos: UTXO[]
  targets: TxOutput[]
  feeRate?: number
  changeAddress: string
}): UTXOSelection | null

// Select UTXOs preferring exact matches (blackjack strategy)
selectUTXOsBlackjack(params: {
  utxos: UTXO[]
  targets: TxOutput[]
  changeAddress: string
}): UTXOSelection | null

// Select UTXOs minimizing change (branch & bound)
selectUTXOsBnB(params: {
  utxos: UTXO[]
  targets: TxOutput[]
  changeAddress: string
  costOfChange?: number
}): UTXOSelection | null
```

#### Fee Calculation

```typescript
// Calculate transaction fee
calculateFee(
  inputCount: number,
  outputCount: number,
  memoLength?: number
): number

// Estimate transaction size in bytes
estimateTransactionSize(
  inputCount: number,
  outputCount: number,
  memoLength?: number
): number
```

#### Script Generation

```typescript
// Create P2PKH script from address
createPayToAddressScript(address: string): Buffer

// Create OP_RETURN script from memo
createMemoScript(memo: string): Buffer
```

#### NAPI-Specific Functions

```typescript
// Initialize Zcash library (required before using NAPI functions)
initZecLib(): void

// Get output viewing key from vault public key
getOutputViewingKey(vaultPubkey: Buffer): Buffer
```

### Types

```typescript
// Network options
export enum Network {
  Mainnet = "mainnet",
  Testnet = "testnet",
  Regtest = "regtest",
}

// UTXO for transaction inputs
export interface UTXO {
  txid: string;
  vout: number;
  value: number; // zatoshis
  height: number;
  script?: string; // hex-encoded, optional for compatibility
}

// Transaction output
export interface TxOutput {
  address: string;
  value: number; // zatoshis
  memo?: string; // optional OP_RETURN data
}

// Unsigned transaction
export interface UnsignedTransaction {
  version: number; // 5 for current Zcash
  inputs: UTXO[]; // with required scripts
  outputs: TxOutput[];
  lockTime: number;
  sighashes: Buffer[];
}

// UTXO selection result
export interface UTXOSelection {
  inputs: UTXO[]; // selected inputs
  outputs: TxOutput[]; // includes change output if needed
  fee: number; // calculated fee
}

// RPC configuration
export interface Config {
  host: string;
  port: number;
  username: string;
  password: string;
  network: Network | string;
}
```

### NAPI Types

The library uses typed NAPI bindings for communication with the Rust backend:

```typescript
// NAPI network enum (matches Rust implementation)
export enum NapiNetwork {
  Main = "Main",
  Test = "Test",
  Regtest = "Regtest",
}

// NAPI transaction structure passed to/from Rust
export interface NapiPartialTx {
  height: number; // Block height at which transaction is being created
  txid: Buffer; // 32-byte transaction ID
  inputs: NapiUTXO[];
  outputs: NapiOutput[];
  fee: number;
  sighashes: Buffer[]; // Sighashes for each input
  expiryHeight: number; // Block height after which tx expires (0 = never expires)
}

// NAPI UTXO structure
export interface NapiUTXO {
  txid: string; // Hex-encoded transaction ID
  vout: number; // Output index
  value: number; // Amount in zatoshis
  script: string; // Hex-encoded script
}

// NAPI output structure
export interface NapiOutput {
  address: string;
  amount: number; // Amount in zatoshis
  memo: string; // Empty string if no memo
}

// Structure returned by buildPartialTx
export interface BuiltPartialTx extends NapiPartialTx {
  txid: Buffer; // Populated transaction ID
  sighashes: Buffer[]; // Calculated sighashes for signing
  fee: number; // Actual fee (may differ from requested)
}
```

### Extended Transaction Data

The Rust implementation has access to additional transaction fields through `TransactionData<Unauthorized>`:

```typescript
// Additional data available in Rust but not exposed via NAPI
export interface ExtendedTransactionData {
  version: number; // Full version with Overwinter flag
  consensusBranchId: number; // Network upgrade identifier
  lockTime: number; // Transaction lock time
  expiryHeight: number; // Block height when tx expires
  transparentBundle?: {
    inputs: Array<{
      prevOut: { txid: string; vout: number };
      scriptSig: Buffer; // Full unlocking script
      sequence: number; // For relative lock time
    }>;
    outputs: Array<{
      value: number;
      scriptPubKey: Buffer; // Full locking script
    }>;
  };
  // Shielded components (not used in transparent-only implementation)
  saplingBundle?: any;
  orchardBundle?: any;
}
```

To access this extended data, the NAPI bindings would need to be modified to expose these additional fields.

### Additional Utilities

#### RPC Client

```typescript
// Full-featured Zcash RPC client
export class ZcashRPCClient {
  constructor(config: Config);

  // Network info
  getNetworkInfo(): Promise<any>;
  getBlockchainInfo(): Promise<any>;

  // Address operations
  validateAddress(address: string): Promise<any>;
  getBalance(address: string): Promise<string>;
  getUTXOs(address: string): Promise<UTXO[]>;

  // Transaction operations
  sendRawTransaction(txHex: string): Promise<string>;
  getRawTransaction(txid: string, verbose?: boolean): Promise<any>;
}
```

#### Error Handling

```typescript
// Custom error class for Zcash operations
export class ZecError extends Error {
  constructor(message: string);
  name: "ZecError";
}
```

### Legacy/Compatibility Exports

For backward compatibility with existing code:

```typescript
// Legacy function aliases
export const isValidAddr = validateAddress;
export const skToAddr = addressFromPrivateKey;
export const pkToAddr = addressFromPublicKey;
export const getFee = (utxos: UTXO[], outputs: Output[]) => number;
export const buildTx = (config, utxos, outputs, height) => Tx;
export const signAndFinalize = (tx, privateKey, network) => Buffer;

// Legacy network constants
export const Main = "main";
export const Test = "test";
export const Regtest = "regtest";

// Legacy types
export type Output = OutputPKH | OutputMemo;
export interface OutputPKH {
  type: "pkh";
  address: string;
  amount: number;
}
export interface OutputMemo {
  type: "memo";
  memo: string;
}
```

## Usage Examples

### Basic Address Validation

```typescript
import { validateAddress, Network } from "@mayaprotocol/zcash-ts";

const isValid = validateAddress("t1XYZ...", Network.Mainnet);
console.log("Address is valid:", isValid);
```

### Transaction Building

```typescript
import {
  buildTransaction,
  signTransaction,
  calculateTransactionId,
  selectUTXOs,
} from "@mayaprotocol/zcash-ts";

// Select UTXOs for transaction
const selection = selectUTXOs({
  utxos: availableUTXOs,
  targets: [
    { address: "tmXYZ...", value: 100000000 }, // 1 ZEC
  ],
  changeAddress: "tmABC...",
});

if (!selection) throw new Error("Insufficient funds");

// Build unsigned transaction
const unsignedTx = buildTransaction({
  inputs: selection.inputs,
  outputs: selection.outputs,
  network: "testnet",
  blockHeight: 123456,
  vaultPubkey: Buffer.from("...", "hex"), // Optional, enables NAPI features
});

// Sign transaction (signatures from external signer)
const signatures = await externalSigner.sign(unsignedTx.sighashes);
const signedTx = signTransaction(unsignedTx, signatures, "testnet");

// Get transaction ID
const txid = calculateTransactionId(signedTx);
```

### RPC Integration

```typescript
import { ZcashRPCClient, Config } from "@mayaprotocol/zcash-ts";

const config: Config = {
  host: "localhost",
  port: 8232,
  username: "user",
  password: "pass",
  network: "mainnet",
};

const client = new ZcashRPCClient(config);

// Get UTXOs for an address
const utxos = await client.getUTXOs("t1XYZ...");

// Broadcast transaction
const txid = await client.sendRawTransaction(signedTx.toString("hex"));
```

## Testing

```bash
# Run all tests
npm test

# Run specific test suites
npm run test src/__tests__/basic.test.ts

# Run E2E tests (requires Docker)
npm run regtest:up     # Start Zcash regtest node
npm run test:e2e       # Run E2E tests
npm run regtest:down   # Stop regtest node
```

## Architecture

This library provides three levels of functionality:

1. **Pure TypeScript functions** - Address validation, script generation, fee calculation
2. **NAPI bindings** - High-performance transaction building and cryptographic operations via Rust
3. **RPC client** - Direct interaction with Zcash nodes

The library automatically falls back to mock implementations when NAPI bindings are not available, ensuring compatibility across all environments.

## Platform Support

- ✅ Node.js 16+
- ✅ Linux (x64, ARM64)
- ✅ macOS (x64, ARM64)
- ✅ Windows (x64)

## License

MIT
