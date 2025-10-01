#!/bin/bash

set -o pipefail

export SIGNER_NAME="${SIGNER_NAME:=mayachain}"
export SIGNER_PASSWD="${SIGNER_PASSWD:=password}"

. "$(dirname "$0")/core.sh"

if [ ! -f ~/.mayanode/config/genesis.json ]; then
  init_chain
  rm -rf ~/.mayanode/config/genesis.json # set in mayanode render-config
fi

# render tendermint and cosmos configuration files
mayanode render-config

exec mayanode start
