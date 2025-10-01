import {
  TransactionBuilder,
  createTransactionBuilder,
  getPublicKeyFromPrivateKey,
  signSighashes,
  signTransaction,
  calculateTransactionId,
  type UTXO,
} from "../index";

describe("TransactionBuilder", () => {
  // Valid testnet/regtest private key
  const testPrivateKey = "cPuumgiWr52j7Aoo3SSefN6JNBJHnfEmv9TFwbr9kAKqWULvCkrN";
  const testPubkey = getPublicKeyFromPrivateKey(testPrivateKey);
  const testAddress = "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6";
  const changeAddress = "tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU";

  const mockUTXOs: UTXO[] = [
    {
      txid: "abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
      vout: 0,
      value: 100000000, // 1 ZEC
      height: 100,
      script: "76a914" + "a".repeat(40) + "88ac",
    },
    {
      txid: "efgh5678901234efgh5678901234efgh5678901234efgh5678901234efgh5678",
      vout: 1,
      value: 200000000, // 2 ZEC
      height: 101,
      script: "76a914" + "b".repeat(40) + "88ac",
    },
  ];

  describe("Basic building", () => {
    test("should create a simple transaction", () => {
      const builder = new TransactionBuilder("regtest");

      const result = builder
        .addInput(mockUTXOs[0])
        .addOutput(testAddress, 50000000) // 0.5 ZEC
        .setChangeAddress(changeAddress)
        .build(200, testPubkey);

      expect(result.unsignedTx).toBeDefined();
      expect(result.unsignedTx.inputs).toHaveLength(1);
      expect(result.unsignedTx.outputs).toHaveLength(2); // Main output + change
      expect(result.fee).toBeGreaterThan(0);
      expect(result.changeAmount).toBeGreaterThan(0);
      expect(result.totalInput).toBe(100000000);
      expect(result.totalOutput).toBe(100000000 - result.fee);
    });

    test("should handle multiple inputs and outputs", () => {
      const builder = new TransactionBuilder("regtest");

      const result = builder
        .addInputs(mockUTXOs)
        .addOutput(testAddress, 150000000) // 1.5 ZEC
        .addOutput(changeAddress, 100000000) // 1 ZEC
        .setChangeAddress(changeAddress)
        .build(200, testPubkey);

      expect(result.unsignedTx.inputs).toHaveLength(2);
      expect(result.unsignedTx.outputs).toHaveLength(3); // 2 outputs + change
      expect(result.totalInput).toBe(300000000);
    });

    test("should add memo output", () => {
      const builder = new TransactionBuilder("regtest");

      const result = builder
        .addInput(mockUTXOs[0])
        .addOutput(testAddress, 50000000)
        .addOutput(testAddress, 0, "Hello Zcash!") // Memo output
        .setChangeAddress(changeAddress)
        .build(200, testPubkey);

      expect(result.unsignedTx.outputs).toHaveLength(3); // Main + memo + change
      const memoOutput = result.unsignedTx.outputs.find(
        (o) => o.memo === "Hello Zcash!",
      );
      expect(memoOutput).toBeDefined();
      expect(memoOutput?.value).toBe(0);
    });
  });

  describe("UTXO selection", () => {
    test("should select UTXOs automatically", () => {
      const builder = new TransactionBuilder("regtest");

      const result = builder
        .addOutput(testAddress, 150000000) // Need more than first UTXO
        .setChangeAddress(changeAddress)
        .selectUTXOs(mockUTXOs) // Auto-select AFTER setting outputs
        .build(200, testPubkey);

      // selectUTXOs uses accumulative strategy, so it only selects what's needed
      expect(result.unsignedTx.inputs.length).toBeGreaterThanOrEqual(1);
      expect(result.totalInput).toBeGreaterThanOrEqual(150000000);
    });

    test("should use all UTXOs when specified", () => {
      const builder = new TransactionBuilder("regtest");

      const result = builder
        .selectUTXOs(mockUTXOs, "all")
        .addOutput(testAddress, 50000000) // Much less than available
        .setChangeAddress(changeAddress)
        .build(200, testPubkey);

      expect(result.unsignedTx.inputs).toHaveLength(2);
      expect(result.changeAmount).toBeGreaterThan(200000000); // Most goes to change
    });

    test("should throw on insufficient funds", () => {
      const builder = new TransactionBuilder("regtest");

      expect(() => {
        builder
          .selectUTXOs([mockUTXOs[0]]) // Only 1 ZEC
          .addOutput(testAddress, 150000000) // Need 1.5 ZEC
          .build(200, testPubkey);
      }).toThrow("Insufficient funds");
    });
  });

  describe("Change handling", () => {
    test("should not create change output below dust threshold", () => {
      const builder = new TransactionBuilder("regtest");

      const result = builder
        .addInput(mockUTXOs[0])
        .addOutput(testAddress, 99990000) // Leave ~10000 for fee and change
        .setChangeAddress(changeAddress)
        .setDustThreshold(5000)
        .build(200, testPubkey);

      expect(result.changeAmount).toBe(0);
      // Change goes to fee instead
      expect(result.fee).toBeGreaterThan(1000);
    });

    test("should work without change address", () => {
      const builder = new TransactionBuilder("regtest");

      const result = builder
        .addInput(mockUTXOs[0])
        .addOutput(testAddress, 50000000)
        // No change address set
        .build(200, testPubkey);

      expect(result.unsignedTx.outputs).toHaveLength(1); // No change output
      expect(result.changeAmount).toBe(0);
      // Extra goes to fee
      expect(result.fee).toBeGreaterThan(40000000);
    });
  });

  describe("Validation", () => {
    test("should validate addresses", () => {
      const builder = new TransactionBuilder("regtest");

      expect(() => {
        builder.addOutput("invalid_address", 100000);
      }).toThrow("Invalid address");

      expect(() => {
        builder.setChangeAddress("invalid_address");
      }).toThrow("Invalid change address");
    });

    test("should validate output amounts", () => {
      const builder = new TransactionBuilder("regtest");

      expect(() => {
        builder.addOutput(testAddress, -100);
      }).toThrow("Output value must be non-negative");

      expect(() => {
        builder.addOutput(testAddress, 0);
      }).toThrow("Zero-value outputs must have a memo");
    });

    test("should require inputs", () => {
      const builder = new TransactionBuilder("regtest");

      expect(() => {
        builder.addOutput(testAddress, 100000).build(200, testPubkey);
      }).toThrow("No inputs provided");
    });

    test("should require outputs", () => {
      const builder = new TransactionBuilder("regtest");

      expect(() => {
        builder.addInput(mockUTXOs[0]).build(200, testPubkey);
      }).toThrow("No outputs provided");
    });

    test("should require script in inputs", () => {
      const builder = new TransactionBuilder("regtest");
      const utxoNoScript = { ...mockUTXOs[0], script: undefined };

      expect(() => {
        builder
          .addInput(utxoNoScript)
          .addOutput(testAddress, 100000)
          .build(200, testPubkey);
      }).toThrow("missing script");
    });
  });

  describe("State management", () => {
    test("should clear state", () => {
      const builder = new TransactionBuilder("regtest")
        .addInputs(mockUTXOs)
        .addOutput(testAddress, 100000)
        .setChangeAddress(changeAddress);

      builder.clear();

      const state = builder.getState();
      expect(state.inputs).toHaveLength(0);
      expect(state.outputs).toHaveLength(0);
    });

    test("should clone builder", () => {
      const original = new TransactionBuilder("regtest")
        .addInput(mockUTXOs[0])
        .addOutput(testAddress, 100000)
        .setChangeAddress(changeAddress)
        .setFeeRate(20);

      const clone = original.clone();

      // Modify clone
      clone.addInput(mockUTXOs[1]);

      // Original should be unchanged
      expect(original.getState().inputs).toHaveLength(1);
      expect(clone.getState().inputs).toHaveLength(2);

      // But share config
      expect(clone.getState().feeRate).toBe(20);
      expect(clone.getState().changeAddress).toBe(changeAddress);
    });
  });

  describe("Integration with signing", () => {
    test("should produce signable transaction", () => {
      const builder = new TransactionBuilder("regtest");

      const { unsignedTx } = builder
        .addInput(mockUTXOs[0])
        .addOutput(testAddress, 50000000)
        .setChangeAddress(changeAddress)
        .build(200, testPubkey);

      // Should be able to sign
      const signatures = signSighashes(unsignedTx.sighashes, testPrivateKey);
      expect(signatures).toHaveLength(unsignedTx.sighashes.length);

      // Should be able to finalize
      const finalTx = signTransaction(
        unsignedTx,
        signatures,
        "regtest",
        testPubkey,
      );
      expect(finalTx).toBeInstanceOf(Buffer);

      // Should be able to get txid
      const txid = calculateTransactionId(finalTx);
      expect(txid).toMatch(/^[a-f0-9]{64}$/);
    });
  });

  describe("Builder factory", () => {
    test("should create builder with factory function", () => {
      const builder = createTransactionBuilder("testnet");
      expect(builder).toBeInstanceOf(TransactionBuilder);
      expect(builder.getState().network).toBe("testnet");
    });
  });
});
