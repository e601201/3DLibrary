package library

import (
	"io/fs"
	"os"
	"path/filepath"
)

// cacheSubdirs は cache 直下の骨格(requirements.md §10)。
var cacheSubdirs = func() []string {
	dirs := make([]string, len(cacheKinds))
	for i, k := range cacheKinds {
		dirs[i] = k.dir
	}
	return dirs
}()

// CacheSize は cache 配下の合計サイズとファイル数を実測で返す。
// (server 側 dirSize は表示用でエラーを無視する別物。こちらは
// 「実測値」を返す約束のためエラーを伝播する)
func CacheSize(libDir string) (int64, int, error) {
	var size int64
	var count int
	root := filepath.Join(libDir, "cache")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // cache 未作成は空として扱う
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			size += info.Size()
			count++
		}
		return nil
	})
	return size, count, err
}

// ClearCache は cache を丸ごと削除し(直下の迷子ファイルも含む)、
// 空の骨格を作り直す(requirements.md §11)。source・インデックス・
// templates には触れない。キャッシュはいつ削除してもよい派生データで
// あり、再生成は「不足分を一括生成」で行う。
func ClearCache(libDir string) error {
	if err := os.RemoveAll(filepath.Join(libDir, "cache")); err != nil {
		return err
	}
	for _, sub := range cacheSubdirs {
		if err := os.MkdirAll(filepath.Join(libDir, "cache", sub), 0o755); err != nil {
			return err
		}
	}
	return nil
}
