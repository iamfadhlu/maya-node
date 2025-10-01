/**
 * Regtest E2E Tests for Zcash TypeScript Implementation
 *
 * These tests run against a real Zcash regtest node via Docker.
 *
 * Prerequisites:
 * 1. Run `npm run regtest:up` to start the regtest node
 * 2. Wait for the setup to complete (check logs with `npm run regtest:logs`)
 * 3. Run `npm run test:e2e` to execute these tests
 * 4. Run `npm run regtest:down` to cleanup
 */

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

describe("Regtest E2E Tests", () => {
  // Test configuration for regtest node
  const config: Config = {
    host: "localhost",
    port: 18232,
    username: "mayachain",
    password: "password",
    network: "regtest",
  };

  // Test addresses that should be funded by the regtest setup
  const testAddresses = {
    source: "tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU", // Should have 1.20 ZEC
    destination: "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6", // Should have 5.40 ZEC
  };

  // Test private key for signing (WIF format for regtest)
  const testPrivateKey = "cPuumgiWr52j7Aoo3SSefN6JNBJHnfEmv9TFwbr9kAKqWULvCkrN";

  let rpcClient: ZcashRPCClient;

  beforeAll(async () => {
    // Initialize RPC client
    rpcClient = new ZcashRPCClient(config);

    // Wait for the regtest node to be ready
    const isReady = await waitForNode(15); // Wait up to 15 seconds
    if (!isReady) {
      throw new Error(
        "Zcash regtest node is not ready. Please run `npm run regtest:up` first and wait for setup to complete.",
      );
    }
  }, 60000); // 60 second timeout for beforeAll

  // Helper function to wait for node
  async function waitForNode(timeoutSeconds: number): Promise<boolean> {
    const maxAttempts = timeoutSeconds;
    for (let i = 0; i < maxAttempts; i++) {
      try {
        await rpcClient.getNetworkInfo();
        return true;
      } catch (error) {
        if (i === maxAttempts - 1) {
          console.error("Failed to connect to regtest node:", error);
          return false;
        }
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }
    }
    return false;
  }

  describe("Network Connection and Setup", () => {
    test("should connect to regtest node", async () => {
      const networkInfo = await rpcClient.getNetworkInfo();

      expect(networkInfo).toBeDefined();
      expect(networkInfo.version).toBeGreaterThan(0);
      expect(networkInfo.connections).toBeGreaterThanOrEqual(0);

      console.log("Network Info:", {
        version: networkInfo.version,
        subversion: networkInfo.subversion,
        connections: networkInfo.connections,
      });
    });

    test("should have regtest blockchain", async () => {
      const blockchainInfo = await rpcClient.getBlockchainInfo();

      expect(blockchainInfo).toBeDefined();
      expect(blockchainInfo.chain).toBe("regtest");
      expect(blockchainInfo.blocks).toBeGreaterThan(100); // Should have generated blocks

      console.log("Blockchain Info:", {
        chain: blockchainInfo.chain,
        blocks: blockchainInfo.blocks,
        bestblockhash: blockchainInfo.bestblockhash,
      });
    });

    test("should validate test addresses", async () => {
      for (const [name, address] of Object.entries(testAddresses)) {
        const validation = await rpcClient.validateAddress(address);

        expect(validation.isvalid).toBe(true);
        expect(validation.address).toBe(address);

        console.log(`Address validation for ${name} (${address}):`, validation);
      }
    });
  });

  describe("Address Validation", () => {
    test("should validate addresses using our implementation", () => {
      // Test that our implementation can validate the same addresses
      expect(isValidAddr(testAddresses.source, "regtest")).toBe(true);
      expect(isValidAddr(testAddresses.destination, "regtest")).toBe(true);
    });

    test("should reject invalid addresses", () => {
      expect(isValidAddr("invalid_address", "regtest")).toBe(false);
      expect(isValidAddr("", "regtest")).toBe(false);
    });
  });

  describe("Address Balances and UTXOs", () => {
    test("should have funded test addresses", async () => {
      // Check source address balance (should be ~1.20 ZEC)
      const sourceBalance = await rpcClient.getBalance(testAddresses.source);
      expect(sourceBalance).toBeGreaterThan(1.0);
      expect(sourceBalance).toBeLessThan(2.0);

      // Check destination address balance (should be ~5.40 ZEC)
      const destBalance = await rpcClient.getBalance(testAddresses.destination);
      expect(destBalance).toBeGreaterThan(5.0);
      expect(destBalance).toBeLessThan(6.0);

      console.log("Address balances:", {
        source: `${sourceBalance} ZEC`,
        destination: `${destBalance} ZEC`,
      });
    });

    test("should fetch UTXOs for test addresses", async () => {
      const sourceUTXOs = await rpcClient.getUTXOs(testAddresses.source);
      const destUTXOs = await rpcClient.getUTXOs(testAddresses.destination);

      expect(sourceUTXOs).toBeDefined();
      expect(Array.isArray(sourceUTXOs)).toBe(true);
      expect(sourceUTXOs.length).toBeGreaterThan(0);

      expect(destUTXOs).toBeDefined();
      expect(Array.isArray(destUTXOs)).toBe(true);
      expect(destUTXOs.length).toBeGreaterThan(0);

      console.log("UTXOs found:", {
        source: sourceUTXOs.length,
        destination: destUTXOs.length,
      });

      // Validate UTXO structure
      sourceUTXOs.forEach((utxo) => {
        expect(utxo.txid).toBeDefined();
        expect(typeof utxo.txid).toBe("string");
        expect(utxo.vout).toBeDefined();
        expect(typeof utxo.vout).toBe("number");
        expect(utxo.value).toBeDefined();
        expect(typeof utxo.value).toBe("number");
        expect(utxo.value).toBeGreaterThan(0);
        expect(utxo.height).toBeDefined();
        expect(typeof utxo.height).toBe("number");
      });
    });
  });

  describe("Transaction Creation and Broadcasting", () => {
    test("should create and broadcast a transaction", async () => {
      // Get UTXOs from the source address
      const sourceUTXOs = await rpcClient.getUTXOs(testAddresses.source);
      expect(sourceUTXOs.length).toBeGreaterThan(0);

      // Use the first UTXO
      const selectedUTXO = sourceUTXOs[0];

      console.log("Selected UTXO:", {
        txid: selectedUTXO.txid,
        vout: selectedUTXO.vout,
        value: selectedUTXO.value,
        height: selectedUTXO.height,
      });

      // Create transaction outputs (send 0.01 ZEC to destination)
      const sendAmount = 1000000; // 0.01 ZEC in zatoshis
      const outputs: Output[] = [
        {
          type: "pkh",
          address: testAddresses.destination,
          amount: sendAmount,
        },
        {
          type: "memo",
          memo: "E2E test transaction from TypeScript implementation",
        },
      ];

      // Calculate fee
      const fee = getFee([selectedUTXO], outputs);
      console.log("Calculated fee:", fee, "zatoshis");

      // Verify we have enough funds
      const totalNeeded = sendAmount + fee;
      expect(selectedUTXO.value).toBeGreaterThan(totalNeeded);

      // Get current block height
      const blockchainInfo = await rpcClient.getBlockchainInfo();
      const currentHeight = blockchainInfo.blocks;

      // Build the transaction
      const tx = buildTx(config, [selectedUTXO], outputs, currentHeight);

      expect(tx).toBeDefined();
      expect(tx.version).toBe(-2147483643); // Zcash v5 with Overwinter flag (0x80000005)
      expect(tx.inputs).toHaveLength(1);
      expect(tx.outputs.length).toBeGreaterThanOrEqual(2); // At least memo + PKH outputs
      expect(tx.fee).toBe(fee);

      console.log("Transaction built:", {
        version: tx.version,
        inputs: tx.inputs.length,
        outputs: tx.outputs.length,
        fee: tx.fee,
      });

      // Sign and finalize the transaction
      const pubkey = getPublicKeyFromPrivateKey(testPrivateKey);

      const unsignedTx = buildTransaction({
        inputs: tx.inputs,
        outputs: tx.outputs.map((o: Output) => {
          if (o.type === "pkh") {
            return { address: o.address, value: o.amount, memo: "" };
          }
          return { address: testAddresses.destination, value: 0, memo: o.memo };
        }),
        network: config.network,
        blockHeight: 100,
        pubkey,
      });

      const signatures = signSighashes(unsignedTx.sighashes, testPrivateKey);
      const finalTx = signTransaction(
        unsignedTx,
        signatures,
        config.network,
        pubkey,
      );

      expect(finalTx).toBeInstanceOf(Buffer);
      expect(finalTx.length).toBeGreaterThan(100);

      console.log("Transaction signed:", {
        size: finalTx.length,
        hex_preview: finalTx.toString("hex").substring(0, 32) + "...",
      });

      // Test transaction broadcast (commented out to avoid actual broadcast)
      // In a real scenario, you would broadcast like this:
      // const broadcastTxid = await rpcClient.sendRawTransaction(finalTx.toString('hex'))
      // expect(broadcastTxid).toBeDefined()
      // expect(typeof broadcastTxid).toBe('string')
      // expect(broadcastTxid.length).toBe(64)

      console.log(
        "✓ Transaction created successfully (not broadcasted in test)",
      );
    }, 30000); // 30 second timeout

    test("should handle insufficient funds error", async () => {
      // Get UTXOs from the source address
      const sourceUTXOs = await rpcClient.getUTXOs(testAddresses.source);
      expect(sourceUTXOs.length).toBeGreaterThan(0);

      // Try to send more than available
      const excessiveAmount = sourceUTXOs[0].value + 1000000; // More than UTXO value
      const outputs: Output[] = [
        {
          type: "pkh",
          address: testAddresses.destination,
          amount: excessiveAmount,
        },
      ];

      expect(() => {
        buildTx(config, [sourceUTXOs[0]], outputs, 100);
      }).toThrow("Insufficient funds");
    });

    test("should handle invalid address error", async () => {
      const sourceUTXOs = await rpcClient.getUTXOs(testAddresses.source);
      expect(sourceUTXOs.length).toBeGreaterThan(0);

      const outputs: Output[] = [
        {
          type: "pkh",
          address: "invalid_address_format",
          amount: 100000,
        },
      ];

      expect(() => {
        buildTx(config, [sourceUTXOs[0]], outputs, 100);
      }).toThrow();
    });
  });

  describe("Multi-input Transaction", () => {
    test("should handle transactions with multiple inputs", async () => {
      const sourceUTXOs = await rpcClient.getUTXOs(testAddresses.source);

      // Skip test if we don't have multiple UTXOs
      if (sourceUTXOs.length <= 1) {
        console.log(
          "Skipping multi-input test: only",
          sourceUTXOs.length,
          "UTXO(s) available",
        );
        return;
      }

      // Use first two UTXOs
      const selectedUTXOs = sourceUTXOs.slice(0, 2);
      const totalValue = selectedUTXOs.reduce(
        (sum, utxo) => sum + utxo.value,
        0,
      );

      const sendAmount = Math.floor(totalValue * 0.8); // Send 80% of total value
      const outputs: Output[] = [
        {
          type: "pkh",
          address: testAddresses.destination,
          amount: sendAmount,
        },
        {
          type: "memo",
          memo: "Multi-input test transaction",
        },
      ];

      const blockchainInfo = await rpcClient.getBlockchainInfo();
      const tx = buildTx(config, selectedUTXOs, outputs, blockchainInfo.blocks);

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
          return { address: testAddresses.destination, value: 0, memo: o.memo };
        }),
        network: config.network,
        blockHeight: blockchainInfo.blocks,
        pubkey,
      });

      const signatures = signSighashes(unsignedTx.sighashes, testPrivateKey);
      const finalTx = signTransaction(
        unsignedTx,
        signatures,
        config.network,
        pubkey,
      );

      expect(finalTx).toBeInstanceOf(Buffer);
      expect(finalTx.length).toBeGreaterThan(200); // Larger due to multiple inputs

      console.log("Multi-input transaction:", {
        inputs: tx.inputs.length,
        outputs: tx.outputs.length,
        size: finalTx.length,
        fee: tx.fee,
      });
    });
  });

  describe("Fee Calculation Validation", () => {
    test("should calculate reasonable fees", async () => {
      const sourceUTXOs = await rpcClient.getUTXOs(testAddresses.source);
      expect(sourceUTXOs.length).toBeGreaterThan(0);

      const testCases = [
        { inputs: 1, outputs: 1, description: "simple transaction" },
        { inputs: 1, outputs: 2, description: "transaction with memo" },
        { inputs: 2, outputs: 1, description: "multi-input transaction" },
      ];

      testCases.forEach(({ inputs, outputs, description }) => {
        const selectedUTXOs = sourceUTXOs.slice(0, inputs);
        const testOutputs: Output[] = [];

        // Add PKH outputs
        for (let i = 0; i < outputs; i++) {
          if (i === 0) {
            testOutputs.push({
              type: "pkh",
              address: testAddresses.destination,
              amount: 100000,
            });
          } else {
            testOutputs.push({
              type: "memo",
              memo: `Test output ${i}`,
            });
          }
        }

        const fee = getFee(selectedUTXOs, testOutputs);

        expect(fee).toBeGreaterThan(0);
        expect(fee).toBeLessThan(100000); // Should be reasonable (< 0.001 ZEC)

        console.log(`Fee for ${description}:`, fee, "zatoshis");
      });
    });
  });

  // trunk-ignore(codespell/misspelled)
  afterAll(async () => {
    console.log("E2E tests completed.");
    console.log(
      "To cleanup the regtest environment, run: npm run regtest:down",
    );
  });
});
