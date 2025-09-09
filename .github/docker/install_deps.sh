#!/usr/bin/env bash
VERSION=$(git rev-parse HEAD | cut -c -8)

function install_deps_debian12() {
  apt -y install ruby-rubygems make rpm git snapd curl binutils
  curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
  mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}



function install_deps_debian11() {
  apt -y install ruby-rubygems make rpm git snapd curl binutils
  curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
  mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}


function install_deps_ubuntu20.04() {
  apt -y install ruby make rpm git snapd curl binutils
  curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
  mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}

function install_deps_ubuntu22.04() {
  apt -y install ruby-rubygems make rpm git snapd curl binutils
  curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
  mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}

function install_deps_ubuntu24.04() {
  apt -y install ruby-rubygems make rpm git snapd curl binutils
  curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
  mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  install /opt/golang/go/bin/go /usr/local/bin/
  gem install fpm
}

function install_deps_redhat-ubi9() {
  microdnf -y install ruby rpmdevtools make git
  curl -L https://go.dev/dl/go1.24.6.linux-amd64.tar.gz -o /tmp/go1.24.6.linux-amd64.tar.gz
  mkdir -p /opt/golang && tar -zxvf /tmp/go1.24.6.linux-amd64.tar.gz -C /opt/golang
  install /opt/golang/go/bin/go /usr/local/bin/
  install /opt
  gem install fpm
}