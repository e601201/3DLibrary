// Package library はライブラリディレクトリの構造と規則を一手に担う。
//   - 初期化: 骨格(source / cache / templates)の作成と同梱テンプレートの
//     配置。既存の中身があるディレクトリには一切書き込まない(requirements.md §4)
//   - source: カテゴリ・アセットの列挙規則(Categories / AssetTitles)、
//     アセットの作成(CreateAsset)とアセットメタの読み書き(ReadTags /
//     WriteTags)。source へ書き込むのはこの 2 経路のみ
//   - cache: キャッシュのパス決定(CachePaths)、全削除(ClearCache)、
//     孤児キャッシュの掃除(PruneCache)
package library

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/e601201/3DLibrary/internal/index"
)

// アプリ同梱のテンプレート。ライブラリ初期化時に templates/ へ配置される。
// Blender 5.2 LTS で作成(requirements.md §9 の規定どおり)。
//
//go:embed templates/empty.blend
var emptyBlend []byte

// skeletonDirs はライブラリの骨格。cache 配下は cacheSubdirs から導出する
// (キャッシュ種別の追加が 1 箇所で済むように)。
var skeletonDirs = func() []string {
	dirs := []string{"source"}
	for _, sub := range cacheSubdirs {
		dirs = append(dirs, filepath.Join("cache", sub))
	}
	return append(dirs, "templates")
}()

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
		if e.Name() == index.DBFileName {
			continue
		}
		// OS が勝手に作る不可視ファイルも「中身」とはみなさない。
		// これを数えると、Finder で作って開いただけの空フォルダが
		// .DS_Store のせいで既存ライブラリ扱いになり、骨格が作られない。
		if IsHidden(e.Name()) {
			continue
		}
		return false, nil
	}
	return true, nil
}

// IsHidden は OS 標準で不可視として扱われるファイル名かを返す。
// macOS の Finder は開いただけのフォルダに .DS_Store を、Windows の
// エクスプローラーは Thumbs.db / desktop.ini を書き込む。利用者からは
// 見えないので、空ディレクトリ判定にもファイル一覧にも含めない。
func IsHidden(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	return name == "Thumbs.db" || name == "desktop.ini"
}
