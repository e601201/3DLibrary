package library

import (
	"os"
	"path/filepath"
)

// Categories は source 直下のカテゴリ名を返す(requirements.md §4 カテゴリ)。
// カテゴリは固定の集合ではなく、source 直下のディレクトリがすべてである。
// ファイルと不可視エントリはカテゴリではない。
func Categories(libDir string) ([]string, error) {
	return subdirs(filepath.Join(libDir, "source"))
}

// AssetTitles はカテゴリ直下のアセットのタイトルを返す(requirements.md §5)。
// タイトル = アセットディレクトリ名。それより深い階層はアセットの私有物
// なので関知しない。model.blend の有無は問わない(不完全アセットもアセット
// であり、スキャンにも現れる)。
func AssetTitles(libDir, category string) ([]string, error) {
	return subdirs(filepath.Join(libDir, "source", category))
}

// subdirs は dir 直下の可視ディレクトリ名を返す。カテゴリもアセットも
// 「ディレクトリだけを数える」規則は同じなので、ここに一本化している。
func subdirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, e := range entries {
		if e.IsDir() && !IsHidden(e.Name()) {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
