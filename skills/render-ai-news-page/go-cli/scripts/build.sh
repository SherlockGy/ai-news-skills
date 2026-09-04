#!/usr/bin/env sh
set -eu

log_prefix='[buildRenderAiNewsPage AI 新闻单页面构建]'
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cli_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
bin_directory="$cli_root/bin"

cd "$cli_root"
host_goos=$(go env GOHOSTOS)
host_goarch=$(go env GOHOSTARCH)
printf '%s %s\n' "$log_prefix" '运行测试'
GOOS=$host_goos GOARCH=$host_goarch go test ./...
mkdir -p "$bin_directory"

build_target() {
  target_os=$1
  output_name=$2
  output_path="$bin_directory/$output_name"
  printf '%s 构建 %s/amd64 -> %s\n' "$log_prefix" "$target_os" "$output_path"
  GOOS=$target_os GOARCH=amd64 go build -trimpath -ldflags '-s -w' -o "$output_path" ./cmd/render-ai-news-page
  test -f "$output_path"
}

build_target windows render-ai-news-page-windows-amd64.exe
build_target linux render-ai-news-page-linux-amd64
build_target darwin render-ai-news-page-darwin-amd64

printf '%s %s\n' "$log_prefix" '三平台 amd64 构建完成'
