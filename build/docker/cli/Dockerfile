FROM registry.gitlab.com/mayachain/devops/node-launcher:bitcoin-daemon-24.1 as bitcoin
FROM registry.gitlab.com/mayachain/devops/node-launcher:dash-daemon-22.0.0 as dash
FROM registry.gitlab.com/mayachain/devops/node-launcher:thornode-daemon-mocknet-1.126.0 as thorchain

FROM golang:1.22.3 AS build

RUN apt-get update && \
  apt-get install --no-install-recommends -y protobuf-compiler=3.12.4-1+deb11u1 jq=1.6-1 && \
  rm -rf /var/lib/apt/lists/*

# build geth since the image is alpine based
RUN go install github.com/ethereum/go-ethereum/cmd/geth@v1.11.5

# copy chain clis
COPY --from=bitcoin /usr/local/bin/bitcoin-cli /usr/local/bin/bitcoin-cli
COPY --from=dash /usr/local/bin/dash-cli /usr/local/bin/dash-cli
COPY --from=thorchain /usr/bin/thornode /usr/local/bin/thornode

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TAG=mocknet
RUN make install
