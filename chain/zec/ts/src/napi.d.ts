/**
 * TypeScript declarations for the Zcash NAPI bindings
 * These match the Rust structures defined in napi_bindings.rs
 */

/** Network enum matching Rust's NapiNetwork */
export enum NapiNetwork {
  Main = "Main",
  Test = "Test",
  Regtest = "Regtest",
}

/** UTXO structure as expected by NAPI functions */
export interface NapiUTXO {
  /** Transaction ID (hex string) */
  txid: string;
  /** Block height where transaction was confirmed */
  height: number;
  /** Output index (vout) */
  vout: number;
  /** Script hex string */
  script: string;
  /** Value in zatoshis */
  value: number;
}

/** Output structure for NAPI functions */
export interface NapiOutput {
  /** Destination address */
  address: string;
  /** Amount in zatoshis */
  amount: number;
  /** Optional memo (empty string if none) */
  memo: string;
}

/** Partial transaction structure used by NAPI */
export interface NapiPartialTx {
  /** Block height at which transaction is being created */
  height: number;
  /** Transaction ID (32-byte Buffer) */
  txid: Buffer;
  /** Transaction inputs */
  inputs: NapiUTXO[];
  /** Transaction outputs */
  outputs: NapiOutput[];
  /** Fee amount (as number, will be converted from f64) */
  fee: number;
  /** Sighashes for signing (array of Buffers) */
  sighashes: Buffer[];
  /** Block height after which transaction expires (0 = never expires) */
  expiryHeight: number;
  /** Transaction version calculated by Rust consensus rules */
  version: number;
}

/**
 * Complete transaction structure returned by build_ptx
 * This contains additional fields populated by the Rust code
 */
export interface BuiltPartialTx extends NapiPartialTx {
  /** Populated transaction ID after building */
  txid: Buffer;
  /** Calculated sighashes for each input */
  sighashes: Buffer[];
  /** Actual fee (may differ from requested) */
  fee: number;
}

/**
 * NAPI Module interface - functions exported by the Rust library
 */
export interface ZcashNapiModule {
  /** Initialize the Zcash library (sets up configuration) */
  initZecLib(): void;

  /**
   * Build a partial transaction (unauthorized)
   * @param vault - Public key of the vault (Buffer)
   * @param ptx - Partial transaction data
   * @param network - Network to use
   * @returns Built partial transaction with sighashes
   */
  buildPartialTx(
    vault: Buffer,
    ptx: NapiPartialTx,
    network: NapiNetwork | string,
  ): BuiltPartialTx;

  /**
   * Apply signatures to a partial transaction
   * @param vault - Public key of the vault (Buffer)
   * @param ptx - Partial transaction with sighashes
   * @param signatures - Array of DER-encoded signatures
   * @param network - Network to use
   * @returns Serialized signed transaction (Buffer)
   */
  applyTxSignatures(
    vault: Buffer,
    ptx: NapiPartialTx,
    signatures: Buffer[],
    network: NapiNetwork | string,
  ): Buffer;

  /**
   * Get the Output Viewing Key for a vault
   * @param vault - Public key of the vault
   * @returns 32-byte OVK
   */
  getOutputViewingKey(vault: Buffer): Buffer;

  /**
   * Compute transaction ID
   * @param vault - Public key of the vault
   * @param ptx - Partial transaction
   * @param network - Network to use
   * @returns Transaction ID as hex string
   */
  computeTransactionId(
    vault: Buffer,
    ptx: NapiPartialTx,
    network: NapiNetwork | string,
  ): string;

  /**
   * Validate a Zcash address
   * @param address - Address to validate
   * @param network - Network to validate against
   * @throws Error if address is invalid
   */
  validateZecAddress(address: string, network: NapiNetwork | string): void;

  /** Test function for addon verification */
  testAddon(): string;
}

/**
 * Extended transaction data available in the Rust implementation but not fully exposed via NAPI
 *
 * The Rust `build_unauthorized_tx` function creates a `TransactionData<Unauthorized>` structure
 * that contains significantly more information than what's exposed through the NAPI bindings.
 *
 * Currently, the NAPI `buildPartialTx` only returns the fields in `NapiPartialTx`.
 * To access these additional fields, the Rust NAPI bindings would need to be modified
 * to expose a richer return type.
 *
 * This interface documents what additional data could be made available if needed.
 */
export interface ExtendedTransactionData {
  /**
   * Full transaction version with Overwinter flag
   * e.g., 0x80000005 (2147483653) for v5 with Overwinter
   */
  version: number;

  /**
   * Consensus branch ID for network upgrades
   * Identifies which network upgrade rules apply
   */
  consensusBranchId: number;

  /**
   * Lock time (nLockTime)
   * Transaction cannot be included in blocks before this time
   * Can be unix timestamp or block height
   */
  lockTime: number;

  /**
   * Expiry height for transaction
   * Transaction expires and cannot be mined after this block height
   * 0 means no expiry
   */
  expiryHeight: number;

  /**
   * Transparent inputs and outputs with full script data
   * This includes the actual scripts, not just addresses
   */
  transparentBundle?: {
    inputs: Array<{
      // Previous output being spent
      prevOut: {
        txid: string; // Transaction ID (hex)
        vout: number; // Output index
      };
      // Full script data
      scriptSig: Buffer; // Unlocking script (witness data added during signing)
      sequence: number; // Sequence number (used for relative lock time)
    }>;
    outputs: Array<{
      value: number; // Amount in zatoshis
      scriptPubKey: Buffer; // Full locking script (not just address)
    }>;
  };

  /**
   * Sapling shielded bundle (not used in transparent-only implementation)
   * Would contain shielded inputs (spends) and outputs
   */
  saplingBundle?: {
    spends: any[];
    outputs: any[];
    valueBalance: number;
  };

  /**
   * Orchard shielded bundle (not used in transparent-only implementation)
   * Latest shielding technology in Zcash
   */
  orchardBundle?: {
    actions: any[];
    valueBalance: number;
  };

  /**
   * Total value flow between transparent and shielded pools
   * Positive = moving to shielded, negative = moving to transparent
   */
  valueBalance: number;

  /**
   * Binding signature for shielded transactions
   * Not used in transparent-only transactions
   */
  bindingSig?: Buffer;
}

// Module declaration for direct native module import
declare module "*.node" {
  const napiModule: ZcashNapiModule;
  export = napiModule;
}
