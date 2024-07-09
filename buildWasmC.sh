#!/bin/bash

# this script will build wasm from c contracts

# get directory from args
if [ -z "$1" ]
then
    echo "No directory supplied"
    exit 1
fi

echo "Building WASM for $1"
name=$(basename $1)
clang -cc1 -Wno-int-conversion -Wno-array-bounds -Wno-gnu-folding-constant -Wno-pointer-sign -emit-llvm -triple=wasm32-unknown-unknown-wasm -o $1/${name}.ll -Ofast $1/${name}.c
llvm-link -o $1/${name}.ll $1/${name}.ll
llc -O3 -filetype=obj $1/${name}.ll -o $1/${name}.o
wasm-ld --no-entry $1/${name}.o -o $1/output/${name}.wasm --strip-all -allow-undefined --export-all
# remove intermediate files
rm $1/${name}.ll $1/${name}.o
