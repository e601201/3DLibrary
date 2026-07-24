// Package scan は source ディレクトリを歩いてアセット一覧を組み立てる。
// source に対して完全に読み取り専用で、Blender は起動しない
// (requirements.md §7 スキャン)。
package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/e601201/3DLibrary/internal/index"
	"github.com/e601201/3DLibrary/internal/library"
)

// Scan は libDir/source 直下のカテゴリ/アセットを列挙して返す。
//   - カテゴリ = source 直下のディレクトリ(隠しディレクトリは除く)
//   - アセット = カテゴリ直下のディレクトリ。それより深い階層は関知しない
//   - model.blend を持たないアセットは不完全アセットとして返す
//   - キャッシュ(GLB・サムネイル・抽出メタデータのポリゴン数)が
//     あれば表示用として取り込む(requirements.md §7 スキャン)
//
// requirements.md §7 のうち未実装の取り込みは後続チケットで行う:
// meta.json のタグ反映(#8)、陳腐化判定(#9)。
// いずれもこの関数に足すこと(index.ReplaceAll 参照)。
func Scan(libDir string) ([]index.Asset, error) {
	sourceDir := filepath.Join(libDir, "source")
	categories, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, err
	}

	var assets []index.Asset
	for _, cat := range categories {
		if !cat.IsDir() || strings.HasPrefix(cat.Name(), ".") {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(sourceDir, cat.Name()))
		if err != nil {
			return nil, err
		}
		for _, ent := range entries {
			if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
				continue
			}
			assets = append(assets, readAsset(libDir, sourceDir, cat.Name(), ent.Name()))
		}
	}
	return assets, nil
}

func readAsset(libDir, sourceDir, category, title string) index.Asset {
	asset := index.Asset{Title: title, Category: category, IsIncomplete: true}

	blendPath := filepath.Join(sourceDir, category, title, "model.blend")
	info, err := os.Stat(blendPath)
	if err == nil && !info.IsDir() {
		asset.IsIncomplete = false
		asset.Path = blendPath
		asset.Size = info.Size()
		asset.UpdatedAt = info.ModTime()
		attachCache(&asset, libDir)
	}
	return asset
}

// attachCache は存在するキャッシュをインデックス行へ表示用に取り込む。
// キャッシュは読むだけで、無ければ NULL のまま(未生成は「—」表示)。
func attachCache(asset *index.Asset, libDir string) {
	paths := library.CachePaths(libDir, asset.Category, asset.Title)
	if fileExists(paths.Thumbnail) {
		asset.ThumbnailPath = &paths.Thumbnail
	}
	if fileExists(paths.GLB) {
		asset.GlbPath = &paths.GLB
	}
	if b, err := os.ReadFile(paths.Metadata); err == nil {
		var meta struct {
			PolygonCount int `json:"polygonCount"`
		}
		if json.Unmarshal(b, &meta) == nil {
			asset.PolygonCount = &meta.PolygonCount
		}
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
