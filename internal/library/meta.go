package library

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// assetMeta は meta.json(アセットメタ)の内容。ユーザー編集データの
// 保存先で、ソースの一部・復元対象(requirements.md §5、ADR-0001)。
type assetMeta struct {
	Tags []string `json:"tags"`
}

func metaPath(libDir, category, title string) string {
	return filepath.Join(AssetDir(libDir, category, title), "meta.json")
}

// ReadTags は meta.json のタグを返す。ファイルが無ければタグ空。
func ReadTags(libDir, category, title string) ([]string, error) {
	b, err := os.ReadFile(metaPath(libDir, category, title))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var meta assetMeta
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, fmt.Errorf("parse meta.json: %w", err)
	}
	return meta.Tags, nil
}

// WriteTags はタグを正規化して meta.json に保存する。ファイルが無ければ
// 作成する(初回タグ付け)。アプリが source へ書き込む 2 経路のうちの 1 つ。
func WriteTags(libDir, category, title string, tags []string) error {
	assetDir := AssetDir(libDir, category, title)
	if info, err := os.Stat(assetDir); err != nil || !info.IsDir() {
		// インデックス参照後にディレクトリが消えた場合も 404 に分類できるよう
		// os.ErrNotExist を包む
		return fmt.Errorf("asset directory not found: %s/%s: %w", category, title, os.ErrNotExist)
	}
	meta := assetMeta{Tags: NormalizeTags(tags)}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath(libDir, category, title), append(b, '\n'), 0o644)
}

// NormalizeTags は空白トリム・空要素除去・重複除去(先勝ち)を行う。
func NormalizeTags(tags []string) []string {
	normalized := []string{}
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		normalized = append(normalized, tag)
	}
	return normalized
}
