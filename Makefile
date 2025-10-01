########################################################################################
# Environment Checks
########################################################################################

CHECK_ENV:=$(shell ./scripts/check-env.sh)
ifneq ($(CHECK_ENV),)
$(error Check environment dependencies.)
endif

########################################################################################
# Config
########################################################################################


.PHONY: build test tools export healthcheck run-mocknet build-mocknet stop-mocknet ps-mocknet reset-mocknet logs-mocknet openapi

# compiler flags
NOW=$(shell date +'%Y-%m-%d_%T')
COMMIT:=$(shell git log -1 --format='%H')
VERSION:=$(shell cat version)
TAG?=mocknet
ldflags = -X gitlab.com/mayachain/mayanode/constants.Version=$(VERSION) \
		  -X gitlab.com/mayachain/mayanode/constants.GitCommit=$(COMMIT) \
		  -X gitlab.com/mayachain/mayanode/constants.BuildTime=${NOW} \
		  -X github.com/cosmos/cosmos-sdk/version.Name=MAYAChain \
		  -X github.com/cosmos/cosmos-sdk/version.AppName=mayanode \
		  -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
		  -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
		  -X github.com/cosmos/cosmos-sdk/version.BuildTags=$(TAG)

# golang settings
TEST_DIR?="./..."

# check kernel to set CGO_VARS
KERNEL_NAME = $(shell uname)
ifeq ($(KERNEL_NAME), Linux)
	CGO_VARS=CGO_LDFLAGS="-L${PWD}/lib -lradix_engine_toolkit_uniffi -lzec" LD_LIBRARY_PATH="${PWD}/lib" CGO_ENABLED=1 
else ifeq ($(KERNEL_NAME), Darwin)
	CGO_VARS=CGO_LDFLAGS="-L${PWD}/lib -lradix_engine_toolkit_uniffi -lzec" DYLD_LIBRARY_PATH="${PWD}/lib" CGO_ENABLED=1 
else
	@echo "Invalid kernel, can't set CGO_VARS"
endif

BUILD_FLAGS := -ldflags '$(ldflags)' -tags ${TAG}
TEST_BUILD_FLAGS := -parallel=1 -tags=mocknet
GOBIN?=${GOPATH}/bin
BINARIES=./cmd/mayanode ./cmd/bifrost ./tools/generate ./tools/recover-keyshare-backup
BIFROST_UTXO_CLIENT_PKGS := ./bifrost/pkg/chainclients/bitcoin/... \
      ./bifrost/pkg/chainclients/dash/... \
	  ./bifrost/pkg/chainclients/zcash/...


# docker tty args are disabled in CI
ifndef CI
DOCKER_TTY_ARGS=-it
endif

# image build settings
BRANCH?=$(shell git rev-parse --abbrev-ref HEAD | sed -e 's/master/mocknet/g')
GITREF=$(shell git rev-parse --short HEAD)
BUILDTAG?=$(shell git rev-parse --abbrev-ref HEAD | sed -e 's/master/mocknet/g;s/develop/mocknet/g;s/testnet-multichain/testnet/g')
ifdef CI_COMMIT_BRANCH # pull branch name from CI, if available
	BRANCH=$(shell echo ${CI_COMMIT_BRANCH} | sed 's/master/mocknet/g')
	BUILDTAG=$(shell echo ${CI_COMMIT_BRANCH} | sed -e 's/master/mocknet/g;s/develop/mocknet/g;s/testnet-multichain/testnet/g')
endif

all: lint install

# ------------------------------ Generate ------------------------------

generate: go-generate openapi protob-docker

go-generate:
	@go install golang.org/x/tools/cmd/stringer@v0.15.0
	@go generate ./...

SMOKE_PROTO_DIR=test/smoke/mayanode_proto

protob:
	@./scripts/protocgen.sh

protob-docker:
	@docker run --rm -v $(shell pwd):/app -w /app \
		registry.gitlab.com/mayachain/mayanode:builder-v8@sha256:9d39b645cda9368857c8ca0845396b5dd5f01a548ab3df9f396f34cb9687d36e \
		make protob

smoke-protob:
	@echo "Install betterproto..."
	@pip3 install --break-system-packages --upgrade markupsafe==2.0.1 betterproto[compiler]==2.0.0b4
	@rm -rf "${SMOKE_PROTO_DIR}"
	@mkdir -p "${SMOKE_PROTO_DIR}"
	@echo "Processing mayanode proto files..."
	@protoc \
  	-I ./proto \
  	-I ./third_party/proto \
  	--python_betterproto_out="${SMOKE_PROTO_DIR}" \
  	$(shell find ./proto -path -prune -o -name '*.proto' -print0 | xargs -0)

smoke-protob-docker:
	@docker run --rm -v $(shell pwd):/app -w /app \
		registry.gitlab.com/mayachain/mayanode:builder-v8@sha256:9d39b645cda9368857c8ca0845396b5dd5f01a548ab3df9f396f34cb9687d36e \
		sh -c 'make smoke-protob'

$(SMOKE_PROTO_DIR):
	@$(MAKE) smoke-protob-docker

openapi:
	@docker run --rm \
		--user $(shell id -u):$(shell id -g) \
		-v $$PWD/openapi:/mnt \
		openapitools/openapi-generator-cli:v6.0.0@sha256:310bd0353c11863c0e51e5cb46035c9e0778d4b9c6fe6a7fc8307b3b41997a35 \
		generate -i /mnt/openapi.yaml -g go -o /mnt/gen
	@rm openapi/gen/go.mod openapi/gen/go.sum
	@find ./openapi/gen -type f | xargs sed -i '/^[- ]*API version.*$(shell cat version)/d;/APIClient.*$(shell cat version)/d'
	@find ./openapi/gen -type f | grep model | xargs sed -i 's/MarshalJSON(/MarshalJSON_deprecated(/'

# ------------------------------ Build ------------------------------

build: protob
	@${CGO_VARS} go build ${BUILD_FLAGS} ${BINARIES}

install: protob
	@${CGO_VARS} go install ${BUILD_FLAGS} ${BINARIES}

tools:
	${CGO_VARS} go install -tags ${TAG} ./tools/generate
	${CGO_VARS} go install -tags ${TAG} ./tools/pubkey2address

# ------------------------------ Gitlab CI ------------------------------

gitlab-trigger-ci:
	@./scripts/gitlab-trigger-ci.sh

# ------------------------------ Housekeeping ------------------------------

format:
	@git ls-files '*.go' | grep -v -e '^docs/' | xargs gofumpt -w

lint:
	@${CGO_VARS} ./scripts/lint.sh
	./scripts/trunk check -j8 --no-fix --upstream origin/develop

lint-ci:
	@${CGO_VARS} ./scripts/lint.sh
	@./scripts/trunk-ci.sh

clean:
	rm -rf ~/.maya*
	rm -f ${GOBIN}/{generate,mayanode,bifrost}

# ------------------------------ Testing ------------------------------

test-coverage: test-network-specific
	@${CGO_VARS} go test ${TEST_BUILD_FLAGS} -v -coverprofile=coverage.txt -covermode count ${TEST_DIR}
	sed -i '/\.pb\.go:/d' coverage.txt

coverage-report: test-coverage
	@go tool cover -html=coverage.txt

# test-coverage-sum:
# 	@go run gotest.tools/gotestsum --junitfile report.xml --format testname -- ${TEST_BUILD_FLAGS} -v -coverprofile=coverage.txt -covermode count ${TEST_DIR}
# 	sed -i '/\.pb\.go:/d' coverage.txt
# 	@GOFLAGS='${TEST_BUILD_FLAGS}' go run github.com/boumenot/gocover-cobertura < coverage.txt > coverage.xml
# 	@go tool cover -func=coverage.txt
# 	@go tool cover -html=coverage.txt -o coverage.html

test: test-network-specific
	@${CGO_VARS} go test ${TEST_BUILD_FLAGS} ${TEST_DIR}

test-network-specific:
	@${CGO_VARS} go test -tags stagenet ./common
	@${CGO_VARS} go test -tags mainnet ./common ${BIFROST_UTXO_CLIENT_PKGS}
	@${CGO_VARS} go test -tags mocknet ./common ${BIFROST_UTXO_CLIENT_PKGS}

test-race:
	@${CGO_VARS} go test -race ${TEST_BUILD_FLAGS} ${TEST_DIR}

# test-watch:
# 	@gow -c test ${TEST_BUILD_FLAGS} ${TEST_DIR}

test-sync-mainnet:
	@BUILDTAG=mainnet BRANCH=mainnet $(MAKE) docker-gitlab-build
	@docker run --rm -e CHAIN_ID=mayachain-mainnet-v1 -e NET=mainnet registry.gitlab.com/mayachain/mayanode:mainnet

# ------------------------------ Docker Build ------------------------------

docker-gitlab-login:
	docker login -u ${CI_REGISTRY_USER} -p ${CI_REGISTRY_PASSWORD} ${CI_REGISTRY}

docker-gitlab-push:
	./build/docker/semver_tags.sh registry.gitlab.com/mayachain/mayanode ${BRANCH} $(shell cat version) \
		| xargs -n1 | grep registry | xargs -n1 docker push
	docker push registry.gitlab.com/mayachain/mayanode:${GITREF}

docker-gitlab-build:
	docker build -f build/docker/Dockerfile \
		$(shell sh ./build/docker/semver_tags.sh registry.gitlab.com/mayachain/mayanode ${BRANCH} $(shell cat version)) \
		-t registry.gitlab.com/mayachain/mayanode:${GITREF} --build-arg TAG=${BUILDTAG} .

# ------------------------------ Simulation Tests ------------------------------

test-simulation: build-simulation stop-mocknet run-simulation-mocknet
	@docker run --rm ${DOCKER_TTY_ARGS} \
		-e PARALLELISM --network host -w /app \
		mayanode-simtest sh -c 'make _test-simulation'

build-simulation:
	@DOCKER_BUILDKIT=1 docker buildx build --platform linux/amd64 -t mayanode-simtest -f ci/Dockerfile.simtest .

# internal target used in docker build
_build-test-simulation:
	@${CGO_VARS} go build -ldflags '$(ldflags)' -tags mocknet -o /simtest/simtest ./test/simulation/cmd

# internal target used in test run
_test-simulation:
	@cd test/simulation && /simtest/simtest


# ------------------------------ Smoke Tests ------------------------------

SMOKE_DOCKER_OPTS = --network=host --rm -e RUNE=MAYA.CACAO -e LOGLEVEL=INFO -e PYTHONPATH=/app -w /app -v ${PWD}/test/smoke:/app

smoke-unit-test:
	@docker run ${SMOKE_DOCKER_OPTS} \
		-e EXPORT=${EXPORT} \
		-e EXPORT_EVENTS=${EXPORT_EVENTS} \
		registry.gitlab.com/mayachain/mayanode:smoke \
		sh -c 'python -m unittest tests/test_*'

smoke-build-image:
	@docker pull registry.gitlab.com/mayachain/mayanode:smoke || true
	@docker buildx build --platform linux/amd64 --cache-from registry.gitlab.com/mayachain/mayanode:smoke \
		-f test/smoke/Dockerfile -t registry.gitlab.com/mayachain/mayanode:smoke \
		./test/smoke

smoke-push-image:
	@docker push registry.gitlab.com/mayachain/mayanode:smoke

smoke: reset-mocknet smoke-build-image
	@docker run ${SMOKE_DOCKER_OPTS} \
		-e BLOCK_SCANNER_BACKOFF=${BLOCK_SCANNER_BACKOFF} \
		-v ${PWD}/test/smoke:/app \
		registry.gitlab.com/mayachain/mayanode:smoke \
		python scripts/smoke.py --fast-fail=True

smoke-no-reset:
	@docker run ${SMOKE_DOCKER_OPTS} \
		-e BLOCK_SCANNER_BACKOFF=${BLOCK_SCANNER_BACKOFF} \
		-v ${PWD}/test/smoke:/app \
		registry.gitlab.com/mayachain/mayanode:smoke \
		python scripts/smoke.py --fast-fail=True

smoke-remote-ci: reset-mocknet
	@docker run ${SMOKE_DOCKER_OPTS} \
		-e BLOCK_SCANNER_BACKOFF=${BLOCK_SCANNER_BACKOFF} \
		registry.gitlab.com/mayachain/mayanode:smoke \
		python scripts/smoke.py --fast-fail=True

# ------------------------------ Regression Tests ------------------------------

test-regression: build-test-regression
	@docker run --rm ${DOCKER_TTY_ARGS} \
		-e ZCASH_SERVER_HOST="" \
		-e ZCASH_SERVER_USER=mayachain \
		-e ZCASH_SERVER_PASSWORD=password \
		-e ZCASH_MAINNET=false \
		-e DEBUG -e RUN -e EXPORT -e TIME_FACTOR -e PARALLELISM -e FAIL_FAST \
		-e AUTO_UPDATE -e IGNORE_FAILURES \
		-e UID=$(shell id -u) -e GID=$(shell id -g) \
		-p 1317:1317 -p 26657:26657 \
		-v $(shell pwd)/test/regression/mnt:/mnt \
		-v $(shell pwd)/test/regression/suites:/app/test/regression/suites \
		-v $(shell pwd)/test/regression/templates:/app/test/regression/templates \
		-w /app mayanode-regtest sh -c 'make _test-regression'

build-test-regression:
	@DOCKER_BUILDKIT=1 docker buildx build . \
		--platform linux/amd64 \
		-t mayanode-regtest \
		-f ci/Dockerfile.regtest \
		--build-arg COMMIT=$(COMMIT)

test-regression-coverage:
	@go tool cover -html=test/regression/mnt/coverage/coverage.txt

# internal target used in docker build - version pinned for consistent app hashes
_build-test-regression:
	@go install -ldflags '$(ldflags)' -tags=mocknet,regtest ./cmd/mayanode
	@go build -ldflags '$(ldflags) -X gitlab.com/mayachain/mayanode/constants.Version=1.999.0' \
		-cover -tags=mocknet,regtest -o /regtest/cover-mayanode ./cmd/mayanode
	@go build -ldflags '$(ldflags) -X gitlab.com/mayachain/mayanode/constants.Version=1.999.0' \
		-tags mocknet -o /regtest/regtest ./test/regression/cmd

# internal target used in test run
_test-regression:
	@rm -rf /mnt/coverage && mkdir -p /mnt/coverage
	@cd test/regression && /regtest/regtest
	@go tool covdata textfmt -i /mnt/coverage -o /mnt/coverage/coverage.txt
	@grep -v -E -e archive.go -e 'v[0-9]+.go' -e openapi/gen /mnt/coverage/coverage.txt > /mnt/coverage/coverage-filtered.txt
	@go tool cover -func /mnt/coverage/coverage-filtered.txt > /mnt/coverage/func-coverage.txt
	@awk '/^total:/ {print "Regression Coverage: " $$3}' /mnt/coverage/func-coverage.txt
	@chown -R ${UID}:${GID} /mnt

# ------------------------------ Single Node Mocknet ------------------------------

cli-mocknet:
	@docker compose -f build/docker/docker-compose.yml run --rm cli

run-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard up -d

run-simulation-mocknet:
	@BTC_MASTER_ADDR="bcrt1qf4l5dlqhaujgkxxqmug4stfvmvt58vx2h44c39" \
		docker compose -f build/docker/docker-compose.yml --profile mocknet up -d

stop-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard down -v

halt-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard down

build-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard build

bootstrap-mocknet: $(SMOKE_PROTO_DIR)
	@docker run ${SMOKE_DOCKER_OPTS} \
		-e BLOCK_SCANNER_BACKOFF=${BLOCK_SCANNER_BACKOFF} \
    -v ${PWD}/test/smoke:/app \
		registry.gitlab.com/mayachain/mayanode:smoke \
		python scripts/smoke.py --bootstrap-only=True

ps-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard images
	@docker compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard ps

logs-mocknet:
	@docker compose -f build/docker/docker-compose.yml logs -f mayanode bifrost

reset-mayanode-mocknet: stop-mayanode-mocknet build-mayanode-mocknet run-mayanode-mocknet

build-mayanode-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mayanode build --no-cache

stop-mayanode-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mayanode stop

run-mayanode-mocknet:
	@docker compose -f build/docker/docker-compose.yml --profile mayanode up -d
 
reset-mocknet: stop-mocknet build-mocknet run-mocknet

restart-mocknet: stop-mocknet run-mocknet

# ------------------------------ Mocknet EVM Fork ------------------------------

reset-mocknet-fork-%: stop-mocknet
	@./tools/evm/run-mocknet-fork.sh $*

# ------------------------------ Multi Node Mocknet ------------------------------

run-mocknet-cluster:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard up -d

stop-mocknet-cluster:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard down -v

build-mocknet-cluster:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard build --no-cache

ps-mocknet-cluster:
	@docker compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard images
	@docker compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard ps

reset-mocknet-cluster: stop-mocknet-cluster build-mocknet-cluster run-mocknet-cluster

rename:
	@./scripts/rename.sh
