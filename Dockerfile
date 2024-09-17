FROM golang:1.22-bookworm AS builder

COPY . /go/src/github.com/klever-io/klever-go

WORKDIR /go/src/github.com/klever-io/klever-go

ARG arg_version
ENV VERSION=$arg_version

RUN make build

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y ca-certificates curl --no-install-recommends \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates 

# Create a non-root user
RUN addgroup --system --gid 1001 klever \
    && adduser --system --uid 1001 --ingroup klever klever

# Copy the built binary from the builder stage
COPY --from=builder --chown=klever:klever /go/src/github.com/klever-io/klever-go/bin/validator /usr/local/bin/
RUN chmod 0555 /usr/local/bin/validator

# Copy the necessary library
COPY --from=builder --chown=klever:klever /go/src/github.com/klever-io/klever-go/kvm/wasmer2/libvmexeccapi.so /lib/libvmexeccapi.so
RUN chmod 0444 /lib/libvmexeccapi.so

COPY --from=builder --chown=klever:klever /go/src/github.com/klever-io/klever-go/bin/seednode /usr/local/bin/
RUN chmod 0555 /usr/local/bin/seednode

COPY --from=builder --chown=klever:klever /go/src/github.com/klever-io/klever-go/bin/operator /usr/local/bin/
RUN chmod 0555 /usr/local/bin/operator

COPY --from=builder --chown=klever:klever /go/src/github.com/klever-io/klever-go/bin/keygenerator /usr/local/bin/
RUN chmod 0555 /usr/local/bin/keygenerator

USER klever

# Copy seednode configuration example file
COPY --chown=klever:klever config/seednode/config.yaml /opt/klever-blockchain/config/seednode/config.yaml
RUN chmod 0444 /opt/klever-blockchain/config/seednode/config.yaml

# Create necessary directories
RUN mkdir -p /opt/klever-blockchain/stats && \
    mkdir -p /opt/klever-blockchain/db && \
    mkdir -p /opt/klever-blockchain/logs && \
    chown -R klever:klever /opt/klever-blockchain
    
WORKDIR /opt/klever-blockchain

HEALTHCHECK --interval=10s CMD exit 0

ENTRYPOINT [ "/usr/local/bin/validator", "--use-log-view" ]
