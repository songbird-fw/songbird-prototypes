# Build from the repository root:
#   docker build -f PoC-hadron-os/Dockerfile -t songbird-hadron:0.1.1 --build-arg VERSION=0.1.1 .
# TODO: pin every base image by digest (see README).

ARG PAYLOAD_DIR=archive/PoC-basic-xdp-firewall

# 1. Payload: compile the eBPF object and the static Go binary from source
FROM golang:1.23-bookworm AS payload
ARG PAYLOAD_DIR
RUN apt-get update \
 && apt-get install -y --no-install-recommends clang llvm linux-libc-dev \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY ${PAYLOAD_DIR}/ ./
RUN clang -O2 -g -target bpf -I/usr/include/x86_64-linux-gnu -Ibpf/include \
      -c bpf/firewall.c -o bpf/firewall.bpf.o \
 && CGO_ENABLED=0 go build -trimpath -o /out/firewall .

# 2. Kairos tooling
FROM quay.io/kairos/kairos-init:v0.13.0 AS kairos-init

# 3. Hadron image with the payload
FROM ghcr.io/kairos-io/hadron:v0.0.1-beta2 AS base
ARG VERSION=v1.0.0

RUN --mount=type=bind,from=kairos-init,src=/kairos-init,dst=/kairos-init \
  /kairos-init -l debug -s install --version "${VERSION}" && \
  /kairos-init -l debug -s init --version "${VERSION}"

COPY --from=payload /out/firewall /usr/bin/firewall
RUN chmod +x /usr/bin/firewall
