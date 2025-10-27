ARG BASE_IMAGE
FROM $BASE_IMAGE
ARG ENV_DISTRO
ENV ENV_DISTRO=${ENV_DISTRO}
ARG REPO_NAME
ENV REPO_NAME=${REPO_NAME}
#noop for non-golang projects
ENV GOROOT=/opt/golang/go/
#noop for RHEL based enviornments
ENV DEBIAN_FRONTEND=noninteractive

SHELL ["/bin/bash", "-euox", "pipefail", "-c"]
ADD .github/docker/entrypoint.sh /tmp/entrypoint.sh
ADD .github/docker/install_deps.sh /tmp/install_deps.sh
RUN source /tmp/install_deps.sh && "install_deps_$ENV_DISTRO"
ADD . /opt/$REPO_NAME
WORKDIR /opt/$REPO_NAME
ENTRYPOINT [".github/docker/entrypoint.sh"]
CMD ["-b"]