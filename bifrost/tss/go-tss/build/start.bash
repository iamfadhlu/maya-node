#!/bin/sh
set -euo pipefail

printf '%s' "${PRIVKEY:?PRIVKEY not set}" | \
  exec /go/bin/tss -tss-port :8080 -p2p-port 6668 -loglevel debug

echo $PRIVKEY | /go/bin/tss -tss-port :8080 -p2p-port 6668 -loglevel debug
