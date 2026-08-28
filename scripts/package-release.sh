#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root_dir"

version=${1:-dev}
version_number=${version#v}
commit=${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || printf unknown)}
build_date=${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}
release_dir="$root_dir/release"
work_dir="$release_dir/work"
ldflags="-s -w -X github.com/gemini-fly/oms-platform/internal/buildinfo.Version=${version} -X github.com/gemini-fly/oms-platform/internal/buildinfo.Commit=${commit} -X github.com/gemini-fly/oms-platform/internal/buildinfo.Date=${build_date}"

rm -rf "$release_dir"
mkdir -p "$work_dir"

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  os=${target%/*}
  arch=${target#*/}
  package_name="oms-platform_${version_number}_${os}_${arch}"
  package_dir="$work_dir/$package_name"
  binary_name=oms-platform
  if [[ "$os" == "windows" ]]; then
    binary_name=oms-platform.exe
  fi

  mkdir -p "$package_dir/deploy"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "$ldflags" -o "$package_dir/$binary_name" ./cmd/sy-platform-api
  cp README.md LICENSE .env.example docker-compose.yml "$package_dir/"
  cp deploy/oms-platform.service "$package_dir/deploy/"

  if [[ "$os" == "windows" ]]; then
    (cd "$work_dir" && zip -qr "$release_dir/${package_name}.zip" "$package_name")
  else
    tar -C "$work_dir" -czf "$release_dir/${package_name}.tar.gz" "$package_name"
  fi
done

rm -rf "$work_dir"
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$release_dir" && sha256sum *.tar.gz *.zip > checksums.txt)
else
  (cd "$release_dir" && shasum -a 256 *.tar.gz *.zip > checksums.txt)
fi

printf 'release packages written to %s\n' "$release_dir"
