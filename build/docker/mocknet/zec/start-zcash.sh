#!/bin/sh

SIGNER_NAME="${SIGNER_NAME:=mayachain}"
SIGNER_PASSWD="${SIGNER_PASSWD:=password}"
ZCASH_MASTER_ADDR="${ZCASH_MASTER_ADDR:=tmGn7qKhnpbnwZPsaPt9XXAQz1Q3HAUCrwh}"
BLOCK_TIME=${BLOCK_TIME:=1}
alias zcash='zcash-cli -regtest -rpcuser="$SIGNER_NAME" -rpcpassword="$SIGNER_PASSWD"'

# TODO: Review usage
# -insightexplorer=1 \

#  For details see
#  https://zcash.github.io/zcash/dev/regtest.html
#  https://github.com/zcash/zcash/blob/master/src/consensus/upgrades.cpp
zcashd \
  -regtest=1 \
  -txindex \
  -nuparams=c8e71055:1 \
  -mineraddress="$ZCASH_MASTER_ADDR" \
  -minetolocalwallet=0 \
  -experimentalfeatures=1 \
  -lightwalletd=1 \
  -rpcuser="$SIGNER_NAME" \
  -rpcpassword="$SIGNER_PASSWD" \
  -rpcallowip=0.0.0.0/0 \
  -rpcbind=0.0.0.0 \
  -rpcbind="$(hostname)" &

# give time to zcashd to start
INIT_BLOCKS=500
while true; do
  zcash generate "$INIT_BLOCKS" && break
  sleep 5
done

# mine a new block every BLOCK_TIME
block=$((INIT_BLOCKS + 1))
while true; do
  echo "------------------------------ BLOCK: $block ------------------------------"
  zcash generate 1
  sleep "$BLOCK_TIME"

  block=$((block + 1))
done
