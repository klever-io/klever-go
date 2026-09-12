# Klever Blockchain Docker Images

This directory contains Docker configurations for building and running Klever Blockchain nodes.

## Available Image Variants

Klever Blockchain provides multiple Docker image variants to suit different use cases and deployment scenarios:

### Base Image Types

| Base Image | Size | Use Case |
|------------|------|----------|
| **Debian (Trixie)** | ~300-400MB | Production deployments, full compatibility |
| **Alpine Linux** | ~165-215MB | Resource-constrained environments, minimal footprint |

### Image Flavors

| Flavor | Description | Binaries Included |
|--------|-------------|-------------------|
| **Full** | Complete node with all binaries | validator, seednode, operator, keygenerator |
| **Validator** | Validator-only (optimized) | validator only |

### Supported Architectures

All images are built for multiple architectures:
- `linux/amd64` (x86_64)
- `linux/arm64` (aarch64)

Docker will automatically pull the correct architecture for your system.

## Image Tags

### Tag Format

Images follow this tagging pattern:
```
kleverapp/klever-go:[variant-][environment-]<version>[network-suffix]
```

### Examples

| Tag | Description |
|-----|-------------|
| `latest` | Latest mainnet Debian full image |
| `v1.0.0` | Specific version mainnet Debian full image |
| `val-latest` | Latest mainnet Debian validator-only image |
| `alpine-latest` | Latest mainnet Alpine full image |
| `alpine-val-latest` | Latest mainnet Alpine validator-only image |
| `latest-testnet` | Latest testnet Debian full image |
| `dev-latest` | Latest devnet Debian full image |

## Quick Start

The examples below publish host port 8080. That only reaches the REST API if the
process also binds beyond loopback. The binary default is `localhost:8080`. Do
not combine a published port with `--rest-api-interface=0.0.0.0:8080` (or
`:8080`) unless authentication and TLS sit in front of the listener. See
[SECURITY.md](../SECURITY.md).

### Running a Full Node (Debian)

```bash
docker run -d \
  --name klever-node \
  -p 8080:8080 \
  -v klever-data:/opt/klever-blockchain \
  kleverapp/klever-go:latest
```

### Running a Validator Node (Alpine)

```bash
docker run -d \
  --name klever-validator \
  -p 8080:8080 \
  -v klever-data:/opt/klever-blockchain \
  kleverapp/klever-go:alpine-val-latest
```

### Using Docker Compose

```bash
docker-compose up -d
```

## Building Images Locally

### Prerequisites

- Docker 20.10+
- Docker Buildx (for multi-arch builds)
- Go 1.25+
- `modvendor` tool

### Single-Platform Builds

Build for your current platform only:

```bash
# Production builds (mainnet)
make docker-build                        # Debian full
make docker-build-validator              # Debian validator
make docker-build-alpine                 # Alpine full
make docker-build-alpine-validator       # Alpine validator

# Development builds (use FOR_DEV variable)
FOR_DEV=dev- make docker-build           # Tagged as dev-<version>

# Testnet builds (use FOR_TESTNET variable)
FOR_TESTNET=-testnet make docker-build   # Tagged as <version>-testnet

# Combined dev + testnet
FOR_DEV=dev- FOR_TESTNET=-testnet make docker-build  # Tagged as dev-<version>-testnet
```

### Multi-Architecture Builds

Build and push images for both amd64 and arm64:

```bash
# Setup buildx builder (one time)
make docker-builder-setup

# Build and push specific variant
make docker-buildx-full                  # Debian full
make docker-buildx-validator             # Debian validator
make docker-buildx-alpine-full           # Alpine full
make docker-buildx-alpine-validator      # Alpine validator

# Build and push all variants
make docker-buildx-all
```

**Note:** Multi-architecture builds automatically push to the registry. Ensure you're logged in to Docker Hub:

```bash
docker login
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `KLEVER_HOME` | `/opt/klever-blockchain` | Base directory for blockchain data |
| `ENABLE_HEALTHCHECK` | `false` | Enable/disable healthcheck endpoint |
| `STALE_THRESHOLD` | `20` | Blocks behind threshold for health check |

### Volume Mounts

| Path | Purpose |
|------|---------|
| `/opt/klever-blockchain/db` | Blockchain database |
| `/opt/klever-blockchain/logs` | Application logs |
| `/opt/klever-blockchain/config` | Configuration files |
| `/opt/klever-blockchain/stats` | Statistics data |
| `/opt/klever-blockchain/compiledSCStorage` | Smart contract storage |

### User Configuration

Images run as non-root user `klever` (UID 999, GID 999) by default. You can customize this during build:

```bash
docker build \
  --build-arg USER_ID=1000 \
  --build-arg GROUP_ID=1000 \
  -f docker/Dockerfile .
```

## Health Checks

All images include a built-in healthcheck that can be enabled:

```bash
docker run -d \
  -e ENABLE_HEALTHCHECK=true \
  -e STALE_THRESHOLD=20 \
  kleverapp/klever-go:latest
```

Check health status:

```bash
docker inspect --format='{{.State.Health.Status}}' klever-node
```

## Image Comparison

### Size Comparison (Approximate)

| Variant | Uncompressed Size |
|---------|-------------------|
| Debian Full | ~300-400MB |
| Debian Validator | ~280-380MB |
| Alpine Full | ~165-215MB |
| Alpine Validator | ~150-200MB |

**Note:** Testnet images are slightly larger due to additional debugging symbols. Actual sizes may vary based on the specific version and build configuration.

### Performance

- **Debian images**: Slightly better performance, broader compatibility
- **Alpine images**: Minimal overhead, faster startup, smaller resource footprint

### Choosing the Right Image

**Use Debian images when:**
- Running in production with standard infrastructure
- Maximum compatibility is required
- Debugging tools may be needed

**Use Alpine images when:**
- Minimizing image size is a priority
- Running in resource-constrained environments
- Deploying to edge locations or IoT devices

**Use Full images when:**
- Running multiple node types on the same container
- Administrative tools are needed
- Operating a seednode or other node types

**Use Validator images when:**
- Running dedicated validator nodes
- Minimizing attack surface
- Optimizing for single-purpose deployments

## Security Considerations

### Non-Root User

All images run as the non-root `klever` user by default for security best practices.

### Read-Only Filesystem

Images support read-only root filesystem (except for required data directories):

```bash
docker run -d \
  --read-only \
  --tmpfs /tmp \
  -v klever-data:/opt/klever-blockchain \
  kleverapp/klever-go:latest
```

### Security Updates

- Debian images are based on Debian Trixie (stable)
- Alpine images use Alpine Linux 3.19 (pinned for stability)
- Regular security updates should be applied by rebuilding images

## Troubleshooting

### Permission Issues

If you encounter permission errors with volume mounts:

```bash
# Set correct ownership on host
sudo chown -R 999:999 /path/to/klever-data

# Or use custom UID/GID matching your host user
docker build --build-arg USER_ID=$(id -u) --build-arg GROUP_ID=$(id -g) ...
```

### Alpine-Specific Issues

Alpine uses musl libc instead of glibc. If you encounter library compatibility issues, use Debian-based images instead.

### Multi-Arch Build Issues

If buildx fails:

```bash
# Remove and recreate builder
make docker-builder-remove
make docker-builder-setup
```

## Continuous Integration

Images are automatically built and published via GitHub Actions on release. See `.github/workflows/release-docker.yaml` for details.

### Workflow Inputs

- **Release Type**: mainnet, testnet, devnet
- **Latest Tag**: Whether to push `latest` tag
- **Flavor**: Which image variant(s) to build (all, full-debian, validator-debian, full-alpine, validator-alpine)

## Development

### Local Testing

Test images locally before pushing:

```bash
# Build without pushing
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg arg_version=test \
  -f docker/Dockerfile \
  -t kleverapp/klever-go:test \
  .

# Load for local testing (single platform)
docker buildx build \
  --platform linux/amd64 \
  --build-arg arg_version=test \
  -f docker/Dockerfile \
  --load \
  -t kleverapp/klever-go:test \
  .
```

### Debugging

Enter a running container:

```bash
docker exec -it klever-node sh
```

View logs:

```bash
docker logs -f klever-node
```

## Migration Guide

### From Single-Arch to Multi-Arch

Multi-arch images are drop-in replacements. Simply pull the new tag:

```bash
docker pull kleverapp/klever-go:latest
docker-compose up -d
```

Docker automatically selects the correct architecture.

### From Full to Validator Images

If migrating to validator-only images:

1. Ensure you only need the validator binary
2. Update your image tag to include `val-` prefix
3. All data and configurations remain compatible

```bash
# Before
image: kleverapp/klever-go:latest

# After
image: kleverapp/klever-go:val-latest
```

## Contributing

When adding new Dockerfiles or modifying existing ones:

1. Test on both amd64 and arm64 (use QEMU if needed)
2. Update this README with any new variants
3. Update the Makefile with appropriate targets
4. Update GitHub Actions workflow if needed
5. Document any breaking changes

## License

See the root LICENSE file for details.
