#!/bin/sh
# Installs the osf CLI (Open Story Format) on macOS and Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/lostwood-games/open-story-foundation/main/install.sh | sh
#
# Environment:
#   OSF_VERSION      version to install, e.g. 0.1.0 (default: the latest release)
#   OSF_INSTALL_DIR  where to put the binary (default: ~/.local/bin)
set -eu

repo="https://github.com/lostwood-games/open-story-foundation"
dir="${OSF_INSTALL_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) echo "osf: unsupported system $(uname -s). On Windows, use install.ps1." >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "osf: unsupported CPU $(uname -m)." >&2; exit 1 ;;
esac

asset="osf_${os}_${arch}.tar.gz"
if [ -n "${OSF_DOWNLOAD_URL:-}" ]; then
  base="$OSF_DOWNLOAD_URL" # a folder holding the assets, for testing
elif [ -n "${OSF_VERSION:-}" ]; then
  base="$repo/releases/download/v${OSF_VERSION#v}"
else
  base="$repo/releases/latest/download"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $base/$asset"
curl -fsSL -o "$tmp/$asset" "$base/$asset"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt"

want="$(grep " $asset\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
if command -v sha256sum >/dev/null 2>&1; then
  got="$(sha256sum "$tmp/$asset" | cut -d' ' -f1)"
else
  got="$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1)"
fi
if [ -z "$want" ] || [ "$want" != "$got" ]; then
  echo "osf: checksum mismatch for $asset" >&2
  exit 1
fi

tar -C "$tmp" -xzf "$tmp/$asset"
mkdir -p "$dir"
mv "$tmp/osf" "$dir/osf"
chmod +x "$dir/osf"

echo "Installed $("$dir/osf" version | head -n 1) to $dir/osf"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) echo "Add $dir to your PATH, for example: echo 'export PATH=\"$dir:\$PATH\"' >> ~/.profile" ;;
esac
