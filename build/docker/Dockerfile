#
# MAYAChain
#

#
# Build
FROM registry.gitlab.com/mayachain/mayanode:builder-v8@sha256:9d39b645cda9368857c8ca0845396b5dd5f01a548ab3df9f396f34cb9687d36e AS build

# Enable CGO and specify the Radix library path
ENV CGO_ENABLED=1
ENV CGO_LDFLAGS="-L/usr/lib -lradix_engine_toolkit_uniffi -lzec"

WORKDIR /app

# Copy Radix Engine Toolkit (RET) native lib
COPY ./lib/libradix_engine_toolkit_uniffi.so /usr/lib
# Copy zec lib
COPY ./lib/libzec.so /usr/lib

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TAG=mainnet
ARG NET=mainnet

# Standard build with the system's gcc (glibc)
RUN make install

FROM registry.gitlab.com/mayachain/mayanode:runner-base-v5@sha256:e0e470b2f361b8363128f76a267604a374e0ba4e0ec027519896e594f59884e4

COPY build/scripts /scripts

# Copy the compiled binaries over
COPY --from=build /go/bin/generate /go/bin/recover-keyshare-backup /go/bin/mayanode /go/bin/bifrost /usr/bin/
# Copy Radix Engine Toolkit (RET) native lib
COPY ./lib/libradix_engine_toolkit_uniffi.so /lib

# Copy zec lib
COPY ./lib/libzec.so /lib

# Ensure binaries are executable
RUN chmod +x /usr/bin/generate /usr/bin/recover-keyshare-backup /usr/bin/mayanode /usr/bin/bifrost

# default to mainnet
ARG TAG=mainnet
ENV NET=$TAG

# default to fullnode
CMD ["/scripts/fullnode.sh"]
