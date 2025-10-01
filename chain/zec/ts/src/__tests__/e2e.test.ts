/**
 * End-to-End tests for Zcash NAPI bindings
 *
 * These tests simulate real-world usage scenarios similar to the zcash-old regtest tests
 * but adapted for our NAPI bindings architecture.
 */

import {
  validateZecAddress,
  getOutputViewingKey,
  TransactionBuilder,
  getPublicKeyFromPrivateKey,
  calculateTransactionId,
  type UTXO,
  type Output,
} from "../index";

describe("End-to-End Zcash Tests", () => {
  // Configuration similar to zcash-old regtest
  const config = {
    network: "Regtest" as const,
    pubkey: Buffer.from(
      "03c622fa3be76cd25180d5a61387362181caca77242023be11775134fd37f403f7",
      "hex",
    ),
    privateKey: "cPuumgiWr52j7Aoo3SSefN6JNBJHnfEmv9TFwbr9kAKqWULvCkrN", // Valid testnet private key
    addresses: {
      source: "tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU",
      destination: "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6",
    },
  };

  describe("Address Management", () => {
    test("should validate vault-derived addresses", () => {
      // Test that addresses are valid for the regtest network
      expect(() =>
        validateZecAddress(config.addresses.source, config.network as any),
      ).not.toThrow();

      expect(() =>
        validateZecAddress(config.addresses.destination, config.network as any),
      ).not.toThrow();
    });

    test("should get output viewing key from pubkey", () => {
      const ovk = getOutputViewingKey(config.pubkey);

      expect(ovk).toBeInstanceOf(Buffer);
      expect(ovk.length).toBe(32); // OVK should be 32 bytes

      // OVK should be deterministic for the same pubkey
      const ovk2 = getOutputViewingKey(config.pubkey);
      expect(ovk.equals(ovk2)).toBe(true);
    });
  });

  describe("Transaction Lifecycle using TransactionBuilder", () => {
    test("should create and process a simple transaction", () => {
      // Mock UTXO from regtest (similar to what would come from getUTXOS RPC call)
      const sourceUTXO: UTXO = {
        txid: "e2e_test_source_tx_" + Date.now().toString(16).padStart(32, "0"),
        height: 200,
        vout: 0,
        script: "76a914" + "a".repeat(40) + "88ac", // P2PKH script
        value: 10000000, // 0.1 ZEC in zatoshis
      };

      // Create transaction builder
      const builder = new TransactionBuilder(config.network);

      // Add UTXOs and output
      builder.addInputs([sourceUTXO]);
      builder.addOutput(config.addresses.destination, 9990000); // 0.0999 ZEC

      // Build the unsigned transaction
      const buildResult = builder.build(200, config.pubkey);

      expect(buildResult.unsignedTx).toBeDefined();
      expect(buildResult.fee).toBeGreaterThan(0);
      expect(buildResult.totalInput).toBe(10000000);

      // Sign the transaction
      const signed = TransactionBuilder.sign(
        buildResult,
        config.privateKey,
        config.network,
        config.pubkey,
      );

      expect(signed.txid).toMatch(/^[a-f0-9]{64}$/);
      console.log("E2E Test TXID:", signed.txid);

      expect(signed.rawTx).toBeInstanceOf(Buffer);
      expect(signed.rawTx.length).toBeGreaterThan(100); // Reasonable size for a simple tx

      console.log("E2E Test Results:");
      console.log("- Transaction ID:", signed.txid);
      console.log("- Final transaction size:", signed.rawTx.length, "bytes");
      console.log(
        "- Transaction hex (first 32 bytes):",
        signed.rawTx.subarray(0, 32).toString("hex"),
      );
    });

    test("should handle transaction with memo", () => {
      const memoUTXO: UTXO = {
        txid: "memo_test_tx_" + Date.now().toString(16).padStart(32, "0"),
        height: 150,
        vout: 1,
        script: "76a914" + "b".repeat(40) + "88ac",
        value: 5000000, // 0.05 ZEC
      };

      // Create transaction builder
      const builder = new TransactionBuilder(config.network);

      // Add UTXO and regular output
      builder.addInputs([memoUTXO]);
      builder.addOutput(config.addresses.destination, 4990000);

      // Add memo as a zero-value output with memo
      builder.addOutput(
        config.addresses.destination,
        0,
        "Test memo for E2E transaction with OP_RETURN data",
      );

      // Build and sign
      const buildResult = builder.build(150, config.pubkey);
      const signed = TransactionBuilder.sign(
        buildResult,
        config.privateKey,
        config.network,
        config.pubkey,
      );

      expect(signed.txid).toMatch(/^[a-f0-9]{64}$/);
      expect(signed.fee).toBeLessThanOrEqual(10000); // Should have reasonable fee

      console.log("Memo transaction TXID:", signed.txid);
      console.log("Transaction fee:", signed.fee, "zatoshis");
    });

    test("should handle multiple UTXOs with automatic change", () => {
      // Create multiple UTXOs with various amounts
      const utxos: UTXO[] = [
        {
          txid: "multi_utxo_1_" + Date.now().toString(16).padStart(32, "0"),
          height: 100,
          vout: 0,
          script: "76a914" + "1".repeat(40) + "88ac",
          value: 3000000, // 0.03 ZEC
        },
        {
          txid: "multi_utxo_2_" + Date.now().toString(16).padStart(32, "0"),
          height: 101,
          vout: 1,
          script: "76a914" + "2".repeat(40) + "88ac",
          value: 2000000, // 0.02 ZEC
        },
        {
          txid: "multi_utxo_3_" + Date.now().toString(16).padStart(32, "0"),
          height: 102,
          vout: 0,
          script: "76a914" + "3".repeat(40) + "88ac",
          value: 1500000, // 0.015 ZEC
        },
      ];

      // Create builder and configure
      const builder = new TransactionBuilder(config.network);
      builder.setChangeAddress(config.addresses.source); // Set change address

      // Add output that requires multiple UTXOs
      builder.addOutput(config.addresses.destination, 4500000); // 0.045 ZEC

      // Use automatic UTXO selection after adding outputs
      builder.selectUTXOs(utxos);

      // Build - should automatically select appropriate UTXOs and create change
      const buildResult = builder.build(105, config.pubkey);
      const signed = TransactionBuilder.sign(
        buildResult,
        config.privateKey,
        config.network,
        config.pubkey,
      );

      expect(signed.txid).toMatch(/^[a-f0-9]{64}$/);
      expect(buildResult.unsignedTx.inputs.length).toBeGreaterThanOrEqual(2); // Should need at least 2 UTXOs
      expect(signed.fee).toBeLessThanOrEqual(50000); // Reasonable fee for multi-input tx

      console.log("Multi-UTXO transaction:");
      console.log("- Used UTXOs:", buildResult.unsignedTx.inputs.length);
      console.log("- Total input:", signed.totalInput, "zatoshis");
      console.log("- Change amount:", signed.changeAmount, "zatoshis");
      console.log("- Fee:", signed.fee, "zatoshis");
    });

    test("should calculate correct fees for different transaction sizes", () => {
      const testCases = [
        {
          name: "Single input, single output",
          utxos: 1,
          outputs: 1,
          baseValue: 1000000,
        },
        {
          name: "Multiple inputs, single output",
          utxos: 3,
          outputs: 1,
          baseValue: 3000000,
        },
        {
          name: "Single input, multiple outputs",
          utxos: 1,
          outputs: 3,
          baseValue: 2000000,
        },
      ];

      testCases.forEach(({ name, utxos, outputs, baseValue }) => {
        // Create test UTXOs
        const testUtxos: UTXO[] = Array.from({ length: utxos }, (_, i) => ({
          txid: `fee_test_${i}_${Date.now().toString(16)}`.padStart(64, "0"),
          height: 100 + i,
          vout: i,
          script: "76a914" + i.toString().repeat(40) + "88ac",
          value: Math.floor(baseValue / utxos),
        }));

        const builder = new TransactionBuilder(config.network);
        builder.addInputs(testUtxos);

        // Add outputs
        const totalInput = testUtxos.reduce((sum, utxo) => sum + utxo.value, 0);
        const outputAmount = Math.floor((totalInput - 20000) / outputs); // Leave room for fee

        for (let i = 0; i < outputs; i++) {
          builder.addOutput(config.addresses.destination, outputAmount);
        }

        const buildResult = builder.build(100, config.pubkey);
        const signed = TransactionBuilder.sign(
          buildResult,
          config.privateKey,
          config.network,
          config.pubkey,
        );

        expect(signed.fee).toBeGreaterThan(0);
        expect(signed.fee).toBeLessThanOrEqual(50000); // Max reasonable fee
        console.log(`${name}: Fee = ${signed.fee} zatoshis`);
      });
    });
  });

  describe("TransactionBuilder Advanced Features", () => {
    test("should set custom fee rate", () => {
      const utxo: UTXO = {
        txid: "fee_rate_test".padEnd(64, "0"),
        height: 100,
        vout: 0,
        script: "76a914" + "a".repeat(40) + "88ac",
        value: 1000000,
      };

      // Test with different fee rates
      const feeRates = [1, 5, 10, 20];

      feeRates.forEach((rate) => {
        const builder = new TransactionBuilder(config.network);
        builder.addInputs([utxo]);
        builder.addOutput(config.addresses.destination, 900000);
        builder.setFeeRate(rate);

        const buildResult = builder.build(100, config.pubkey);
        const signed = TransactionBuilder.sign(
          buildResult,
          config.privateKey,
          config.network,
          config.pubkey,
        );
        console.log(
          `Fee rate ${rate} sat/byte resulted in fee: ${signed.fee} zatoshis`,
        );

        expect(signed.fee).toBeGreaterThan(0);
      });
    });

    test("should set custom change address", () => {
      const utxo: UTXO = {
        txid: "change_addr_test".padEnd(64, "0"),
        height: 100,
        vout: 0,
        script: "76a914" + "b".repeat(40) + "88ac",
        value: 1000000,
      };

      const customChangeAddr = "tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU";

      const builder = new TransactionBuilder(config.network);
      builder.addInputs([utxo]);
      builder.addOutput(config.addresses.destination, 500000); // Half, ensuring change
      builder.setChangeAddress(customChangeAddr);

      const buildResult = builder.build(100, config.pubkey);
      const signed = TransactionBuilder.sign(
        buildResult,
        config.privateKey,
        config.network,
        config.pubkey,
      );

      expect(signed.rawTx).toBeInstanceOf(Buffer);
      expect(signed.fee).toBeGreaterThan(0);
      expect(signed.changeAmount).toBeGreaterThan(0);

      // The builder should have created a change output to the custom address
      console.log("Transaction with custom change address created");
      console.log("- Change amount:", signed.changeAmount, "zatoshis");
    });

    test("should handle no change when amount too small", () => {
      const utxo: UTXO = {
        txid: "no_change_test".padEnd(64, "0"),
        height: 100,
        vout: 0,
        script: "76a914" + "c".repeat(40) + "88ac",
        value: 1000000,
      };

      const builder = new TransactionBuilder(config.network);
      builder.addInputs([utxo]);
      builder.addOutput(config.addresses.destination, 990000); // Almost all, leaving minimal for fee
      builder.setChangeAddress(config.addresses.source);

      const buildResult = builder.build(100, config.pubkey);
      const signed = TransactionBuilder.sign(
        buildResult,
        config.privateKey,
        config.network,
        config.pubkey,
      );

      // Change should be 0 as it would be dust
      expect(signed.changeAmount).toBe(0);
      console.log("Transaction without change output, fee:", signed.fee);
    });

    test("should clear and rebuild transaction", () => {
      const utxos: UTXO[] = [
        {
          txid: "clear_test_1".padEnd(64, "0"),
          height: 100,
          vout: 0,
          script: "76a914" + "1".repeat(40) + "88ac",
          value: 500000,
        },
        {
          txid: "clear_test_2".padEnd(64, "0"),
          height: 100,
          vout: 1,
          script: "76a914" + "2".repeat(40) + "88ac",
          value: 700000,
        },
      ];

      const builder = new TransactionBuilder(config.network);

      // First build
      builder.addInputs(utxos);
      builder.addOutput(config.addresses.destination, 300000);

      // Clear and rebuild differently
      builder.clear();
      builder.addInputs(utxos);
      builder.addOutput(config.addresses.destination, 500000);
      builder.addOutput(config.addresses.source, 200000);

      const buildResult = builder.build(100, config.pubkey);
      const signed = TransactionBuilder.sign(
        buildResult,
        config.privateKey,
        config.network,
        config.pubkey,
      );

      expect(signed.rawTx).toBeInstanceOf(Buffer);
      expect(signed.fee).toBeGreaterThan(0);
      expect(buildResult.unsignedTx.outputs.length).toBe(2); // Two outputs we specified
      console.log("Rebuilt transaction fee:", signed.fee);
    });
  });

  describe("Error Scenarios with TransactionBuilder", () => {
    test("should throw on insufficient funds", () => {
      const insufficientUTXO: UTXO = {
        txid: "insufficient_funds_test".padEnd(64, "0"),
        height: 50,
        vout: 0,
        script: "76a914" + "f".repeat(40) + "88ac",
        value: 1000, // Only 0.00001 ZEC
      };

      const builder = new TransactionBuilder(config.network);
      builder.addInputs([insufficientUTXO]);
      builder.addOutput(config.addresses.destination, 10000000); // 0.1 ZEC - way more than available

      expect(() => builder.build(50, config.pubkey)).toThrow(
        "Insufficient funds",
      );
    });

    test("should throw when no inputs added", () => {
      const builder = new TransactionBuilder(config.network);
      builder.addOutput(config.addresses.destination, 100000);

      expect(() => builder.build(100, config.pubkey)).toThrow(
        "No inputs provided",
      );
    });

    test("should throw when no outputs added", () => {
      const utxo: UTXO = {
        txid: "no_output_test".padEnd(64, "0"),
        height: 100,
        vout: 0,
        script: "76a914" + "a".repeat(40) + "88ac",
        value: 100000,
      };

      const builder = new TransactionBuilder(config.network);
      builder.addInputs([utxo]);

      expect(() => builder.build(100, config.pubkey)).toThrow(
        "No outputs provided",
      );
    });

    test("should handle invalid addresses", () => {
      const utxo: UTXO = {
        txid: "invalid_addr_test".padEnd(64, "0"),
        height: 100,
        vout: 0,
        script: "76a914" + "a".repeat(40) + "88ac",
        value: 100000,
      };

      const builder = new TransactionBuilder(config.network);
      builder.addInputs([utxo]);

      // This should throw when trying to add output with invalid address
      expect(() => builder.addOutput("invalid_address", 50000)).toThrow();
    });
  });
});
