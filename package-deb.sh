#!/usr/bin/env bash
set -e

# package information
readonly PACKAGE_NAME="toglacier"
readonly VENDOR="Rafael Dantas Justo"
readonly MAINTAINER="Rafael Dantas Justo <rafael@justo.net.br>"
readonly URL="https://rafael.net.br"
readonly LICENSE="MIT"
readonly DESCRIPTION="Send data to Amazon Glacier service periodically."

# install information
readonly INSTALL_PATH="/usr/local/bin"
readonly TMP_PATH="/tmp/toglacier/$INSTALL_PATH"

exit_error() {
  echo "$1. Abort" 1>&2
  exit 1
}

prepare() {
  rm -f toglacier*.deb 2>/dev/null

  mkdir -p $TMP_PATH || exit_error "Cannot create the temporary path"
}

compile() {
  local project_path=`echo $GOPATH | cut -d: -f1`
  project_path=$project_path/src/github.com/rafaeljusto/toglacier

  cd $project_path || exit_error "Cannot change directory"
  go build || exit_error "Compile error"

  mv toglacier $TMP_PATH || exit_error "Error copying binary"
  cd - 1>/dev/null
}

build_deb() {
  local project_path=`echo $GOPATH | cut -d: -f1`
  project_path=$project_path/src/github.com/rafaeljusto/toglacier

  local version=`echo "$VERSION" | awk -F "-" '{ print $1 }'`
  local release=`echo "$VERSION" | awk -F "-" '{ print $2 }'`

  fpm -s dir -t deb \
    -n $PACKAGE_NAME -v "$version" --iteration "$release" --vendor "$VENDOR" \
    --maintainer "$MAINTAINER" --url $URL --license "$LICENSE" --description "$DESCRIPTION" \
    --deb-user root --deb-group root \
    --prefix / -C /tmp/toglacier usr/local/bin
}

cleanup() {
  rm -rf /tmp/toglacier
}

VERSION=$1

usage() {
 echo "Usage: $1 <version>"
}

if [ -z "$VERSION" ]; then
  echo "Undefined VERSION!"
  usage $0
  exit 1
fi

prepare
compile
build_deb
cleanup