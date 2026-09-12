# Klever Blockchain

Official Go implementation of the Klever blockchain protocol.

[![License](https://img.shields.io/badge/license-GPL%20v3-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8.svg)](https://golang.org/)
[![Release](https://img.shields.io/github/v/release/klever-io/klever-go)](https://github.com/klever-io/klever-go/releases)

## Overview

Klever is a high-performance blockchain network designed for decentralized applications, asset management, and smart contract execution. This repository contains the core node implementation, virtual machine (KVM), and CLI tools for interacting with the Klever network.

**Key Features:**

- **High Performance**: Optimized slot-based consensus mechanism
- **Smart Contracts**: WebAssembly-based virtual machine (KVM) for secure contract execution
- **Native KApps**: Protocol-level applications (KDA, staking, governance, marketplace)
- **Developer Friendly**: Comprehensive CLI tools and REST API

## Table of Contents

- [Installation](#installation)
- [Building from Source](#building-from-source)
- [Quick Start](#quick-start)
- [Running a Node](#running-a-node)
- [CLI Tools](#cli-tools)
- [Configuration](#configuration)
- [Development](#development)
- [Testing](#testing)
- [API Documentation](#api-documentation)
- [Architecture](#architecture)
- [Contributing](#contributing)
- [Security](#security)
- [Support](#support)
- [Resources](#resources)
- [License](#license)

## Installation

### Prerequisites

- Go 1.25 or higher
- Git
- Make
- Protocol Buffers compiler (protoc) - for development
- Swag - for API documentation generation

### Quick Install

```bash
git clone https://github.com/klever-io/klever-go.git
cd klever-go
make prepare
make build
```

This will build all binaries in the `cmd/` directories.

## Building from Source

### Build All Components

```bash
make build
```

This creates the following executables in `./bin/`:
- `validator` - Full blockchain validator node
- `operator` - CLI tool for blockchain operations
- `seednode` - Network bootstrap node
- `keygenerator` - Key generation utility
- `connector` - Node connection provider

### Build Individual Components

```bash
# Build only the validator node
make build-validator

# Build only the seed node
make build-seednode

# Build only the operator CLI
make build-operator

# Build only the key generator
make build-keygenerator
```

### Docker Build

```bash
make docker-build
```

## Quick Start

### Generate Keys

```bash
./bin/keygenerator --help
```

Generate validator and wallet keys for running a node or submitting transactions. See [cmd/keygenerator/CLI.md](cmd/keygenerator/CLI.md) for detailed options.

### Run a Validator Node

```bash
./bin/validator
```

By default, the node looks for configuration files in `./config/node/`. See [Running a Node](#running-a-node) for details.

### Run a Seed Node

```bash
./bin/seednode
```

By default, the seed node looks for configuration in `./config/seednode/`. See [cmd/seednode/CLI.md](cmd/seednode/CLI.md) for detailed options.

### Use the Operator CLI

```bash
# View available commands
./bin/operator --help

# View account commands
./bin/operator account --help

# View KDA commands
./bin/operator kda --help

# View validator commands
./bin/operator validator --help
```

For detailed CLI documentation, see [cmd/operator/docs/](cmd/operator/docs/).

## Running a Node

For comprehensive node setup and operation instructions, see the official documentation:

**[Node Operations Guide](https://docs.klever.org/node-operations#how-to-run-a-node)**

### Quick Reference

**Validator Node**:
```bash
./bin/validator
```

> **Note:** A node participates in consensus only if its public key is registered and staked in the network. Otherwise, it syncs the blockchain as an observer.

**Seed Node** (bootstrap peer for network discovery):
```bash
./bin/seednode
```

### Configuration Files

By default, the node looks for configuration files in `./config/node/`. Each file can be overridden individually:

| File | Flag | Description |
|------|------|-------------|
| `config.yaml` | `--config` | Main node configuration (P2P, logging, storage) |
| `enableEpochs.yaml` | `--config-epochs` | Feature activation schedule by epoch |
| `gasScheduleV1.yaml` | `--config-gas-schedule` | Smart contract gas costs |
| `api.yaml` | `--config-api` | REST API endpoint configuration |
| `external.yaml` | `--config-external` | External integrations (ElasticSearch) |
| `genesis.json` | `--genesis-file` | Genesis block configuration |
| `nodesSetup.json` | `--nodes-setup-file` | Initial validator set |

**Example** - Override a specific config:
```bash
./bin/validator --config-external /path/to/custom/external.yaml
```

For detailed configuration options, see the [Node Operations documentation](https://docs.klever.org/node-operations).

## Development

### Project Structure

```
klever-go/
├── cmd/                    # Executables (node, operator, seednode)
├── core/                   # Core blockchain logic
│   ├── consensus/          # Consensus mechanism
│   ├── process/            # Block and transaction processing
│   └── kapp/               # Protocol-level KApps
├── kvm/                    # Klever Virtual Machine
│   ├── vmhost/             # VM execution engine
│   └── wasmer2/            # WASM runtime
├── data/                   # Data structures (blocks, transactions)
├── storage/                # Storage backends (LevelDB, memory)
├── network/                # P2P networking and API
├── crypto/                 # Cryptographic operations
├── config/                 # Configuration files
└── integrationTest/        # Integration tests
```

### Adding a New Feature

1. Implement the feature in the appropriate package
2. Add unit tests alongside your code (`*_test.go`)
3. Update configuration if needed (`enableEpochs.yaml`)
4. Add integration tests in `integrationTest/`
5. Update documentation and CHANGELOG

### Code Style

- Run `make goimports` before committing
- Follow standard Go conventions and idioms
- See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines

## Testing

### Run All Tests

```bash
make tests
```

### Unit Tests

```bash
make tests-unit
```

### Integration Tests

```bash
make tests-integration
```

### KVM Tests

```bash
make tests-kvm
```

### E2E Tests

```bash
make tests-e2e
```

### Specific Package Tests

```bash
go test ./core/process/block/preprocess/
go test ./kvm/vmhost/...
```

### Test Coverage

```bash
go test -cover ./...
```

## API Documentation

The node exposes a REST API for blockchain queries and operations.

**Swagger Documentation**: Auto-generated API docs are available at:
- Running node: `http://localhost:8080/swagger/index.html`
- Source file: [docs/node_swagger.yaml](docs/node_swagger.yaml)

## Architecture

Klever blockchain features:

- **Consensus**: Slot-based mechanism with validator coordination
- **KVM**: WebAssembly-based smart contract execution engine
- **KApps**: Protocol-level applications (KDA, Staking, Governance, ITO, Marketplace)
- **Storage**: Pluggable backends with multi-layer caching
- **Networking**: libp2p-based P2P network

For detailed architecture documentation, see [docs.klever.org](https://docs.klever.org)

## Contributing

We welcome contributions! Please read:

- **[CONTRIBUTING.md](CONTRIBUTING.md)** - Contribution guidelines and workflow
- **[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)** - Community standards

Quick start:
1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Run `make tests` and `make goimports`
5. Submit a Pull Request

## Security

**Report security vulnerabilities**: See [SECURITY.md](SECURITY.md)

For security audit information, contact: security@klever.io

## Support

The **[Klever Forum](https://forum.klever.org)** is the official channel for technical support and community feedback. Use it to:
- Ask technical questions about running nodes or development
- Discuss ideas and get guidance from the community
- Engage with the Klever team and other developers

For **bug reports** and **feature requests**, please use [GitHub Issues](https://github.com/klever-io/klever-go/issues).

## Resources

- **Website**: https://klever.org
- **Documentation**: https://docs.klever.org
- **Explorer**: https://kleverscan.org
- **Forum**: https://forum.klever.org

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

This project is derived from [MultiversX](https://github.com/multiversx/mx-chain-go) (formerly Elrond), originally licensed under GPL v3. We acknowledge and thank the MultiversX team for their foundational work.

Built with:
- [Go](https://golang.org/)
- [libp2p](https://libp2p.io/)
- [Wasmer](https://wasmer.io/)
- [Protocol Buffers](https://protobuf.dev/)
