import bs58check from "bs58check";

import { pushData } from "./writer";

export function memoToScript(memo: string): Buffer {
  const opr = Buffer.alloc(memo.length + 4);
  opr[1] = 0x6a;

  let offset = 2;
  const pml = pushData(memo.length);
  pml.copy(opr, offset);
  offset += pml.length;
  Buffer.from(memo).copy(opr, offset);
  offset += memo.length;
  opr[0] = offset - 1;
  const script = opr.subarray(0, offset);
  return script;
}

export function addressToScript(address: string): Buffer {
  const addrb = bs58check.decode(address);

  // Extract the 20-byte public key hash (skip 2-byte prefix, take 20 bytes)
  const pkh = Buffer.alloc(20);
  Buffer.from(addrb).copy(pkh, 0, 2, 22); // Fixed: copy from bytes 2-22 of addrb to pkh

  // Create P2PKH script: OP_DUP OP_HASH160 <20-byte-pkh> OP_EQUALVERIFY OP_CHECKSIG
  const script = Buffer.alloc(25); // Fixed: 25 bytes not 26
  script[0] = 0x76; // OP_DUP
  script[1] = 0xa9; // OP_HASH160
  script[2] = 0x14; // Push 20 bytes
  pkh.copy(script, 3); // Copy 20-byte PKH
  script[23] = 0x88; // OP_EQUALVERIFY
  script[24] = 0xac; // OP_CHECKSIG

  return script;
}

export function writeSigScript(signature: Uint8Array, pk: Uint8Array) {
  const buf = Buffer.alloc(5 + signature.length + pk.length);
  let offset = 0;

  const psl = pushData(signature.length + 1);
  psl.copy(buf, offset);
  offset += psl.length;
  Buffer.from(signature).copy(buf, offset);
  offset += signature.length;
  buf[offset] = 1;
  offset += 1;

  const pkl = pushData(pk.length);
  pkl.copy(buf, offset);
  offset += pkl.length;
  Buffer.from(pk).copy(buf, offset);
  offset += pk.length;

  return buf.subarray(0, offset);
}
