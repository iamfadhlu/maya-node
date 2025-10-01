import {
  isValidAddr,
  skToAddr,
  pkToAddr,
  getFee,
  Network,
  ZecError,
  type Config,
  type UTXO,
  type Output,
} from "../index";

describe("Basic Zcash Library Functions", () => {
  describe("Address Validation", () => {
    test("should validate mainnet addresses", () => {
      // Valid mainnet transparent address (t1...)
      expect(isValidAddr("t1QLmq7VDqCBwq9rfVGcANZTQ3ZhNJ5orA2", "main")).toBe(
        true,
      );
    });

    test("should validate testnet addresses", () => {
      // Valid testnet transparent address (tm...)
      expect(isValidAddr("tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU", "test")).toBe(
        true,
      );
      expect(isValidAddr("tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6", "test")).toBe(
        true,
      );
    });

    test("should validate regtest addresses", () => {
      // Valid regtest addresses (same format as testnet)
      expect(
        isValidAddr("tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU", "regtest"),
      ).toBe(true);
      expect(
        isValidAddr("tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6", "regtest"),
      ).toBe(true);
    });

    test("should reject invalid addresses", () => {
      expect(isValidAddr("invalid_address", "test")).toBe(false);
      expect(isValidAddr("", "test")).toBe(false);
      expect(isValidAddr("1234567890", "main")).toBe(false);
    });

    test("should reject wrong network addresses", () => {
      // Mainnet address on testnet should fail
      expect(isValidAddr("t1YjP7UnrspUQFmEMfMHfMD3eVGFdJhVMhH", "test")).toBe(
        false,
      );

      // Testnet address on mainnet should fail
      expect(isValidAddr("tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU", "main")).toBe(
        false,
      );
    });
  });

  describe("Address Generation", () => {
    test("should generate address from private key", () => {
      // Use a simple hex private key for testing
      const privateKey =
        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";
      const address = skToAddr(privateKey, "test");

      expect(address).toBeDefined();
      expect(typeof address).toBe("string");
      expect(address.startsWith("tm")).toBe(true);
    });

    test("should generate address from public key", () => {
      const publicKey = Buffer.from(
        "03c622fa3be76cd25180d5a61387362181caca77242023be11775134fd37f403f7",
        "hex",
      );
      const address = pkToAddr(publicKey, "test");

      expect(address).toBeDefined();
      expect(typeof address).toBe("string");
      expect(address.startsWith("tm")).toBe(true);
    });
  });

  describe("Fee Calculation", () => {
    test("should calculate fees correctly", () => {
      const mockUTXOs: UTXO[] = [
        {
          txid: "abc123",
          vout: 0,
          value: 1000000, // 0.01 ZEC
          height: 100,
        },
      ];

      const mockOutputs: Output[] = [
        {
          type: "pkh",
          address: "tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU",
          amount: 500000,
        },
      ];

      const fee = getFee(mockUTXOs, mockOutputs);

      expect(typeof fee).toBe("number");
      expect(fee).toBeGreaterThan(0);
      expect(fee).toBeLessThan(100000); // Reasonable fee
    });
  });

  describe("Network enum", () => {
    test("should have correct network values", () => {
      expect(Network.Main).toBe("main");
      expect(Network.Test).toBe("test");
      expect(Network.Regtest).toBe("regtest");
    });
  });

  describe("Error Handling", () => {
    test("ZecError should be properly typed", () => {
      const error = new ZecError("Test error");
      expect(error).toBeInstanceOf(Error);
      expect(error.name).toBe("ZecError");
      expect(error.message).toBe("Test error");
    });
  });
});
