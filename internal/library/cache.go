package library

import "path/filepath"

// CacheSet はあるアセットのキャッシュ 3 点のパス(requirements.md §10)。
type CacheSet struct {
	GLB       string
	Thumbnail string
	Metadata  string
}

// CachePaths はアセット(category/title)のキャッシュファイルパスを返す。
// キー = カテゴリ/タイトルのディレクトリ階層。
func CachePaths(libDir, category, title string) CacheSet {
	return CacheSet{
		GLB:       filepath.Join(libDir, "cache", "glb", category, title+".glb"),
		Thumbnail: filepath.Join(libDir, "cache", "thumbnails", category, title+".png"),
		Metadata:  filepath.Join(libDir, "cache", "metadata", category, title+".json"),
	}
}
