FROM golang:1.23.4-bookworm

ENV GOPATH=/go
ENV GOBIN=/go/bin
ENV GOOS=linux
ENV CGO_ENABLED=1

# Install required packages - no musl-tools needed anymore
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    jq=1.6-2.1 \
    protobuf-compiler=3.21.12-3 \
    build-essential=12.9 \
    linux-libc-dev=6.1.135-1 \
    libusb-1.0-0-dev=2:1.0.26-1 && \
    rm -rf /var/lib/apt/lists/*

