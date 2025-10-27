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
ARG PACKAGE_NAME
ENV PACKAGE_NAME=${PACKAGE_NAME}
#noop for non-golang projects
ENV GOROOT=/opt/golang/go/
#noop for RHEL based enviornments
ENV DEBIAN_FRONTEND=noninteractive
ENV TEST_MODE="true"


SHELL ["/bin/bash", "-euox", "pipefail", "-c"]
ADD . /opt/$REPO_NAME
WORKDIR /opt/$REPO_NAME

RUN source .github/packaging/common/project/test/header.sh && source .github/packaging/common/project/test/install_deps.sh && install_deps $ENV_DISTRO

ENTRYPOINT [".github/packaging/common/test/entrypoint.sh"]
CMD ["-t"]
