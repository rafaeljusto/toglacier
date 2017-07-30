#!/usr/bin/env bash
set -e

# package information
readonly PACKAGE_NAME="toglacier"
readonly COMMENT="toglacier - Backup to AWS Glacier"
readonly MAINTAINER="Rafael Dantas Justo <rafael@justo.net.br>"
readonly URL="https://rafael.net.br"
readonly DESCRIPTION="Send data to Amazon Glacier service periodically."

# install information
readonly TMP_PATH="/tmp/toglacier/"
readonly SCRIPTS_PATH="$TMP_PATH/usr/local/etc/rc.d"
readonly BIN_PATH="$TMP_PATH/usr/local/bin/"
readonly CONF_PATH="$TMP_PATH/etc/"

exit_error() {
  echo "$1. Abort" 1>&2
  exit 1
}

prepare() {
  mkdir -p $SCRIPTS_PATH || exit_error "Cannot create the temporary scripts path"
  mkdir -p $BIN_PATH || exit_error "Cannot create the temporary binary path"
  mkdir -p $CONF_PATH || exit_error "Cannot create the temporary configuration path"
}

copy_files() {
  local project_path=`echo $GOPATH | cut -d: -f1`
  project_path=$project_path/src/github.com/rafaeljusto/toglacier
  local version=`echo "$VERSION" | awk -F "-" '{ print $1 }'`
  local release=`echo "$VERSION" | awk -F "-" '{ print $2 }'`

  cp $project_path/deploy/txz/toglacier $SCRIPTS_PATH/toglacier || exit_error "Cannot copy execution script"

  # calculate the files size
  local files_size=0
  for f in `find $TMP_PATH -type f`
  do
    size=`stat --printf="%s" $f`
    files_size=`expr $files_size + $size`
  done

  # calculate the SHA256 of the files
  local manifest_file=""
  for file in `find $TMP_PATH -type f`
  do
    hash=`sha256sum $file | awk '{ print $1 }'`
    base=`echo $file | cut -c 15-`
    manifest_file="$manifest_file \"$base\":\"1\$${hash}\","
  done

  cat > $TMP_PATH/+COMPACT_MANIFEST <<EOF
{
"name":"$PACKAGE_NAME",
"origin":"tools/toglacier",
"version":"$version,$release",
"comment":"$COMMENT",
"maintainer":"$MAINTAINER",
"www":"$URL",
"abi":"FreeBSD:10:amd64",
"arch":"freebsd:10:x86:64",
"prefix":"/usr/local/bin",
"flatsize":$files_size,
"licenselogic":"single",
"desc":"$DESCRIPTION",
"categories":["tools"]
}
EOF

  cat > $TMP_PATH/+MANIFEST <<EOF
{
"name":"$PACKAGE_NAME",
"origin":"tools/toglacier",
"version":"$version,$release",
"comment":"$COMMENT",
"maintainer":"$MAINTAINER",
"www":"$URL",
"abi":"FreeBSD:10:amd64",
"arch":"freebsd:10:x86:64",
"prefix":"/usr/local/bin",
"flatsize":$files_size,
"licenselogic":"single",
"desc":"$DESCRIPTION",
"categories":["tools"],
"files":{
$manifest_file
}
}
EOF
}

compile() {
  local project_path=`echo $GOPATH | cut -d: -f1`
  local program_path=""
  local current_path=`pwd`

  program_path=$project_path/src/github.com/rafaeljusto/toglacier/cmd/toglacier
  cd $program_path || exit_error "Cannot change directory"
  env GOOS=freebsd GOARCH=amd64 go build -ldflags "-X github.com/rafaeljusto/toglacier/internal/config.Version=$VERSION" || exit_error "Compilation error"
  mv $program_path/toglacier $BIN_PATH || exit_error "Error copying binary"
  cp $program_path/toglacier.yml $CONF_PATH/toglacier.yml.sample || exit_error "Error copying configuration sample"

  program_path=$project_path/src/github.com/rafaeljusto/toglacier/cmd/toglacier-storage
  cd $program_path || exit_error "Cannot change directory"
  env GOOS=freebsd GOARCH=amd64 go build -ldflags "-X github.com/rafaeljusto/toglacier/internal/config.Version=$VERSION" || exit_error "Compilation error"
  mv $program_path/toglacier-storage $BIN_PATH || exit_error "Error copying binary"

  cd $current_path
}

build_txz() {
  local version=`echo "$VERSION" | awk -F "-" '{ print $1 }'`
  local release=`echo "$VERSION" | awk -F "-" '{ print $2 }'`
  local current_path=`pwd`
  local file=toglacier-${version}-${release}.txz

  cd $TMP_PATH

  find . -type f | cut -c 3- | sort | xargs tar -cJf "$current_path/$file" --transform 's,^usr,/usr,' --owner=root --group=wheel
}

cleanup() {
  rm -rf $TMP_PATH
}

VERSION=$1

uso() {
  echo "Usage: $1 <version>"
}

if [ -z "$VERSION" ]; then
  echo "Undefined VERSION!"
  uso $0
  exit 1
fi

cleanup
prepare
compile
copy_files
build_txz
cleanup