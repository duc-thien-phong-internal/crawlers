#!/bin/bash

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do # resolve $SOURCE until the file is no longer a symlink
  DIR="$(cd -P "$(dirname "$SOURCE")" >/dev/null 2>&1 && pwd)"
  SOURCE="$(readlink "$SOURCE")"
  [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE" # if $SOURCE was a relative symlink, we need to resolve it relative to the path where the symlink file was located
done
DIR="$(cd -P "$(dirname "$SOURCE")" >/dev/null 2>&1 && pwd)"
BASEDIR=$(realpath ${DIR}/../..)
echo "Basedir: ${BASEDIR}"

OS="windows"

OUTDIR=${BASEDIR}/bin/executable/cicb/${OS}/
SRC=cicb
EXECUTABLE=cicb.exe

echo "=======> Building ${OS}....."

env GOOS=${OS} GOARCH=amd64 go build -o ${OUTDIR}${EXECUTABLE} ${BASEDIR}/cmd/${SRC} && cd $OUTDIR
zip -r ${SRC}.zip ${EXECUTABLE}
cd -
echo "CICB executable: ${OUTDIR}${EXECUTABLE}"