
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

  git pull origin
  echo "tags"
  git tag

  echo "build_package.sh version: $(git describe --long)"
  export VERSION=$(git describe --long)
  # package
  cd $PKG_DIR
  make clean
  echo "building package for $BUILD_DISTRO"

  if [[ $ENV_DISTRO == *"ubuntu"* ]]; then
    make deb
  elif [[ $ENV_DISTRO == *"debian"* ]]; then
    make deb
  elif [[ $ENV_DISTRO == *"ubi"* ]]; then
    make rpm
  else
    make tar
  fi

  mkdir -p /tmp/output/$ENV_DISTRO
  cp -a $PKG_DIR/target/* /tmp/output/$ENV_DISTRO
}