#!/usr/bin/env bash
set -xeuo pipefail
env

if [ -d ".git" ]; then
    GIT_DIR=$(pwd)
    PKG_DIR=$GIT_DIR/pkg
fi

function install_deps_debian12() {
  apt -y install ruby-rubygems make rpm git snapd curl binutils
  curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
  mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  gem install fpm
}


function install_deps_debian11() {
  apt -y install ruby-rubygems make rpm git snapd curl binutils
  curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
  mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  gem install fpm
}


function install_deps_ubuntu20.04() {
  apt -y install ruby make rpm git snapd curl binutils
  curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
  mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  gem install fpm
}

function install_deps_ubuntu22.04() {
  apt -y install ruby-rubygems make rpm git snapd curl binutils
  curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
  mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  gem install fpm
}

function install_deps_ubuntu24.04() {
  apt -y install ruby-rubygems make rpm git snapd curl binutils
  curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
  mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  gem install fpm
}

function install_deps_redhat_ubi9() {
  microdnf -y install ruby rpmdevtools make git
  curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
  mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  gem install fpm
}

function build_packages(){
  if [ "$ENV_DISTRO" = "" ]; then
    echo "ENV_DISTRO is not set"
    return
  fi
  export PATH=$PATH:/opt/golang/go/bin
  GIT_DIR=$(git rev-parse --show-toplevel)

  # build
  cd "$GIT_DIR"
  make clean
  make

  # package
  cd $PKG_DIR
  make clean
  make

  mkdir -p /tmp/output/$ENV_DISTRO
  cp -a $PKG_DIR/target/* /tmp/output/$ENV_DISTRO
}

function build_ubuntu_images() {
  docker build  -t asconfig-pkg-builder-ubuntu-2004 -f .github/docker/Dockerfile-ubuntu20.04 .
  docker build  -t asconfig-pkg-builder-ubuntu-2204 -f .github/docker/Dockerfile-ubuntu22.04 .
  docker build  -t asconfig-pkg-builder-ubuntu-2404 -f .github/docker/Dockerfile-ubuntu24.04 .
}

function build_redhat_images() {
  docker build -t asconfig-pkg-builder-redhat-ubi9 -f .github/docker/Dockerfile-redhat_ubi9 .
}

function build_debian_images() {
  docker build -t asconfig-pkg-builder-debian-11 -f .github/docker/Dockerfile-debian11 .
  docker build -t asconfig-pkg-builder-debian-12 -f .github/docker/Dockerfile-debian12 .
}

function build_package_ubuntu20.04() {
  docker run -v $(realpath ../dist):/tmp/output asconfig-pkg-builder-ubuntu-2004
  ls -laht ../dist
}

function build_package_ubuntu22.04() {
  docker run -v $(realpath ../dist):/tmp/output asconfig-pkg-builder-ubuntu-2204
  ls -laht ../dist
}

function build_package_ubuntu24.04() {
  docker run -v $(realpath ../dist):/tmp/output asconfig-pkg-builder-ubuntu-2404
  ls -laht ../dist
}

function build_package_redhat_ubi9() {
  docker run -v $(realpath ../dist):/tmp/output asconfig-pkg-builder-redhat-ubi9
  ls -laht ../dist
}

function build_package_debian11() {
  docker run -v $(realpath ../dist):/tmp/output asconfig-pkg-builder-debian-11
  ls -laht ../dist
}

function build_package_debian12() {
  docker run -v $(realpath ../dist):/tmp/output asconfig-pkg-builder-debian-12
  ls -laht ../dist
}

SCRIPT_DIR="$(dirname "$(realpath "$0")")"

INSTALL=false
BUILD_INTERNAL=false
BUILD_CONTAINERS=false
EXECUTE_BUILD=false
BUILD_DISTRO=all

while getopts "ibced:" opt; do
    case ${opt} in
        i )
            INSTALL=true
            ;;
        b )
            BUILD_INTERNAL=true
            ;;
        c )
            BUILD_CONTAINERS=true
            ;;
        e )
            EXECUTE_BUILD=true
            ;;
        d )
            BUILD_DISTRO="$OPTARG"
            ;;
    esac
done

shift $((OPTIND -1))

if [ "$INSTALL" = false ] && [ "$BUILD_INTERNAL" = false ] && [ "$BUILD_CONTAINERS" = false ] && [ "$EXECUTE_BUILD" = false ];
then
    echo """Error: Options:
    -i ( install )
    -b ( build internal )
    -c ( build containers )
    -e ( execute docker package build )
    -d [ redhat | ubuntu | debian ]""" 1>&2
    exit 1
fi

if grep -q 20.04 /etc/os-release; then
  ENV_DISTRO="ubuntu20.04"
elif grep -q 22.04 /etc/os-release; then
  ENV_DISTRO="ubuntu22.04"
elif grep -q 24.04 /etc/os-release; then
  ENV_DISTRO="ubuntu24.04"
elif grep -q "platform:el9" /etc/os-release; then
  ENV_DISTRO="redhat_ubi9"
elif grep -q "bullseye" /etc/os-release; then
  ENV_DISTRO="debian11"
elif grep -q "bookworm" /etc/os-release; then
  ENV_DISTRO="debian12"
else
  cat /etc/os-release
  echo "os not supported"
fi


if [ "$INSTALL" = "true" ]; then
  if [ "$ENV_DISTRO" = "ubuntu20.04" ]; then
      echo "installing dependencies for Ubuntu 20.04"
      install_deps_ubuntu20.04
  elif [ "$ENV_DISTRO" = "ubuntu22.04" ]; then
      echo "installing dependencies for Ubuntu 22.04"
      install_deps_ubuntu22.04
  elif [ "$ENV_DISTRO" = "ubuntu24.04" ]; then
      echo "installing dependencies for Ubuntu 24.04"
      install_deps_ubuntu24.04
  elif [ "$ENV_DISTRO" = "redhat_ubi9" ]; then
      echo "installing dependencies for RedHat UBI9"
      install_deps_redhat_ubi9
  elif [ "$ENV_DISTRO" = "debian11" ]; then
      echo "installing dependencies for Debian 11"
      install_deps_debian11
  elif [ "$ENV_DISTRO" = "debian12" ]; then
      echo "installing dependencies for Debian 12"
      install_deps_debian12
  else
      cat /etc/os-release
      echo "distro not supported"
  fi
elif [ "$BUILD_INTERNAL" = "true" ]; then
  build_packages
elif [ "$BUILD_CONTAINERS" = "true" ]; then
  if [ -n "$BUILD_DISTRO" ]; then
    if [ "$BUILD_DISTRO" = "ubuntu" ]; then
      build_ubuntu_images
    elif [ "$BUILD_DISTRO" = "redhat" ]; then
      build_redhat_images
    elif [ "$BUILD_DISTRO" = "debian" ]; then
      build_debian_images
    elif [ "$BUILD_DISTRO" = "all" ]; then
        build_ubuntu_images
        build_redhat_images
        build_debian_images
    else
      echo "Unsupported distro: $BUILD_DISTRO"
      exit 1
    fi
  fi
fi

if [ "$EXECUTE_BUILD" = "true" ]; then
   if [ "$BUILD_DISTRO" = "ubuntu20.04" ]; then
        echo "building package for Ubuntu 20.04"
        build_package_ubuntu20.04
    elif [ "$BUILD_DISTRO" = "ubuntu22.04" ]; then
        echo "building package for Ubuntu 22.04"
        build_package_ubuntu22.04
    elif [ "$BUILD_DISTRO" = "ubuntu24.04" ]; then
        echo "building package for Ubuntu 24.04"
        build_package_ubuntu24.04
    elif [ "$BUILD_DISTRO" = "redhat_ubi9" ]; then
        echo "building package for RedHat UBI9"
        build_package_redhat_ubi9
    elif [ "$BUILD_DISTRO" = "debian11" ]; then
        echo "building package for Debian 11"
        build_package_debian11
    elif [ "$BUILD_DISTRO" = "debian12" ]; then
        echo "building package for Debian 12"
        build_package_debian12
    elif [ "$BUILD_DISTRO" = "all" ]; then
        echo "building package for Ubuntu 20.04"
        build_package_ubuntu20.04
        echo "building package for Ubuntu 22.04"
        build_package_ubuntu22.04
        echo "building package for Ubuntu 24.04"
        build_package_ubuntu24.04
        #echo "building package for RedHat UBI9"
        #build_package_redhat_ubi9
        echo "building package for Debian 11"
        build_package_debian11
        echo "building package for Debian 12"
        build_package_debian12
    else
        cat /etc/os-release
        echo "distro not supported"
    fi
fi