// Package index は検索・高速表示のための SQLite インデックスを提供する。
// DB は純粋なインデックスであり、ソースから全内容を再構築できる。
// ユーザーデータは一切持たない(requirements.md §6、ADR-0001)。
package index

import (
	"encoding/json"
	"time"
)

// Tag は meta.json のタグをスキャン時に写し取った射影(requirements.md §6)。
// JSON ではタグ名の文字列としてやり取りする。
type Tag struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"uniqueIndex"`
}

func (t Tag) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Name)
}

func (t *Tag) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &t.Name)
}

// Asset は assets テーブルの 1 行。JSON フィールド名は API 規約に従い camelCase。
// ID はスキャンごとに変わりうるため、永続参照には使わない。
type Asset struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	// Path は model.blend の絶対パス。不完全アセットでは空。
	Path          string  `json:"path"`
	ThumbnailPath *string `json:"thumbnailPath"`
	GlbPath       *string `json:"glbPath"`
	// SpritePath は全周スプライト(ADR-0003)。未生成なら NULL。
	SpritePath   *string `json:"spritePath"`
	PolygonCount *int    `json:"polygonCount"`
	// Size は model.blend のバイト数。不完全アセットでは 0。
	Size         int64 `json:"size"`
	IsIncomplete bool  `json:"isIncomplete"`
	IsStale      bool  `json:"isStale"`
	// UpdatedAt は model.blend の更新日時(GORM の自動更新は使わない)。
	UpdatedAt time.Time `gorm:"autoUpdateTime:false" json:"updatedAt"`
	CreatedAt time.Time `json:"createdAt"`
	// Tags は meta.json の射影。書き込みは ReplaceAll(スキャン)のみ。
	Tags []Tag `gorm:"many2many:asset_tags" json:"tags"`
}
