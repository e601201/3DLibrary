// Package library はライブラリディレクトリの初期化を担う。
// 骨格(source / cache / templates)の作成と同梱テンプレートの配置のみを行い、
// 既存の中身があるディレクトリには一切書き込まない(requirements.md §4)。
package library

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/e601201/3DLibrary/internal/index"
)

// アプリ同梱のテンプレート。ライブラリ初期化時に templates/ へ配置される。
// Blender 5.2 LTS で作成(requirements.md §9 の規定どおり)。
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
	entries, err := f.ReadDir(-1)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		// インデックスはアプリ生成の派生物なので空判定では無視する
		// (起動時スキャンが初期化より先に DB を作ることがある)
		if e.Name() != index.DBFileName {
			return false, nil
		}
	}
	return true, nil
}
