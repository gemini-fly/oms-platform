#!/bin/sh
set -eu

repo=${OMS_PLATFORM_REPOSITORY:-gemini-fly/oms-platform}
version=${1:-${OMS_PLATFORM_VERSION:-}}
prefix=${PREFIX:-$HOME/.local}

if [ -z "$version" ]; then
  latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")
  version=${latest_url##*/}
fi

case $(uname -s) in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) printf 'unsupported operating system: %s\n' "$(uname -s)" >&2; exit 1 ;;
esac

case $(uname -m) in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) printf 'unsupported architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
esac

version_number=${version#v}
archive="oms-platform_${version_number}_${os}_${arch}.tar.gz"
base_url="https://github.com/$repo/releases/download/$version"
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

curl -fL "$base_url/$archive" -o "$tmp_dir/$archive"
curl -fL "$base_url/checksums.txt" -o "$tmp_dir/checksums.txt"
grep "  $archive\$" "$tmp_dir/checksums.txt" > "$tmp_dir/checksum.txt"

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$tmp_dir" && sha256sum -c checksum.txt)
else
  (cd "$tmp_dir" && shasum -a 256 -c checksum.txt)
fi

tar -C "$tmp_dir" -xzf "$tmp_dir/$archive"
mkdir -p "$prefix/bin"
install -m 0755 "$tmp_dir/oms-platform_${version_number}_${os}_${arch}/oms-platform" "$prefix/bin/oms-platform"

printf 'installed oms-platform %s to %s/bin/oms-platform\n' "$version" "$prefix"
printf 'add %s/bin to PATH when needed\n' "$prefix"
