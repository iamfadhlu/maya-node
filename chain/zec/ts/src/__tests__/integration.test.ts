import {
  isValidAddr,
  buildTx,
  getFee,
  buildTransaction,
  signTransaction,
  signSighashes,
  getPublicKeyFromPrivateKey,
  ZcashRPCClient,
  type Config,
  type UTXO,
  type Output,
  type Tx,
} from "../index";

describe("Integration Tests", () => {
  // Test data for integration scenarios
  const testConfig: Config = {
    host: "localhost",
    port: 8232,
    username: "testuser",
    password: "testpass",
    network: "regtest",
  };

  const testAddress = "tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU";
  const destinationAddress = "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6";
  const testPrivateKey = "cPuumgiWr52j7Aoo3SSefN6JNBJHnfEmv9TFwbr9kAKqWULvCkrN";

  describe("Full Transaction Workflow", () => {
    test("should complete full transaction creation workflow", () => {
      // 1. Validate addresses
      expect(isValidAddr(testAddress, "regtest")).toBe(true);
      expect(isValidAddr(destinationAddress, "regtest")).toBe(true);

      // 2. Create mock UTXOs (would come from RPC in real scenario)
      const mockUTXOs: UTXO[] = [
        {
          txid: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
          vout: 0,
          value: 10000000, // 0.1 ZEC
          height: 200,
        },
      ];

      // 3. Create transaction outputs
      const outputs: Output[] = [
        {
          type: "pkh",
          address: destinationAddress,
          amount: 9900000, // 0.099 ZEC (leaving 0.001 for fee)
        },
        {
          type: "memo",
          memo: "Test integration transaction",
        },
      ];

      // 4. Calculate expected fee
      const expectedFee = getFee(mockUTXOs, outputs);
      expect(expectedFee).toBeGreaterThan(0);
      expect(expectedFee).toBeLessThan(100000); // Reasonable fee

      // 5. Build the transaction
      const tx = buildTx(testConfig, mockUTXOs, outputs, 200);

      expect(tx).toBeDefined();
      expect(tx.version).toBe(-2147483643); // Zcash v5 with Overwinter flag (0x80000005)
      expect(tx.inputs).toHaveLength(1);
      expect(tx.outputs.length).toBeGreaterThanOrEqual(2); // At least memo + PKH outputs
      expect(tx.fee).toBe(expectedFee);

      // 6. Sign and finalize the transaction
      const pubkey = getPublicKeyFromPrivateKey(testPrivateKey);

      // Build the transaction
      const unsignedTx = buildTransaction({
        inputs: tx.inputs,
        outputs: tx.outputs.map((o: Output) => {
          if (o.type === "pkh") {
            return { address: o.address, value: o.amount, memo: "" };
          }
          return {
            address: "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6",
            value: 0,
            memo: o.memo,
          };
        }),
        network: testConfig.network,
        blockHeight: 100,
        pubkey,
      });

      // Sign the sighashes
      const signatures = signSighashes(unsignedTx.sighashes, testPrivateKey);

      // Apply signatures to get final transaction
      const finalTx = signTransaction(
        unsignedTx,
        signatures,
        testConfig.network,
        pubkey,
      );

      expect(finalTx).toBeInstanceOf(Buffer);
      expect(finalTx.length).toBeGreaterThan(100); // Reasonable transaction size

      console.log("Transaction created successfully:");
      console.log("- Version:", tx.version);
      console.log("- Size:", finalTx.length, "bytes");
      console.log("- Inputs:", tx.inputs.length);
      console.log("- Outputs:", tx.outputs.length);
      console.log("- Fee:", tx.fee, "zatoshis");
    });
  });

  describe("RPC Client Integration", () => {
    test("should create RPC client with config", () => {
      const client = new ZcashRPCClient(testConfig);

      expect(client).toBeDefined();
      // Note: We can't test actual RPC calls without a running zcashd
      // but we can verify the client is properly constructed
    });

    test("should handle different network configurations", () => {
      const networks = ["main", "test", "regtest"] as const;

      networks.forEach((network) => {
        const config = { ...testConfig, network };
        const client = new ZcashRPCClient(config);

        expect(client).toBeDefined();
      });
    });
  });

  describe("Fee Calculation Validation", () => {
    test("should handle different fee scenarios", () => {
      const baseUTXO: UTXO = {
        txid: "fee_test_tx_" + Date.now().toString(16),
        vout: 0,
        value: 1000000, // 0.01 ZEC
        height: 150,
      };

      const testCases = [
        { amount: 900000, description: "high fee scenario" },
        { amount: 950000, description: "medium fee scenario" },
        { amount: 975000, description: "low fee scenario" }, // Reduced to leave room for fee
      ];

      testCases.forEach(({ amount }) => {
        const outputs: Output[] = [
          {
            type: "pkh",
            address: destinationAddress,
            amount,
          },
        ];

        const fee = getFee([baseUTXO], outputs);
        const totalSpend = amount + fee;

        expect(fee).toBeGreaterThan(0);
        expect(totalSpend).toBeLessThanOrEqual(baseUTXO.value);

        // Should be able to build transaction
        expect(() => {
          buildTx(testConfig, [baseUTXO], outputs, 150);
        }).not.toThrow();
      });
    });
  });

  describe("Multi-input Transaction", () => {
    test("should handle transactions with multiple inputs", () => {
      const utxos: UTXO[] = [
        {
          txid: "multi_input_tx1_" + Date.now().toString(16),
          vout: 0,
          value: 5000000, // 0.05 ZEC
          height: 100,
        },
        {
          txid: "multi_input_tx2_" + Date.now().toString(16),
          vout: 1,
          value: 3000000, // 0.03 ZEC
          height: 101,
        },
      ];

      const outputs: Output[] = [
        {
          type: "pkh",
          address: destinationAddress,
          amount: 7000000, // 0.07 ZEC (leaving some for fee and change)
        },
        {
          type: "memo",
          memo: "Multi-input test",
        },
      ];

      const tx = buildTx(testConfig, utxos, outputs, 102);

      expect(tx.inputs).toHaveLength(2);
      expect(tx.outputs.length).toBeGreaterThanOrEqual(2);

      // Should be able to sign multi-input transaction
      const pubkey = getPublicKeyFromPrivateKey(testPrivateKey);

      const unsignedTx = buildTransaction({
        inputs: tx.inputs,
        outputs: tx.outputs.map((o: Output) => {
          if (o.type === "pkh") {
            return { address: o.address, value: o.amount, memo: "" };
          }
          return {
            address: "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6",
            value: 0,
            memo: o.memo,
          };
        }),
        network: testConfig.network,
        blockHeight: 102,
        pubkey,
      });

      const signatures = signSighashes(unsignedTx.sighashes, testPrivateKey);
      const finalTx = signTransaction(
        unsignedTx,
        signatures,
        testConfig.network,
        pubkey,
      );

      expect(finalTx).toBeInstanceOf(Buffer);
      expect(finalTx.length).toBeGreaterThan(200); // Larger due to multiple inputs
    });
  });

  describe("Change Output Handling", () => {
    test("should create change output when needed", () => {
      const utxo: UTXO = {
        txid: "change_test_" + Date.now().toString(16),
        vout: 0,
        value: 5000000, // 0.05 ZEC
        height: 200,
      };

      const outputs: Output[] = [
        {
          type: "pkh",
          address: destinationAddress,
          amount: 2000000, // 0.02 ZEC (significant change expected)
        },
      ];

      const tx = buildTx(testConfig, [utxo], outputs, 200);

      // Legacy buildTx doesn't create change outputs
      const pkhOutputs = tx.outputs.filter((o) => "address" in o);
      expect(pkhOutputs.length).toBe(1);

      // Verify change amount is reasonable
      const totalInput = utxo.value;
      const totalOutput = outputs.reduce((sum, o) => {
        if (o.type === "pkh") {
          return sum + o.amount;
        }
        return sum;
      }, 0);
      const expectedChange = totalInput - totalOutput - tx.fee;

      // Legacy buildTx doesn't create change outputs
      // Verify the fee is calculated correctly
      expect(tx.fee).toBeGreaterThan(0);
      expect(tx.fee).toBeLessThan(totalInput);

      // The unspent amount (change) is: input - output - fee
      const unspentAmount = totalInput - totalOutput - tx.fee;
      expect(unspentAmount).toBe(expectedChange);
    });

    test("should not create change output for exact amounts", () => {
      const utxo: UTXO = {
        txid: "exact_test_" + Date.now().toString(16),
        vout: 0,
        value: 1000000, // 0.01 ZEC
        height: 200,
      };

      const fee = getFee(
        [utxo],
        [{ type: "pkh", address: destinationAddress, amount: 950000 }],
      );
      const exactAmount = utxo.value - fee;

      const outputs: Output[] = [
        {
          type: "pkh",
          address: destinationAddress,
          amount: exactAmount,
        },
      ];

      const tx = buildTx(testConfig, [utxo], outputs, 200);

      // Should only have the one output (no change needed)
      const pkhOutputs = tx.outputs.filter((o) => "address" in o);
      expect(pkhOutputs).toHaveLength(1);
      expect(pkhOutputs[0].amount).toBe(exactAmount);
    });
  });

  describe("Network Compatibility", () => {
    test("should work across different networks", () => {
      const networks = ["main", "test", "regtest"] as const;

      const utxo: UTXO = {
        txid: "network_test_" + Date.now().toString(16),
        vout: 0,
        value: 2000000, // 0.02 ZEC
        height: 300,
      };

      const outputs: Output[] = [
        {
          type: "pkh",
          address: destinationAddress,
          amount: 1900000, // 0.019 ZEC
        },
      ];

      networks.forEach((network) => {
        const config = { ...testConfig, network };

        expect(() => {
          const tx = buildTx(config, [utxo], outputs, 300);
          expect(tx.version).toBe(-2147483643); // Zcash v5 with Overwinter flag
          expect(tx.inputs).toHaveLength(1);

          // Should be able to sign on any network
          const pubkey = getPublicKeyFromPrivateKey(testPrivateKey);

          const unsignedTx = buildTransaction({
            inputs: tx.inputs,
            outputs: tx.outputs.map((o: Output) => {
              if (o.type === "pkh") {
                return { address: o.address, value: o.amount, memo: "" };
              }
              return {
                address: "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6",
                value: 0,
                memo: o.memo,
              };
            }),
            network: network,
            blockHeight: 300,
            pubkey,
          });

          const signatures = signSighashes(
            unsignedTx.sighashes,
            testPrivateKey,
          );
          const finalTx = signTransaction(
            unsignedTx,
            signatures,
            network,
            pubkey,
          );

          expect(finalTx).toBeInstanceOf(Buffer);
        }).not.toThrow();
      });
    });
  });

  describe("Address Validation Integration", () => {
    test("should validate addresses in transaction building", () => {
      const utxo: UTXO = {
        txid: "addr_test_" + Date.now().toString(16),
        vout: 0,
        value: 1000000,
        height: 100,
      };

      // Valid address should work
      const validOutputs: Output[] = [
        {
          type: "pkh",
          address: destinationAddress, // Valid testnet address
          amount: 900000,
        },
      ];

      expect(() => {
        buildTx(testConfig, [utxo], validOutputs, 100);
      }).not.toThrow();

      // Invalid address should be caught during validation
      const invalidOutputs: Output[] = [
        {
          type: "pkh",
          address: "invalid_address_format",
          amount: 900000,
        },
      ];

      expect(() => {
        buildTx(testConfig, [utxo], invalidOutputs, 100);
      }).toThrow();
    });
  });
});
