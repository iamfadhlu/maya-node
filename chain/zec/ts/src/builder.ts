import { sumBy, sortBy } from "lodash";
import { UTXO, ValidUTXO, TxOutput, UTXOSelection, Network } from "./types";

// Helper to ensure UTXO has script
function ensureValidUTXO(utxo: UTXO): ValidUTXO {
  return {
    ...utxo,
    script: utxo.script || "76a914" + "00".repeat(20) + "88ac", // Default P2PKH script
  };
}

// Zcash fee constants (in zatoshis)
const BASE_RELAY_FEE = 10000; // Base fee
const MARGINAL_FEE = 5000; // Additional fee per input/output
const MEMO_OUTPUT_SIZE = 34; // Size of each memo output

/**
 * Calculate transaction fee based on inputs, outputs, and memo
 */
export function getFee(
  inputCount: number,
  outputCount: number,
  memoLength: number = 0,
): number {
  let totalInputs = Math.max(inputCount, 1);
  let totalOutputs = Math.max(outputCount, 2);

  // Account for memo outputs (memo is split into 34-byte chunks)
  if (memoLength > 0) {
    const memoOutputs = Math.ceil((memoLength + 2) / MEMO_OUTPUT_SIZE); // +2 for overhead
    totalOutputs += memoOutputs;
  }

  return (
    totalInputs * MARGINAL_FEE + totalOutputs * MARGINAL_FEE + BASE_RELAY_FEE
  );
}

/**
 * Estimate transaction size in bytes
 */
export function estimateTransactionSize(
  inputCount: number,
  outputCount: number,
  memoLength: number = 0,
): number {
  const inputSize = 150; // Approximate size per input
  const outputSize = 34; // Size per output
  const baseSize = 40; // Transaction overhead

  let totalOutputs = outputCount;
  if (memoLength > 0) {
    totalOutputs += Math.ceil((memoLength + 2) / MEMO_OUTPUT_SIZE);
  }

  return baseSize + inputCount * inputSize + totalOutputs * outputSize;
}

/**
 * Select UTXOs for a transaction using accumulative strategy
 * Same approach as xchain-bitcoin using coinselect/accumulative algorithm
 */
export function selectUTXOs(params: {
  utxos: UTXO[];
  targets: TxOutput[];
  feeRate?: number; // Optional, uses fixed fee if not provided
  changeAddress: string;
}): UTXOSelection | null {
  const { utxos, targets, changeAddress } = params;

  // Calculate target amount
  const targetAmount = sumBy(targets, "value");
  const totalMemoLength = sumBy(targets, (t) => t.memo?.length || 0);

  if (targetAmount <= 0) {
    return null;
  }

  // Sort UTXOs by value (descending) - accumulative strategy
  const validUtxos = utxos.map(ensureValidUTXO);
  const sortedUTXOs = sortBy(validUtxos, ["value"]).reverse();

  // Accumulate inputs until we have enough
  const selectedInputs: ValidUTXO[] = [];
  let accumulated = 0;

  for (const utxo of sortedUTXOs) {
    selectedInputs.push(utxo);
    accumulated += utxo.value;

    // Calculate fee with current selection
    const outputCount = targets.length + 1; // +1 for potential change
    const fee = getFee(selectedInputs.length, outputCount, totalMemoLength);
    const totalNeeded = targetAmount + fee;

    if (accumulated >= totalNeeded) {
      const outputs = [...targets];
      const change = accumulated - totalNeeded;

      // Add change output if above dust threshold (546 zatoshis)
      if (change > 546) {
        outputs.push({
          address: changeAddress,
          value: change,
        });
      }

      return {
        inputs: selectedInputs,
        outputs,
        fee,
      };
    }
  }

  // Insufficient funds
  return null;
}
