#
# Aerospike asconfig - Ubuntu 24.04
#
# https://github.com/aerospike/asconfig
#
# Supports amd64 and arm64.
# Update version-specific ARGs with: docker/update-version.sh
#

ARG ASCONFIG_AMD64_URL="https://aerospike.jfrog.io/artifactory/database-deb-dev-local/pool/noble/aerospike-asconfig/aerospike-asconfig_0.0.0_ubuntu24.04_x86_64.deb"
ARG ASCONFIG_AMD64_SHA256="PLACEHOLDER"
ARG ASCONFIG_ARM64_URL="https://aerospike.jfrog.io/artifactory/database-deb-dev-local/pool/noble/aerospike-asconfig/aerospike-asconfig_0.0.0_ubuntu24.04_aarch64.deb"
ARG ASCONFIG_ARM64_SHA256="PLACEHOLDER"

FROM ubuntu:24.04

LABEL org.opencontainers.image.title="Aerospike asconfig" \
      org.opencontainers.image.description="asconfig, a tool for managing Aerospike configuration." \
      org.opencontainers.image.source="https://github.com/aerospike/asconfig" \
      org.opencontainers.image.base.name="docker.io/library/ubuntu:24.04" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.vendor="Aerospike"

ARG ASCONFIG_AMD64_URL
ARG ASCONFIG_AMD64_SHA256
ARG ASCONFIG_ARM64_URL
ARG ASCONFIG_ARM64_SHA256
# When set to a filename, the package is copied from the build context instead
# of downloaded from ASCONFIG_*_URL. Set by docker/docker-build.sh -u <local-dir>.
# ASCONFIG_LOCAL_PKG        - used by single-arch test builds (legacy).
# ASCONFIG_LOCAL_PKG_AMD64  - used by multi-arch push builds for amd64.
# ASCONFIG_LOCAL_PKG_ARM64  - used by multi-arch push builds for arm64.
ARG ASCONFIG_LOCAL_PKG=""
ARG ASCONFIG_LOCAL_PKG_AMD64=""
ARG ASCONFIG_LOCAL_PKG_ARM64=""

SHELL ["/bin/bash", "-Eeuo", "pipefail", "-c"]

# hadolint ignore=DL3008
RUN --mount=type=bind,source=.,target=/build-ctx \
    export DEBIAN_FRONTEND=noninteractive && \
    ARCH="$(dpkg --print-architecture)" && \
    if [ "${ARCH}" = "amd64" ]; then \
        PKG_URL="${ASCONFIG_AMD64_URL}"; PKG_SHA="${ASCONFIG_AMD64_SHA256}"; \
        LOCAL_PKG="${ASCONFIG_LOCAL_PKG_AMD64:-${ASCONFIG_LOCAL_PKG}}"; \
    else \
        PKG_URL="${ASCONFIG_ARM64_URL}"; PKG_SHA="${ASCONFIG_ARM64_SHA256}"; \
        LOCAL_PKG="${ASCONFIG_LOCAL_PKG_ARM64:-${ASCONFIG_LOCAL_PKG}}"; \
    fi && \
    apt-get update -y && \
    apt-get install -y --no-install-recommends ca-certificates && \
    if [ -n "${LOCAL_PKG}" ] && [ -f "/build-ctx/${LOCAL_PKG}" ]; then \
        cp "/build-ctx/${LOCAL_PKG}" /tmp/asconfig.deb; \
    else \
        apt-get install -y --no-install-recommends curl && \
        curl -fsSL --retry 3 --retry-delay 3 "${PKG_URL}" -o /tmp/asconfig.deb && \
        apt-get purge -y --auto-remove curl; \
    fi && \
    if [ "${PKG_SHA}" != "PLACEHOLDER" ]; then \
        echo "${PKG_SHA}  /tmp/asconfig.deb" | sha256sum -c -; \
    fi && \
    apt-get install -y /tmp/asconfig.deb && \
    rm -f /tmp/asconfig.deb && \
    rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["asconfig"]
