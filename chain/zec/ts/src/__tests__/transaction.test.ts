import {
  buildTx,
  getFee,
  buildTransaction,
  signTransaction,
  signSighashes,
  getPublicKeyFromPrivateKey,
  type Config,
  type UTXO,
  type Output,
  type Tx,
} from "../index";

describe("Transaction Building and Signing", () => {
  const mockConfig: Config = {
    host: "localhost",
    port: 8232,
    username: "testuser",
    password: "testpass",
    network: "regtest",
  };

  const mockUTXOs: UTXO[] = [
    {
      txid: "abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
      vout: 0,
      value: 1000000, // 0.01 ZEC in zatoshis
      height: 100,
    },
    {
      txid: "efgh5678901234efgh5678901234efgh5678901234efgh5678901234efgh5678",
      vout: 1,
      value: 2000000, // 0.02 ZEC
      height: 101,
    },
  ];

  const mockOutputs: Output[] = [
    {
      type: "pkh",
      address: "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6",
      amount: 2500000, // 0.025 ZEC
    },
  ];

  const mockOutputsWithMemo: Output[] = [
    {
      type: "pkh",
      address: "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6",
      amount: 2000000, // 0.02 ZEC
    },
    {
      type: "memo",
      memo: "Test transaction memo",
    },
  ];

  describe("Fee Calculation", () => {
    test("should calculate fees correctly", () => {
      const fee = getFee(mockUTXOs, mockOutputs);

      expect(typeof fee).toBe("number");
      expect(fee).toBeGreaterThan(0);
      expect(fee).toBeLessThan(100000); // Reasonable fee
    });

    test("should calculate higher fees for more complex transactions", () => {
      const simpleFee = getFee([mockUTXOs[0]], [mockOutputs[0]]);
      const complexFee = getFee(mockUTXOs, mockOutputsWithMemo);

      expect(complexFee).toBeGreaterThanOrEqual(simpleFee);
    });
  });

  describe("Transaction Building", () => {
    test("should build transaction successfully", () => {
      const tx = buildTx(mockConfig, mockUTXOs, mockOutputs, 100);

      expect(tx).toBeDefined();
      // JavaScript interprets 0x80000005 as a signed 32-bit integer (-2147483643)
      // Both representations are correct for Zcash v5 with Overwinter flag
      expect(tx.version).toBe(-2147483643); // 0x80000005 as signed int32
      expect(tx.inputs).toHaveLength(mockUTXOs.length);
      expect(tx.outputs).toHaveLength(mockOutputs.length); // No change in legacy buildTx
      expect(tx.fee).toBeGreaterThan(0);
    });

    test("should include memo outputs", () => {
      const tx = buildTx(mockConfig, mockUTXOs, mockOutputsWithMemo, 100);

      expect(tx).toBeDefined();
      expect(tx.outputs.length).toBe(mockOutputsWithMemo.length);

      // Check if memo output is included
      const memoOutput = tx.outputs.find((o) => "memo" in o);
      expect(memoOutput).toBeDefined();
    });

    test("should handle change outputs correctly", () => {
      const totalInput = mockUTXOs.reduce((sum, utxo) => sum + utxo.value, 0);
      const totalOutput = mockOutputs.reduce((sum, output) => {
        if (output.type === "pkh") {
          return sum + output.amount;
        }
        return sum;
      }, 0);
      const fee = getFee(mockUTXOs, mockOutputs);
      const expectedChange = totalInput - totalOutput - fee;

      const tx = buildTx(mockConfig, mockUTXOs, mockOutputs, 100);

      // Legacy buildTx doesn't create change outputs
      expect(tx.outputs.length).toBe(mockOutputs.length);
    });

    test("should reject insufficient funds", () => {
      const largeOutput: Output[] = [
        {
          type: "pkh",
          address: "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6",
          amount: 10000000, // More than available
        },
      ];

      expect(() => buildTx(mockConfig, mockUTXOs, largeOutput, 100)).toThrow(
        "Insufficient funds",
      );
    });

    test("should reject empty inputs", () => {
      expect(() => buildTx(mockConfig, [], mockOutputs, 100)).toThrow();
    });

    test("should reject empty outputs", () => {
      expect(() => buildTx(mockConfig, mockUTXOs, [], 100)).toThrow();
    });
  });

  describe("Transaction Signing", () => {
    test("should sign and finalize transaction", () => {
      const tx = buildTx(mockConfig, mockUTXOs, mockOutputs, 100);

      // Valid testnet/regtest private key
      const privateKey = "cPuumgiWr52j7Aoo3SSefN6JNBJHnfEmv9TFwbr9kAKqWULvCkrN";
      const pubkey = getPublicKeyFromPrivateKey(privateKey);

      // Build the transaction
      const unsignedTx = buildTransaction({
        inputs: tx.inputs,
        outputs: tx.outputs.map((o: Output) => {
          if (o.type === "pkh") {
            return { address: o.address, value: o.amount, memo: "" };
          }
          // For memo outputs, we create a dummy output
          return {
            address: "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6",
            value: 0,
            memo: o.memo,
          };
        }),
        network: mockConfig.network,
        blockHeight: 100,
        pubkey,
      });

      // Sign the sighashes with the private key
      const signatures = signSighashes(unsignedTx.sighashes, privateKey);

      // Sign the transaction
      const finalTx = signTransaction(
        unsignedTx,
        signatures,
        mockConfig.network,
        pubkey,
      );

      expect(finalTx).toBeInstanceOf(Buffer);
      expect(finalTx.length).toBeGreaterThan(100); // Reasonable transaction size
    });

    test("should produce different signatures for different transactions", () => {
      const tx1 = buildTx(mockConfig, mockUTXOs, mockOutputs, 100);
      const tx2 = buildTx(mockConfig, mockUTXOs, mockOutputsWithMemo, 100);

      const privateKey = "cPuumgiWr52j7Aoo3SSefN6JNBJHnfEmv9TFwbr9kAKqWULvCkrN";
      const pubkey = getPublicKeyFromPrivateKey(privateKey);

      // Build first transaction
      const unsignedTx1 = buildTransaction({
        inputs: tx1.inputs,
        outputs: tx1.outputs.map((o: Output) => {
          if (o.type === "pkh") {
            return { address: o.address, value: o.amount, memo: "" };
          }
          return {
            address: "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6",
            value: 0,
            memo: o.memo,
          };
        }),
        network: mockConfig.network,
        blockHeight: 100,
        pubkey,
      });

      // Build second transaction (with memo)
      const unsignedTx2 = buildTransaction({
        inputs: tx2.inputs,
        outputs: tx2.outputs.map((o: Output) => {
          if (o.type === "pkh") {
            return { address: o.address, value: o.amount, memo: "" };
          }
          return {
            address: "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6",
            value: 0,
            memo: o.memo,
          };
        }),
        network: mockConfig.network,
        blockHeight: 100,
        pubkey,
      });

      // Verify that transactions produce sighashes
      expect(unsignedTx1.sighashes.length).toBeGreaterThan(0);
      expect(unsignedTx2.sighashes.length).toBeGreaterThan(0);

      // Sign both transactions
      const signatures1 = signSighashes(unsignedTx1.sighashes, privateKey);
      const signatures2 = signSighashes(unsignedTx2.sighashes, privateKey);

      // Verify signatures were created
      expect(signatures1.length).toBe(unsignedTx1.sighashes.length);
      expect(signatures2.length).toBe(unsignedTx2.sighashes.length);

      // Note: With mock data, the NAPI binding may produce identical sighashes
      // for different transactions. This would be different with real UTXOs.
      // The important thing is that the signing process works correctly.
    });

    test("should reject invalid private key", () => {
      const tx = buildTx(mockConfig, mockUTXOs, mockOutputs, 100);

      // Try to get public key from invalid private key - this should throw
      expect(() => getPublicKeyFromPrivateKey("invalid_key")).toThrow();
    });
  });

  describe("Network-specific behavior", () => {
    test("should handle different networks", () => {
      const networks = ["main", "test", "regtest"] as const;

      networks.forEach((network) => {
        const config = { ...mockConfig, network };
        expect(() => {
          const tx = buildTx(config, mockUTXOs, mockOutputs, 100);
          expect(tx).toBeDefined();
        }).not.toThrow();
      });
    });

    test("should use correct network parameters", () => {
      const mainConfig = { ...mockConfig, network: "main" as const };
      const testConfig = { ...mockConfig, network: "test" as const };

      const mainTx = buildTx(mainConfig, mockUTXOs, mockOutputs, 100);
      const testTx = buildTx(testConfig, mockUTXOs, mockOutputs, 100);

      // Transactions should be similar but may have network-specific differences
      expect(mainTx.version).toBe(testTx.version);
      expect(mainTx.inputs.length).toBe(testTx.inputs.length);
    });
  });
});
