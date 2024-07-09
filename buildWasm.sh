#!/bin/bash

current_dir=$(pwd)
# KLEVER-VM-SDK-RS path
KLEVER_VM_SDK_RS_PATH=../klever-vm-sdk-rs

# Build KLEVER-VM-SDK-RS
cd $KLEVER_VM_SDK_RS_PATH
cargo build --release
# Build Tests/Examples contracts
./target/release/sc-meta all build --path ./contracts

# Go back to the current directory
cd $current_dir

# build c-contracts
CONTRACT_LIST=(
    "./kvm/test/contracts/big-floats-2"
    "./kvm/test/contracts/exec-same-ctx-simple-parent"
    "./kvm/test/contracts/misc"
    "./kvm/test/contracts/deployer"
    "./kvm/test/contracts/exec-same-ctx-recursive"
    "./kvm/test/contracts/deployer-fromanother-contract"
    "./kvm/test/contracts/answer"
    "./kvm/test/contracts/exec-sync-ctx-multiple/beta"
    "./kvm/test/contracts/exec-sync-ctx-multiple/delta"
    "./kvm/test/contracts/exec-sync-ctx-multiple/gamma"
    "./kvm/test/contracts/exec-sync-ctx-multiple/alpha"
    "./kvm/test/contracts/exec-dest-ctx-by-caller/parent"
    "./kvm/test/contracts/exec-dest-ctx-by-caller/child"
    "./kvm/test/contracts/baseOps"
    "./kvm/test/contracts/managed-buffers"
    "./kvm/test/contracts/exec-dest-ctx-child"
    "./kvm/test/contracts/upgrader-fromanother-contract"
    "./kvm/test/contracts/signatures"
    "./kvm/test/contracts/num-with-fp"
    "./kvm/test/contracts/big-floats"
    "./kvm/test/contracts/exec-dest-ctx-builtin"
    "./kvm/test/contracts/deployer-parent"
    "./kvm/test/contracts/exec-dest-ctx-recursive-child"
    "./kvm/test/contracts/init-wrong"
    "./kvm/test/contracts/exec-same-ctx-parent"
    "./kvm/test/contracts/deployer-child"
    "./kvm/test/contracts/init-correct"
    "./kvm/test/contracts/exec-same-ctx-recursive-child"
    "./kvm/test/contracts/breakpoint"
    "./kvm/test/contracts/exec-same-ctx-simple-child"
    "./kvm/test/contracts/exec-dest-ctx-parent"
    "./kvm/test/contracts/memgrow-wrong"
    "./kvm/test/contracts/exec-same-ctx-recursive-parent"
    "./kvm/test/contracts/exec-dest-ctx-recursive-parent"
    "./kvm/test/contracts/bad-empty"
    "./kvm/test/contracts/exec-dest-ctx-recursive"
    "./kvm/test/contracts/counter"
    "./kvm/test/contracts/init-simple"
    "./kvm/test/contracts/exec-dest-ctx-kda/basic"
    "./kvm/test/contracts/exec-same-ctx-child"
    "./kvm/test/contracts/opcodes"
    "./kvm/test/contracts/bad-misc"
    "./kvm/test/contracts/timelocks"
    "./kvm/test/contracts/erc20"
)

for contract in "${CONTRACT_LIST[@]}"
do
    ./buildWasmC.sh $contract
done

# copy builded contracts to the current directory
cp $KLEVER_VM_SDK_RS_PATH/contracts/feature-tests/erc-style-contracts/erc20/output/erc20.wasm kvm/test/erc20-rust/output/erc20.wasm
cp $KLEVER_VM_SDK_RS_PATH/contracts/feature-tests/kda-system-sc-mock/output/kda-system-sc-mock.wasm ./kvm/test/features/kda-system-sc-mock/output/kda-system-sc-mock.wasm
cp $KLEVER_VM_SDK_RS_PATH/contracts/feature-tests/payable-features/output/payable-features.wasm ./kvm/test/features/kda-system-sc-mock/output/payable-features.wasm
cp $KLEVER_VM_SDK_RS_PATH/contracts/feature-tests/payable-features/output/payable-features.wasm ./kvm/test/features/payable-features/output/payable-features.wasm
cp $KLEVER_VM_SDK_RS_PATH/contracts/feature-tests/basic-features/output/basic-features.wasm ./kvm/test/features/basic-features/output/basic-features.wasm
cp $KLEVER_VM_SDK_RS_PATH/contracts/feature-tests/managed-map-features/output/managed-map-features.wasm ./kvm/test/features/managed-map-features/output/managed-map-features.wasm
cp $KLEVER_VM_SDK_RS_PATH/contracts/feature-tests/formatted-message-features/output/formatted-message-features.wasm ./kvm/test/features/formatted-message-features/output/formatted-message-features.wasm
cp $KLEVER_VM_SDK_RS_PATH/contracts/feature-tests/basic-features/output/basic-features.wasm ./kvm/test/features/basic-features-no-small-int-api/output/features-no-small-int-api.wasm
cp $KLEVER_VM_SDK_RS_PATH/contracts/feature-tests/big-float-features/output/big-float-features.wasm ./kvm/test/features/big-float-features/output/big-float-features.wasm
cp $KLEVER_VM_SDK_RS_PATH/contracts/feature-tests/alloc-features/output/alloc-features.wasm ./kvm/test/features/alloc-features/output/alloc-features.wasm
cp $KLEVER_VM_SDK_RS_PATH/contracts/examples/factorial/output/factorial.wasm ./kvm/test/factorial/output/factorial.wasm
cp $KLEVER_VM_SDK_RS_PATH/contracts/examples/digital-cash/output/digital-cash.wasm ./kvm/test/digital-cash/output/digital-cash.wasm
cp $KLEVER_VM_SDK_RS_PATH/contracts/examples/crowdfunding-kda/output/crowdfunding-kda.wasm ./kvm/test/crowdfunding-kda/output/crowdfunding-kda.wasm
cp $KLEVER_VM_SDK_RS_PATH/contracts/examples/adder/output/adder.wasm ./kvm/test/adder/output/adder.wasm
cp ./kvm/test/contracts/erc20/output/erc20.wasm ./kvm/test/erc20-c/contracts/erc20-c.wasm
