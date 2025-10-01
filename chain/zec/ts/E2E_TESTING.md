# End-to-End Testing with Zcash Regtest

This guide explains how to run comprehensive E2E tests against a real Zcash regtest network.

## Prerequisites

- Docker and Docker Compose installed
- Node.js and npm installed
- Built NAPI bindings (`npm run build`)

## Quick Start

```bash
# 1. Start the regtest network
npm run regtest:up

# 2. Wait for setup (check logs)
npm run regtest:logs

# 3. Run E2E tests
npm run test:e2e

# 4. Cleanup when done
npm run regtest:down
```

## Detailed Setup

### 1. Starting the Regtest Network

```bash
npm run regtest:up
```

This command:

- Starts a Zcash regtest node in Docker
- Exposes RPC on `localhost:18232`
- Runs the setup script to:
  - Generate 150 initial blocks
  - Create a unified address
  - Shield coinbase rewards
  - Fund test addresses with ZEC:
    - `tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6`: 5.40 ZEC
    - `tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU`: 1.20 ZEC

### 2. Monitoring Setup

```bash
# Watch the setup process
npm run regtest:logs

# Look for this message:
# "Regtest setup complete!"
```

The setup process takes 30-60 seconds. Wait for completion before running tests.

### 3. Running E2E Tests

```bash
# Run all E2E tests
npm run test:e2e

# Run with verbose output
npm run test:e2e -- --verbose

# Run specific test
npm run test:e2e -- --testNamePattern="Network Connection"
```

### 4. Cleanup

```bash
# Stop and remove containers
npm run regtest:down
```

## Test Coverage

The E2E tests cover:

### Network Connection

- ✅ Connect to regtest node
- ✅ Verify regtest blockchain
- ✅ Validate test addresses

### Address and Balance Management

- ✅ Check funded test addresses
- ✅ Fetch UTXOs from addresses
- ✅ Validate UTXO structure

### NAPI Library Integration

- ✅ Address validation via NAPI
- ✅ Output viewing key generation
- ✅ Library initialization

### Transaction Creation

- ✅ Build partial transactions
- ✅ Compute transaction IDs
- ✅ Apply signatures (mock)
- ✅ Handle validation errors

### Blockchain Interaction

- ✅ Generate new blocks
- ✅ Fetch transaction details
- ✅ RPC communication

## Test Configuration

The tests use this configuration:

```typescript
const config: ZcashConfig = {
  server: {
    host: "http://localhost:18232",
    user: "mayachain",
    password: "password",
  },
  mainnet: false,
};
```

## Test Addresses

Pre-funded addresses used in tests:

| Address                               | Balance  | Purpose                 |
| ------------------------------------- | -------- | ----------------------- |
| `tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU` | 1.20 ZEC | Transaction source      |
| `tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6` | 5.40 ZEC | Transaction destination |

## Troubleshooting

### Node Not Ready

```bash
Error: Zcash regtest node is not ready
```

**Solution**: Wait longer for setup, check logs with `npm run regtest:logs`

### Port Already in Use

```bash
Error: Port 18232 already in use
```

**Solution**: Stop existing containers with `npm run regtest:down`

### Docker Issues

```bash
Error: Cannot connect to Docker daemon
```

**Solution**: Ensure Docker is running, check `docker ps`

### Connection Refused

```bash
Error: ECONNREFUSED localhost:18232
```

**Solution**:

1. Check container is running: `docker ps`
2. Check logs: `npm run regtest:logs`
3. Restart: `npm run regtest:down && npm run regtest:up`

### Test Timeouts

```bash
Timeout - Async callback was not invoked
```

**Solution**: Increase Jest timeout or ensure regtest setup completed

## Manual Testing

You can also interact with the regtest node manually:

```bash
# Connect to container
docker exec -it zcash-regtest-e2e bash

# Run zcash-cli commands
zcash-cli -datadir=/opt/zcash/data/regtest -rpcport=18232 getinfo
zcash-cli -datadir=/opt/zcash/data/regtest -rpcport=18232 getblockchaininfo
zcash-cli -datadir=/opt/zcash/data/regtest -rpcport=18232 getaddressbalance tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU
```

## Integration with CI/CD

For CI environments, you can run tests in a single command:

```bash
# Start, test, and cleanup in one go
npm run regtest:up && npm run test:e2e && npm run regtest:down
```

Or use a timeout for safety:

```bash
timeout 300 npm run regtest:up && npm run test:e2e && npm run regtest:down
```

## Development Workflow

1. **Development**: Use mock tests for fast iteration

   ```bash
   npm test -- src/__tests__/mock.test.ts
   ```

2. **Integration**: Run unit and integration tests

   ```bash
   npm test -- --testPathIgnorePatterns=regtest-e2e
   ```

3. **E2E Validation**: Run full E2E tests before releases

   ```bash
   npm run regtest:up && npm run test:e2e && npm run regtest:down
   ```

## Performance Notes

- **Setup time**: 30-60 seconds
- **Test execution**: 10-30 seconds
- **Total time**: ~90 seconds
- **Resources**: ~500MB RAM, minimal CPU

The regtest network is lightweight and perfect for testing without mainnet/testnet dependencies.
