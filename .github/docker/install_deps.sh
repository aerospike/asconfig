#!/usr/bin/env bash





function install_deps_debian11() {
  apt -y install ruby-rubygems make rpm git snapd curl binutils

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-arm64.tar.gz -o /tmp/go1.24.6.linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}

function install_deps_debian12() {
  apt -y install ruby-rubygems make rpm git snapd curl binutils

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-arm64.tar.gz -o /tmp/go1.24.6.linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}

function install_deps_debian13() {
  apt -y install ruby-rubygems make rpm git snapd curl binutils

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-arm64.tar.gz -o /tmp/go1.24.6.linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}

function install_deps_ubuntu20.04() {
  apt -y install ruby make rpm git snapd curl binutils

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-arm64.tar.gz -o /tmp/go1.24.6.linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}

function install_deps_ubuntu22.04() {
  apt -y install ruby-rubygems make rpm git snapd curl binutils

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-arm64.tar.gz -o /tmp/go1.24.6.linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}

function install_deps_ubuntu24.04() {
  apt -y install ruby-rubygems make rpm git snapd curl binutils

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-arm64.tar.gz -o /tmp/go1.24.6.linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}

function install_deps_redhat-el8() {
  microdnf -y install ruby rpmdevtools make git

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-arm64.tar.gz -o /tmp/go1.24.6.linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}

function install_deps_redhat-el9() {
  microdnf -y install ruby rpmdevtools make git

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-arm64.tar.gz -o /tmp/go1.24.6.linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}


function install_deps_redhat-amazon-2023() {
  microdnf -y install ruby rpmdevtools make git

  if [ "$(uname -m)" = "x86_64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  elif [ "$(uname -m)" = "aarch64" ]; then
      curl -L https://go.dev/dl/go1.24.6.linux-arm64.tar.gz -o /tmp/go1.24.6.linux-arm64.tar.gz
      mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-arm64.tar.gz -C /opt/golang
  else
      echo "unknown arch $(uname -m)"
      exit 1
  fi
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}