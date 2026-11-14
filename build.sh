#!/bin/sh
# Cross-compiles the terminal browser for every desktop target into dist/.
set -e
cd "$(dirname "$0")"
mkdir -p dist
build() {
  os=$1; arch=$2; out=dist/asciiweb-$os-$arch
  [ "$os" = windows ] && out=$out.exe
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$out" ./cmd/asciiweb
  echo "built $out"
}
build darwin arm64
build darwin amd64
build windows amd64
build windows arm64
build linux amd64
build linux arm64
