#!/bin/bash
nitro \
  --dev \
  --http.api net,web3,eth,txpool,debug,personal \
  --http.vhosts=* \
  --http.addr 0.0.0.0 \
  --http.port 8547
