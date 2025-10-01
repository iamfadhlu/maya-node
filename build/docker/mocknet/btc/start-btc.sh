#!/bin/sh

SIGNER_NAME="${SIGNER_NAME:=mayachain}"
SIGNER_PASSWD="${SIGNER_PASSWD:=password}"
MASTER_ADDR="${BTC_MASTER_ADDR:=bcrt1qf4l5dlqhaujgkxxqmug4stfvmvt58vx2h44c39}"
BLOCK_TIME=${BLOCK_TIME:=1}
alias btc='bitcoin-cli -regtest -rpcuser="$SIGNER_NAME" -rpcpassword="$SIGNER_PASSWD"'

send_btc() {
  sender="$1"
  recipient="$2"
  amount="$3"
  fee="$4"

  # Check balance
  balance=$(btc getbalance "*" 1)
  echo "New wallet balance: $balance"

  # Now create the transaction using the wallet
  # The sendtoaddress command has this syntax:
  # sendtoaddress "address" amount ( "comment" "comment_to" subtractfeefromamount replaceable conf_target "estimate_mode" fee_rate avoid_reuse )
  echo "Creating and sending transaction..."
  # Get unspent outputs
  unspent=$(btc listunspent 1 9999)
  # If balance is not at least 2 BTC exit with an error
  txid=$(echo "$unspent" | jq -r '.[0].txid')
  vout=$(echo "$unspent" | jq -r '.[0].vout')
  input_amount=$(echo "$unspent" | jq -r '.[0].amount')

  # Calculate change
  change=$(awk -v input="$input_amount" -v amt="$amount" -v f="$fee" 'BEGIN {printf "%.8f", input - amt - f}')

  # Create and sign transaction
  raw_tx=$(btc createrawtransaction "[{\"txid\": \"$txid\",\"vout\": $vout}]" "{\"$recipient\": $amount, \"$sender\": $change, \"data\": \"6e6f6f70\" }")

  signed_tx=$(btc signrawtransactionwithwallet "$raw_tx" | jq -r '.hex')
  tx_id=$(btc sendrawtransaction "$signed_tx")

  echo "Transaction sent: $tx_id"

  return 0
}

bitcoind -regtest -txindex -rpcuser="$SIGNER_NAME" -rpcpassword="$SIGNER_PASSWD" -rpcallowip=0.0.0.0/0 -rpcbind=127.0.0.1 -rpcbind="$(hostname)" -deprecatedrpc=create_bdb &

# give time to bitcoind to start
while true; do
  btc generatetoaddress 100 "$MASTER_ADDR" && break
  sleep 5
done

btc createwallet "" false false "" false false false
sender=$(btc getnewaddress "" "bech32")
btc generatetoaddress 100 "$sender"
btc importaddress "$sender" "" true
echo "Sender is $sender"

# mine a new block every BLOCK_TIME
block=1
while true; do
  echo "------------------------------ BLOCK: $block ------------------------------"
  if [ $block -eq 10 ]; then
    send_btc "$sender" "bcrt1qzf3gsk7edzwl9syyefvfhle37cjtql35tlzesk" 1.0 0.00025
  fi
  btc generatetoaddress 1 "$MASTER_ADDR"
  sleep "$BLOCK_TIME"

  block=$((block + 1))
done
