FROM quay.io/kairos/kairos-init:v0.13.0 AS kairos-init

FROM ghcr.io/kairos-io/hadron:v0.0.1-beta2 AS base
ARG VERSION=v1.0.0

RUN --mount=type=bind,from=kairos-init,src=/kairos-init,dst=/kairos-init \
  /kairos-init -l debug -s install --version "${VERSION}" && \
  /kairos-init -l debug -s init --version "${VERSION}"

COPY firewall /usr/bin/firewall
RUN chmod +x /usr/bin/firewall