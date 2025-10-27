ARG BASE_IMAGE
FROM $BASE_IMAGE
ARG ENV_DISTRO
ENV ENV_DISTRO=${ENV_DISTRO}
ARG REPO_NAME
ENV REPO_NAME=${REPO_NAME}
ARG JF_USERNAME
ENV JF_USERNAME=${JF_USERNAME}
ARG JF_TOKEN
ENV JF_TOKEN=${JF_TOKEN}
ARG PKG_VERSION
ENV PKG_VERSION=${PKG_VERSION}
#noop for RHEL based enviornments
ENV DEBIAN_FRONTEND=noninteractive
SHELL ["/bin/bash", "-euox", "pipefail", "-c"]
ADD .github/docker/test/entrypoint.sh /tmp/entrypoint.sh
ADD .github/docker/test/install_test_deps.sh /tmp/install_test_deps.sh
RUN source /tmp/install_deps.sh && "install_test_deps_$ENV_DISTRO"
ADD . /opt/$REPO_NAME
WORKDIR /opt/$REPO_NAME
ENTRYPOINT ["/opt/${REPO_NAME}/.github/docker/test/entrypoint.sh"]
CMD ["-t"]
