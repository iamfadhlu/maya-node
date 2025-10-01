#!/bin/sh
set -x

zcashd \
  -regtest=1 \
  -txindex \
  -nuparams=c8e71055:1 \
  -minetolocalwallet=0 \
  -experimentalfeatures=1 \
  -lightwalletd=1 \
  -rpcallowip=0.0.0.0/0 \
  -rpcbind=0.0.0.0 \
  -rpcbind="$(hostname)" &

echo "Generating initial blocks..."
# give time to zcashd to start
INIT_BLOCKS=150
while true; do
  zcash-cli -regtest generate "$INIT_BLOCKS" && break
  sleep 5
done

# Create a new account
echo "Creating new Zcash account..."
zcash-cli -regtest -rpcport=18232 z_getnewaccount

# Get a unified address
echo "Getting unified address..."
UA=$(zcash-cli -regtest -rpcport=18232 listaddresses | jq -r '.[0].unified[0].addresses[0].address')
echo "Unified Address: $UA"

# Shield coinbase to unified address
echo "Shielding coinbase to unified address..."
zcash-cli -regtest -rpcport=18232 z_shieldcoinbase '*' "$UA"
sleep 5

# Check operation result
zcash-cli -regtest -rpcport=18232 z_getoperationresult

# Generate more blocks
zcash-cli -regtest -rpcport=18232 generate 10
sleep 1

# Send funds to test addresses
echo "Sending funds to test addresses..."

# Send to first test address (tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6)
zcash-cli -regtest -rpcport=18232 z_sendmany "$UA" '[{"address": "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6", "amount": 5.40}]' 1 null 'AllowRevealedRecipients'
sleep 5
zcash-cli -regtest -rpcport=18232 z_getoperationresult

# Generate blocks to confirm
zcash-cli -regtest -rpcport=18232 generate 10
sleep 1

# Send to second test address (tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU)
zcash-cli -regtest -rpcport=18232 z_sendmany "$UA" '[{"address": "tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU", "amount": 1.20}]' 1 null 'AllowRevealedRecipients'
sleep 5
zcash-cli -regtest -rpcport=18232 z_getoperationresult

# Generate final blocks
zcash-cli -regtest -rpcport=18232 generate 10

echo "Regtest setup complete!"
echo "Test addresses funded:"
echo "  tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6: 5.40 ZEC"
echo "  tmP9jLgTnhDdKdWJCm4BT2t6acGnxqP14yU: 1.20 ZEC"
echo "RPC available at localhost:18232"

# Keep the container running by waiting for the background zcashd process
wait
