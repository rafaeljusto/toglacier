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
readonly TMP_PATH="/tmp/toglacier/"
readonly BIN_PATH="$TMP_PATH/usr/local/bin/"
readonly CONF_PATH="$TMP_PATH/etc/"

exit_error() {
  echo "$1. Abort" 1>&2
  exit 1
}

prepare() {
  rm -f toglacier*.deb 2>/dev/null

  mkdir -p $BIN_PATH || exit_error "Cannot create the temporary path"
  mkdir -p $CONF_PATH || exit_error "Cannot create the temporary path"
}

compile() {
  local project_path=`echo $GOPATH | cut -d: -f1`
  local program_path=""
  local current_path=`pwd`

  program_path=$project_path/src/github.com/rafaeljusto/toglacier/cmd/toglacier
  cd $program_path || exit_error "Cannot change directory"
  env GOOS=linux GOARCH=amd64 go build -ldflags "-X github.com/rafaeljusto/toglacier/internal/config.Version=$VERSION" || exit_error "Compile error"
  mv toglacier $BIN_PATH || exit_error "Error copying binary"
  cp toglacier.yml $CONF_PATH/toglacier.yml.sample || exit_error "Error copying configuration sample"

  program_path=$project_path/src/github.com/rafaeljusto/toglacier/cmd/toglacier-storage
  cd $program_path || exit_error "Cannot change directory"
  env GOOS=linux GOARCH=amd64 go build -ldflags "-X github.com/rafaeljusto/toglacier/internal/config.Version=$VERSION" || exit_error "Compile error"
  mv toglacier-storage $BIN_PATH || exit_error "Error copying binary"

  cd $current_path
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
    --prefix / -C $TMP_PATH usr/local/bin etc
}

cleanup() {
  rm -rf $TMP_PATH
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