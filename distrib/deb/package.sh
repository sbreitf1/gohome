#!/bin/bash

set -e

VERSION=$(git describe --exact-match --tags HEAD | sed 's/v//g')
if [[ -z $VERSION ]]; then
    echo "no version detected! git tag like 'v0.0.0' on current commit is required."
    exit 1
fi
echo "package version v${VERSION}"

echo "compile application"
mkdir -p ./gohome/usr/bin
CGO_ENABLED=0 go build -o ./gohome/usr/bin/gohome ../../

echo "prepare package files"
mkdir -p ./gohome/DEBIAN
echo "Package: gohome
Version: ${VERSION}
Maintainer: sbreitf1
Architecture: all
Description: Calculates your leave times from Matrix entries." > ./gohome/DEBIAN/control

dpkg-deb --build gohome
mv gohome.deb gohome-${VERSION}.deb
