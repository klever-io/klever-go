# KVM - Klever Virtual Machine

WebAssembly-based virtual machine for executing smart contracts on the Klever blockchain.

## Overview

KVM provides a sandboxed execution environment powered by Wasmer 2.x, with gas metering and blockchain context access for smart contracts.

## Directory Structure

- `vmhost/` - VM host implementation and context management
- `wasmer2/` - Wasmer runtime integration and shared libraries
- `executor/` - WASM executor abstraction
- `crypto/` - Cryptographic operations (hashing, signing)
- `scenarioexec/` - JSON-based scenario testing framework
- `test/contracts/` - Example smart contracts for testing
- `mock/` - Mock implementations for unit testing

## Testing

```bash
# Run KVM tests
make tests-kvm

# Run specific package
go test ./kvm/vmhost/...
```

## Configuration

Gas costs are defined in `config/node/gasScheduleV1.yaml`.
