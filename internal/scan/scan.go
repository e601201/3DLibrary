// Package scan は source ディレクトリを歩いてアセット一覧を組み立てる。
// source に対して完全に読み取り専用で、Blender は起動しない
// (requirements.md §7 スキャン)。
package scan

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/e601201/3DLibrary/internal/index"
	"github.com/e601201/3DLibrary/internal/library"
)

// Scan は libDir/source 直下のカテゴリ/アセットを列挙して返す。
//   - カテゴリ・アセットの列挙規則は library.Categories / library.AssetTitles
//     に一本化している(キャッシュの掃除が同じ規則で生死を判定できるように)
//   - アセットより深い階層はアセットの私有物で関知しない
//   - model.blend を持たないアセットは不完全アセットとして返す
//   - キャッシュ(GLB・サムネイル・抽出メタデータのポリゴン数)が
//     あれば表示用として取り込む(requirements.md §7 スキャン)
//   - meta.json のタグをインデックスへ射影する
//   - 陳腐化判定: model.blend の mtime > キャッシュの mtime、または
//     サムネイルの実サイズが thumbnailSize と不一致なら IsStale
//
// thumbnailSize は設定値(0 ならサイズ照合をしない)。
func Scan(libDir string, thumbnailSize int) ([]index.Asset, error) {
	categories, err := library.Categories(libDir)
	if err != nil {
		return nil, err
	}

	var assets []index.Asset
	for _, cat := range categories {
		titles, err := library.AssetTitles(libDir, cat)
		if err != nil {
			return nil, err
		}
		for _, title := range titles {
			assets = append(assets, readAsset(libDir, cat, title, thumbnailSize))
		}
	}
	return assets, nil
}

func readAsset(libDir, category, title string, thumbnailSize int) index.Asset {
	asset := index.Asset{Title: title, Category: category, IsIncomplete: true}

	blendPath := filepath.Join(library.AssetDir(libDir, category, title), "model.blend")
	info, err := os.Stat(blendPath)
	if err == nil && !info.IsDir() {
		asset.IsIncomplete = false
		asset.Path = blendPath
		asset.Size = info.Size()
		asset.UpdatedAt = info.ModTime()
		attachCache(&asset, libDir, thumbnailSize)
	}
	// タグは不完全アセットにも付きうる(meta.json はソースの一部)。
	// 壊れた meta.json はタグ空として扱い、スキャン全体は止めない
	// (ただし黙って消えたように見えないようログには残す)
	if tags, err := library.ReadTags(libDir, category, title); err == nil {
		for _, name := range tags {
			asset.Tags = append(asset.Tags, index.Tag{Name: name})
		}
	} else {
		log.Printf("meta.json を読めません(タグ空として扱います)%s/%s: %v", category, title, err)
	}
	return asset
}

// attachCache は存在するキャッシュをインデックス行へ表示用に取り込み、
// 陳腐化を判定する。キャッシュは読むだけで、無ければ NULL のまま
// (未生成は「—」表示。未生成は陳腐化とは区別する)。
func attachCache(asset *index.Asset, libDir string, thumbnailSize int) {
	paths := library.CachePaths(libDir, asset.Category, asset.Title)

	// 陳腐化判定(requirements.md §7): 既存キャッシュのどれかが
	// model.blend より古ければ要更新(存在確認と同じ stat を使う)
	staleIfOld := func(info os.FileInfo) {
		if info.ModTime().Before(asset.UpdatedAt) {
			asset.IsStale = true
		}
	}
	if info, err := os.Stat(paths.Thumbnail); err == nil && !info.IsDir() {
		asset.ThumbnailPath = &paths.Thumbnail
		staleIfOld(info)
		// サムネイルサイズ変更の検知: PNG の実サイズが設定と違えば要更新
		if thumbnailSize > 0 {
			if w, ok := pngWidth(paths.Thumbnail); ok && w != thumbnailSize {
				asset.IsStale = true
			}
		}
	}
	if info, err := os.Stat(paths.GLB); err == nil && !info.IsDir() {
		asset.GlbPath = &paths.GLB
		staleIfOld(info)
	}
	if info, err := os.Stat(paths.Metadata); err == nil && !info.IsDir() {
		staleIfOld(info)
		if b, err := os.ReadFile(paths.Metadata); err == nil {
			var meta struct {
				PolygonCount int `json:"polygonCount"`
			}
			if json.Unmarshal(b, &meta) == nil {
				asset.PolygonCount = &meta.PolygonCount
			}
		}
	}
}

// pngWidth は PNG ヘッダ(IHDR)から幅を読む。PNG でなければ ok=false。
func pngWidth(path string) (int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	header := make([]byte, 24)
	if _, err := io.ReadFull(f, header); err != nil {
		return 0, false
	}
	signature := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.Equal(header[:8], signature) || string(header[12:16]) != "IHDR" {
		return 0, false
	}
	return int(binary.BigEndian.Uint32(header[16:20])), true
}
