# KKKKKKKKK    KKKKKKKLLLLLLLLLLL             EEEEEEEEEEEEEEEEEEEEEEVVVVVVVV           VVVVVVVVEEEEEEEEEEEEEEEEEEEEEERRRRRRRRRRRRRRRRR
# K:::::::K    K:::::KL:::::::::L             E::::::::::::::::::::EV::::::V           V::::::VE::::::::::::::::::::ER::::::::::::::::R
# K:::::::K    K:::::KL:::::::::L             E::::::::::::::::::::EV::::::V           V::::::VE::::::::::::::::::::ER::::::RRRRRR:::::R
# K:::::::K   K::::::KLL:::::::LL             EE::::::EEEEEEEEE::::EV::::::V           V::::::VEE::::::EEEEEEEEE::::ERR:::::R     R:::::R
# KK::::::K  K:::::KKK  L:::::L                 E:::::E       EEEEEE V:::::V           V:::::V   E:::::E       EEEEEE  R::::R     R:::::R
#   K:::::K K:::::K     L:::::L                 E:::::E               V:::::V         V:::::V    E:::::E               R::::R     R:::::R
#   K::::::K:::::K      L:::::L                 E::::::EEEEEEEEEE      V:::::V       V:::::V     E::::::EEEEEEEEEE     R::::RRRRRR:::::R
#   K:::::::::::K       L:::::L                 E:::::::::::::::E       V:::::V     V:::::V      E:::::::::::::::E     R:::::::::::::RR
#   K:::::::::::K       L:::::L                 E:::::::::::::::E        V:::::V   V:::::V       E:::::::::::::::E     R::::RRRRRR:::::R
#   K::::::K:::::K      L:::::L                 E::::::EEEEEEEEEE         V:::::V V:::::V        E::::::EEEEEEEEEE     R::::R     R:::::R
#   K:::::K K:::::K     L:::::L                 E:::::E                    V:::::V:::::V         E:::::E               R::::R     R:::::R
# KK::::::K  K:::::KKK  L:::::L         LLLLLL  E:::::E       EEEEEE        V:::::::::V          E:::::E       EEEEEE  R::::R     R:::::R
# K:::::::K   K::::::KLL:::::::LLLLLLLLL:::::LEE::::::EEEEEEEE:::::E         V:::::::V         EE::::::EEEEEEEE:::::ERR:::::R     R:::::R
# K:::::::K    K:::::KL::::::::::::::::::::::LE::::::::::::::::::::E          V:::::V          E::::::::::::::::::::ER::::::R     R:::::R
# K:::::::K    K:::::KL::::::::::::::::::::::LE::::::::::::::::::::E           V:::V           E::::::::::::::::::::ER::::::R     R:::::R
# KKKKKKKKK    KKKKKKKLLLLLLLLLLLLLLLLLLLLLLLLEEEEEEEEEEEEEEEEEEEEEE            VVV            EEEEEEEEEEEEEEEEEEEEEERRRRRRRR     RRRRRRR

###############################################################################
# Usage                                                                       #
###############################################################################
#
# Common workflows
#   make build                  Build all binaries into ./bin/
#   make all                    Run validator node (default log level)
#   make debug                  Run node in debug mode
#   make tests-unit             Run unit tests
#   make profile                Run node with pprof on :8080 (see pprof-* targets)
#   make docker-build           Build Debian Docker image (see docker/Makefile)
#   make help                   List every available target
#
# Environment variables
#   LOG=*:DEBUG                 Log level filter (default: *:INFO)
#   VERBOSE=1                   Add -v to go test invocations
#   RELEASE=1                   Strip DWARF + trim paths (CI / release builds)
#   FOR_DEV=dev-                Prefix docker image tags with "dev-"
#   FOR_TESTNET=-testnet        Suffix docker image tags with "-testnet"
#   E2E_NODE_URL, E2E_PROXY_URL Used by tests-e2e
#   NODE                        Used by connector (node URL)
#   ARGS                        Forwarded to benchmark target
#
###############################################################################

# Safer recipe execution: if a recipe fails, delete its target so the next
# invocation doesn't treat a half-written file as up-to-date.
.DELETE_ON_ERROR:

###############################################################################
# Configuration                                                               #
###############################################################################

# LOAD builder info
ifndef VERSION
SHELL := /bin/bash
VERSION := $(shell git describe --always --long --dirty --tag)
endif
ldflags := -X 'main.appVersion=${VERSION}'

# Release builds strip DWARF, build IDs, and trim source paths for smaller,
# reproducible binaries. Set RELEASE=1 in CI / docker release builds.
# Local `make build` keeps DWARF + symbols so delve still works on developer
# machines. Panic stack traces and pprof remain readable on RELEASE=1 builds
# because Go's runtime symbol table (pclntab) is independent of DWARF.
GO_BUILD_FLAGS :=
ifeq ($(RELEASE),1)
ldflags += -s -w -buildid=
GO_BUILD_FLAGS := -trimpath -buildvcs=false
endif

ifdef FOR_TESTNET
FOR_TESTNET := -testnet
endif

ifdef FOR_DEV
FOR_DEV := dev-
endif

UNAME_S := $(shell uname -s)

ENV_FLAG :=
ifeq ($(UNAME_S),Darwin)
	ENV_FLAG += env DYLD_LIBRARY_PATH=$(shell pwd)/kvm/wasmer2
else
	ENV_FLAG += env LD_LIBRARY_PATH=$(shell pwd)/kvm/wasmer2
endif

ifdef VERBOSE
VERBOSE=-v
endif

ifndef LOG
LOG=*:INFO
endif

GOCMD=go
GORUN=$(ENV_FLAG) $(GOCMD) run -ldflags="$(ldflags)"
GOBUILD=$(GOCMD) build $(GO_BUILD_FLAGS) -ldflags="$(ldflags) -extldflags '-Wl,-rpath,\$$ORIGIN,-rpath,@executable_path'"

.DEFAULT_GOAL := help

###############################################################################
# Help                                                                        #
###############################################################################

.PHONY: help version
help: ## Show this help message
	@echo "Klever Blockchain - Available Make Targets"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

version: ## Print the build version string (git describe)
	@echo $(VERSION)

###############################################################################
# Run & Debug                                                                 #
###############################################################################

.PHONY: all debug trace redundancy seednode import-from-localdb runsc-trace
.PHONY: profile pprof-goroutines pprof-heartbeat pprof-top

all: ## Run validator node with default log level
	$(GORUN) ./cmd/node --log-level="${LOG}" --use-log-view

debug: ## Run node in debug mode
	$(GORUN) ./cmd/node --log-level=*:DEBUG --use-log-view

trace: ## Run node in trace mode (verbose logging)
	$(GORUN) ./cmd/node --log-level=*:TRACE,ntp:INFO,debug/p2p:INFO,state:DEBUG,trie:INFO,facade:INFO,sharding/networksharding:INFO,p2p/libp2p:INFO,basichost:INFO,dht:INFO,pubsub:INFO,heartbeat/process:INFO,statistics/machine:INFO,process/rating:INFO,consensus/chronology:INFO --use-log-view --log-save

redundancy: ## Run node with redundancy for testing
	$(GORUN) ./cmd/node --log-level="${LOG}" --redundancy-level=1 --working-directory=./db/db1  --p2p-seed=node1 --rest-api-interface=127.0.0.1:8091 --use-log-view #--log-save

seednode: ## Run seednode for network bootstrap
	$(GORUN) ./cmd/seednode --log-level=*:DEBUG --rest-api-interface=8081

import-from-localdb: ## Import blockchain data from local database
	$(GORUN) ./cmd/node --log-level="${LOG}" --use-log-view --import-db=./db/local --import-db-no-sig-check

runsc-trace: ## Run node with smart-contract trace logging (wipes ./db first)
	rm -rf db
	$(GORUN) ./cmd/node --use-log-view --log-level=*:INFO,process/block:DEBUG,process/transaction:DEBUG,process/transaction.smartcontract:TRACE,process/smartcontract:DEBUG,vm/host:TRACE,vm/metering:DEBUG

# --- pprof -------------------------------------------------------------------
# Live runtime profiling. Start the node with `make profile`, then query the
# pprof endpoint from another terminal.
#
# Typical workflow
#   make profile               # terminal 1: start node with pprof on :8080
#   make pprof-top             # terminal 2 (after ~20s bootstrap)
#   make pprof-heartbeat       # focused heartbeat-monitor check
#   go tool pprof http://127.0.0.1:8080/debug/pprof/heap   # interactive heap
#
# Reference: https://pkg.go.dev/net/http/pprof

profile: ## Run node with pprof enabled on 127.0.0.1:8080/debug/pprof
	$(GORUN) ./cmd/node --log-level="${LOG}" --use-log-view \
		--profile-mode --rest-api-interface=127.0.0.1:8080

pprof-goroutines: ## Dump all live goroutines from a running --profile-mode node
	@curl -s "http://127.0.0.1:8080/debug/pprof/goroutine?debug=2"

pprof-heartbeat: ## Show only the heartbeat monitor goroutine stack (with state header)
	@curl -s "http://127.0.0.1:8080/debug/pprof/goroutine?debug=2" | \
		grep -B 1 -A 4 "startValidatorProcessing.func1"

pprof-top: ## Top 30 functions by live goroutine count (spot fan-out / leaks)
	@curl -s "http://127.0.0.1:8080/debug/pprof/goroutine?debug=2" | \
		awk '/^goroutine /{getline; sub(/\(.*/,""); print}' | \
		sort | uniq -c | sort -rn | head -30

###############################################################################
# Tools                                                                       #
###############################################################################

.PHONY: newkey connector benchmark

newkey: ## Generate new validator keys
	$(GORUN) ./cmd/keygenerator

connector: ## Run terminal UI connector against a node (NODE=<url>)
	go run ./cmd/connector/main.go node --address="${NODE}" --log-level="${LOG}"

benchmark: ## Run full validator benchmark (ARGS=<args>)
	$(GORUN) ./cmd/benchmark $(ARGS)

###############################################################################
# Build                                                                       #
###############################################################################

.PHONY: build build-validator build-seednode build-operator build-keygenerator build-benchmark clean

build: build-validator build-seednode build-operator build-keygenerator build-benchmark ## Build all binaries

build-validator: ## Build validator node binary
	$(GOBUILD) -o ./bin/validator ./cmd/node

build-seednode: ## Build seednode binary
	$(GOBUILD) -o ./bin/seednode ./cmd/seednode

build-operator: ## Build operator tools binary
	$(GOBUILD) -o ./bin/operator ./cmd/operator

build-keygenerator: ## Build key generator binary
	$(GOBUILD) -o ./bin/keygenerator ./cmd/keygenerator

build-benchmark: ## Build validator benchmark tool
	$(GOBUILD) -o ./bin/benchmark ./cmd/benchmark

clean: ## Remove build artifacts and caches
	@echo "Cleaning build artifacts..."
	@rm -rf ./bin/
	@rm -rf ./vendor/
	@go clean -testcache
	@go clean -cache
	@echo "Clean complete"

###############################################################################
# Docker                                                                      #
###############################################################################
# Docker-related targets are isolated in docker/Makefile (they have their own
# registry / multi-arch builder model). Targets become available via include.

include docker/Makefile

###############################################################################
# Code Generation & Docs                                                      #
###############################################################################

.PHONY: vm-generate-rs gen-doc

vm-generate-rs: ## Generate VM hooks from Rust SDK
	cd kvm/vmhost/vmhooks && go run generate/cmd/eiGenMain.go

gen-doc: ## Generate Swagger API documentation
	swag init -d ./cmd/node,./network/api -o ./docs --parseInternal --parseDependency --instanceName="node"

###############################################################################
# Development Setup                                                           #
###############################################################################

.PHONY: prepare ensure-dependencies goimports

prepare: ## Install development dependencies
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/goware/modvendor@latest

ensure-dependencies: ## Ensure Go dependencies are up to date
	go mod tidy

goimports: ## Format Go code and organize imports
	@goimports -w -d $(shell find . -type f -name '*.go' \
		! -name '*.pb.go' \
		! -name '*_setter.go' \
		! -name 'opcodeCost.go' \
		! -name 'wasmer2ImportsCgo.go' \
		! -name 'wasmer2Names.go' \
		! -name 'wrapperVMHooks_test.go' \
		! -name 'executorMockFunc.go' \
		! -name 'gasCostWASM.go' \
		! -path "./vendor/*")

###############################################################################
# Tests                                                                       #
###############################################################################

.PHONY: tests tests-unit tests-integration tests-kvm tests-e2e

tests: tests-unit tests-integration tests-kvm tests-e2e ## Run all tests

tests-unit: ## Run unit tests
	go clean -testcache
	go test ${VERBOSE} $(shell go list ./... | grep -v "integrationTest" | grep -v "kvm")

tests-integration: ## Run integration tests
	go clean -testcache
	go test ${VERBOSE} ./integrationTest/...

tests-kvm: ## Run KVM (smart contract) tests
	go clean -testcache
	go test ${VERBOSE} -timeout 1500s ./kvm/...

tests-e2e: ## Run end-to-end tests
	if [ ! -d klever-go-e2e ]; then git clone git@github.com:klever-io/klever-go-e2e.git; fi
	cd klever-go-e2e && go mod tidy && make build && cd ..
	klever-go-e2e/bin/klever-go-e2e --node="${E2E_NODE_URL}" --proxy="${E2E_PROXY_URL}"
