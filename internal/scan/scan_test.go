package scan

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// buildSource は source ディレクトリのテスト用ツリーを作る。
func buildSource(t *testing.T) string {
	t.Helper()
	src := t.TempDir()

	write := func(rel string, data string) {
		t.Helper()
		path := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("Props/Wooden Chair/model.blend", "blend-data-chair")
	write("Props/Wooden Chair/textures/wood.png", "png")
	write("Props/NoBlend/notes.md", "no blend here") // 不完全アセット
	write("Characters/Hero/model.blend", "blend-data-hero")
	// カテゴリ直下より深い階層はアセットの私有物(スキャン対象外)
	write("Props/Wooden Chair/subdir/inner/model.blend", "must be ignored")
	// source 直下のファイルはカテゴリではない
	write("README.txt", "not a category")
	// 隠しディレクトリは無視する
	write(".hidden/Whatever/model.blend", "ignored")

	return src
}

func TestScanFindsAssets(t *testing.T) {
	src := buildSource(t)
	assets, err := Scan(src)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	byKey := map[string]bool{}
	for _, a := range assets {
		byKey[a.Category+"/"+a.Title] = true
	}
	want := []string{"Props/Wooden Chair", "Props/NoBlend", "Characters/Hero"}
	if len(assets) != len(want) {
		t.Fatalf("got %d assets (%v), want %d", len(assets), byKey, len(want))
	}
	for _, k := range want {
		if !byKey[k] {
			t.Errorf("missing asset %s", k)
		}
	}
}

func TestScanCompleteAsset(t *testing.T) {
	src := buildSource(t)
	assets, err := Scan(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range assets {
		if a.Title != "Wooden Chair" {
			continue
		}
		if a.IsIncomplete {
			t.Error("Wooden Chair should be complete")
		}
		wantPath := filepath.Join(src, "Props", "Wooden Chair", "model.blend")
		if a.Path != wantPath {
			t.Errorf("Path = %q, want %q", a.Path, wantPath)
		}
		if a.Size != int64(len("blend-data-chair")) {
			t.Errorf("Size = %d", a.Size)
		}
		info, _ := os.Stat(wantPath)
		if !a.UpdatedAt.Equal(info.ModTime()) {
			t.Errorf("UpdatedAt = %v, want blend mtime %v", a.UpdatedAt, info.ModTime())
		}
		return
	}
	t.Fatal("Wooden Chair not found")
}

func TestScanIncompleteAsset(t *testing.T) {
	src := buildSource(t)
	assets, err := Scan(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range assets {
		if a.Title != "NoBlend" {
			continue
		}
		if !a.IsIncomplete {
			t.Error("NoBlend should be incomplete")
		}
		if a.Path != "" {
			t.Errorf("Path = %q, want empty", a.Path)
		}
		if a.Size != 0 {
			t.Errorf("Size = %d, want 0", a.Size)
		}
		return
	}
	t.Fatal("NoBlend not found")
}

func TestScanDoesNotWriteToSource(t *testing.T) {
	src := buildSource(t)
	before := snapshot(t, src)
	if _, err := Scan(src); err != nil {
		t.Fatal(err)
	}
	after := snapshot(t, src)
	if len(before) != len(after) {
		t.Fatalf("file count changed: %d -> %d", len(before), len(after))
	}
	for path, mtime := range before {
		if !after[path].Equal(mtime) {
			t.Errorf("%s was modified", path)
		}
	}
}

func TestScanMissingSourceDirReturnsError(t *testing.T) {
	if _, err := Scan(filepath.Join(t.TempDir(), "source")); err == nil {
		t.Fatal("missing source dir should error")
	}
}

func snapshot(t *testing.T, root string) map[string]time.Time {
	t.Helper()
	files := map[string]time.Time{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		files[path] = info.ModTime()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
