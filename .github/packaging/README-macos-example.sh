#!/usr/bin/env bash
# If you are on MacOS you can use the following to test
docker run -t -i \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(realpath $(pwd)/../):$(realpath $(pwd)/../) \
  --workdir=$(pwd) \
  ubuntu:22.04 \
  bash -c \
  'apt -y update; 
   apt -y install git; 
   apt -y install docker.io; 
   $(pwd)/.github/packaging/common/entrypoint.sh -c -d el9'
docker run -t -i \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(realpath $(pwd)/../):$(realpath $(pwd)/../) \
  --workdir=$(pwd) \
  ubuntu:22.04 \
  bash -c \
  'apt -y update; 
   apt -y install git; 
   apt -y install docker.io; 
   $(pwd)/.github/packaging/common/entrypoint.sh -e -d el9'