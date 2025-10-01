// Import low-level utilities
import {
  isValidAddr as _isValidAddr,
  skToAddr as _skToAddr,
  pkToAddr as _pkToAddr,
  getPublicKeyFromPrivateKey as _getPublicKeyFromPrivateKey,
  getPublicKeyHashFromAddress as _getPublicKeyHashFromAddress,
} from "./addr";

import { memoToScript, addressToScript } from "./script";

import { getFee as calculateTransactionFee, selectUTXOs } from "./builder";

// Import types for use in function signatures
import type {
  UTXO,
  ValidUTXO,
  TxOutput,
  UnsignedTransaction,
  Config,
} from "./types";

// Import NAPI bindings (required)
import * as path from "path";
import * as fs from "fs";
import type {
  ZcashNapiModule,
  NapiPartialTx,
  NapiNetwork,
  BuiltPartialTx,
} from "./napi";

let zcashLib: ZcashNapiModule;
const binaryName = "zec.linux-x64-gnu.node";
const possiblePaths = [
  // Same directory as the compiled JS file
  path.join(__dirname, binaryName),
  // Parent directory (current approach)
  path.join(__dirname, "..", binaryName),
  // Package root directory (for when installed as dependency)
  path.join(__dirname, "..", "..", binaryName),
  // Try to resolve from package root
  path.join(__dirname, "..", "..", "..", binaryName),
];

// Try require.resolve for proper module resolution
try {
  const resolvedPath = require.resolve(`../${binaryName}`);
  possiblePaths.unshift(resolvedPath);
} catch {
  // require.resolve failed, continue with other paths
}

let loadError: Error | null = null;
let foundPath: string | null = null;

// Try each possible path
for (const binaryPath of possiblePaths) {
  try {
    // Check if file exists before trying to require it
    if (fs.existsSync(binaryPath)) {
      foundPath = binaryPath;
      zcashLib = require(binaryPath);
      break;
    }
  } catch (error) {
    loadError = error as Error;
  }
}

// If not found, provide detailed error message
if (!foundPath) {
  const searchedPaths = possiblePaths.join("\n  - ");
  const errorMessage = loadError
    ? `Failed to load Zcash NAPI bindings from any of the following locations:\n  - ${searchedPaths}\n\nLast error: ${loadError.message}`
    : `Zcash NAPI binary '${binaryName}' not found in any of the following locations:\n  - ${searchedPaths}\n\nPlease ensure the native module is properly built and installed.`;

  throw new Error(errorMessage);
}

// Export types
export type {
  Network as NetworkEnum,
  UTXO,
  ValidUTXO,
  TxOutput,
  UnsignedTransaction,
  UTXOSelection,
  PartialTx,
  Config, // Legacy support
} from "./types";

// Export NAPI types
export type {
  NapiNetwork,
  NapiUTXO,
  NapiOutput,
  NapiPartialTx,
  BuiltPartialTx,
  ZcashNapiModule,
  ExtendedTransactionData,
} from "./napi";

// Export Network object for backward compatibility with tests
export const Network = {
  Main: "main" as const,
  Test: "test" as const,
  Regtest: "regtest" as const,
  Mainnet: "mainnet" as const,
  Testnet: "testnet" as const,
};

// Legacy type aliases for backward compatibility
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

// Export core address & key management functions
export function validateAddress(address: string, network: string): boolean {
  return _isValidAddr(address, network);
}

export function addressFromPrivateKey(
  privateKey: string,
  network: string,
): string {
  return _skToAddr(privateKey, network);
}

export function addressFromPublicKey(
  publicKey: Buffer,
  network: string,
): string {
  return _pkToAddr(publicKey, network);
}

export function getPublicKeyFromPrivateKey(privateKey: string): Buffer {
  return _getPublicKeyFromPrivateKey(privateKey);
}

export function getPublicKeyHashFromAddress(address: string): Buffer {
  return _getPublicKeyHashFromAddress(address);
}

// Export script utilities
export function createMemoScript(memo: string): Buffer {
  return memoToScript(memo);
}

export function createPayToAddressScript(address: string): Buffer {
  return addressToScript(address);
}

// Export UTXO & fee calculation
export function calculateFee(
  inputCount: number,
  outputCount: number,
  memoLength: number = 0,
): number {
  return calculateTransactionFee(inputCount, outputCount, memoLength);
}

export { selectUTXOs };

// NAPI-connected functions
export function buildTransaction(params: {
  inputs: UTXO[];
  outputs: TxOutput[];
  network: string;
  blockHeight: number;
  pubkey: Buffer; // Required for NAPI - public key of the address that owns the UTXOs
  expiryHeight?: number; // Block height after which tx expires (0 = never expires, default = blockHeight + 40)
}): UnsignedTransaction {
  // Convert to NAPI format
  const napiOutputs = params.outputs.map((o) => ({
    address: o.address,
    amount: o.value,
    memo: o.memo || "",
  }));

  // Ensure UTXOs have scripts
  const validInputs = params.inputs.map((utxo) => ({
    ...utxo,
    script: utxo.script || "76a914" + "00".repeat(20) + "88ac",
  }));

  // Default expiry height: 40 blocks from current height (standard practice)
  // 0 means never expires
  const expiryHeight =
    params.expiryHeight !== undefined
      ? params.expiryHeight
      : params.blockHeight + 40;

  const partialTx: NapiPartialTx = {
    height: params.blockHeight,
    txid: Buffer.alloc(32),
    inputs: validInputs,
    outputs: napiOutputs,
    fee: 0,
    sighashes: [],
    expiryHeight: expiryHeight,
    version: 0, // Will be populated by Rust
  };

  // Normalize network for NAPI call
  const normalizedNetwork = normalizeNetwork(params.network);

  const result = zcashLib.buildPartialTx(
    params.pubkey,
    partialTx,
    normalizedNetwork,
  );

  return {
    version: result.version, // Version calculated by Rust consensus rules
    inputs: result.inputs,
    outputs: params.outputs,
    lockTime: 0,
    sighashes: result.sighashes,
    height: params.blockHeight,
    expiryHeight: expiryHeight,
  };
}

export function signTransaction(
  unsignedTx: UnsignedTransaction,
  signatures: Buffer[],
  network: string,
  pubkey: Buffer, // Required for NAPI - public key of the address that owns the UTXOs
): Buffer {
  console.log("signTransaction called with:", {
    version: unsignedTx.version,
    height: unsignedTx.height,
    network,
    signatureCount: signatures.length,
  });

  const napiOutputs = unsignedTx.outputs.map((o) => ({
    address: o.address,
    amount: o.value,
    memo: o.memo || "",
  }));

  const partialTx: NapiPartialTx = {
    height: unsignedTx.height,
    txid: Buffer.alloc(32),
    inputs: unsignedTx.inputs,
    outputs: napiOutputs,
    fee: 0,
    sighashes: unsignedTx.sighashes,
    expiryHeight: unsignedTx.expiryHeight,
    version: unsignedTx.version,
  };

  // Normalize network for NAPI call
  const normalizedNetwork = normalizeNetwork(network);
  return zcashLib.applyTxSignatures(
    pubkey,
    partialTx,
    signatures,
    normalizedNetwork,
  );
}

export function signSighashes(
  sighashes: Buffer[],
  privateKey: string,
): Buffer[] {
  const secp256k1 = require("@noble/curves/secp256k1").secp256k1;
  const bs58checkModule = require("bs58check");
  const bs58check = bs58checkModule.default || bs58checkModule;

  // Convert private key from WIF or hex to bytes
  let privKeyBytes: Uint8Array;

  // Try WIF format first (most common)
  try {
    const decoded = bs58check.decode(privateKey);
    // Skip the version byte and compression flag if present
    privKeyBytes = decoded.slice(
      1,
      decoded.length === 34 ? -1 : decoded.length,
    );
  } catch {
    // Not WIF, try hex
    if (privateKey.length === 64) {
      privKeyBytes = Buffer.from(privateKey, "hex");
    } else {
      throw new Error("Invalid private key format");
    }
  }

  // Sign each sighash
  return sighashes.map((sighash) => {
    const signature = secp256k1.sign(sighash, privKeyBytes);
    return Buffer.from(signature.toDERRawBytes());
  });
}

export function calculateTransactionId(transactionBytes: Buffer): string {
  const crypto = require("crypto");
  return crypto.createHash("sha256").update(transactionBytes).digest("hex");
}

export function getOutputViewingKey(pubkey: Buffer): Buffer {
  return zcashLib.getOutputViewingKey(pubkey);
}

// Legacy exports for backward compatibility with tests
export const isValidAddr = validateAddress;
export const skToAddr = addressFromPrivateKey;
export const pkToAddr = addressFromPublicKey;

// Legacy getFee function that matches old signature
export function getFee(utxos: UTXO[], outputs: Output[]): number {
  const totalMemoLength = outputs.reduce((total, output) => {
    if (output.type === "memo") {
      return total + output.memo.length;
    }
    return total;
  }, 0);
  return calculateFee(utxos.length, outputs.length, totalMemoLength);
}

// Legacy transaction functions (for backward compatibility with tests)
export function buildTx(
  _config: Config,
  utxos: UTXO[],
  outputs: Output[],
  _height: number,
): Tx {
  // Validate inputs
  if (utxos.length === 0) {
    throw new Error("No inputs provided");
  }

  // Validate outputs
  if (outputs.length === 0) {
    throw new Error("No outputs provided");
  }

  // Ensure all UTXOs have script field
  const validUtxos = utxos.map((utxo) => ({
    ...utxo,
    script: utxo.script || "76a914" + "00".repeat(20) + "88ac",
  }));

  // Calculate total input value
  const totalInput = utxos.reduce((sum, utxo) => sum + utxo.value, 0);

  // Calculate total output value
  const totalOutput = outputs.reduce((sum, output) => {
    if (output.type === "pkh") {
      return sum + output.amount;
    }
    return sum;
  }, 0);

  // Calculate fee
  const fee = getFee(utxos, outputs);

  // Check for insufficient funds
  if (totalInput < totalOutput + fee) {
    throw new Error("Insufficient funds");
  }

  // Check for invalid addresses
  for (const output of outputs) {
    if (output.type === "pkh" && !_isValidAddr(output.address, "regtest")) {
      throw new Error(`Invalid address: ${output.address}`);
    }
  }

  // Zcash v5 with Overwinter flag
  const OVERWINTER_FLAG = 0x80000000;
  const VERSION_5 = 5;
  const version = OVERWINTER_FLAG | VERSION_5;

  return {
    version: version, // 0x80000005 = 2147483653
    inputs: validUtxos,
    outputs: outputs,
    fee: fee,
  };
}

// Legacy aliases
export const validateZecAddress = validateAddress;
export const buildPartialTx = buildTransaction;
export const computeTransactionId = calculateTransactionId;
export const applyTxSignatures = signTransaction;

// Error class
export class ZecError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ZecError";
  }
}

// Export RPC client
export { ZcashRPCClient } from "./rpc";

// Backward compatible Network constants
export const Main = Network.Main;
export const Test = Network.Test;
export const Regtest = Network.Regtest;

// Helper function to normalize network values for NAPI calls
function normalizeNetwork(network: string): string {
  const lowered = network.toLowerCase();
  switch (lowered) {
    case "main":
    case "mainnet":
      return "Main";
    case "test":
    case "testnet":
      return "Test";
    case "regtest":
      return "Regtest";
    default:
      throw new Error(`Unknown network: ${network}`);
  }
}

// Transaction Builder
export interface BuildResult {
  unsignedTx: UnsignedTransaction;
  fee: number;
  totalInput: number;
  totalOutput: number;
  changeAmount: number;
}

export interface SignedTransaction {
  rawTx: Buffer;
  txid: string;
  fee: number;
  totalInput: number;
  totalOutput: number;
  changeAmount: number;
}

export class TransactionBuilder {
  private network: string;
  private inputs: UTXO[] = [];
  private outputs: TxOutput[] = [];
  private changeAddress?: string;
  private feeRate: number = 10; // zatoshis per byte default
  private dustThreshold: number = 546; // Same as Bitcoin

  constructor(network: string = "mainnet") {
    this.network = network.toLowerCase();
  }

  /**
   * Add a UTXO to spend
   */
  addInput(utxo: UTXO): this {
    this.inputs.push(utxo);
    return this;
  }

  /**
   * Add multiple UTXOs to spend
   */
  addInputs(utxos: UTXO[]): this {
    this.inputs.push(...utxos);
    return this;
  }

  /**
   * Add an output
   */
  addOutput(address: string, value: number, memo?: string): this {
    // Validate address
    if (!validateAddress(address, this.network)) {
      throw new Error(
        `Invalid address for network ${this.network}: ${address}`,
      );
    }

    // Allow 0 value for memo outputs
    if (value < 0) {
      throw new Error("Output value must be non-negative");
    }

    if (value === 0 && !memo) {
      throw new Error("Zero-value outputs must have a memo");
    }

    this.outputs.push({
      address,
      value,
      memo: memo || "",
    });
    return this;
  }

  /**
   * Set the change address (if not set, no change output will be created)
   */
  setChangeAddress(address: string): this {
    if (!validateAddress(address, this.network)) {
      throw new Error(
        `Invalid change address for network ${this.network}: ${address}`,
      );
    }
    this.changeAddress = address;
    return this;
  }

  /**
   * Set the fee rate in zatoshis per byte
   */
  setFeeRate(zatoshisPerByte: number): this {
    if (zatoshisPerByte <= 0) {
      throw new Error("Fee rate must be positive");
    }
    this.feeRate = zatoshisPerByte;
    return this;
  }

  /**
   * Set the dust threshold (minimum change amount)
   */
  setDustThreshold(threshold: number): this {
    if (threshold < 0) {
      throw new Error("Dust threshold must be non-negative");
    }
    this.dustThreshold = threshold;
    return this;
  }

  /**
   * Select UTXOs automatically to meet the target outputs
   */
  selectUTXOs(
    availableUTXOs: UTXO[],
    strategy: "accumulative" | "all" = "accumulative",
  ): this {
    if (strategy === "all") {
      this.inputs = [...availableUTXOs];
      return this;
    }

    // Calculate target amount needed
    const targetAmount = this.outputs.reduce((sum, out) => sum + out.value, 0);

    // Estimate fee for initial calculation
    const estimatedFee = calculateFee(1, this.outputs.length + 1) * 2; // Rough estimate

    // Use accumulative selection
    const selection = selectUTXOs({
      utxos: availableUTXOs,
      targets: this.outputs,
      feeRate: this.feeRate,
      changeAddress:
        this.changeAddress || availableUTXOs[0]?.script ? "dummy" : "",
    });

    if (!selection) {
      throw new Error("Insufficient funds for transaction");
    }

    this.inputs = selection.inputs;
    return this;
  }

  /**
   * Build the transaction
   */
  build(blockHeight: number, pubkey: Buffer): BuildResult {
    // Validate we have inputs and outputs
    if (this.inputs.length === 0) {
      throw new Error("No inputs provided");
    }

    if (this.outputs.length === 0) {
      throw new Error("No outputs provided");
    }

    // Ensure all inputs have scripts
    const validInputs: ValidUTXO[] = this.inputs.map((input) => {
      if (!input.script) {
        throw new Error(`Input ${input.txid}:${input.vout} missing script`);
      }
      return input as ValidUTXO;
    });

    // Calculate totals
    const totalInput = this.inputs.reduce((sum, utxo) => sum + utxo.value, 0);
    const totalOutput = this.outputs.reduce((sum, out) => sum + out.value, 0);

    // Calculate fee based on transaction size
    // Count outputs including potential change
    const outputCount = this.changeAddress
      ? this.outputs.length + 1
      : this.outputs.length;
    const estimatedFee = calculateFee(this.inputs.length, outputCount);

    // Calculate change
    const change = totalInput - totalOutput - estimatedFee;

    // Prepare final outputs
    const finalOutputs = [...this.outputs];

    // Add change output if above dust threshold
    if (change > this.dustThreshold && this.changeAddress) {
      finalOutputs.push({
        address: this.changeAddress,
        value: change,
        memo: "",
      });
    } else if (change > 0 && change <= this.dustThreshold) {
      // Change is dust, add it to fee instead
      console.warn(
        `Change amount ${change} is below dust threshold ${this.dustThreshold}, adding to fee`,
      );
    }

    // Validate we're not losing money
    const finalTotalOutput = finalOutputs.reduce(
      (sum, out) => sum + out.value,
      0,
    );
    const finalFee = totalInput - finalTotalOutput;

    if (finalFee < 0) {
      throw new Error(
        "Insufficient funds: inputs do not cover outputs and fees",
      );
    }

    // Build the unsigned transaction
    const unsignedTx = buildTransaction({
      inputs: validInputs,
      outputs: finalOutputs,
      network: this.network,
      blockHeight,
      pubkey,
      expiryHeight: 0, // Never expires
    });

    return {
      unsignedTx,
      fee: finalFee,
      totalInput,
      totalOutput: finalTotalOutput,
      changeAmount:
        change > this.dustThreshold && this.changeAddress ? change : 0,
    };
  }

  /**
   * Get current state for debugging
   */
  getState() {
    return {
      network: this.network,
      inputs: this.inputs,
      outputs: this.outputs,
      changeAddress: this.changeAddress,
      feeRate: this.feeRate,
      dustThreshold: this.dustThreshold,
      totalInput: this.inputs.reduce((sum, utxo) => sum + utxo.value, 0),
      totalOutput: this.outputs.reduce((sum, out) => sum + out.value, 0),
    };
  }

  /**
   * Clear all inputs and outputs
   */
  clear(): this {
    this.inputs = [];
    this.outputs = [];
    return this;
  }

  /**
   * Create a new builder with the same configuration
   */
  clone(): TransactionBuilder {
    const newBuilder = new TransactionBuilder(this.network);
    newBuilder.inputs = [...this.inputs];
    newBuilder.outputs = [...this.outputs];
    newBuilder.changeAddress = this.changeAddress;
    newBuilder.feeRate = this.feeRate;
    newBuilder.dustThreshold = this.dustThreshold;
    return newBuilder;
  }

  /**
   * Sign a built transaction
   */
  static sign(
    buildResult: BuildResult,
    privateKey: string,
    network: string,
    pubkey: Buffer,
  ): SignedTransaction {
    // Sign the sighashes
    const signatures = signSighashes(
      buildResult.unsignedTx.sighashes,
      privateKey,
    );

    // Apply signatures to create final transaction (signTransaction will normalize the network internally)
    const rawTx = signTransaction(
      buildResult.unsignedTx,
      signatures,
      network,
      pubkey,
    );

    // Calculate transaction ID
    const txid = calculateTransactionId(rawTx);

    return {
      rawTx,
      txid,
      fee: buildResult.fee,
      totalInput: buildResult.totalInput,
      totalOutput: buildResult.totalOutput,
      changeAmount: buildResult.changeAmount,
    };
  }
}

/**
 * Convenience function to create a new transaction builder
 */
export function createTransactionBuilder(
  network: string = "mainnet",
): TransactionBuilder {
  return new TransactionBuilder(network);
}
