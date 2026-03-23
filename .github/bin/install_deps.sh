#!/usr/bin/env bash
set -xeuo pipefail

export GOLANG_VERSION="1.25.7"
export FPM_VERSION="1.17.0"

export CURL_RETRY_OPTS=(--retry 5 --retry-delay 5)

function install_deps_debian11() {
  rm -rf /var/lib/apt/lists/*
  apt-get clean
  apt-get update -o Acquire::Retries=5
  apt -y install ruby-rubygems make rpm git curl binutils

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-amd64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-arm64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm -v "$FPM_VERSION"
  rm -rf /var/lib/apt/lists/*
}

function install_deps_debian12() {
  rm -rf /var/lib/apt/lists/*
  apt-get clean
  apt-get update -o Acquire::Retries=5
  apt -y install ruby-rubygems make rpm git curl binutils

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-amd64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-arm64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm -v "$FPM_VERSION"
  rm -rf /var/lib/apt/lists/*
}

function install_deps_debian13() {
  rm -rf /var/lib/apt/lists/*
  apt-get clean
  apt-get update -o Acquire::Retries=5
  apt -y install ruby-rubygems make rpm git curl binutils

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-amd64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-arm64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm -v "$FPM_VERSION"
  rm -rf /var/lib/apt/lists/*
}

function install_deps_ubuntu20.04() {
  rm -rf /var/lib/apt/lists/*
  apt-get clean
  apt-get update -o Acquire::Retries=5
  apt -y install ruby make rpm git curl binutils

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-amd64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-arm64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm -v "$FPM_VERSION"
  rm -rf /var/lib/apt/lists/*
}

function install_deps_ubuntu22.04() {
  rm -rf /var/lib/apt/lists/*
  apt-get clean
  apt-get update -o Acquire::Retries=5
  apt -y install ruby-rubygems make rpm git curl binutils

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-amd64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-arm64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm -v "$FPM_VERSION"
  rm -rf /var/lib/apt/lists/*
}

function install_deps_ubuntu24.04() {
  rm -rf /var/lib/apt/lists/*
  apt-get clean
  apt-get update -o Acquire::Retries=5
  apt -y install ruby-rubygems make rpm git curl binutils

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-amd64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-arm64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm -v "$FPM_VERSION"
  rm -rf /var/lib/apt/lists/*
}

function install_deps_el8() {
  dnf clean all
  dnf -y update
  dnf module enable -y ruby:2.7
  dnf -y install ruby ruby-devel redhat-rpm-config rubygems rpm-build make git
  gem install --no-document fpm

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-amd64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-arm64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm -v "$FPM_VERSION"
  dnf clean all
}

function install_deps_el9() {
  dnf clean all
  dnf -y update
  dnf -y install ruby rpmdevtools make git

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-amd64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-arm64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm -v "$FPM_VERSION"
  dnf clean all
}

function install_deps_el10() {
  dnf clean all
  dnf -y update
  dnf -y install ruby rpmdevtools make git

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-amd64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-arm64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm -v "$FPM_VERSION"
  dnf clean all
}

function install_deps_amzn2023() {
  dnf clean all
  dnf -y update
  dnf -y install ruby rpmdevtools make git

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-amd64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L "${CURL_RETRY_OPTS[@]}" https://go.dev/dl/go"$GOLANG_VERSION".linux-arm64.tar.gz -o /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go"$GOLANG_VERSION".linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm -v "$FPM_VERSION"
  dnf clean all
}
