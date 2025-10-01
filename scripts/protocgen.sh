#!/usr/bin/env bash

set -euo pipefail

# Delete any existing protobuf generated files.
find . -name "*.pb.go" -delete

go install github.com/regen-network/cosmos-proto/protoc-gen-gocosmos

# shellcheck disable=SC2038
find proto/ -path -prune -o -name '*.proto' | sort | uniq |
  while read -r DIR; do
    find "$DIR" -maxdepth 1 -name '*.proto' |
      xargs protoc \
        -I "proto" \
        -I "third_party/proto" \
        --gocosmos_out=plugins=interfacetype+grpc,Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types:.
  done

# Move proto files to the right places.
cp -r gitlab.com/mayachain/mayanode/* ./
rm -rf gitlab.com

# Generate proto files for go-tss.
echo "Generating proto files for go-tss"
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
pushd bifrost/tss/go-tss
protoc --go_out=module=gitlab.com/mayachain/mayanode/bifrost/tss/go-tss:. ./messages/*.proto
popd
