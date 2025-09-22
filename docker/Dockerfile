FROM golang:1.25-bookworm AS builder

# NOSONAR
COPY . /go/src/github.com/klever-io/klever-go

WORKDIR /go/src/github.com/klever-io/klever-go

ARG arg_version
ENV VERSION=$arg_version

RUN make build

FROM debian:bookworm-slim

# Install dependencies
RUN apt-get update && \
    apt-get install --no-install-recommends -y \
    ca-certificates \
    curl \
    gosu && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* && \
    update-ca-certificates

# Create a non-root user with configurable UID/GID
ARG USER_ID=999
ARG GROUP_ID=999
RUN groupadd --system --gid ${GROUP_ID} klever && \
    useradd --system --uid ${USER_ID} --gid ${GROUP_ID} --create-home klever

# Set up directories and permissions
ENV KLEVER_HOME=/opt/klever-blockchain
ENV KLEVER_DIRS="stats db logs config compiledSCStorage"

RUN mkdir -p ${KLEVER_HOME} && \
    for dir in ${KLEVER_DIRS}; do \
        mkdir -p "${KLEVER_HOME}/${dir}"; \
        chown klever:klever "${KLEVER_HOME}/${dir}"; \
        chmod 0750 "${KLEVER_HOME}/${dir}"; \
    done && \
    chown klever:klever ${KLEVER_HOME} && \
    chmod 0750 ${KLEVER_HOME}

# Copy binaries and config
COPY --from=builder --chown=root:klever --chmod=550 /go/src/github.com/klever-io/klever-go/bin/* /usr/local/bin/
COPY --from=builder --chown=root:klever --chmod=440 /go/src/github.com/klever-io/klever-go/kvm/wasmer2/libvmexeccapi.so /lib/libvmexeccapi.so
COPY --chown=root:klever --chmod=440 config/seednode/config.yaml ${KLEVER_HOME}/config/seednode/config.yaml

# Copy scripts
COPY --chown=root:klever --chmod=550 docker/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY --chown=root:klever --chmod=550 docker/healthcheck.sh /usr/local/bin/healthcheck.sh

# Set environment variables
ENV ENABLE_HEALTHCHECK=false
ENV STALE_THRESHOLD=20

# Set working directory
WORKDIR ${KLEVER_HOME}

# Add healthcheck
HEALTHCHECK --interval=15s --timeout=10s --start-period=100s --retries=3 \
    CMD if [ "$ENABLE_HEALTHCHECK" = "true" ]; then /usr/local/bin/healthcheck.sh; else exit 0; fi

# Use non-root user
USER klever

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["/usr/local/bin/validator", "--use-log-view", "--log--save"]