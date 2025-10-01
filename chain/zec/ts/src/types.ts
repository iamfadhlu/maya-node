// Network configuration
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
  script?: string; // hex-encoded script (optional for backward compatibility, required for NAPI)
}

// Internal UTXO type with required script
export interface ValidUTXO extends UTXO {
  script: string; // Always required for internal use
}

// Transaction output
export interface TxOutput {
  address: string;
  value: number; // zatoshis
  memo?: string; // optional OP_RETURN data
}

// Unsigned transaction structure
export interface UnsignedTransaction {
  version: number; // Transaction version (5 for current Zcash)
  inputs: ValidUTXO[]; // Internal use requires script
  outputs: TxOutput[];
  lockTime: number;
  sighashes: Buffer[]; // For signing
  height: number; // Block height for transaction (needed for signing)
  expiryHeight: number; // Block height after which tx expires (0 = never expires)
}

// UTXO selection result
export interface UTXOSelection {
  inputs: ValidUTXO[]; // Internal use requires script
  outputs: TxOutput[];
  fee: number;
}

// NAPI-compatible types for Rust interop
export interface PartialTx {
  height: number;
  txid: Buffer;
  inputs: ValidUTXO[]; // NAPI requires script
  outputs: NapiOutput[];
  fee: number;
  sighashes: Buffer[];
}

export interface NapiOutput {
  address: string;
  amount: number;
  memo: string;
}

// Legacy RPC config (kept for backward compatibility)
export interface Config {
  host: string;
  port: number;
  username: string;
  password: string;
  network: Network | "mainnet" | "testnet" | "regtest" | "main" | "test"; // Support both enum and string values
}
