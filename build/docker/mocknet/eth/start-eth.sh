#!/bin/sh

ETH_BLOCK_TIME="${ETH_BLOCK_TIME:=5}"

geth --ws \
  --ws.addr=0.0.0.0 \
  --ws.api=eth,net,web3,debug,txpool \
  --ws.origins="*" \
  --dev \
  --dev.period "$ETH_BLOCK_TIME" \
  --verbosity 2 \
  --networkid 15 \
  --datadir "data" \
  -mine \
  -http \
  --http.addr 0.0.0.0 \
  --http.port 8545 \
  --http.api "eth,net,web3,miner,txpool,debug" \
  --http.corsdomain "*" \
  -nodiscover \
  --http.vhosts="*" \
  --allow-insecure-unlock
