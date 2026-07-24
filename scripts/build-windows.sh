#!/usr/bin/env bash
# Windows ネイティブ実行ファイル (3dlibrary.exe) をビルドする。
#
# 開発・ビルドは WSL、実行は Windows という構成のためのヘルパー。
# フロントエンド (Vite) を internal/web/static へ出力してバイナリに埋め込み、
# CGO 不要 (pure-Go の glebarez/sqlite) なのでそのままクロスコンパイルできる。
#
# 使い方:
#   scripts/build-windows.sh [出力先.exe]
# 出力先を省略すると OUT_EXE 環境変数、なければ既定の Windows パスへ出力する。
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out="${1:-${OUT_EXE:-/mnt/c/Users/karth/3DLibrary/3dlibrary.exe}}"

echo "==> フロントエンドをビルド (-> internal/web/static)"
cd "$repo_root/frontend"
if [ ! -d node_modules ]; then npm install; fi
npm run build

echo "==> Windows exe をクロスコンパイル: $out"
mkdir -p "$(dirname "$out")"
cd "$repo_root"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o "$out" ./cmd/3dlibrary

echo "==> 完了"
ls -la "$out"
