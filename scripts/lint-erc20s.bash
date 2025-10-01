#!/bin/bash
set -euo pipefail

possibly_die() {
  if [[ ${CI_MERGE_REQUEST_TITLE-} == *"#check-lint-warning"* ]]; then
    echo "Merge request is marked unsafe. Please carefully review errors above."
  else
    exit 1
  fi
}

check_token_list() {
  local LIST="$1"
  local TOKEN_FORMAT=".tokens[] | .address | ascii_downcase"
  local XRD_TOKEN_FORMAT=".[].address | ascii_downcase"
  local XRD_DIR="common/tokenlist/radixtokens/"

  # Change token format depending of path
  if [[ $1 == $XRD_DIR* ]]; then
    TOKEN_FORMAT=$XRD_TOKEN_FORMAT
  fi

  echo "Linting $LIST"

  git show origin/develop:"$LIST" |
    jq -r "$TOKEN_FORMAT" |
    sort -n >/tmp/orig_erc20_token_list.txt

  jq -r "$TOKEN_FORMAT" <"$LIST" |
    sort -n >/tmp/modified_erc20_token_list.txt

  # shellcheck disable=SC2155
  local REMOVALS=$(comm -23 /tmp/orig_erc20_token_list.txt /tmp/modified_erc20_token_list.txt)
  if [ -n "$REMOVALS" ]; then
    printf "Tokens removed:\n%s\n" "$REMOVALS"
    possibly_die
  fi

  # TODO: enable this check when the existing dupes are removed.
  # local DUPES=$(cat /tmp/orig_erc20_token_list.txt /tmp/modified_erc20_token_list.txt | sort -n | uniq --count | awk '$1 > 2 {print}')
  # if [ -n "$DUPES" ]; then
  #     printf "Tokens already in list:\n%s\n" "$DUPES"
  #     possibly_die
  # fi
}

check_token_list common/tokenlist/ethtokens/eth_mainnet_latest.json
check_token_list common/tokenlist/arbtokens/arb_mainnet_latest.json
check_token_list common/tokenlist/radixtokens/radix_mainnet_latest.json

echo "OK"
