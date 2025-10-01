#!/bin/bash

set -o pipefail

# default ulimit is set too low for mayanode in some environments
ulimit -n 65535

PORT_P2P=26656
PORT_RPC=26657
[ "$NET" = "mainnet" ] && PORT_P2P=27146 && PORT_RPC=27147
[ "$NET" = "stagenet" ] && PORT_P2P=27146 && PORT_RPC=27147
export PORT_P2P PORT_RPC

# validate required environment
if [ -z "$SIGNER_NAME" ]; then
  echo "SIGNER_NAME must be set"
  exit 1
fi
if [ -z "$SIGNER_PASSWD" ]; then
  echo "SIGNER_PASSWD must be set"
  exit 1
fi

# adds an account node into the genesis file
add_node_account() {
  NODE_ADDRESS=$1
  VALIDATOR=$2
  NODE_PUB_KEY=$3
  VERSION=$4
  BOND_ADDRESS=$5
  NODE_PUB_KEY_ED25519=$6
  IP_ADDRESS=$7
  MEMBERSHIP=$8

  # Add node
  jq --arg IP_ADDRESS "$IP_ADDRESS" --arg VERSION "$VERSION" --arg BOND_ADDRESS "$BOND_ADDRESS" --arg VALIDATOR "$VALIDATOR" --arg NODE_ADDRESS "$NODE_ADDRESS" --arg NODE_PUB_KEY "$NODE_PUB_KEY" --arg NODE_PUB_KEY_ED25519 "$NODE_PUB_KEY_ED25519" '.app_state.mayachain.node_accounts += [ { "node_address": $NODE_ADDRESS, "version": $VERSION, "ip_address": $IP_ADDRESS, "status": "Active", "bond":"100000000", "active_block_height": "0", "bond_address":$BOND_ADDRESS, "signer_membership": [], "validator_cons_pub_key":$VALIDATOR, "pub_key_set":{"secp256k1":$NODE_PUB_KEY, "ed25519":$NODE_PUB_KEY_ED25519 } }]' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.mayanode/config/genesis.json
  if [ -n "$MEMBERSHIP" ]; then
    jq --arg MEMBERSHIP "$MEMBERSHIP" '.app_state.mayachain.node_accounts[-1].signer_membership += [$MEMBERSHIP]' ~/.mayanode/config/genesis.json >/tmp/genesis.json
    mv /tmp/genesis.json ~/.mayanode/config/genesis.json
  fi

  # Add bond provider
  jq --arg NODE_ADDRESS "$NODE_ADDRESS" --arg BOND_ADDRESS "$BOND_ADDRESS" '.app_state.mayachain.bond_providers += [
   {
     "node_address": $NODE_ADDRESS,
     "node_operator_fee": "0",
     "providers": [
       {
         "bond_address": $BOND_ADDRESS,
         "bonded": true
       }
     ]
   }
 ]' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.mayanode/config/genesis.json
}

reserve() {
  jq --arg RESERVE "$1" '.app_state.mayachain.reserve = $RESERVE' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.mayanode/config/genesis.json
}

asgard() {
  jq --arg ASGARD "$1" '.app_state.mayachain.asgard = $ASGARD' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.mayanode/config/genesis.json
}

disable_mint() {
  jq '.app_state.mint = {"minter":{"annual_provisions":"0.0","inflation":"0.0"},"params":{"blocks_per_year":"5256000","goal_bonded":"0.75000000000000000","inflation_max":"0.0","inflation_min":"0.0","inflation_rate_change":"0.0","mint_denom":"cacao"}}' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.mayanode/config/genesis.json
}

disable_bank_send() {
  jq '.app_state.bank.params.default_send_enabled = false' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.mayanode/config/genesis.json

  jq '.app_state.transfer.params.send_enabled = false' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.mayanode/config/genesis.json
}

add_pool() {
  ASSET=$1
  DECIMALS=$2

  jq --arg ASSET "$ASSET" --arg DECIMALS "$DECIMALS" '.app_state.mayachain.pools += [{
    "asset": $ASSET,
    "balance_cacao":  "0",
    "balance_asset":  "0",
    "LP_units": "0",
    "status": "Available",
    "status_since": "0",
    "decimals": $DECIMALS,
    "synth_units": "0",
    "pending_inbound_cacao": "0",
    "pending_inbound_asset": "0",
}]' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.mayanode/config/genesis.json
}

add_account() {
  ADDRS=$(jq --arg ADDRESS "$1" '.app_state.auth.accounts[] | select(.address == $ADDRESS) .address' <~/.mayanode/config/genesis.json)

  if [ -z "$ADDRS" ]; then
    #If account doesn't exist, create account with asset
    jq --arg ADDRESS "$1" --arg ASSET "$2" --arg AMOUNT "$3" '.app_state.auth.accounts += [{
          "@type": "/cosmos.auth.v1beta1.BaseAccount",
          "address": $ADDRESS,
          "pub_key": null,
          "account_number": "0",
          "sequence": "0"
      }]' <~/.mayanode/config/genesis.json >/tmp/genesis.json
    # "coins": [ { "denom": $ASSET, "amount": $AMOUNT } ],
    mv /tmp/genesis.json ~/.mayanode/config/genesis.json

    jq --arg ADDRESS "$1" --arg ASSET "$2" --arg AMOUNT "$3" '.app_state.bank.balances += [{
          "address": $ADDRESS,
          "coins": [ { "denom": $ASSET, "amount": $AMOUNT } ],
      }]' <~/.mayanode/config/genesis.json >/tmp/genesis.json
    mv /tmp/genesis.json ~/.mayanode/config/genesis.json
  else
    #If account exist, add balance
    PREV_AMOUNT=$(jq --arg ADDRESS "$1" --arg ASSET "$2" '.app_state.bank.balances[] | select(.address == $ADDRESS) .coins[] | select(.denom == $ASSET) .amount' <~/.mayanode/config/genesis.json)
    if [ -z "$PREV_AMOUNT" ]; then
      # Add new balance to address from non-exiting asset
      jq --arg ADDRESS "$1" --arg ASSET "$2" --arg AMOUNT "$3" '.app_state.bank.balances = [(
        .app_state.bank.balances[] | select(.address == $ADDRESS) .coins += [{
        "denom": $ASSET,
        "amount": $AMOUNT
        }])]' <~/.mayanode/config/genesis.json >/tmp/genesis.json
      mv /tmp/genesis.json ~/.mayanode/config/genesis.json
    else
      # Add balance to address from existing asset
      jq --arg ADDRESS "$1" --arg ASSET "$2" --arg AMOUNT "$3" '(.app_state.bank.balances[] | select(.address == $ADDRESS)).coins = [
        .app_state.bank.balances[] | select(.address == $ADDRESS).coins[] | select(.denom == $ASSET).amount += $AMOUNT
        ]' <~/.mayanode/config/genesis.json >/tmp/genesis.json
      mv /tmp/genesis.json ~/.mayanode/config/genesis.json
    fi
  fi
}

add_lp() {
  ASSET=$1
  RUNE_ADDRESS=$2
  ASSET_ADDRESS=$3
  UNITS=$4
  ASSET_AMOUNT=$5
  NODE_ADDRESS=$6

  if [ "$RUNE_ADDRESS" = "$NODE_ADDRESS" ]; then
    if [ "$BOND" = "true" ]; then
      BONDED_NODES=$(jq -n --arg RUNE_ADDRESS "$RUNE_ADDRESS" --arg UNITS "$UNITS" \
        '[{"node_address": $RUNE_ADDRESS, "units": $UNITS}]')
    else
      BONDED_NODES='[]'
    fi

    jq --arg ASSET "$ASSET" --arg RUNE_ADDRESS "$RUNE_ADDRESS" --arg ASSET_ADDRESS "$ASSET_ADDRESS" --arg UNITS "$UNITS" --arg ASSET_AMOUNT "$ASSET_AMOUNT" --arg NODE_ADDRESS "$NODE_ADDRESS" --argjson BONDED_NODES "$BONDED_NODES" \ '.app_state.mayachain.liquidity_providers += [
    {
      "asset": $ASSET,
      "cacao_address": $RUNE_ADDRESS,
      "asset_address": $ASSET_ADDRESS,
      "last_add_height": "1",
      "last_withdraw_height": "0",
      "units": $UNITS,
      "pending_cacao": "0",
      "pending_asset": "0",
      "pending_tx_Id": "",
      "cacao_deposit_value": $UNITS,
      "asset_deposit_value": $ASSET_AMOUNT,
      "node_bond_address": null,
      "withdraw_counter": "0",
      "last_withdraw_counter_height": "0",
      "bonded_nodes": $BONDED_NODES
    }]' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  else
    jq --arg ASSET "$ASSET" --arg RUNE_ADDRESS "$RUNE_ADDRESS" --arg ASSET_ADDRESS "$ASSET_ADDRESS" --arg UNITS "$UNITS" --arg ASSET_AMOUNT "$ASSET_AMOUNT" --arg NODE_ADDRESS "$NODE_ADDRESS" \ '.app_state.mayachain.liquidity_providers += [
    {
      "asset": $ASSET,
      "cacao_address": $RUNE_ADDRESS,
      "asset_address": $ASSET_ADDRESS,
      "last_add_height": "1",
      "last_withdraw_height": "0",
      "units": $UNITS,
      "pending_cacao": "0",
      "pending_asset": "0",
      "pending_tx_Id": "",
      "cacao_deposit_value": $UNITS,
      "asset_deposit_value": $ASSET_AMOUNT,
      "node_bond_address": null,
      "withdraw_counter": "0",
      "last_withdraw_counter_height": "0"
    }]' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  fi
  mv /tmp/genesis.json ~/.mayanode/config/genesis.json

  # Add balance to liquidity pool
  jq --arg ASSET "$ASSET" --arg UNITS "$UNITS" '(.app_state.mayachain.pools[] | select(.asset == $ASSET) | .balance_cacao) = ((.app_state.mayachain.pools[] | select(.asset == $ASSET) | .balance_cacao | tonumber)+($UNITS|tonumber) | tostring)' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.mayanode/config/genesis.json

  # Add asset balance to pool
  jq --arg ASSET "$ASSET" --arg ASSET_AMOUNT "$ASSET_AMOUNT" '(.app_state.mayachain.pools[] | select(.asset == $ASSET) | .balance_asset) = ((.app_state.mayachain.pools[] | select(.asset == $ASSET) | .balance_asset | tonumber)+($ASSET_AMOUNT|tonumber) | tostring)' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.mayanode/config/genesis.json

  # Add balance to liquidity pool units
  jq --arg ASSET "$ASSET" --arg UNITS "$UNITS" '(.app_state.mayachain.pools[] | select(.asset == $ASSET) | .LP_units) = ((.app_state.mayachain.pools[] | select(.asset == $ASSET) | .LP_units | tonumber)+($UNITS|tonumber) | tostring)' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.mayanode/config/genesis.json
}

add_mayaname() {
  NAME=$1
  EXPIRE=$2
  OWNER=$3
  ADDRESS=$4
  CHAIN=$5

  # Check if mayanames array exist
  MAYANAMES=$(jq '.app_state.mayachain.mayanames' <~/.mayanode/config/genesis.json)
  if [ -z "$MAYANAMES" ]; then
    jq --arg NAME "$NAME" --arg EXPIRE "$EXPIRE" --arg OWNER "$OWNER" --arg ADDRESS "$ADDRESS" --arg CHAIN "$CHAIN" '.app_state.mayachain.mayanames = [
        {
          "name": $NAME,
          "expire_block_height": $EXPIRE,
          "owner": $OWNER,
          "aliases": [
            {"address": $ADDRESS, "chain": $CHAIN}
          ]
        }
      ]' <~/.mayanode/config/genesis.json >/tmp/genesis.json
  else
    # Check if mayaname already exists
    NAMECHECK=$(jq --arg NAME "$1" 'try .app_state.mayachain.mayanames[] | select(.name == $NAME) .name' <~/.mayanode/config/genesis.json)
    if [ -z "$NAMECHECK" ]; then
      jq --arg NAME "$NAME" --arg EXPIRE "$EXPIRE" --arg OWNER "$OWNER" --arg ADDRESS "$ADDRESS" --arg CHAIN "$CHAIN" '.app_state.mayachain.mayanames += [
        {
          "name": $NAME,
          "expire_block_height": $EXPIRE,
          "owner": $OWNER,
          "aliases": [
            {"address": $ADDRESS, "chain": $CHAIN}
          ]
        }
      ]' <~/.mayanode/config/genesis.json >/tmp/genesis.json
    else
      jq --arg NAME "$NAME" --arg ADDRESS "$ADDRESS" --arg CHAIN "$CHAIN" '.app_state.mayachain.mayanames = [(.app_state.mayachain.mayanames[] | select(.name == $NAME) .aliases += [{"address": $ADDRESS, "chain": $CHAIN}])]' <~/.mayanode/config/genesis.json >/tmp/genesis.json
    fi
  fi
  mv /tmp/genesis.json ~/.mayanode/config/genesis.json
}

# inits a thorchain with a command separate list of usernames
init_chain() {
  OLD_IFS=IFS
  IFS=","

  echo "Init chain"
  mayanode init local --chain-id "$CHAIN_ID"
  echo "$SIGNER_PASSWD" | mayanode keys list --keyring-backend file

  for user in "$@"; do # iterate over our list of comma separated users "alice,jack"
    mayanode add-genesis-account "$user" 100000000cacao
  done

  IFS=OLD_IFS
}

fetch_genesis() {
  echo "Fetching genesis from $1:$PORT_RPC"
  until
    curl -s "$1:$PORT_RPC" 1>/dev/null 2>&1
  do
    sleep 3
  done
  curl -s "$1:$PORT_RPC/genesis" | jq .result.genesis >~/.mayanode/config/genesis.json
}

fetch_genesis_from_seeds() {
  OLD_IFS=$IFS
  IFS=","
  for SEED in $1; do
    echo "Fetching genesis from seed $SEED"
    curl -sL --fail -m 30 "$SEED:$PORT_RPC/genesis" | jq .result.genesis >~/.mayanode/config/genesis.json || continue
    break
  done
  IFS=$OLD_IFS
}

fetch_node_id() {
  until
    curl -s "$1:$PORT_RPC" 1>/dev/null 2>&1
  do
    sleep 3
  done
  curl -s "$1:$PORT_RPC/status" | jq -r .result.node_info.id
}

set_node_keys() {
  SIGNER_NAME="$1"
  SIGNER_PASSWD="$2"
  PEER="$3"
  NODE_PUB_KEY="$(echo "$SIGNER_PASSWD" | mayanode keys show thorchain --pubkey --keyring-backend file | mayanode pubkey)"
  NODE_PUB_KEY_ED25519="$(mayanode ed25519)"
  VALIDATOR="$(mayanode tendermint show-validator | mayanode pubkey --bech cons)"
  echo "Setting MAYANode keys"
  printf "%s\n%s\n" "$SIGNER_PASSWD" "$SIGNER_PASSWD" | mayanode tx mayachain set-node-keys "$NODE_PUB_KEY" "$NODE_PUB_KEY_ED25519" "$VALIDATOR" --node "tcp://$PEER:$PORT_RPC" --from "$SIGNER_NAME" --yes
}

set_ip_address() {
  SIGNER_NAME="$1"
  SIGNER_PASSWD="$2"
  PEER="$3"
  NODE_IP_ADDRESS="${4:-$(curl -s http://whatismyip.akamai.com)}"
  echo "Setting MAYANode IP address $NODE_IP_ADDRESS"
  printf "%s\n%s\n" "$SIGNER_PASSWD" "$SIGNER_PASSWD" | mayanode tx mayachain set-ip-address "$NODE_IP_ADDRESS" --node "tcp://$PEER:$PORT_RPC" --from "$SIGNER_NAME" --yes
}

fetch_version() {
  mayanode query mayachain version --output json | jq -r .version
}

create_maya_user() {
  SIGNER_NAME="$1"
  SIGNER_PASSWD="$2"
  SIGNER_SEED_PHRASE="$3"

  echo "Checking if MAYANode Thor '$SIGNER_NAME' account exists"
  echo "$SIGNER_PASSWD" | mayanode keys show "$SIGNER_NAME" --keyring-backend file 1>/dev/null 2>&1
  # shellcheck disable=SC2181
  if [ $? -ne 0 ]; then
    echo "Creating MAYANode Thor '$SIGNER_NAME' account"
    if [ -n "$SIGNER_SEED_PHRASE" ]; then
      printf "%s\n%s\n%s\n" "$SIGNER_SEED_PHRASE" "$SIGNER_PASSWD" "$SIGNER_PASSWD" | mayanode keys --keyring-backend file add "$SIGNER_NAME" --recover
    else
      sig_pw=$(printf "%s\n%s\n" "$SIGNER_PASSWD" "$SIGNER_PASSWD")
      RESULT=$(echo "$sig_pw" | mayanode keys --keyring-backend file add "$SIGNER_NAME" --output json 2>&1)
      SIGNER_SEED_PHRASE=$(echo "$RESULT" | jq -r '.mnemonic')
    fi
  fi
  NODE_PUB_KEY_ED25519=$(printf "%s\n" "$SIGNER_SEED_PHRASE" | mayanode ed25519)
}
