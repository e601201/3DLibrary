#!/bin/bash
# 3DLibrary をダブルクリックで起動するランチャー (macOS)。
#
# Finder でこのファイルをダブルクリックするとターミナルが開き、アプリが起動して
# ブラウザが開く。終了するときはそのターミナルウィンドウを閉じる。
# Dock に入れておきたい場合は、このファイルを Dock へドラッグする。
set -u

url="http://127.0.0.1:8765"
port=8765

# シンボリックリンク経由で起動されても自分の場所を特定する
src="${BASH_SOURCE[0]}"
while [ -L "$src" ]; do
	dir="$(cd -P "$(dirname "$src")" && pwd)"
	src="$(readlink "$src")"
	case "$src" in
	/*) ;;
	*) src="$dir/$src" ;;
	esac
done
repo_root="$(cd -P "$(dirname "$src")/.." && pwd)"
bin="$repo_root/bin/3dlibrary"

# エラーで終わるときに、メッセージを読む前にウィンドウが閉じないようにする
pause_and_exit() {
	echo
	echo "何かキーを押すと閉じます。"
	read -r -n 1 -s
	exit "$1"
}

# すでに起動していれば二重に起動しない(ポートは固定なので 2 つ目は必ず失敗する)。
# ブラウザだけ開き直す。
if nc -z 127.0.0.1 "$port" >/dev/null 2>&1; then
	echo "3DLibrary はすでに起動しています。ブラウザを開きます。"
	open "$url"
	exit 0
fi

if [ ! -x "$bin" ]; then
	echo "実行ファイルが見つかりません:"
	echo "  $bin"
	echo
	printf "いまビルドしますか? [y/N]: "
	read -r answer
	case "$answer" in
	[yY]*)
		echo
		if ! "$repo_root/scripts/build.sh"; then
			echo
			echo "ビルドに失敗しました。ターミナルで scripts/build.sh を実行して原因を確認してください。"
			pause_and_exit 1
		fi
		;;
	*)
		echo "ターミナルで scripts/build.sh を実行してからもう一度お試しください。"
		pause_and_exit 1
		;;
	esac
	if [ ! -x "$bin" ]; then
		echo
		echo "ビルドは終わりましたが、実行ファイルが見つかりません: $bin"
		pause_and_exit 1
	fi
fi

echo "3DLibrary を起動します。"
echo "終了するときは、このウィンドウを閉じてください。"
echo
exec "$bin"
