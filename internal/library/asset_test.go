package library

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// newLibrary は初期化済みライブラリ(empty.blend 入り)を返す。
func newLibrary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestListTemplates(t *testing.T) {
	dir := newLibrary(t)
	got, err := ListTemplates(dir)
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(got) != 1 || got[0] != "empty.blend" {
		t.Fatalf("templates = %v, want [empty.blend]", got)
	}

	// templates に .blend を置くと現れる。.blend 以外は無視
	if err := os.WriteFile(filepath.Join(dir, "templates", "studio.blend"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "templates", "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = ListTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "empty.blend" || got[1] != "studio.blend" {
		t.Fatalf("templates = %v, want [empty.blend studio.blend]", got)
	}
}

func TestCreateAssetBuildsSkeleton(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Wooden Chair", "empty.blend", nil); err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	assetDir := filepath.Join(dir, "source", "Props", "Wooden Chair")
	for _, sub := range []string{"textures", "references", "renders"} {
		if info, err := os.Stat(filepath.Join(assetDir, sub)); err != nil || !info.IsDir() {
			t.Errorf("%s: %v", sub, err)
		}
	}
	if _, err := os.Stat(filepath.Join(assetDir, "notes.md")); err != nil {
		t.Errorf("notes.md: %v", err)
	}

	// model.blend はテンプレートのコピー
	tmpl, err := os.ReadFile(filepath.Join(dir, "templates", "empty.blend"))
	if err != nil {
		t.Fatal(err)
	}
	blend, err := os.ReadFile(filepath.Join(assetDir, "model.blend"))
	if err != nil {
		t.Fatalf("model.blend: %v", err)
	}
	if string(blend) != string(tmpl) {
		t.Error("model.blend should be a copy of the template")
	}

	// meta.json はタグ空
	var meta struct {
		Tags []string `json:"tags"`
	}
	b, err := os.ReadFile(filepath.Join(assetDir, "meta.json"))
	if err != nil {
		t.Fatalf("meta.json: %v", err)
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		t.Fatalf("meta.json parse: %v", err)
	}
	if meta.Tags == nil || len(meta.Tags) != 0 {
		t.Errorf("tags = %v, want empty array", meta.Tags)
	}
}

func TestCreateAssetWritesInitialTags(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", []string{" Wood ", "", "Seating", "Wood"}); err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	// WriteTags と同じ正規化(トリム・空除去・重複除去)を通る
	tags, err := ReadTags(dir, "Props", "Chair")
	if err != nil {
		t.Fatalf("ReadTags: %v", err)
	}
	if len(tags) != 2 || tags[0] != "Wood" || tags[1] != "Seating" {
		t.Fatalf("tags = %v, want [Wood Seating]", tags)
	}
}

func TestCreateAssetMakesNewCategory(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Vehicles", "Car", "empty.blend", nil); err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "source", "Vehicles")); err != nil || !info.IsDir() {
		t.Fatalf("new category dir: %v", err)
	}
}

func TestCreateAssetRefusesExisting(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "source", "Props", "Chair", "notes.md")
	if err := os.WriteFile(marker, []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err == nil {
		t.Fatal("existing asset must not be overwritten")
	}
	b, _ := os.ReadFile(marker)
	if string(b) != "precious" {
		t.Error("existing files must be untouched")
	}
}

func TestCreateAssetRejectsBadNames(t *testing.T) {
	dir := newLibrary(t)
	bad := []struct{ category, title string }{
		{"", "Chair"},
		{"Props", ""},
		{"..", "Chair"},
		{"Props", ".."},
		{"a/b", "Chair"},
		{"Props", "a/b"},
		{"Props", `a\b`},
		{".hidden", "Chair"}, // 隠しディレクトリはスキャンに現れないため拒否
		{"Props", ".hidden"},
	}
	for _, tc := range bad {
		err := CreateAsset(dir, tc.category, tc.title, "empty.blend", nil)
		if err == nil {
			t.Errorf("CreateAsset(%q, %q) should fail", tc.category, tc.title)
			continue
		}
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("CreateAsset(%q, %q): error should be ErrInvalidInput, got %v", tc.category, tc.title, err)
		}
	}
}

func TestCreateAssetRejectsBadTemplate(t *testing.T) {
	dir := newLibrary(t)
	for _, tmpl := range []string{"", "missing.blend", "../templates/empty.blend", "notes.txt"} {
		err := CreateAsset(dir, "Props", "Chair", tmpl, nil)
		if err == nil {
			t.Errorf("template %q should be rejected", tmpl)
			continue
		}
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("template %q: error should be ErrInvalidInput, got %v", tmpl, err)
		}
	}
}
