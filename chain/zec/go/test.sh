#!/bin/sh
set -x
ROOT_LIB=$(realpath ../../../lib)
LD_LIBRARY_PATH="${ROOT_LIB}" CGO_LDFLAGS="-L${ROOT_LIB} -lzec" go test -v ./zec/...
