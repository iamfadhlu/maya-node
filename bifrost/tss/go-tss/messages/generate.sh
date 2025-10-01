#!/usr/bin/env bash
set -euo pipefail

protoc --go_out=. *.proto
