/**
 * Example showing how to use the typed NAPI bindings
 * This demonstrates the type safety provided by the TypeScript declarations
 */

import {
  buildTransaction,
  signTransaction,
  signSighashes,
  getPublicKeyFromPrivateKey,
  type NapiPartialTx,
  type BuiltPartialTx,
  type UTXO,
  type TxOutput,
  type UnsignedTransaction,
} from "../index";

// Example of building a transaction with full type safety
async function exampleBuildTransaction() {
  // Private key (WIF format)
  const privateKey = "cN9spWsvaxA8taS7DFMxnk1yJD2gaF2PX1npuTpy3vuZFJdwavaw";

  // Get public key from private key
  const pubkey = getPublicKeyFromPrivateKey(privateKey);

  // Typed UTXO inputs
  const inputs: UTXO[] = [
    {
      txid: "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b",
      vout: 0,
      value: 100000000, // 1 ZEC
      height: 2000000, // Block height where this UTXO was confirmed
      script: "76a914" + "89abcdefabbaabbaabbaabbaabbaabbaabbaabba" + "88ac",
    },
  ];

  // Typed outputs
  const outputs: TxOutput[] = [
    {
      address: "t1Hsc1LR8yKnbbe3twRp88p6vFfC5t7DLbs",
      value: 50000000, // 0.5 ZEC
      memo: "",
    },
    {
      address: "t1XYpvRRMiZP8vkYL6v2wqmJ3Vm5wdNXHWq",
      value: 49990000, // ~0.5 ZEC minus fee
      memo: "Change output",
    },
  ];

  // Build transaction with type-safe parameters
  const unsignedTx: UnsignedTransaction = buildTransaction({
    inputs,
    outputs,
    network: "testnet",
    blockHeight: 2500000,
    pubkey,
    expiryHeight: 0, // Transaction never expires
  });

  console.log("Built unsigned transaction:", {
    version: unsignedTx.version,
    inputCount: unsignedTx.inputs.length,
    outputCount: unsignedTx.outputs.length,
    sighashCount: unsignedTx.sighashes.length,
    height: unsignedTx.height,
  });

  // Sign the transaction
  const signatures = signSighashes(unsignedTx.sighashes, privateKey);

  // Apply signatures with type safety
  const signedTx: Buffer = signTransaction(
    unsignedTx,
    signatures,
    "testnet",
    pubkey,
  );

  console.log("Signed transaction:", signedTx.toString("hex"));

  return signedTx;
}

// Example showing the NapiPartialTx structure
function demonstrateNapiTypes() {
  // This is the structure passed to NAPI functions
  const partialTx: NapiPartialTx = {
    height: 2500000,
    txid: Buffer.alloc(32),
    inputs: [
      {
        txid: "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b",
        vout: 0,
        value: 100000000,
        height: 2000000, // Block height where this UTXO was confirmed
        script: "76a91489abcdefabbaabbaabbaabbaabbaabbaabbaabba88ac",
      },
    ],
    outputs: [
      {
        address: "t1Hsc1LR8yKnbbe3twRp88p6vFfC5t7DLbs",
        amount: 50000000,
        memo: "",
      },
    ],
    fee: 10000,
    sighashes: [],
    expiryHeight: 2500040, // 40 blocks from current height (0 = never expires)
    version: 0, // Will be populated by Rust consensus rules
  };

  // The NAPI buildPartialTx would return a BuiltPartialTx
  // which includes populated txid and sighashes
  const builtTx: BuiltPartialTx = {
    ...partialTx,
    txid: Buffer.from("abcd...", "hex"), // Populated by Rust
    sighashes: [Buffer.from("sighash1...", "hex")], // Calculated by Rust
    fee: 10000, // Actual fee
    version: 2147483653, // 0x80000005 - calculated by Rust consensus rules
  };

  return { partialTx, builtTx };
}

// Note about ExtendedTransactionData:
// The Rust implementation has access to much more transaction data
// through the TransactionData<Unauthorized> structure, including:
// - Full version with consensus branch ID
// - Lock time and expiry height
// - Complete transparent bundle with scripts
// - Sapling/Orchard bundle data (for shielded transactions)
//
// To expose this data, the NAPI bindings would need to be extended
// to return a richer structure than just NapiPartialTx.

// Run the example if called directly
if (require.main === module) {
  exampleBuildTransaction()
    .then(() => console.log("Example completed successfully"))
    .catch((err) => console.error("Example failed:", err));
}
