// Package library はライブラリディレクトリの初期化を担う。
// 骨格(source / cache / templates)の作成と同梱テンプレートの配置のみを行い、
// 既存の中身があるディレクトリには一切書き込まない(requirements.md §4)。
package library

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// アプリ同梱のテンプレート。ライブラリ初期化時に templates/ へ配置される。
// NOTE: 本来は Blender 4.5 LTS で作成する規定(requirements.md §9)だが、
// 現状は Blender 5.0.1 で生成したファイルを同梱している。4.5 系ユーザーへの
// 配布前に 4.5 LTS で作り直すこと。
//
//go:embed templates/empty.blend
var emptyBlend []byte

var skeletonDirs = []string{
	"source",
	filepath.Join("cache", "glb"),
	filepath.Join("cache", "thumbnails"),
	filepath.Join("cache", "metadata"),
	"templates",
}

// Ensure は dir をライブラリとして使える状態にする。
//   - 空のディレクトリ → 骨格と empty.blend を作成する
//   - 中身があるディレクトリ → 既存ライブラリとみなし、何も書き込まない
//   - 存在しないパス → エラー(タイポで意図しない場所にライブラリを
//     作ってしまわないよう、ディレクトリは利用者が先に用意する)
func Ensure(dir string) error {
	empty, err := isEmptyDir(dir)
	if err != nil {
		return err
	}
	if !empty {
		return nil
	}
	for _, d := range skeletonDirs {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, "templates", "empty.blend"), emptyBlend, 0o644)
}

func isEmptyDir(dir string) (bool, error) {
	f, err := os.Open(dir)
	if os.IsNotExist(err) {
		return false, fmt.Errorf("directory does not exist: %s", dir)
	}
	if err != nil {
		return false, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("not a directory: %s", dir)
	}
	if _, err := f.ReadDir(1); err == io.EOF {
		return true, nil
	} else if err != nil {
		return false, err
	}
	return false, nil
}
