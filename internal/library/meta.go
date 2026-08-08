package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Meta は meta.json(アセットメタ)の内容。ユーザー編集データの
// 保存先で、ソースの一部・復元対象(requirements.md §5、ADR-0001)。
type Meta struct {
	Tags []string `json:"tags"`
	// Private は非公開(CONTEXT.md)。非公開のアセットはリモート閲覧に
	// とって存在しない。既定は公開で、公開のとき旗は書かない
	// (旗のない既存 meta.json がすべて公開のままになる)。
	Private bool `json:"private,omitempty"`
}

func metaPath(libDir, category, title string) string {
	return filepath.Join(AssetDir(libDir, category, title), "meta.json")
}

// errParseMeta は meta.json が JSON として読めない(壊れている)エラー。
var errParseMeta = errors.New("parse meta.json")

// ReadMeta は meta.json を読む。ファイルが無ければゼロ値(タグ空・公開)。
func ReadMeta(libDir, category, title string) (Meta, error) {
	b, err := os.ReadFile(metaPath(libDir, category, title))
	if os.IsNotExist(err) {
		return Meta{}, nil
	}
	if err != nil {
		return Meta{}, err
	}
	var meta Meta
	if err := json.Unmarshal(b, &meta); err != nil {
		return Meta{}, fmt.Errorf("%w: %v", errParseMeta, err)
	}
	return meta, nil
}

// WriteTags はタグを正規化して meta.json に保存する。タグ以外のフィールド
// (公開状態)は保たれる。ファイルが無ければ作成する(初回タグ付け)。
func WriteTags(libDir, category, title string, tags []string) error {
	return updateMeta(libDir, category, title, func(meta *Meta) {
		meta.Tags = NormalizeTags(tags)
	})
}

// WritePrivate は公開状態を meta.json に保存する。タグは保たれる。
func WritePrivate(libDir, category, title string, private bool) error {
	return updateMeta(libDir, category, title, func(meta *Meta) {
		meta.Private = private
	})
}

// updateMeta は meta.json を読み・書き換え・保存する。アプリが source へ
// 書き込む 2 経路のうちの 1 つ(もう 1 つはアセット作成)。壊れた meta.json は
// ゼロ値から書き直す(読めない内容を保全のために残しても復元できない)。
// それ以外の読み取り失敗(権限エラー等)は中断する。ゼロ値のまま書き込むと
// 読めなかっただけの他フィールドまで消してしまう。
func updateMeta(libDir, category, title string, modify func(*Meta)) error {
	assetDir := AssetDir(libDir, category, title)
	if info, err := os.Stat(assetDir); err != nil || !info.IsDir() {
		// インデックス参照後にディレクトリが消えた場合も 404 に分類できるよう
		// os.ErrNotExist を包む
		return fmt.Errorf("asset directory not found: %s/%s: %w", category, title, os.ErrNotExist)
	}
	meta, err := ReadMeta(libDir, category, title)
	if err != nil && !errors.Is(err, errParseMeta) {
		return err
	}
	modify(&meta)
	if meta.Tags == nil {
		meta.Tags = []string{}
	}
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
