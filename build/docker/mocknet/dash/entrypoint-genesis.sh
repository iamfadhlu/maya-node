#!/bin/bash

BLOCK_TIME=${BLOCK_TIME:=5}

. /scripts/core.sh

genesisAddress=""

verifyinstantsendandchainlocks() {
  while true; do
    echo "Verifying instantsend and chainlock..."
    hash=$(dash-cli sendtoaddress "$genesisAddress" 10)
    sleep "$BLOCK_TIME"
    rawtx=$(dash-cli getrawtransaction "$hash" true)
    echo "--> tx hash: $hash"
    instantlock=$(echo "$rawtx" | jq '.instantlock')
    chainlock=$(echo "$rawtx" | jq '.chainlock')

    if [[ $instantlock == "true" && $chainlock == "true" ]]; then
      echo "--> instantlock OK"
      echo "--> chainlock OK"
      break
    fi

    if [[ $instantlock != "true" ]]; then
      echo "--> instantlock FAILED"
    fi
    if [[ $chainlock != "true" ]]; then
      echo "--> chainlock FAILED"
    fi
  done
}

genesis() {
  printsignerconfig
  echo "BLOCK_TIME               $BLOCK_TIME"

  writedashdconfig

  # trunk-ignore(shellcheck/SC2154)
  dashd 1>"$logPath" &
  # trunk-ignore(shellcheck/SC2034): Used in core.sh when this process is running
  dashdpid="$!"

  waitforverificationprogresscomplete "$(hostname)"

  echo "Creating default wallet"
  dash-cli createwallet ""

  echo "Creating genesis address"
  genesisAddress=$(dash-cli getnewaddress "")
  echo "--> $genesisAddress"

  # trunk-ignore(shellcheck/SC2154)
  echo "Generating $initialBlocks initial blocks..."
  dash-cli generatetoaddress "$initialBlocks" "$genesisAddress"

  waitforblock "$(hostname)" "$initialBlocks"

  while true; do
    dash-cli generatetoaddress 1 "$genesisAddress"
    sleep "$BLOCK_TIME"
  done &

  waitforblock dash2 "$initialBlocks"
  waitformasternodestatus dash2 READY
  waitformasternodesync dash2

  waitforblock dash3 "$initialBlocks"
  waitformasternodestatus dash3 READY
  waitformasternodesync dash3

  waitforblock dash4 "$initialBlocks"
  waitformasternodestatus dash4 READY
  waitformasternodesync dash4

  waitforpeerconnections "$(hostname)" 3

  echo "Activating sporks..."
  # dash-cli sporkupdate SPORK_2_INSTANTSEND_ENABLED 0
  # dash-cli sporkupdate SPORK_3_INSTANTSEND_BLOCK_FILTERING 0
  dash-cli sporkupdate SPORK_9_SUPERBLOCKS_ENABLED 0
  dash-cli sporkupdate SPORK_17_QUORUM_DKG_ENABLED 0
  dash-cli sporkupdate SPORK_19_CHAINLOCKS_ENABLED 0
  # dash-cli sporkupdate SPORK_21_QUORUM_ALL_CONNECTED 1

  while true; do
    dash-cli generatetoaddress 1 "$genesisAddress" &>/dev/null
    count=$(dash-cli quorum list | jq ".llmq_test | length")
    if [[ $count -ge "2" ]]; then
      break
    fi
    sleep 0.1
  done &

  waitforquorumwithname llmq_test
  printtimetostart

  waitBlocks=8
  waitTime=$((BLOCK_TIME * waitBlocks))
  echo "Waiting for $waitTime seconds ($waitBlocks blocks)"
  sleep "$waitTime"

  verifyinstantsendandchainlocks
  sleep 8

  echo "Following dashd log from start:"

  tail -f -n +1 "$logPath"
}

genesis &
exitonsigterm
