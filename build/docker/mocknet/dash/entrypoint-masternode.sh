#!/bin/bash

. /scripts/core.sh

masternode() {
  printsignerconfig
  writedashdconfig

  # trunk-ignore(shellcheck/SC2154)
  waitforblock dash1 "$initialBlocks"

  # trunk-ignore(shellcheck/SC2154)
  dashd -addnode=dash1 1>"$logPath" &
  dashdpid="$!"

  waitforblock "$(hostname)" "$initialBlocks"

  echo "Creating default wallet"
  dash-cli createwallet ""

  echo "Determining genesis address from genesis node"
  genesisAddress=$(dash-cli -rpcconnect=dash1 getaddressesbylabel "" | jq -r 'keys | .[0]')
  if [[ $genesisAddress == "" ]]; then
    echo "Failed to request genesis node address"
    sleep infinity
  else
    echo "--> $genesisAddress"
  fi

  ownerAddress=$(dash-cli getnewaddress "")
  collateralAddress=$(dash-cli getnewaddress "")
  # trunk-ignore(shellcheck/SC2034): Used in core.sh when printing
  collateralVout=1
  ipPort="$NODE_IP:19899"
  votingAddress=$(dash-cli getnewaddress "")
  operatorBls=$(dash-cli bls generate)
  operatorPrivkey=$(echo "$operatorBls" | jq -r '.secret')
  operatorPubkey=$(echo "$operatorBls" | jq -r '.public')
  operatorReward=0
  payoutAddress=$(dash-cli getnewaddress "")
  # trunk-ignore(shellcheck/SC2034): Used in core.sh when printing
  feeSourceAddress=$ownerAddress
  fundAddress=$(dash-cli getnewaddress "")

  echo "Sending 1001 DASH to masternode fund address $fundAddress"
  fundHash=$(dash-cli -rpcconnect=dash1 sendtoaddress "$fundAddress" 1001 2>&1)
  echo "Sending 1000 DASH to address yZnyJAdouDu3gmAuhG3dTc66hroS4AXxnL"
  fundHash=$(dash-cli -rpcconnect=dash1 sendtoaddress "yZnyJAdouDu3gmAuhG3dTc66hroS4AXxnL" 1000 2>&1)
  if [[ $fundHash == *"error code"* ]]; then
    echo "Fund transaction failed: $fundHash"
    sleep infinity
  else
    echo "Fund tx hash: $fundHash"
  fi

  echo "Sending 1001 DASH to masternode collateral address $collateralAddress"
  collateralHash=$(dash-cli -rpcconnect=dash1 sendtoaddress "$collateralAddress" 1001 2>&1)
  if [[ $collateralHash == *"error code"* ]]; then
    echo "Collateral transaction failed: $collateralHash"
    sleep infinity
  else
    echo "Collateral tx hash: $collateralHash"
  fi

  echo "Generating confirmation blocks..."
  dash-cli -rpcconnect=dash1 generatetoaddress 20 "$genesisAddress"

  echo "Balance of fund address"
  dash-cli getaddressbalance \""$fundAddress"\"

  echo "Balance of collateral address"
  dash-cli getaddressbalance \""$collateralAddress"\"

  echo "Current addresses and balances:"
  dash-cli listaddressgroupings

  printmasternodeconfig

  echo "Sending protx register command"
  registerHash=$(dash-cli protx register_fund \
    "$collateralAddress" \
    "$ipPort" \
    "$ownerAddress" \
    "$operatorPubkey" \
    "$votingAddress" \
    "$operatorReward" \
    "$payoutAddress" \
    "$fundAddress")

  echo "Protx registration tx hash: $registerHash"

  if [[ $registerHash == "" ]]; then
    echo "Protx register failed."
    sleep infinity
  fi

  while true; do
    output=$(dash-cli -rpcconnect=dash1 getrawtransaction "$registerHash" true 2>&1)
    if [[ $output == *"No such mempool or blockchain transaction."* || $output == *"error code"* ]]; then
      echo "Protx not accepted by genesis node, re-sending..."
      rawtx=$(dash-cli getrawtransaction "$registerHash" 2>&1)
      dash-cli sendrawtransaction "$rawtx" &>/dev/null
      sleep 1
    else
      echo "Protx accepted by genesis node!"
      break
    fi
  done

  echo "Setting masternode operator bls key in dash.conf"
  # trunk-ignore(shellcheck/SC2154): Set in core.sh
  echo "  masternodeblsprivkey=$operatorPrivkey" >>"$configPath"

  echo "Restarting dashd to initiate masternode sync"
  killpidandwait "$dashdpid"
  sleep 2
  dashd -connect=dash1 1>"$logPath" &
  dashdpid="$!"

  echo "Started new dashd process $dashdpid"
  waitforverificationprogresscomplete "$(hostname)"
  waitformasternodestatus "$(hostname)" READY
  dash-cli mnsync next
  dash-cli mnsync next
  waitformasternodesync "$(hostname)"

  echo "Adding other masternode peers..."
  dash-cli addnode dash2 add
  dash-cli addnode dash3 add
  dash-cli addnode dash4 add
  waitforpeerconnections "$(hostname)" 3

  waitforquorumwithname llmq_test

  printtimetostart

  echo "Following dashd log from start:"
  tail -f -n +1 "$logPath"
}

masternode &
exitonsigterm
