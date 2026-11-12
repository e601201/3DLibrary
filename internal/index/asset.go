// Package index は検索・高速表示のためのインメモリ・インデックスを提供する。
// 純粋なインデックスであり、ソースから全内容を再構築できる。
// ユーザーデータは一切持たない(requirements.md §6、ADR-0001)。
package index

import (
	"encoding/json"
	"time"
)

// Tag は meta.json のタグをスキャン時に写し取った射影(requirements.md §6)。
// JSON ではタグ名の文字列としてやり取りする。
type Tag struct {
	Name string
}

func (t Tag) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Name)
}

func (t *Tag) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &t.Name)
}

// Asset はインデックスの 1 行。JSON フィールド名は API 規約に従い camelCase。
// ID はスキャンごとに変わりうるため、永続参照には使わない。
type Asset struct {
	ID       uint   `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	// Path は model.blend の絶対パス。不完全アセットでは空。
	Path          string  `json:"path"`
	ThumbnailPath *string `json:"thumbnailPath"`
	GlbPath       *string `json:"glbPath"`
	// SpritePath は全周スプライト(ADR-0003)。未生成なら nil。
	SpritePath *string `json:"spritePath"`
	// UsdzPath は AR 表示用の USDZ(ADR-0008)。未生成なら nil。
	UsdzPath     *string `json:"usdzPath"`
	PolygonCount *int    `json:"polygonCount"`
	// Size は model.blend のバイト数。不完全アセットでは 0。
	Size         int64 `json:"size"`
	IsIncomplete bool  `json:"isIncomplete"`
	IsStale      bool  `json:"isStale"`
	// IsPrivate は meta.json の公開状態の射影。非公開のアセットは
	// リモート閲覧にとって存在しない(CONTEXT.md「非公開」)。
	IsPrivate bool `json:"isPrivate"`
	// UpdatedAt は model.blend の更新日時。
	UpdatedAt time.Time `json:"updatedAt"`
	// CreatedAt はインデックスに取り込まれた日時(空なら ReplaceAll が補う)。
	CreatedAt time.Time `json:"createdAt"`
	// Tags は meta.json の射影。書き込みは ReplaceAll(スキャン)のみ。
	Tags []Tag `json:"tags"`
}
