// Package scan は source ディレクトリを歩いてアセット一覧を組み立てる。
// source に対して完全に読み取り専用で、Blender は起動しない
// (requirements.md §7 スキャン)。
package scan

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/e601201/3DLibrary/internal/index"
)

// Scan は sourceDir 直下のカテゴリ/アセットを列挙して返す。
//   - カテゴリ = source 直下のディレクトリ(隠しディレクトリは除く)
//   - アセット = カテゴリ直下のディレクトリ。それより深い階層は関知しない
//   - model.blend を持たないアセットは不完全アセットとして返す
//
// requirements.md §7 のうち未実装の取り込みは後続チケットで行う:
// meta.json のタグ反映(#8)、cache/metadata からのポリゴン数等の取り込み(#6)、
// 陳腐化判定(#9)。いずれもこの関数に足すこと(index.ReplaceAll 参照)。
func Scan(sourceDir string) ([]index.Asset, error) {
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
			assets = append(assets, readAsset(sourceDir, cat.Name(), ent.Name()))
		}
	}
	return assets, nil
}

func readAsset(sourceDir, category, title string) index.Asset {
	asset := index.Asset{Title: title, Category: category, IsIncomplete: true}

	blendPath := filepath.Join(sourceDir, category, title, "model.blend")
	info, err := os.Stat(blendPath)
	if err == nil && !info.IsDir() {
		asset.IsIncomplete = false
		asset.Path = blendPath
		asset.Size = info.Size()
		asset.UpdatedAt = info.ModTime()
	}
	return asset
}
