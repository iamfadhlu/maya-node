// Jest setup file for global test configuration

// Mock the NAPI bindings for testing
jest.mock(
  "../zec.linux-x64-gnu.node",
  () => ({
    buildPartialTx: jest.fn(
      (_vaultPubkey: Buffer, partialTx: any, _network: string) => {
        // Mock implementation
        return {
          ...partialTx,
          sighashes: [Buffer.alloc(32, 1), Buffer.alloc(32, 2)],
        };
      },
    ),
    applyTxSignatures: jest.fn(
      (
        _vaultPubkey: Buffer,
        _partialTx: any,
        _signatures: Buffer[],
        _network: string,
      ) => {
        // Mock implementation - return a fake transaction
        return Buffer.alloc(250, 0);
      },
    ),
    getOutputViewingKey: jest.fn((_vaultPubkey: Buffer) => {
      // Mock implementation
      return Buffer.alloc(32, 0);
    }),
  }),
  { virtual: true },
);
