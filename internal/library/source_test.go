package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCategories(t *testing.T) {
	dir := newLibrary(t)
	src := filepath.Join(dir, "source")
	for _, name := range []string{"Props", "Characters", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(src, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// source 直下のファイルはカテゴリではない
	if err := os.WriteFile(filepath.Join(src, "README.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Categories(dir)
	if err != nil {
		t.Fatalf("Categories: %v", err)
	}
	assertNames(t, "Categories", got, "Characters", "Props")
}

func TestCategoriesMissingSourceDirReturnsError(t *testing.T) {
	if _, err := Categories(t.TempDir()); err == nil {
		t.Fatal("missing source dir should error")
	}
}

func TestAssetTitles(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	// model.blend が無いディレクトリも不完全アセットとして数える
	if err := os.MkdirAll(filepath.Join(dir, "source", "Props", "NoBlend"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 不可視ディレクトリとファイルはアセットではない
	if err := os.MkdirAll(filepath.Join(dir, "source", "Props", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "source", "Props", "notes.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := AssetTitles(dir, "Props")
	if err != nil {
		t.Fatalf("AssetTitles: %v", err)
	}
	assertNames(t, "AssetTitles", got, "Chair", "NoBlend")
}

func TestAssetTitlesMissingCategoryReturnsError(t *testing.T) {
	if _, err := AssetTitles(newLibrary(t), "Nope"); err == nil {
		t.Fatal("missing category should error")
	}
}

// assertNames は順不同で名前の集合を比較する。
func assertNames(t *testing.T, label string, got []string, want ...string) {
	t.Helper()
	wanted := map[string]bool{}
	for _, name := range want {
		wanted[name] = true
	}
	if len(got) != len(wanted) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
	for _, name := range got {
		if !wanted[name] {
			t.Errorf("%s: unexpected %q (got %v, want %v)", label, name, got, want)
		}
	}
}
