#!/usr/bin/env bash
# Cross-compiles the osf CLI into dist/: one archive per platform plus
# checksums.txt. Asset names carry no version, so
#   https://github.com/lostwood-games/open-story-foundation/releases/latest/download/<asset>
# always points at the newest release; install.sh and install.ps1 rely on this.
#
#   bash scripts/dist.sh 0.1.0
set -euo pipefail

version="${1:?usage: bash scripts/dist.sh <version, e.g. 0.1.0>}"
version="${version#v}"
targets="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"

root="$(cd "$(dirname "$0")/.." && pwd)"
dist="$root/dist"
rm -rf "$dist"
mkdir -p "$dist/stage"

cd "$root/src/cli"
for t in $targets; do
  os="${t%/*}" arch="${t#*/}"
  name="osf_${os}_${arch}"
  exe=osf
  [ "$os" = windows ] && exe=osf.exe
  mkdir -p "$dist/stage/$name"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X main.version=${version}" -o "$dist/stage/$name/$exe" ./cmd/osf
  cp "$root/LICENSE" "$dist/stage/$name/"
  if [ "$os" = windows ]; then
    (cd "$dist/stage/$name" && zip -q "$dist/$name.zip" ./*)
  else
    (cd "$dist/stage/$name" && tar -czf "$dist/$name.tar.gz" ./*)
  fi
  echo "built dist/$name"
done

rm -rf "$dist/stage"
cd "$dist"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum osf_* >checksums.txt
else
  shasum -a 256 osf_* >checksums.txt
fi
cat checksums.txt
