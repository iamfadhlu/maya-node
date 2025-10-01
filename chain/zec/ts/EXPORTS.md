# @mayaprotocol/zcash-ts Exports

This document provides a comprehensive list of all exports from the @mayaprotocol/zcash-ts package.

## Types

### Core Types

```typescript
// Network enumeration
export enum Network {
  Mainnet = "mainnet",
  Testnet = "testnet",
  Regtest = "regtest",
}

// UTXO interface for transaction inputs
export interface UTXO {
  txid: string;
  vout: number;
  value: number; // zatoshis
  height: number;
  script?: string; // hex-encoded script (optional)
}

// ValidUTXO ensures script is present
export interface ValidUTXO extends UTXO {
  script: string; // hex-encoded script (required)
}

// Transaction output
export interface TxOutput {
  address: string;
  value: number; // zatoshis
  memo?: string; // optional OP_RETURN data
}

// Unsigned transaction structure
export interface UnsignedTransaction {
  version: number; // 5 for current Zcash
  inputs: ValidUTXO[];
  outputs: TxOutput[];
  lockTime: number;
  sighashes: Buffer[];
}

// UTXO selection result
export interface UTXOSelection {
  inputs: ValidUTXO[];
  outputs: TxOutput[];
  fee: number;
}

// Partial transaction for NAPI interface
export interface PartialTx {
  height: number;
  txid: Buffer;
  inputs: ValidUTXO[];
  outputs: NapiOutput[];
  fee: number;
  sighashes: Buffer[];
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

### Legacy Types (for backward compatibility)

```typescript
export interface OutputPKH {
  type: "pkh";
  address: string;
  amount: number;
}

export interface OutputMemo {
  type: "memo";
  memo: string;
}

export type Output = OutputPKH | OutputMemo;

export interface Tx {
  version: number;
  inputs: UTXO[];
  outputs: Output[];
  fee: number;
}
```

## Functions

### Address Management

```typescript
// Validate a Zcash address for a specific network
export function validateAddress(address: string, network: string): boolean;

// Generate address from private key (WIF or hex format)
export function addressFromPrivateKey(
  privateKey: string,
  network: string,
): string;

// Generate address from public key buffer
export function addressFromPublicKey(
  publicKey: Buffer,
  network: string,
): string;

// Get public key from private key
export function getPublicKeyFromPrivateKey(privateKey: string): Buffer;

// Extract public key hash from address (note: this is NOT the public key)
export function getPublicKeyHashFromAddress(address: string): Buffer;
```

### Transaction Building

```typescript
// Build an unsigned transaction with NAPI bindings
export function buildTransaction(params: {
  inputs: UTXO[];
  outputs: TxOutput[];
  network: string;
  blockHeight: number;
  pubkey: Buffer; // Public key of the address that owns the UTXOs
}): UnsignedTransaction;

// Sign sighashes with a private key to create signatures
export function signSighashes(
  sighashes: Buffer[],
  privateKey: string, // WIF or hex format
): Buffer[];

// Sign a transaction with provided signatures
export function signTransaction(
  unsignedTx: UnsignedTransaction,
  signatures: Buffer[],
  network: string,
  pubkey: Buffer, // Public key of the address that owns the UTXOs
): Buffer;

// Calculate transaction ID from raw transaction bytes
export function calculateTransactionId(transactionBytes: Buffer): string;
```

### UTXO Selection

```typescript
// Select UTXOs using accumulative strategy
export function selectUTXOs(params: {
  utxos: UTXO[];
  targets: TxOutput[];
  feeRate?: number;
  changeAddress: string;
}): UTXOSelection | null;
```

### Fee Calculation

```typescript
// Calculate transaction fee
export function calculateFee(
  inputCount: number,
  outputCount: number,
  memoLength?: number,
): number;
```

### Script Generation

```typescript
// Create P2PKH script from address
export function createPayToAddressScript(address: string): Buffer;

// Create OP_RETURN script from memo
export function createMemoScript(memo: string): Buffer;
```

### NAPI-Specific Functions

```typescript
// Get output viewing key from public key
export function getOutputViewingKey(pubkey: Buffer): Buffer;
```

### Classes

```typescript
// Custom error class for Zcash operations
export class ZecError extends Error {
  constructor(message: string);
  name: "ZecError";
}

// Zcash RPC client
export class ZcashRPCClient {
  constructor(config: Config);
  // See rpc.ts for full method list
}

// Transaction builder for high-level transaction construction
export class TransactionBuilder {
  constructor(network?: string);
  addInput(utxo: UTXO): this;
  addInputs(utxos: UTXO[]): this;
  addOutput(address: string, value: number, memo?: string): this;
  setChangeAddress(address: string): this;
  setFeeRate(zatoshisPerByte: number): this;
  setDustThreshold(threshold: number): this;
  selectUTXOs(availableUTXOs: UTXO[], strategy?: "accumulative" | "all"): this;
  build(blockHeight: number, pubkey: Buffer): BuildResult;
  static sign(
    buildResult: BuildResult,
    privateKey: string,
    network: string,
    pubkey: Buffer,
  ): SignedTransaction;
  getState(): object;
  clear(): this;
  clone(): TransactionBuilder;
}

// Factory function for TransactionBuilder
export function createTransactionBuilder(network?: string): TransactionBuilder;

// Build result from TransactionBuilder
export interface BuildResult {
  unsignedTx: UnsignedTransaction;
  fee: number;
  totalInput: number;
  totalOutput: number;
  changeAmount: number;
}

// Signed transaction result
export interface SignedTransaction {
  rawTx: Buffer;
  txid: string;
  fee: number;
  totalInput: number;
  totalOutput: number;
  changeAmount: number;
}
```

## Legacy Exports (for backward compatibility)

### Function Aliases

```typescript
export const isValidAddr = validateAddress;
export const skToAddr = addressFromPrivateKey;
export const pkToAddr = addressFromPublicKey;
export const validateZecAddress = validateAddress;
export const buildPartialTx = buildTransaction;
export const computeTransactionId = calculateTransactionId;
export const applyTxSignatures = signTransaction;

// Legacy getFee function
export function getFee(utxos: UTXO[], outputs: Output[]): number;

// Legacy transaction functions
export function buildTx(
  config: Config,
  utxos: UTXO[],
  outputs: Output[],
  height: number,
): Tx;
```

### Network Constants

```typescript
export const Network = {
  Main: "main",
  Test: "test",
  Regtest: "regtest",
  Mainnet: "mainnet",
  Testnet: "testnet",
};

// Individual exports
export const Main = "main";
export const Test = "test";
export const Regtest = "regtest";
```

## Usage Examples

### Basic Address Operations

```typescript
import {
  validateAddress,
  addressFromPrivateKey,
  getPublicKeyFromPrivateKey,
  Network,
} from "@mayaprotocol/zcash-ts";

// Validate address
const isValid = validateAddress("t1XYZ...", Network.Mainnet);

// Get address from private key
const address = addressFromPrivateKey("privateKeyHex", Network.Mainnet);

// Get public key from private key
const pubkey = getPublicKeyFromPrivateKey("privateKeyHex");
```

### Transaction Building and Signing

#### Using TransactionBuilder (Recommended)

```typescript
import {
  TransactionBuilder,
  getPublicKeyFromPrivateKey,
} from "@mayaprotocol/zcash-ts";

// Get public key from your private key
const pubkey = getPublicKeyFromPrivateKey(privateKey);

// Create a transaction builder
const builder = new TransactionBuilder("testnet");

// Method 1: Manual UTXO selection
const buildResult = builder
  .addInput(utxo1)
  .addInput(utxo2)
  .addOutput("tmXYZ...", 100000000) // 1 ZEC
  .addOutput("tmABC...", 0, "Hello Zcash!") // Memo output
  .setChangeAddress("tmDEF...")
  .build(blockHeight, pubkey);

// Method 2: Automatic UTXO selection
const builder2 = new TransactionBuilder("testnet");
const buildResult2 = builder2
  .selectUTXOs(availableUTXOs) // Auto-select UTXOs
  .addOutput("tmXYZ...", 100000000) // 1 ZEC
  .setChangeAddress("tmDEF...")
  .setFeeRate(10) // 10 zatoshis per byte
  .build(blockHeight, pubkey);

// Sign the transaction (two-step process)
const signed = TransactionBuilder.sign(
  buildResult,
  privateKey,
  "testnet",
  pubkey,
);

console.log("Transaction ID:", signed.txid);
console.log("Fee:", signed.fee, "zatoshis");
console.log("Change:", signed.changeAmount, "zatoshis");
console.log("Raw transaction:", signed.rawTx.toString("hex"));

// The signed.rawTx is ready to broadcast
// await rpcClient.sendRawTransaction(signed.rawTx.toString('hex'))
```

#### Using Low-Level Functions

```typescript
import {
  buildTransaction,
  signTransaction,
  signSighashes,
  calculateTransactionId,
  selectUTXOs,
  getPublicKeyFromPrivateKey,
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

// Get public key from your private key
const pubkey = getPublicKeyFromPrivateKey(privateKey);

// Build unsigned transaction
const unsignedTx = buildTransaction({
  inputs: selection.inputs,
  outputs: selection.outputs,
  network: "testnet",
  blockHeight: 123456,
  pubkey, // Required: public key of the address that owns the UTXOs
});

// Sign the sighashes with your private key
const signatures = signSighashes(unsignedTx.sighashes, privateKey);

// Apply signatures to create final transaction
const signedTx = signTransaction(unsignedTx, signatures, "testnet", pubkey);

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

## Important Notes

1. **NAPI Bindings Required**: This library requires NAPI bindings to be available. There are no fallback implementations.

2. **Public Key Requirement**: The `buildTransaction` and `signTransaction` functions require the public key of the address that owns the UTXOs. This is a fundamental requirement of the Zcash transaction building process.

3. **Network Names**: The library accepts both short ('main', 'test') and long ('mainnet', 'testnet') network names for compatibility.

4. **Script Requirements**: While the UTXO interface has an optional script field for backward compatibility, the transaction building functions require the script to be present (ValidUTXO type).

5. **Legacy Support**: Many legacy exports are maintained for backward compatibility with existing code. New code should use the modern function names.

6. **Error Handling**: Functions throw ZecError instances for Zcash-specific errors, and standard Error instances for general failures.

## Transaction Flow

1. **Prepare**: Get UTXOs and select which ones to spend
2. **Build**: Create unsigned transaction with `buildTransaction` (requires pubkey)
3. **Sign**: Sign each sighash with your private key
4. **Finalize**: Apply signatures with `signTransaction` (requires pubkey)
5. **Broadcast**: Send the transaction to the network
