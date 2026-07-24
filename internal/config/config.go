// Package config はアプリ設定の読み書きを提供する。
// 設定はライブラリの外、OS 標準の設定ディレクトリに JSON で保存される
// (requirements.md §11)。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// Config はアプリ設定。JSON フィールド名は API 規約に従い camelCase。
type Config struct {
	BlenderPath   string `json:"blenderPath"`
	LibraryDir    string `json:"libraryDir"`
	ThumbnailSize int    `json:"thumbnailSize"`
	Theme         string `json:"theme"`
}

var (
	thumbnailSizes = []int{256, 512, 1024}
	themes         = []string{"dark", "light", "system"}
)

// Default は初回起動時の設定を返す。
func Default() Config {
	return Config{ThumbnailSize: 512, Theme: "system"}
}

// Validate は設定値の妥当性を検査する。
func (c Config) Validate() error {
	if !slices.Contains(thumbnailSizes, c.ThumbnailSize) {
		return fmt.Errorf("thumbnailSize must be one of %v, got %d", thumbnailSizes, c.ThumbnailSize)
	}
	if !slices.Contains(themes, c.Theme) {
		return fmt.Errorf("theme must be one of %v, got %q", themes, c.Theme)
	}
	return nil
}

// Store は設定ファイル 1 つの読み書きを担う。
type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

// Load は設定を読み込む。ファイルが無ければデフォルト値を返す。
// 手編集や旧バージョンでフィールドが欠けていてもデフォルト値で補う。
func (s *Store) Load() (Config, error) {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}
	c := Default()
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", s.path, err)
	}
	return c, nil
}

// Save は設定を保存する。親ディレクトリが無ければ作成する。
func (s *Store) Save(c Config) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o644)
}
