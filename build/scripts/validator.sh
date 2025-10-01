#!/bin/bash

set -o pipefail

. "$(dirname "$0")/core.sh"

if [ "$NET" = "mocknet" ] || [ "$NET" = "testnet" ]; then
  echo "Loading unsafe init for mocknet and testnet..."
  . "$(dirname "$0")/core-unsafe.sh"
fi

PEER="${PEER:=none}"          # the hostname of a seed node set as tendermint persistent peer
PEER_API="${PEER_API:=$PEER}" # the hostname of a seed node API if different
BINANCE=${BINANCE:=$PEER:26660}

if [ ! -f ~/.mayanode/config/genesis.json ]; then
  echo "Setting MAYANode as Validator node"

  create_maya_user "$SIGNER_NAME" "$SIGNER_PASSWD" "$SIGNER_SEED_PHRASE"

  NODE_ADDRESS=$(echo "$SIGNER_PASSWD" | mayanode keys show "$SIGNER_NAME" -a --keyring-backend file)
  init_chain "$NODE_ADDRESS"
  rm -rf ~/.mayanode/config/genesis.json # set in mayanode render-config

  if [ "$NET" = "mocknet" ]; then
    if [ "$PEER" = "none" ]; then
      echo "Missing PEER"
      exit 1
    fi

    # wait for peer
    until curl -s "$PEER:$PORT_RPC" 1>/dev/null 2>&1; do
      echo "Waiting for peer: $PEER:$PORT_RPC"
      sleep 3
    done

    # create a binance wallet and bond/register
    # gen_bnb_address
    # ADDRESS=$(cat ~/.bond/address.txt)

    # add liquidity for bond
    # until printf "%s\n" "$SIGNER_PASSWD" | mayanode tx mayachain deposit 10000000000 cacao "add:BNB.BNB:$ADDRESS" --node tcp://"$PEER":26657 --from "$SIGNER_NAME" --keyring-backend file --chain-id "$CHAIN_ID" --yes --gas 'auto' >log && grep -v 'out of gas' log; do
    # sleep 5
    # done
    # sleep 40 # wait for thorchain to commit a block , otherwise it get the wrong sequence number

    # until "$(dirname "$0")/mock/add.sh" binance "$PEER" "$ADDRESS" "$NODE_ADDRESS" 1000000000000 BNB; do
    #   sleep 5
    # done
    # sleep 10 # wait for maya to process txin

    # bond our liquidity
    SIGNER_ADDRESS=$(echo "$SIGNER_PASSWD" | mayanode keys show "$SIGNER_NAME" -a --keyring-backend file)
    until printf "%s\n" "$SIGNER_PASSWD" | mayanode tx mayachain deposit 100000000 cacao "bond:btc.btc:10000000000:$SIGNER_ADDRESS" --node tcp://"$PEER":26657 --from "$SIGNER_NAME" --keyring-backend=file --chain-id "$CHAIN_ID" --yes --gas 'auto' >log && grep -v 'out of gas' log; do
      sleep 5
    done
    sleep 10 # wait for thorchain to commit a block , otherwise it get the wrong sequence number

    NODE_PUB_KEY=$(echo "$SIGNER_PASSWD" | mayanode keys show "$SIGNER_NAME" --pubkey --keyring-backend=file | mayanode pubkey)
    VALIDATOR=$(mayanode tendermint show-validator | mayanode pubkey --bech cons)

    # set node keys
    NODE_PUB_KEY_ED25519=$(echo "password" | mayanode ed25519)
    until printf "%s\n" "$SIGNER_PASSWD" | mayanode tx mayachain set-node-keys "$NODE_PUB_KEY" "$NODE_PUB_KEY_ED25519" "$VALIDATOR" --node tcp://"$PEER":26657 --from "$SIGNER_NAME" --keyring-backend=file --chain-id "$CHAIN_ID" --yes; do
      sleep 5
    done
    sleep 10 # wait for thorchain to commit a block

    IP=$(ifconfig eth0 | grep -o -E 'addr:([0-9]{1,3}\.){3}[0-9]{1,3}')
    NODE_IP_ADDRESS=${IP#*addr:}

    EXTERNAL_IP=$NODE_IP_ADDRESS
    echo "This is your ip: $EXTERNAL_IP"

    until printf "%s\n" "$SIGNER_PASSWD" | mayanode tx mayachain set-ip-address "$NODE_IP_ADDRESS" --node tcp://"$PEER":26657 --from "$SIGNER_NAME" --keyring-backend=file --chain-id "$CHAIN_ID" --yes; do
      sleep 5
    done

    sleep 10 # wait for thorchain to commit a block
    # set node version
    until printf "%s\n" "$SIGNER_PASSWD" | mayanode tx mayachain set-version --node tcp://"$PEER":26657 --from "$SIGNER_NAME" --keyring-backend=file --chain-id "$CHAIN_ID" --yes; do
      sleep 5
    done

  else
    echo "Your MAYANode address: $NODE_ADDRESS"
    echo "Send your bond to that address"
  fi
fi

# render tendermint and cosmos configuration files
mayanode render-config

export SIGNER_NAME
export SIGNER_PASSWD
exec mayanode start
