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


# LOAD builder info
ifndef VERSION
SHELL := /bin/bash
VERSION := $(shell git describe --always --long --dirty --tag)
endif
ldflags := -X 'main.appVersion=${VERSION}'

ifdef FOR_TESTNET
FOR_TESTNET="-testnet"
endif

ifdef FOR_DEV
FOR_DEV="dev-"
endif

UNAME_S := $(shell uname -s)

ENV_FLAG :=
ifeq ($(UNAME_S),Darwin)
	ENV_FLAG += "env DYLD_LIBRARY_PATH=$(shell pwd)/kvm/wasmer2"
else
	ENV_FLAG += "env LD_LIBRARY_PATH=$(shell pwd)/kvm/wasmer2"
endif

GOCMD=go
GORUN=$(GOCMD) run -exec $(ENV_FLAG) -ldflags="$(ldflags)"
GOBUILD=$(GOCMD) build -ldflags="$(ldflags)"


############################
###        Run node      ###
############################
.PHONY: all debug trace redundancy seednode
all:
	$(GORUN) ./cmd/node --log-level=*:INFO --use-log-view

# run node in debug mode
debug:
	$(GORUN) ./cmd/node --log-level=*:DEBUG --use-log-view

# run node in trace mode
trace:
	$(GORUN) ./cmd/node --log-level=*:TRACE,ntp:INFO,debug/p2p:INFO,state:DEBUG,trie:INFO,facade:INFO,sharding/networksharding:INFO,p2p/libp2p:INFO,basichost:INFO,dht:INFO,pubsub:INFO,heartbeat/process:INFO,statistics/machine:INFO,process/rating:INFO,consensus/chronology:INFO --use-log-view --log-save

redundancy:
	$(GORUN) ./cmd/node --log-level=*:INFO --redundancy-level=1 --working-directory=./db/db1  --p2p-seed=node1 --rest-api-interface=127.0.0.1:8091 --use-log-view #--log-save

seednode:
	$(GORUN) ./cmd/seednode --log-level=*:DEBUG --rest-api-interface=8081


############################
###       Key Tools      ###
############################
.PHONY: newkey
newkey:
	$(GORUN) ./cmd/keygenerator


############################
###         BUILD        ###
############################

.PHONY: build build-validator build-seenode build-operator build-keygenerator build-batch docker-build docker-build-alpine
build: build-validator build-seenode build-operator build-keygenerator

build-validator:
	$(GOBUILD) -o ./bin/validator ./cmd/node

build-seenode:
	$(GOBUILD) -o ./bin/seednode ./cmd/seednode

build-operator:
	$(GOBUILD) -o ./bin/operator ./cmd/operator

build-keygenerator:
	$(GOBUILD) -o ./bin/keygenerator ./cmd/keygenerator

build-batch:
    CGO_ENABLED=0 go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o batchsend ./cmd/batchsend

docker-vendor:
	go mod vendor
	modvendor -copy="**/*.c **/*.h **/*.proto **/*.a" -v

docker-build: docker-vendor
	docker build --build-arg arg_version=${VERSION} -t kleverapp/klever-go:${FOR_DEV}${VERSION}${FOR_TESTNET} -t kleverapp/klever-go:${FOR_DEV}latest${FOR_TESTNET} .

docker-build-validator: docker-vendor
	docker build --build-arg arg_version=${VERSION} -t kleverapp/klever-go:val-${FOR_DEV}${VERSION}${FOR_TESTNET} -f Dockerfile.validator .

# validator only app with Alpine Docker image
docker-build-alpine: docker-vendor
	docker build --build-arg arg_version=${VERSION} -t kleverapp/klever-go:val-${FOR_DEV}${VERSION}${FOR_TESTNET}-alpine -t kleverapp/klever-go:val-${FOR_DEV}latest${FOR_TESTNET}-alpine -f Dockerfile.alpine .

vm-generate-rs:
	cd kvm/vmhost/vmhooks && go run generate/cmd/eiGenMain.go

############################
###    Test and Docs     ###
############################
.PHONY: prepare ensure-dependencies gen-doc
prepare:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/goware/modvendor@latest

ensure-dependencies:
	go mod tidy

goimports:
	@goimports -w -d $(shell find . -type f -name '*.go' \
		! -name '*.pb.go' \
		! -name '*_setter.go' \
		! -name 'opcodeCost.go' \
		! -name 'wasmer2ImportsCgo.go' \
		! -name 'wasmer2Names.go' \
		! -name 'executorMockFunc.go' \
		! -name 'gasCostWASM.go' \
		! -path "./vendor/*")

gen-doc:
	swag init -d ./cmd/node,./network/api -o ./docs --parseInternal --parseDependency --instanceName="node"

runsc-trace:
	rm -rf db
	$(GORUN) ./cmd/node --use-log-view --log-level=*:INFO,process/transaction:DEBUG,process/transaction.smartcontract:TRACE,process/smartcontract:DEBUG,vm/host:TRACE,vm/metering:DEBUG

node1:
	$(GORUN) ./cmd/node --log-level=*:DEBUG,ntp:INFO,debug/p2p:INFO,facade:INFO,sharding/networksharding:INFO,p2p/libp2p:INFO,basichost:INFO,dht:INFO,pubsub:INFO,heartbeat/process:INFO,statistics/machine:INFO,process/rating:INFO,consensus/chronology:INFO --validator-key-pem-file=./config/node/validatorKey1.pem --working-directory=./db/db1  --p2p-seed=node1 --rest-api-interface=127.0.0.1:8091 --use-log-view #--log-save

node2:
	$(GORUN) ./cmd/node --log-level=*:TRACE,ntp:INFO,debug/p2p:TRACE,facade:INFO,sharding/networksharding:INFO,p2p/libp2p:TRACE,basichost:INFO,dht:INFO,pubsub:INFO,heartbeat/process:INFO,statistics/machine:INFO,process/rating:INFO,consensus/chronology:INFO --validator-key-pem-file=./config/node/validatorKey2.pem --working-directory=./db/db2  --p2p-seed=node2 --rest-api-interface=127.0.0.1:8092 --use-log-view #--log-save

node3:
	$(GORUN) ./cmd/node --log-level=*:TRACE,ntp:INFO,debug/p2p:TRACE,facade:INFO,sharding/networksharding:INFO,p2p/libp2p:TRACE,basichost:INFO,dht:INFO,pubsub:INFO,heartbeat/process:INFO,statistics/machine:INFO,process/rating:INFO,consensus/chronology:INFO --validator-key-pem-file=./config/node/validatorKey3.pem --working-directory=./db/db3  --p2p-seed=node3 --rest-api-interface=127.0.0.1:8093 --use-log-view #--log-save

############################
###       BackTest       ###
############################

backtest:
	go run ./cmd/backtest --generate

singlebacktest:
	$(GORUN) ./cmd/node --log-level=*:INFO --use-log-view --import-db=./db/mainnet --import-db-no-sig-check

############################
###  Integration Tests   ###
############################

tests-unit:
	go clean -testcache
	go test $(shell go list ./... | grep -v "integrationTest")

tests-integration:
	go clean -testcache
	go test ./integrationTest/...

tests-e2e:
	go run ./cmd/tests --node="${E2E_NODE_URL}" --proxy="${E2E_PROXY_URL}"

ifndef LOG
LOG=*:INFO
endif

connector:
	go run ./cmd/connector/main.go node --address="${NODE}" --log-level="${LOG}"

############################
###  Functional Tests    ###
############################

functional:
	go run ./cmd/tests --key-file="./walletKey.pem"

ui:
	go run ./cmd/operator-ui
