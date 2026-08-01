package library

import (
	"os"
	"path/filepath"
	"testing"
)

// seedAssetCache は 1 アセット分のキャッシュ 3 点を作る。
func seedAssetCache(t *testing.T, dir, category, title string) CacheSet {
	t.Helper()
	paths := CachePaths(dir, category, title)
	for _, p := range []string{paths.GLB, paths.Thumbnail, paths.Metadata} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("cache"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

func assertGone(t *testing.T, paths CacheSet) {
	t.Helper()
	for _, p := range []string{paths.GLB, paths.Thumbnail, paths.Metadata} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("orphan cache should be removed: %s (%v)", p, err)
		}
	}
}

func assertKept(t *testing.T, paths CacheSet) {
	t.Helper()
	for _, p := range []string{paths.GLB, paths.Thumbnail, paths.Metadata} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("live cache was removed: %s (%v)", p, err)
		}
	}
}

func TestPruneCacheRemovesDeletedCategory(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	kept := seedAssetCache(t, dir, "Props", "Chair")
	// Characters はキャッシュだけ残り、source からは消えたカテゴリ
	orphan := seedAssetCache(t, dir, "Characters", "Hero")

	result, err := PruneCache(dir)
	if err != nil {
		t.Fatalf("PruneCache: %v", err)
	}
	if len(result.Categories) != 1 || result.Categories[0] != "Characters" {
		t.Fatalf("Categories = %v, want [Characters]", result.Categories)
	}
	for _, sub := range cacheSubdirs {
		if _, err := os.Stat(filepath.Join(dir, "cache", sub, "Characters")); !os.IsNotExist(err) {
			t.Errorf("cache/%s/Characters should be removed: %v", sub, err)
		}
	}
	assertGone(t, orphan)
	assertKept(t, kept)
}

func TestPruneCacheRemovesDeletedAsset(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	kept := seedAssetCache(t, dir, "Props", "Chair")
	// Table はカテゴリが生きたまま、アセットだけ消えた
	orphan := seedAssetCache(t, dir, "Props", "Table")

	result, err := PruneCache(dir)
	if err != nil {
		t.Fatalf("PruneCache: %v", err)
	}
	if len(result.Categories) != 0 {
		t.Errorf("Categories = %v, want none (Props is alive)", result.Categories)
	}
	if result.Files != 3 {
		t.Errorf("Files = %d, want 3", result.Files)
	}
	assertGone(t, orphan)
	assertKept(t, kept)
	// 生きているカテゴリのディレクトリ自体は残す
	if _, err := os.Stat(filepath.Join(dir, "cache", "glb", "Props")); err != nil {
		t.Errorf("cache/glb/Props should stay: %v", err)
	}
}

func TestPruneCacheKeepsIncompleteAsset(t *testing.T) {
	dir := newLibrary(t)
	// model.blend を消してもディレクトリが残っていれば不完全アセット。
	// アセットとしては生きているのでキャッシュも残す
	if err := os.MkdirAll(filepath.Join(dir, "source", "Props", "Chair"), 0o755); err != nil {
		t.Fatal(err)
	}
	paths := seedAssetCache(t, dir, "Props", "Chair")

	result, err := PruneCache(dir)
	if err != nil {
		t.Fatalf("PruneCache: %v", err)
	}
	if result.Files != 0 {
		t.Errorf("Files = %d, want 0", result.Files)
	}
	assertKept(t, paths)
}

func TestPruneCacheKeepsEmptyCategory(t *testing.T) {
	dir := newLibrary(t)
	// アセットが 1 つも無くてもカテゴリが source にある限り
	// ディレクトリは残す(中の孤児ファイルだけ消える)
	if err := os.MkdirAll(filepath.Join(dir, "source", "Props"), 0o755); err != nil {
		t.Fatal(err)
	}
	orphan := seedAssetCache(t, dir, "Props", "Chair")

	result, err := PruneCache(dir)
	if err != nil {
		t.Fatalf("PruneCache: %v", err)
	}
	if len(result.Categories) != 0 {
		t.Errorf("Categories = %v, want none", result.Categories)
	}
	assertGone(t, orphan)
	if _, err := os.Stat(filepath.Join(dir, "cache", "glb", "Props")); err != nil {
		t.Errorf("cache/glb/Props should stay: %v", err)
	}
}

func TestPruneCacheKeepsStrayFiles(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	seedAssetCache(t, dir, "Props", "Chair")

	// 掃除の対象は「その種別の拡張子を持つファイル」だけ。迷子ファイルと
	// 見知らぬ拡張子は ClearCache の担当なので触らない
	strays := []string{
		filepath.Join(dir, "cache", "glb", "stray.glb"),
		filepath.Join(dir, "cache", "glb", "Props", "Chair.txt"),
		filepath.Join(dir, "cache", "glb", "Props", ".DS_Store"),
	}
	for _, p := range strays {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := PruneCache(dir); err != nil {
		t.Fatalf("PruneCache: %v", err)
	}
	for _, p := range strays {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("stray file should be left alone: %s (%v)", p, err)
		}
	}
}

func TestPruneCacheKeepsCacheWhenSourceUnreadable(t *testing.T) {
	dir := t.TempDir() // source が無いライブラリ
	paths := seedAssetCache(t, dir, "Props", "Chair")

	if _, err := PruneCache(dir); err == nil {
		t.Fatal("missing source dir should error")
	}
	assertKept(t, paths)
}
