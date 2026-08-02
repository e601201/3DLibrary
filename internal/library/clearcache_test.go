package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheSize(t *testing.T) {
	dir := newLibrary(t)
	seedAssetCache(t, dir, "Props", "Chair")

	size, count, err := CacheSize(dir)
	if err != nil {
		t.Fatalf("CacheSize: %v", err)
	}
	if size != 40 || count != 4 {
		t.Fatalf("size = %d, count = %d, want 40, 4", size, count)
	}
}

func TestCacheSizeEmptyLibrary(t *testing.T) {
	dir := newLibrary(t)
	size, count, err := CacheSize(dir)
	if err != nil {
		t.Fatalf("CacheSize: %v", err)
	}
	if size != 0 || count != 0 {
		t.Fatalf("size = %d, count = %d, want 0, 0", size, count)
	}
}

func TestClearCache(t *testing.T) {
	dir := newLibrary(t)
	seedAssetCache(t, dir, "Props", "Chair")
	// source とインデックスは無傷であること
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "database.db"), []byte("db"), 0o644); err != nil {
		t.Fatal(err)
	}

	// cache 直下の迷子ファイルも「丸ごと削除」の対象
	if err := os.WriteFile(filepath.Join(dir, "cache", "stray.tmp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ClearCache(dir); err != nil {
		t.Fatalf("ClearCache: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cache", "stray.tmp")); !os.IsNotExist(err) {
		t.Error("stray file under cache/ should be removed")
	}

	size, count, err := CacheSize(dir)
	if err != nil {
		t.Fatal(err)
	}
	if size != 0 || count != 0 {
		t.Fatalf("after clear: size = %d, count = %d", size, count)
	}
	// 骨格(空の cache サブディレクトリ)は残る
	for _, sub := range []string{"glb", "thumbnails", "metadata", "sprites"} {
		if info, err := os.Stat(filepath.Join(dir, "cache", sub)); err != nil || !info.IsDir() {
			t.Errorf("cache/%s should be recreated: %v", sub, err)
		}
	}
	// source・database.db・templates は無傷
	if _, err := os.Stat(filepath.Join(dir, "source", "Props", "Chair", "model.blend")); err != nil {
		t.Errorf("source was touched: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "database.db")); err != nil || string(b) != "db" {
		t.Errorf("database.db was touched: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "templates", "empty.blend")); err != nil {
		t.Errorf("templates was touched: %v", err)
	}
}
