#!/bin/bash

set -e

VERSION=$1

if [[ -z $VERSION ]]; then
    VERSION=$(git describe --exact-match --tags HEAD | sed 's/v//g')
    if [[ -z $VERSION ]]; then
        echo "no version detected! git tag like 'v0.0.0' on current commit is required."
        exit 1
    fi
fi

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "invalid version! must be like '0.0.0'."
    exit 1
fi

echo "package version v${VERSION}"

echo "compile application"
mkdir -p ./gohome/usr/bin
CGO_ENABLED=0 GOARCH=amd64 go build -o ./gohome/usr/bin/gohome ../../

INST_SIZE=$(du -ks ./gohome/usr | cut -f 1)

echo "prepare package files"
mkdir -p ./gohome/DEBIAN
echo "Package: gohome
Version: ${VERSION}
Maintainer: sbreitf1
Architecture: amd64
Installed-Size: $INST_SIZE
Homepage: https://github.com/sbreitf1/gohome
Description: Calculates your leave time from Matrix entries." > ./gohome/DEBIAN/control

dpkg-deb --build gohome gohome_${VERSION}_amd64.deb

rm -rf ./gohome
