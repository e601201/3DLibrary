package main

import "runtime/debug"

// devVersion は、ビルドにバージョンが入っていないとき(go run など)の表示。
const devVersion = "dev"

// appVersion はこのバイナリのバージョンを返す。
//
// バージョンの正は git タグで、go build がタグとコミットからモジュールの
// バージョンを自動で書き込む(-ldflags は使わない)。タグの付いていない
// コミットでは疑似バージョン、作業ツリーに変更があれば +dirty が付く。
func appVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return devVersion
	}
	return displayVersion(info.Main.Version)
}

// displayVersion はビルド情報のバージョンを表示用に整える。
// go run では "(devel)"、情報が無ければ空になるので、どちらも dev と出す。
func displayVersion(v string) string {
	if v == "" || v == "(devel)" {
		return devVersion
	}
	return v
}
