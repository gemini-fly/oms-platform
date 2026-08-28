#!/bin/sh
set -eu

repo=${OMS_PLATFORM_REPOSITORY:-gemini-fly/oms-platform}
version=${1:-${OMS_PLATFORM_VERSION:-}}
prefix=${PREFIX:-$HOME/.local}
api_url="https://api.github.com/repos/$repo"

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

tmp_dir=$(mktemp -d)
trap 'rm -r "$tmp_dir"' EXIT HUP INT TERM

if [ -z "$version" ]; then
  curl -fsSL "$api_url/releases/latest" -o "$tmp_dir/release.json"
  version=$(sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' "$tmp_dir/release.json" | head -n 1)
else
  curl -fsSL "$api_url/releases/tags/$version" -o "$tmp_dir/release.json"
fi
if [ -z "$version" ]; then
  printf 'unable to determine the latest release version\n' >&2
  exit 1
fi

version_number=${version#v}
archive="oms-platform_${version_number}_${os}_${arch}.tar.gz"

download_asset() {
  asset_name=$1
  destination=$2
  asset_id=$(awk -v name="$asset_name" '
    /"id":/ { value=$2; gsub(/,/, "", value); id=value }
    index($0, "\"name\": \"" name "\"") { print id; exit }
  ' "$tmp_dir/release.json")
  if [ -z "$asset_id" ]; then
    printf 'release asset not found: %s\n' "$asset_name" >&2
    exit 1
  fi
  curl -fsSL -H 'Accept: application/octet-stream' "$api_url/releases/assets/$asset_id" -o "$destination"
}

download_asset "$archive" "$tmp_dir/$archive"
download_asset checksums.txt "$tmp_dir/checksums.txt"
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
