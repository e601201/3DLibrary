package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// macOS の Blender はアプリバンドル(ディレクトリ)で、実行ファイルはこの中にある。
// Finder 上は Blender.app が 1 個のアプリに見えるため、利用者はバンドル自体の
// パスを入力しがちだが、それを exec すると "permission denied" になる。
const macAppBinaryRel = "Contents/MacOS/Blender"

// NormalizeBlenderPath は利用者が入力した Blender のパスを、実際に起動できる
// パスへ整えて返す。macOS で /Applications/Blender.app のようなアプリバンドルを
// 指定された場合は、中の実行ファイルへ読み替える。
// 空文字(未設定)はそのまま通す。
func NormalizeBlenderPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("Blender が見つかりません: %s", path)
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return path, nil
	}

	inner := filepath.Join(path, filepath.FromSlash(macAppBinaryRel))
	if info, err := os.Stat(inner); err == nil && !info.IsDir() {
		return inner, nil
	}
	if filepath.Ext(path) == ".app" {
		return "", fmt.Errorf(
			"%s はアプリバンドルですが、中に実行ファイル(%s)が見つかりません",
			path, macAppBinaryRel)
	}
	return "", fmt.Errorf(
		"%s はディレクトリです。Blender の実行ファイルを指定してください", path)
}
