#!/usr/bin/env bash
# 3DLibrary をビルドする。
#
# フロントエンド (Vite) を internal/web/static へ出力し、それを go:embed で
# 埋め込んだ単一バイナリを作る。この順序が要で、npm run build を飛ばして
# go build だけ実行すると、ビルドは成功するのに UI が空のバイナリができる
# (internal/web/static/.gitkeep があるため埋め込みが空にならず、失敗しない)。
#
# CGO 不要 (pure-Go の glebarez/sqlite) なので、どの OS からでも
# クロスコンパイルできる。
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
	cat <<'EOF'
使い方: scripts/build.sh [host|windows] [出力先]

  scripts/build.sh                  このマシン向け -> bin/3dlibrary
  scripts/build.sh windows          Windows 向け .exe
  scripts/build.sh windows out.exe  出力先を指定

Windows 向けの既定の出力先:
  WSL      -> %USERPROFILE%\3DLibrary\3dlibrary.exe
              (Windows 側なのでエクスプローラーからそのまま実行できる)
  それ以外 -> bin/3dlibrary.exe

出力先は OUT 環境変数でも指定できる。
EOF
}

# windows_default_out は Windows 向けビルドの既定の出力先を返す。
# WSL 上なら Windows 側のユーザーフォルダへ置く。ユーザー名は
# %USERPROFILE% から取得するので、特定の環境を直書きしない。
windows_default_out() {
	local win_home unix_home
	if command -v wslpath >/dev/null 2>&1 && command -v cmd.exe >/dev/null 2>&1; then
		win_home="$(cmd.exe /c 'echo %USERPROFILE%' 2>/dev/null | tr -d '\r\n')" || win_home=""
		if [ -n "$win_home" ] && unix_home="$(wslpath -u "$win_home" 2>/dev/null)"; then
			printf '%s\n' "$unix_home/3DLibrary/3dlibrary.exe"
			return
		fi
	fi
	printf '%s\n' "$repo_root/bin/3dlibrary.exe"
}

target="${1:-host}"
case "$target" in
-h | --help | help)
	usage
	exit 0
	;;
host)
	# ホストが Windows (Git Bash 等) なら go env が .exe を返す
	default_out="$repo_root/bin/3dlibrary$(go env GOEXE)"
	;;
windows)
	default_out="$(windows_default_out)"
	;;
*)
	echo "不明なターゲット: $target" >&2
	echo >&2
	usage >&2
	exit 1
	;;
esac
out="${2:-${OUT:-$default_out}}"

echo "==> フロントエンドをビルド (-> internal/web/static)"
cd "$repo_root/frontend"
if [ ! -d node_modules ]; then npm install; fi
npm run build

echo "==> バイナリをビルド ($target): $out"
mkdir -p "$(dirname "$out")"
cd "$repo_root"
if [ "$target" = windows ]; then
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o "$out" ./cmd/3dlibrary
else
	CGO_ENABLED=0 go build -o "$out" ./cmd/3dlibrary
fi

echo "==> 完了"
ls -la "$out"
