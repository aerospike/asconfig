ARG BASE_IMAGE
FROM $BASE_IMAGE
ARG ENV_DISTRO
ENV ENV_DISTRO=${ENV_DISTRO}
ARG REPO_NAME
ENV REPO_NAME=${REPO_NAME}
SHELL ["/bin/bash", "-euox", "pipefail", "-c"]

#noop for non-golang projects
ENV GOROOT=/opt/golang/go/

#noop for redhat enviornments
ENV DEBIAN_FRONTEND=noninteractive
RUN if command -v apt; then apt -y update; fi

ADD . /opt/$REPO_NAME
WORKDIR /opt/$REPO_NAME

RUN source .github/docker/header.sh && source .github/docker/install_deps.sh && install_deps $ENV_DISTRO

ADD . /opt/$REPO_NAME
WORKDIR /opt/$REPO_NAME
ENTRYPOINT [".github/docker/entrypoint.sh"]
CMD ["-b"]