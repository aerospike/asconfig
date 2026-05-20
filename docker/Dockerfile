#
# Aerospike asconfig - Docker image.
#
# https://github.com/aerospike/asconfig
#
# Supports linux/amd64 and linux/arm64.
# Driven by docker/docker-build.sh, which passes the version-specific URLs +
# SHA256s as build args (and optionally an in-context .deb via ASCONFIG_LOCAL_PKG*).
# Build directly only if you set those args yourself; otherwise use the script.
#
# Base: debian:trixie-slim. The .deb is built on Ubuntu 24.04; trixie ships a
# recent glibc and common C libs the Go binary may dynamically link to — apt
# installs the .deb and anything it (transitively) needs.
#

ARG ASCONFIG_AMD64_URL=""
ARG ASCONFIG_AMD64_SHA256=""
ARG ASCONFIG_ARM64_URL=""
ARG ASCONFIG_ARM64_SHA256=""

FROM debian:trixie-slim

ARG ASCONFIG_AMD64_URL
ARG ASCONFIG_AMD64_SHA256
ARG ASCONFIG_ARM64_URL
ARG ASCONFIG_ARM64_SHA256
# When set to a filename, the package is copied from the build context instead
# of downloaded from ASCONFIG_*_URL. Set by docker/docker-build.sh -u <local-dir>.
ARG ASCONFIG_LOCAL_PKG=""
ARG ASCONFIG_LOCAL_PKG_AMD64=""
ARG ASCONFIG_LOCAL_PKG_ARM64=""

LABEL org.opencontainers.image.title="Aerospike asconfig" \
      org.opencontainers.image.description="asconfig, a tool for managing Aerospike configuration files." \
      org.opencontainers.image.source="https://github.com/aerospike/asconfig" \
      org.opencontainers.image.base.name="docker.io/library/debian:trixie-slim" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.vendor="Aerospike"

SHELL ["/bin/bash", "-Eeuo", "pipefail", "-c"]

# Fetch (or copy) → verify SHA → apt-install the .deb. apt resolves any
# transitive Depends from the .deb.
# curl is installed only when we need to download, then purged.
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
    if [ -n "${LOCAL_PKG}" ] && [ -f "/build-ctx/${LOCAL_PKG}" ]; then \
        cp "/build-ctx/${LOCAL_PKG}" /tmp/asconfig.deb; \
    else \
        apt-get install -y --no-install-recommends curl ca-certificates && \
        curl -fsSL --retry 3 --retry-delay 3 "${PKG_URL}" -o /tmp/asconfig.deb && \
        apt-get purge -y --auto-remove curl; \
    fi && \
    if [ -n "${PKG_SHA}" ] && [ "${PKG_SHA}" != "PLACEHOLDER" ]; then \
        echo "${PKG_SHA}  /tmp/asconfig.deb" | sha256sum -c -; \
    fi && \
    apt-get install -y --no-install-recommends /tmp/asconfig.deb && \
    rm -f /tmp/asconfig.deb && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

CMD ["asconfig"]
