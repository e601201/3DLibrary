package index

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DBFileName はライブラリ直下に置くインデックスファイル名。
const DBFileName = "database.db"

// Index は 1 つのライブラリの SQLite インデックス(database.db)。
type Index struct {
	db *gorm.DB
}

// Open は path の SQLite を開き、スキーマを最新化する。
func Open(path string) (*Index, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&Asset{}, &Tag{}); err != nil {
		return nil, err
	}
	return &Index{db: db}, nil
}

func (i *Index) Close() error {
	sqlDB, err := i.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// ReplaceAll はインデックスの全行をスキャン結果で置き換える。
// 消えたアセットの行はこれにより削除される。ID はスキャンごとに変わりうる。
//
// インデックスへの書き込みはスキャン経由のこの全置換のみ。後続チケットで
// キャッシュ由来の値(thumbnailPath / polygonCount 等)を持たせる場合も、
// 別経路で行を UPDATE するのではなくスキャンの取り込みに載せること
// (でなければ次の再スキャンで消える)。
func (i *Index) ReplaceAll(assets []Asset) error {
	return i.db.Transaction(func(tx *gorm.DB) error {
		// GORM は条件なしの全件 DELETE を拒否するため、常に真の条件を付ける
		if err := tx.Exec("DELETE FROM asset_tags").Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&Tag{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&Asset{}).Error; err != nil {
			return err
		}
		if len(assets) == 0 {
			return nil
		}
		// SQLite のバインド変数上限(999)を超えないようバッチ挿入する
		// (5,000 アセット規模の全置換で上限に当たる)。タグは重複を
		// 自前で正規化するため関連ごと挿入せず、後から張り直す
		if err := tx.Omit("Tags").CreateInBatches(assets, 50).Error; err != nil {
			return err
		}
		return insertTags(tx, assets)
	})
}

// insertTags は tags / asset_tags を assets の Tags から再構築する。
func insertTags(tx *gorm.DB, assets []Asset) error {
	tagID := map[string]uint{}
	var tags []Tag
	for _, a := range assets {
		for _, t := range a.Tags {
			if _, ok := tagID[t.Name]; !ok {
				tagID[t.Name] = 0
				tags = append(tags, Tag{Name: t.Name})
			}
		}
	}
	if len(tags) == 0 {
		return nil
	}
	if err := tx.CreateInBatches(tags, 100).Error; err != nil {
		return err
	}
	for _, t := range tags {
		tagID[t.Name] = t.ID
	}
	type assetTag struct {
		AssetID uint
		TagID   uint
	}
	var joins []assetTag
	for _, a := range assets {
		for _, t := range a.Tags {
			joins = append(joins, assetTag{AssetID: a.ID, TagID: tagID[t.Name]})
		}
	}
	return tx.Table("asset_tags").CreateInBatches(joins, 100).Error
}

// Sort は一覧の並び順。
type Sort string

const (
	// SortTitle はカテゴリ→タイトル順(既定)。
	SortTitle Sort = ""
	// SortUpdatedDesc は更新日の新しい順。
	SortUpdatedDesc Sort = "updated_desc"
	// SortUpdatedAsc は更新日の古い順。
	SortUpdatedAsc Sort = "updated_asc"
)

// ListOptions は一覧の絞り込み・並び順(requirements.md §7 検索)。
type ListOptions struct {
	// Query はタイトルまたはタグ名の部分一致。ASCII は大文字小文字を
	// 区別しない(SQLite LIKE の仕様。非 ASCII は区別される)。空なら全件。
	Query string
	// Category はカテゴリの完全一致。空なら全カテゴリ。
	Category string
	// Tag はタグの完全一致。空なら全タグ。
	Tag  string
	Sort Sort
}

// List は条件に合うアセットを返す。
func (i *Index) List(opts ListOptions) ([]Asset, error) {
	q := i.db.Model(&Asset{}).Preload("Tags")
	if opts.Query != "" {
		like := "%" + escapeLike(opts.Query) + "%"
		q = q.Where(
			`title LIKE ? ESCAPE '\' OR EXISTS (
				SELECT 1 FROM asset_tags
				JOIN tags ON tags.id = asset_tags.tag_id
				WHERE asset_tags.asset_id = assets.id AND tags.name LIKE ? ESCAPE '\'
			)`, like, like)
	}
	if opts.Category != "" {
		q = q.Where("category = ?", opts.Category)
	}
	if opts.Tag != "" {
		q = q.Where(`EXISTS (
			SELECT 1 FROM asset_tags
			JOIN tags ON tags.id = asset_tags.tag_id
			WHERE asset_tags.asset_id = assets.id AND tags.name = ?
		)`, opts.Tag)
	}
	switch opts.Sort {
	case SortUpdatedDesc:
		q = q.Order("updated_at DESC, category, title")
	case SortUpdatedAsc:
		q = q.Order("updated_at ASC, category, title")
	default:
		q = q.Order("category, title")
	}
	assets := []Asset{}
	err := q.Find(&assets).Error
	return assets, err
}

// escapeLike は LIKE パターンのメタ文字をリテラル扱いにする。
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// ErrNotFound は Find で該当アセットが無い場合のエラー。
var ErrNotFound = errors.New("asset not found")

// Find はカテゴリとタイトルでアセットを 1 件引く。
func (i *Index) Find(category, title string) (Asset, error) {
	var asset Asset
	err := i.db.Where("category = ? AND title = ?", category, title).First(&asset).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Asset{}, fmt.Errorf("%w: %s/%s", ErrNotFound, category, title)
	}
	return asset, err
}

// CategoryCount はカテゴリ名とそのアセット件数。
type CategoryCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TagCount はタグ名とそのアセット件数。
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TagCounts はタグ一覧(名前順)を件数付きで返す。
func (i *Index) TagCounts() ([]TagCount, error) {
	counts := []TagCount{}
	err := i.db.Table("tags").
		Select("tags.name AS name, COUNT(asset_tags.asset_id) AS count").
		Joins("JOIN asset_tags ON asset_tags.tag_id = tags.id").
		Group("tags.name").
		Order("tags.name").
		Find(&counts).Error
	return counts, err
}

// Categories はカテゴリ一覧(名前順)を件数付きで返す。
func (i *Index) Categories() ([]CategoryCount, error) {
	categories := []CategoryCount{}
	err := i.db.Model(&Asset{}).
		Select("category AS name, COUNT(*) AS count").
		Group("category").
		Order("category").
		Find(&categories).Error
	return categories, err
}
