##!/bin/bash
OS="linux"
RUN=0

POSITIONAL=()
while [[ $# -gt 0 ]]
do
key="$1"

case $key in
    -o|--os)
    OS="$2"
    shift # past argument
    shift # past value
    ;;
    -r|--run)
    RUN=1
    shift # past argument
    ;;
    *)    # unknown option
    POSITIONAL+=("$1") # save it in an array for later
    shift # past argument
    ;;
esac
done
set -- "${POSITIONAL[@]}" # restore positional parameters



SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do # resolve $SOURCE until the file is no longer a symlink
  DIR="$(cd -P "$(dirname "$SOURCE")" >/dev/null 2>&1 && pwd)"
  SOURCE="$(readlink "$SOURCE")"
  [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE" # if $SOURCE was a relative symlink, we need to resolve it relative to the path where the symlink file was located
done
DIR="$(cd -P "$(dirname "$SOURCE")" >/dev/null 2>&1 && pwd)"
BASEDIR=$(realpath ${DIR}/../..)

SRC=cmd/cicb
EXECUTABLE=cicb
OUTDIR=${BASEDIR}/bin/executable/${EXECUTABLE}/${OS}/

echo "Basedir: ${BASEDIR}"
echo "Source: ${BASEDIR}/${SRC}"

echo "=======> Building ${OS}....."
echo "OS: ${OS}"

if [ "$OS" = "windows" ]; then
  EXECUTABLE=$EXECUTABLE.exe
fi

cd ${BASEDIR}/${SRC}; echo `pwd`; env GOOS=${OS} GOARCH=amd64 go build -o ${OUTDIR}${EXECUTABLE} ${BASEDIR}/${SRC} && cd - && cd $OUTDIR
zip -r ${EXECUTABLE}.zip ${EXECUTABLE}
cd -



