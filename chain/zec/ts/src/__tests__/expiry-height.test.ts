import {
  buildTransaction,
  getPublicKeyFromPrivateKey,
  type UTXO,
  type TxOutput,
} from "../index";

describe("Expiry Height Feature", () => {
  const testPrivateKey = "cN9spWsvaxA8taS7DFMxnk1yJD2gaF2PX1npuTpy3vuZFJdwavaw";
  const pubkey = getPublicKeyFromPrivateKey(testPrivateKey);

  const mockUTXO: UTXO = {
    txid: "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b",
    vout: 0,
    value: 100000000, // 1 ZEC
    height: 2000000,
    script: "76a91489abcdefabbaabbaabbaabbaabbaabbaabbaabba88ac",
  };

  const mockOutput: TxOutput = {
    address: "t1Hsc1LR8yKnbbe3twRp88p6vFfC5t7DLbs",
    value: 50000000, // 0.5 ZEC
    memo: "",
  };

  test("should set default expiry height (blockHeight + 40)", () => {
    const blockHeight = 2500000;
    const tx = buildTransaction({
      inputs: [mockUTXO],
      outputs: [mockOutput],
      network: "regtest",
      blockHeight,
      pubkey,
      // No expiryHeight specified, should default to blockHeight + 40
    });

    expect(tx.expiryHeight).toBe(blockHeight + 40);
    expect(tx.height).toBe(blockHeight);
  });

  test("should support custom expiry height", () => {
    const blockHeight = 2500000;
    const customExpiryHeight = 2600000;

    const tx = buildTransaction({
      inputs: [mockUTXO],
      outputs: [mockOutput],
      network: "regtest",
      blockHeight,
      pubkey,
      expiryHeight: customExpiryHeight,
    });

    expect(tx.expiryHeight).toBe(customExpiryHeight);
    expect(tx.height).toBe(blockHeight);
  });

  test("should support never expires (expiryHeight = 0)", () => {
    const blockHeight = 2500000;

    const tx = buildTransaction({
      inputs: [mockUTXO],
      outputs: [mockOutput],
      network: "regtest",
      blockHeight,
      pubkey,
      expiryHeight: 0, // Never expires
    });

    expect(tx.expiryHeight).toBe(0);
    expect(tx.height).toBe(blockHeight);
  });

  test("should handle different block height vs expiry height scenarios", () => {
    const testCases = [
      { blockHeight: 1000000, expiryHeight: 0, description: "never expires" },
      {
        blockHeight: 1000000,
        expiryHeight: 1000100,
        description: "expires 100 blocks later",
      },
      {
        blockHeight: 1000000,
        expiryHeight: 1000001,
        description: "expires next block",
      },
      {
        blockHeight: 1000000,
        expiryHeight: undefined,
        description: "default expiry (40 blocks)",
      },
    ];

    testCases.forEach(({ blockHeight, expiryHeight, description }) => {
      const params: any = {
        inputs: [mockUTXO],
        outputs: [mockOutput],
        network: "regtest",
        blockHeight,
        pubkey,
      };

      if (expiryHeight !== undefined) {
        params.expiryHeight = expiryHeight;
      }

      const tx = buildTransaction(params);

      const expectedExpiryHeight =
        expiryHeight !== undefined ? expiryHeight : blockHeight + 40;

      expect(tx.height).toBe(blockHeight);
      expect(tx.expiryHeight).toBe(expectedExpiryHeight);

      console.log(
        `✓ ${description}: blockHeight=${blockHeight}, expiryHeight=${tx.expiryHeight}`,
      );
    });
  });

  test("should include expiry height in unsigned transaction structure", () => {
    const tx = buildTransaction({
      inputs: [mockUTXO],
      outputs: [mockOutput],
      network: "regtest",
      blockHeight: 1000000,
      pubkey,
      expiryHeight: 0,
    });

    // Verify the UnsignedTransaction has all expected fields
    expect(tx).toHaveProperty("version");
    expect(tx).toHaveProperty("inputs");
    expect(tx).toHaveProperty("outputs");
    expect(tx).toHaveProperty("lockTime");
    expect(tx).toHaveProperty("sighashes");
    expect(tx).toHaveProperty("height");
    expect(tx).toHaveProperty("expiryHeight");

    expect(Array.isArray(tx.sighashes)).toBe(true);
    expect(tx.sighashes.length).toBeGreaterThan(0);
  });
});
