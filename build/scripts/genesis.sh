#!/bin/bash

set -o pipefail

. "$(dirname "$0")/core.sh"

if [ "$NET" = "mocknet" ] || [ "$NET" = "testnet" ] || [ "$NET" = "stagenet" ]; then
  echo "Loading unsafe init for mocknet and testnet..."
  . "$(dirname "$0")/core-unsafe.sh"
  . "$(dirname "$0")/mock/state.sh"
fi

NODES="${NODES:=1}"
SEED="${SEED:=mayanode}" # the hostname of the master node
ETH_HOST="${ETH_HOST:=http://ethereum:8545}"
AVAX_HOST="${AVAX_HOST:=http://avalanche:9650/ext/bc/C/rpc}"
ARB_HOST="${ARB_HOST:=http://arbitrum:8547}"
THOR_BLOCK_TIME="${THOR_BLOCK_TIME:=5s}"
CHAIN_ID=${CHAIN_ID:=mayachain}

# this is required as it need to run mayanode init, otherwise tendermint related command doesn't work
if [ "$SEED" = "$(hostname)" ]; then
  if [ ! -f ~/.mayanode/config/priv_validator_key.json ]; then
    init_chain
    # remove the original generate genesis file, as below will init chain again
    rm -rf ~/.mayanode/config/genesis.json
  fi
fi

create_maya_user "$SIGNER_NAME" "$SIGNER_PASSWD" "$SIGNER_SEED_PHRASE"

VALIDATOR=$(mayanode tendermint show-validator | mayanode pubkey --bech cons)
NODE_ADDRESS=$(echo "$SIGNER_PASSWD" | mayanode keys show "$SIGNER_NAME" -a --keyring-backend file)
NODE_PUB_KEY=$(echo "$SIGNER_PASSWD" | mayanode keys show "$SIGNER_NAME" -p --keyring-backend file | mayanode pubkey)
VERSION=$(fetch_version)

if [ "$SEED" = "$(hostname)" ]; then
  echo "Setting MAYANode as genesis"
  if [ ! -f ~/.mayanode/config/genesis.json ]; then
    NODE_IP_ADDRESS=${EXTERNAL_IP:=$(curl -s http://whatismyip.akamai.com)}

    init_chain "$NODE_ADDRESS"
    add_node_account "$NODE_ADDRESS" "$VALIDATOR" "$NODE_PUB_KEY" "$VERSION" "$NODE_ADDRESS" "$NODE_PUB_KEY_ED25519" "$NODE_IP_ADDRESS"

    # disable default bank transfer, and opt to use our own custom one
    disable_bank_send
    disable_mint

    # for mocknet, add initial balances
    echo "Using NET $NET"
    if [ "$NET" = "mocknet" ]; then
      echo "Adding pool and lp bonder for first node"
      add_pool "BTC.BTC" 8
      # 100K cacao of bond
      add_account "$NODE_ADDRESS" cacao 100000000000000
      add_account "$NODE_ADDRESS" maya 8440000000

      # Uses master account of sims to have a healthy btc pool
      # Since MAYAChain needs to start with a bonded pool for the first liquidity node we have to do the accountability side of the BTC.BTC
      # that will later be sent by the ./mock/start-btc.sh script. This script will send a `noop` to the asgard vault to match the amount of
      # BTC.BTC added here.
      add_lp "BTC.BTC" "$NODE_ADDRESS" "bcrt1qqat4ax3ks7x2l7ah572367u39ukeyd4rv46l3g" 10000000000000 100000000 "$NODE_ADDRESS"

      # founders maya tokens
      add_account tmaya1m3t5wwrpfylss8e9g3jvq5chsv2xl3ucn9rdrf cacao 10000000000000
      add_account tmaya1m3t5wwrpfylss8e9g3jvq5chsv2xl3ucn9rdrf maya 1560000000000

      # simulation master
      add_account tmaya1f4l5dlqhaujgkxxqmug4stfvmvt58vx2t8l2rc cacao 100000000000000000

      # local cluster accounts for genesis nodes
      # TODO Move to simulation framework to do step-by-step testing
      # CAT=tmaya1uuds8pd92qnnq0udw0rpg0szpgcslc9p8gps0z
      # add_account "$CAT" cacao 30000000000 # cat
      # add_lp "BTC.BTC" "$CAT" "" 10000000000 "$NODE_ADDRESS"
      # DOG=tmaya1zf3gsk7edzwl9syyefvfhle37cjtql35hdgtzt
      # add_account "$DOG" cacao 10000000000000000 # dog
      # add_lp "BTC.BTC" "$DOG" "" 10000000000 "$NODE_ADDRESS"
      # FOX=tmaya1qk8c8sfrmfm0tkncs0zxeutc8v5mx3pjjcq6rv
      # add_account "$FOX" cacao 300000000000 # fox
      # add_lp "BTC.BTC" "$FOX" "" 10000000000 "$NODE_ADDRESS"
      # PIG=tmaya13wrmhnh2qe98rjse30pl7u6jxszjjwl4fd6gwn
      # add_account "$PIG" cacao 300000000000 # pig
      # add_lp "BTC.BTC" "$PIG" "" 10000000000 "$NODE_ADDRESS"

      add_mayaname itzamna 5256000 tmaya18z343fsdlav47chtkyp0aawqt6sgxsh3g94648 MAYA
      add_mayaname aaluxx 5256000 tmaya1thtn0cdcu2sujtklknlc9wl2dg09a07v0lgnkr tmaya1thtn0cdcu2sujtklknlc9wl2dg09a07v0lgnkr MAYA
      add_mayaname wr 5256000 tmaya14lkndecaw0zkzu0yq4a0qq869hrs8hh7cqajxy 0x259fe33B59ae7C24B34C0cb957271Ae0f7786878 ETH
      add_mayaname wr 5256000 tmaya14lkndecaw0zkzu0yq4a0qq869hrs8hh7cqajxy tmaya1a427q3v96psuj4fnughdw8glt5r7j38lk7v2wj MAYA
      add_mayaname wr 5256000 tmaya14lkndecaw0zkzu0yq4a0qq869hrs8hh7cqajxy tthor1a427q3v96psuj4fnughdw8glt5r7j38lkfjxcz THOR

      echo "Setting up accounts"
      # smoke test accounts
      add_account tmaya1z63f3mzwv3g75az80xwmhrawdqcjpaekkcgpz9 cacao 500000000000000
      add_account tmaya1wz78qmrkplrdhy37tw0tnvn0tkm5pqd6z6lxzw cacao 2500000000000100
      add_account tmaya18f55frcvknxvcpx2vvpfedvw4l8eutuhkt3sy2 cacao 2500000000000100
      add_account tmaya1xwusttz86hqfuk5z7amcgqsg7vp6g8zhsk2n26 cacao 509000000000000

      reserve 22000000000000000
      asgard 50000000000000

      # deploy evm contracts
      deploy_evm_contracts
      set_eth_contract "0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25"
      set_arb_contract "0xda52b25ddB0e3B9CC393b0690Ac62245Ac772527"
      # deploy_arb_router
      deploy_radix_router
    else
      echo "ETH Contract Address: $ETH_CONTRACT"
      set_eth_contract "$ETH_CONTRACT"
      echo "ARB Contract Address: $ARB_CONTRACT"
      set_arb_contract "$ARB_CONTRACT"
      echo "RADIX Contract Address: $XRD_CONTRACT"
      set_xrd_contract "$XRD_CONTRACT"
    fi

    if [ "$NET" = "mainnet" ]; then
      add_pool "BTC.BTC" 8
      add_lp "BTC.BTC" "$NODE_ADDRESS" "" 1000000000000 "$NODE_ADDRESS"

      echo "Setting up accounts"
      add_account maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz cacao 9000000000000000
      add_account maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz maya 8440000000
      add_account maya1m3t5wwrpfylss8e9g3jvq5chsv2xl3uchjja6v maya 1560000000
      # genesis nodes
      add_account maya1q3jj8n8pkvl2kjv3pajdyju4hp92cmxnadknd2 cacao 1000000000
      add_account maya10sy79jhw9hw9sqwdgu0k4mw4qawzl7czewzs47 cacao 1000000000
      add_account maya1gv85v0jvc0rsjunku3qxempax6kmrg5jqh8vmg cacao 1000000000
      add_account maya1vm43yk3jq0evzn2u6a97mh2k9x4xf5mzp62g23 cacao 1000000000
      add_account maya1g8dzs4ywxhf8hynaddw4mhwzlwzjfccakkfch7 cacao 1000000000

      add_lp "BTC.BTC" "maya1q3jj8n8pkvl2kjv3pajdyju4hp92cmxnadknd2" "" 1000000000000 "maya1q3jj8n8pkvl2kjv3pajdyju4hp92cmxnadknd2"
      add_lp "BTC.BTC" "maya10sy79jhw9hw9sqwdgu0k4mw4qawzl7czewzs47" "" 1000000000000 "maya10sy79jhw9hw9sqwdgu0k4mw4qawzl7czewzs47"
      add_lp "BTC.BTC" "maya1gv85v0jvc0rsjunku3qxempax6kmrg5jqh8vmg" "" 1000000000000 "maya1gv85v0jvc0rsjunku3qxempax6kmrg5jqh8vmg"
      add_lp "BTC.BTC" "maya1vm43yk3jq0evzn2u6a97mh2k9x4xf5mzp62g23" "" 1000000000000 "maya1vm43yk3jq0evzn2u6a97mh2k9x4xf5mzp62g23"
      add_lp "BTC.BTC" "maya1g8dzs4ywxhf8hynaddw4mhwzlwzjfccakkfch7" "" 1000000000000 "maya1g8dzs4ywxhf8hynaddw4mhwzlwzjfccakkfch7"

      # Add mayanames
      add_mayaname itzamna 5256000 maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz MAYA
      add_mayaname aaluxx 5256000 maya1thtn0cdcu2sujtklknlc9wl2dg09a07vtger0x maya1thtn0cdcu2sujtklknlc9wl2dg09a07vtger0x MAYA
      add_mayaname wr 5256000 maya14lkndecaw0zkzu0yq4a0qq869hrs8hh7uhvzlp 0x259fe33B59ae7C24B34C0cb957271Ae0f7786878 ETH
      add_mayaname wr 5256000 maya14lkndecaw0zkzu0yq4a0qq869hrs8hh7uhvzlp maya1a427q3v96psuj4fnughdw8glt5r7j38ljfa6hh MAYA
      add_mayaname wr 5256000 maya14lkndecaw0zkzu0yq4a0qq869hrs8hh7uhvzlp thor1a427q3v96psuj4fnughdw8glt5r7j38lj7rkp8 THOR

      reserve 993994900000000
      # liquidity of genesis nodes
      asgard 6000000000000
    fi

    if [ "$NET" = "stagenet" ]; then
      echo "Adding pool and lp bonder for first node"
      add_pool "BTC.BTC" 8

      echo "Setting up accounts"
      add_account smaya18z343fsdlav47chtkyp0aawqt6sgxsh3ctcu6u cacao 9000000000000000
      add_account smaya18z343fsdlav47chtkyp0aawqt6sgxsh3ctcu6u maya 10000000000
      # genesis nodes
      add_account smaya126xjtvyc4gygpa0vffqwluclhmvnfgtz83rcyx cacao 1000000000000
      add_account smaya1nay0rxpjl2gk3nw7gmj3at50nc2xq3fnsgrt0w cacao 1000000000000
      add_account smaya1jltkeg0g56jegjwfld90d2g9fnd2kuwpnp265k cacao 1000000000000
      add_account smaya1qhm0wjsrlw8wpvzrnpj8xxqu87tcucd6hjen09 cacao 1000000000000
      add_account smaya1g5pgvndmtpejhrnkwem3y5tpznkuhd3cearctd cacao 1000000000000
      add_account smaya1mapzy6qfswyfjc6f8g08uj30vng74aqqqpethg cacao 1000000000000
      add_account smaya1nnzlsclmfwzptcqpuz9t7e4cp7dt6qer28evwt cacao 1000000000000

      add_lp "BTC.BTC" "smaya126xjtvyc4gygpa0vffqwluclhmvnfgtz83rcyx" "" 10000000000000 "smaya126xjtvyc4gygpa0vffqwluclhmvnfgtz83rcyx" true
      add_lp "BTC.BTC" "smaya1nay0rxpjl2gk3nw7gmj3at50nc2xq3fnsgrt0w" "" 10000000000000 "smaya1nay0rxpjl2gk3nw7gmj3at50nc2xq3fnsgrt0w" false
      add_lp "BTC.BTC" "smaya1jltkeg0g56jegjwfld90d2g9fnd2kuwpnp265k" "" 10000000000000 "smaya1jltkeg0g56jegjwfld90d2g9fnd2kuwpnp265k" false
      add_lp "BTC.BTC" "smaya1qhm0wjsrlw8wpvzrnpj8xxqu87tcucd6hjen09" "" 10000000000000 "smaya1qhm0wjsrlw8wpvzrnpj8xxqu87tcucd6hjen09" false
      add_lp "BTC.BTC" "smaya1g5pgvndmtpejhrnkwem3y5tpznkuhd3cearctd" "" 10000000000000 "smaya1g5pgvndmtpejhrnkwem3y5tpznkuhd3cearctd" false
      add_lp "BTC.BTC" "smaya1mapzy6qfswyfjc6f8g08uj30vng74aqqqpethg" "" 10000000000000 "smaya1mapzy6qfswyfjc6f8g08uj30vng74aqqqpethg" false
      add_lp "BTC.BTC" "smaya1nnzlsclmfwzptcqpuz9t7e4cp7dt6qer28evwt" "" 10000000000000 "smaya1nnzlsclmfwzptcqpuz9t7e4cp7dt6qer28evwt" false

      reserve 994996000000000
      asgard 70000000000000
    fi

    if [ "$NET" = "testnet" ]; then
      # mint 1m RUNE to reserve for testnet
      reserve 100000000000000

      # add testnet account and balances
      testnet_add_accounts
    fi

    echo "Genesis content"
    cat ~/.mayanode/config/genesis.json
    mayanode validate-genesis --trace
  fi
fi

# setup peer connection, typically only used for some mocknet configurations
if [ "$SEED" != "$(hostname)" ]; then
  if [ ! -f ~/.mayanode/config/genesis.json ]; then
    echo "Setting MAYANode as peer not genesis"

    init_chain "$NODE_ADDRESS"
    fetch_genesis "$SEED"
    NODE_ID=$(fetch_node_id "$SEED")
    echo "NODE ID: $NODE_ID"
    export THOR_TENDERMINT_P2P_PERSISTENT_PEERS="$NODE_ID@$SEED:$PORT_P2P"

    cat ~/.mayanode/config/genesis.json
  fi
fi

# render tendermint and cosmos configuration files
mayanode render-config

export SIGNER_NAME
export SIGNER_PASSWD
exec mayanode start
