package library

import "path/filepath"

// cacheKind はキャッシュ種別 1 つ分の置き場所。キャッシュのパスは
// すべて cache/{dir}/{category}/{title}{ext} の形で、ここから導出する
// (パスの生成と掃除・骨格の作成が同じ定義を共有し、ズレないように)。
type cacheKind struct {
	dir string
	ext string
}

var (
	glbKind       = cacheKind{"glb", ".glb"}
	thumbnailKind = cacheKind{"thumbnails", ".png"}
	metadataKind  = cacheKind{"metadata", ".json"}

	// cacheKinds はキャッシュ 3 点(requirements.md §10)。
	// 骨格の作成・全削除・掃除はこれを回す。
	cacheKinds = []cacheKind{glbKind, thumbnailKind, metadataKind}
)

// path はアセット(category/title)のこの種別のキャッシュパスを返す。
func (k cacheKind) path(libDir, category, title string) string {
	return filepath.Join(libDir, "cache", k.dir, category, title+k.ext)
}

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
		GLB:       glbKind.path(libDir, category, title),
		Thumbnail: thumbnailKind.path(libDir, category, title),
		Metadata:  metadataKind.path(libDir, category, title),
	}
}

// All はキャッシュ 3 点のパスを一括で返す(3 点まとめての作成・確認用)。
func (s CacheSet) All() []string {
	return []string{s.GLB, s.Thumbnail, s.Metadata}
}
