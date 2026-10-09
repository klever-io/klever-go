# Build environment for the Anthropic OSS Scanner (https://red.anthropic.com/oss-scanner).
# The build context is the repository checkout; the scan itself runs without network access.
FROM golang:1.25-trixie

ENV DEBIAN_FRONTEND=noninteractive \
    CGO_ENABLED=1 \
    LD_LIBRARY_PATH=/src/kvm/wasmer2

WORKDIR /src
COPY . /src

# Fetch every module now so tests and rebuilds work offline.
RUN go mod download

# Same flags as `make build`, without the release ldflags.
RUN make build

# Compile (but do not run) every test binary so the build cache is warm offline.
RUN go test -count=1 -run '^$' ./... > /dev/null
