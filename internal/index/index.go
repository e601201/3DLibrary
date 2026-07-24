package index

import (
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
	if err := db.AutoMigrate(&Asset{}); err != nil {
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
		if err := tx.Where("1 = 1").Delete(&Asset{}).Error; err != nil {
			return err
		}
		if len(assets) == 0 {
			return nil
		}
		return tx.Create(&assets).Error
	})
}

// List は全アセットをカテゴリ→タイトル順で返す。
func (i *Index) List() ([]Asset, error) {
	assets := []Asset{}
	err := i.db.Order("category, title").Find(&assets).Error
	return assets, err
}
