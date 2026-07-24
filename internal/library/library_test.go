package library

import (
	"os"
	"path/filepath"
	"testing"
)

var wantSkeletonDirs = []string{
	"source",
	"cache/glb",
	"cache/thumbnails",
	"cache/metadata",
	"templates",
}

func assertSkeleton(t *testing.T, dir string) {
	t.Helper()
	for _, d := range wantSkeletonDirs {
		info, err := os.Stat(filepath.Join(dir, d))
		if err != nil || !info.IsDir() {
			t.Errorf("skeleton dir %s: %v", d, err)
		}
	}
	blend, err := os.ReadFile(filepath.Join(dir, "templates", "empty.blend"))
	if err != nil {
		t.Fatalf("empty.blend: %v", err)
	}
	if len(blend) == 0 {
		t.Fatal("empty.blend is zero bytes")
	}
}

func TestEnsureCreatesSkeletonInEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := Ensure(dir); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	assertSkeleton(t, dir)
}

func TestEnsureFailsWhenDirMissing(t *testing.T) {
	// タイポしたパスに勝手にライブラリを作らない
	dir := filepath.Join(t.TempDir(), "no", "such", "dir")
	if err := Ensure(dir); err == nil {
		t.Fatal("Ensure on a missing path should error")
	}
}

func TestEnsureTreatsDatabaseOnlyDirAsEmpty(t *testing.T) {
	// アプリはインデックス(database.db)をライブラリ直下に作るため、
	// 「database.db しかないディレクトリ」は空とみなして初期化する
	// (起動時スキャンが先に走ると DB だけが先に生まれる)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "database.db"), []byte("db"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(dir); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	assertSkeleton(t, dir)
	if b, err := os.ReadFile(filepath.Join(dir, "database.db")); err != nil || string(b) != "db" {
		t.Error("database.db must be untouched")
	}
}

func TestEnsureLeavesNonEmptyDirUntouched(t *testing.T) {
	// 既存ライブラリ(または無関係なディレクトリ)は一切変更しない
	dir := t.TempDir()
	marker := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(dir); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "source")); !os.IsNotExist(err) {
		t.Error("source/ should not be created in a non-empty dir")
	}
	b, err := os.ReadFile(marker)
	if err != nil || string(b) != "keep" {
		t.Errorf("existing file was touched: %q, %v", b, err)
	}
}

func TestEnsureFailsWhenPathIsFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(file); err == nil {
		t.Fatal("Ensure on a file path should error")
	}
}
