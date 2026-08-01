package scan

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/e601201/3DLibrary/internal/library"
)

// buildSource は source ディレクトリのテスト用ツリーを作る。
func buildSource(t *testing.T) string {
	t.Helper()
	lib := t.TempDir()
	src := filepath.Join(lib, "source")

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

	return lib
}

func TestScanFindsAssets(t *testing.T) {
	lib := buildSource(t)
	assets, err := Scan(lib, 0)
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
	lib := buildSource(t)
	assets, err := Scan(lib, 0)
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
		wantPath := filepath.Join(lib, "source", "Props", "Wooden Chair", "model.blend")
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
	lib := buildSource(t)
	assets, err := Scan(lib, 0)
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
	lib := buildSource(t)
	src := filepath.Join(lib, "source")
	before := snapshot(t, src)
	if _, err := Scan(lib, 0); err != nil {
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

func TestScanPicksUpCache(t *testing.T) {
	lib := buildSource(t)
	// Wooden Chair のキャッシュ 3 点を用意(Broken にはキャッシュなし)
	paths := library.CachePaths(lib, "Props", "Wooden Chair")
	for _, p := range paths.All() {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(paths.GLB, []byte("glb"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Thumbnail, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Metadata, []byte(`{"polygonCount":1234}`), 0o644); err != nil {
		t.Fatal(err)
	}

	assets, err := Scan(lib, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range assets {
		switch a.Title {
		case "Wooden Chair":
			if a.ThumbnailPath == nil || *a.ThumbnailPath != paths.Thumbnail {
				t.Errorf("ThumbnailPath = %v", a.ThumbnailPath)
			}
			if a.GlbPath == nil || *a.GlbPath != paths.GLB {
				t.Errorf("GlbPath = %v", a.GlbPath)
			}
			if a.PolygonCount == nil || *a.PolygonCount != 1234 {
				t.Errorf("PolygonCount = %v", a.PolygonCount)
			}
		case "Hero":
			if a.ThumbnailPath != nil || a.GlbPath != nil || a.PolygonCount != nil {
				t.Errorf("Hero should have no cache: %+v", a)
			}
		}
	}
}

func TestScanProjectsMetaJSONTags(t *testing.T) {
	lib := buildSource(t)
	metaPath := filepath.Join(lib, "source", "Props", "Wooden Chair", "meta.json")
	if err := os.WriteFile(metaPath, []byte(`{"tags":["wood","furniture"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	assets, err := Scan(lib, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range assets {
		switch a.Title {
		case "Wooden Chair":
			if len(a.Tags) != 2 || a.Tags[0].Name != "wood" || a.Tags[1].Name != "furniture" {
				t.Errorf("Tags = %+v", a.Tags)
			}
		case "Hero":
			if len(a.Tags) != 0 {
				t.Errorf("Hero should have no tags: %+v", a.Tags)
			}
		}
	}
}

// writeCache は Wooden Chair のキャッシュ 3 点を mtime 指定で作る。
func writeCache(t *testing.T, lib string, mtime time.Time) library.CacheSet {
	t.Helper()
	paths := library.CachePaths(lib, "Props", "Wooden Chair")
	for _, p := range paths.All() {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("cache"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

func TestScanDetectsStaleCache(t *testing.T) {
	lib := buildSource(t)
	blend := filepath.Join(lib, "source", "Props", "Wooden Chair", "model.blend")
	blendInfo, _ := os.Stat(blend)

	// キャッシュが blend より古い → 要更新
	writeCache(t, lib, blendInfo.ModTime().Add(-time.Hour))
	assets, err := Scan(lib, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range assets {
		if a.Title == "Wooden Chair" && !a.IsStale {
			t.Error("older cache should be stale")
		}
		if a.Title == "Hero" && a.IsStale {
			t.Error("Hero has no cache and must not be stale")
		}
	}

	// キャッシュが blend より新しい → 最新
	writeCache(t, lib, blendInfo.ModTime().Add(time.Hour))
	assets, err = Scan(lib, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range assets {
		if a.Title == "Wooden Chair" && a.IsStale {
			t.Error("newer cache should not be stale")
		}
	}
}

// pngBytes は width×height の最小 PNG ヘッダ(IHDR まで)を作る。
func pngBytes(width, height int) []byte {
	b := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 13, 'I', 'H', 'D', 'R'}
	for _, v := range []int{width, height} {
		b = append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	}
	return b
}

func TestScanDetectsThumbnailSizeChange(t *testing.T) {
	lib := buildSource(t)
	blend := filepath.Join(lib, "source", "Props", "Wooden Chair", "model.blend")
	blendInfo, _ := os.Stat(blend)
	paths := writeCache(t, lib, blendInfo.ModTime().Add(time.Hour)) // mtime は最新

	// 512px で生成済みのサムネイル
	if err := os.WriteFile(paths.Thumbnail, pngBytes(512, 512), 0o644); err != nil {
		t.Fatal(err)
	}
	future := blendInfo.ModTime().Add(time.Hour)
	if err := os.Chtimes(paths.Thumbnail, future, future); err != nil {
		t.Fatal(err)
	}

	check := func(size int, wantStale bool) {
		t.Helper()
		assets, err := Scan(lib, size)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range assets {
			if a.Title == "Wooden Chair" && a.IsStale != wantStale {
				t.Errorf("thumbnailSize=%d: IsStale = %v, want %v", size, a.IsStale, wantStale)
			}
		}
	}
	check(512, false) // 設定と一致 → 最新
	check(256, true)  // 設定変更 → 全サムネイル要更新
	check(0, false)   // サイズ照合なし
}

func TestScanMissingSourceDirReturnsError(t *testing.T) {
	if _, err := Scan(t.TempDir(), 0); err == nil {
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
