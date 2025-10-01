import { secp256k1 } from "@noble/curves/secp256k1";
import { ripemd160 } from "@noble/hashes/ripemd160";
import { sha256 } from "@noble/hashes/sha2";
import * as bs58checkModule from "bs58check";
import * as bs58Module from "bs58";

const bs58check = bs58checkModule.default || bs58checkModule;
const bs58 = bs58Module.default || bs58Module;

// Zcash address prefixes (these are the correct ones for transparent addresses)
export const mainnetPrefix = [0x1c, 0xb8]; // For 't1' addresses
export const testnetPrefix = [0x1d, 0x25]; // For 'tm' addresses

function getNetworkPrefix(network: string): number[] {
  switch (network) {
    case "main":
    case "mainnet":
      return mainnetPrefix;
    case "test":
    case "testnet":
    case "regtest":
      return testnetPrefix;
    default:
      throw new Error(`Unknown network: ${network}`);
  }
}

export function isValidAddr(address: string, network: string): boolean {
  try {
    const prefix = getNetworkPrefix(network);
    // Try bs58 decode first (no checksum validation)
    const decoded = bs58.decode(address);

    // Check if the decoded address starts with the correct prefix
    return decoded[0] === prefix[0] && decoded[1] === prefix[1];
  } catch {
    // If bs58 fails, try bs58check as fallback
    try {
      const prefix = getNetworkPrefix(network);
      const decoded = bs58check.decode(address);
      return decoded[0] === prefix[0] && decoded[1] === prefix[1];
    } catch {
      return false;
    }
  }
}

export function skToAddr(sk: string, network: string): string {
  // Simple hex private key for testing
  if (sk.length === 64) {
    const skBytes = Buffer.from(sk, "hex");
    const pk = secp256k1.getPublicKey(skBytes, true);
    return pkToAddr(pk, network);
  }

  // WIF private key
  try {
    const decoded = bs58check.decode(sk);
    // Skip the version byte and compression flag if present
    const skBytes = decoded.slice(
      1,
      decoded.length === 34 ? -1 : decoded.length,
    );
    const pk = secp256k1.getPublicKey(skBytes, true);
    return pkToAddr(pk, network);
  } catch {
    throw new Error("Invalid private key format");
  }
}

export function pkToAddr(pk: Uint8Array, network: string): string {
  const prefix = getNetworkPrefix(network);
  const pkh = ripemd160(sha256(pk));
  const addr = Buffer.concat([Buffer.from(prefix), Buffer.from(pkh)]);
  return bs58check.encode(addr);
}

export function getPublicKeyFromPrivateKey(privateKey: string): Buffer {
  // Try WIF format first (most common)
  try {
    const decoded = bs58check.decode(privateKey);
    // Skip the version byte and compression flag if present
    const skBytes = decoded.slice(
      1,
      decoded.length === 34 ? -1 : decoded.length,
    );
    return Buffer.from(secp256k1.getPublicKey(skBytes, true));
  } catch {
    // Not WIF, try hex
    if (privateKey.length === 64) {
      try {
        const skBytes = Buffer.from(privateKey, "hex");
        return Buffer.from(secp256k1.getPublicKey(skBytes, true));
      } catch {
        throw new Error("Invalid private key format");
      }
    }
    throw new Error("Invalid private key format");
  }
}

// Extract public key hash from address (note: this is NOT the public key itself)
export function getPublicKeyHashFromAddress(address: string): Buffer {
  try {
    const decoded = bs58check.decode(address);
    // Skip the 2-byte prefix and extract the 20-byte public key hash
    return Buffer.from(decoded.slice(2, 22));
  } catch {
    throw new Error("Invalid address format");
  }
}
