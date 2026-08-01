package library

import (
	"os"
	"path/filepath"
	"testing"
)

// seedAssetCache は 1 アセット分のキャッシュ 3 点(各 10 バイト)を作る。
func seedAssetCache(t *testing.T, dir, category, title string) CacheSet {
	t.Helper()
	paths := CachePaths(dir, category, title)
	for _, p := range paths.All() {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("0123456789"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

func assertGone(t *testing.T, paths CacheSet) {
	t.Helper()
	for _, p := range paths.All() {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("orphan cache should be removed: %s (%v)", p, err)
		}
	}
}

func assertKept(t *testing.T, paths CacheSet) {
	t.Helper()
	for _, p := range paths.All() {
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
	if len(result.RemovedCategories) != 1 || result.RemovedCategories[0] != "Characters" {
		t.Fatalf("RemovedCategories = %v, want [Characters]", result.RemovedCategories)
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
	if len(result.RemovedCategories) != 0 {
		t.Errorf("RemovedCategories = %v, want none (Props is alive)", result.RemovedCategories)
	}
	if result.RemovedFileCount != 3 {
		t.Errorf("RemovedFileCount = %d, want 3", result.RemovedFileCount)
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
	if result.RemovedFileCount != 0 {
		t.Errorf("RemovedFileCount = %d, want 0", result.RemovedFileCount)
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
	if len(result.RemovedCategories) != 0 {
		t.Errorf("RemovedCategories = %v, want none", result.RemovedCategories)
	}
	assertGone(t, orphan)
	if _, err := os.Stat(filepath.Join(dir, "cache", "glb", "Props")); err != nil {
		t.Errorf("cache/glb/Props should stay: %v", err)
	}
}

func TestPruneCacheIgnoresCase(t *testing.T) {
	dir := newLibrary(t)
	// Windows・macOS の既定 FS では大小のみ違う名前は同じ実体。
	// 大小だけを変えたリネームの直後でも、生きているキャッシュを消さない
	if err := CreateAsset(dir, "props", "chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	paths := seedAssetCache(t, dir, "Props", "Chair")

	result, err := PruneCache(dir)
	if err != nil {
		t.Fatalf("PruneCache: %v", err)
	}
	if len(result.RemovedCategories) != 0 || result.RemovedFileCount != 0 {
		t.Fatalf("result = %+v, want nothing removed", result)
	}
	assertKept(t, paths)
}

func TestPruneCacheMergesCaseOnlyCategories(t *testing.T) {
	dir := newLibrary(t)
	// 大小のみ違うカテゴリが併存する場合(大小を区別する FS のみ起こりうる)、
	// 照合用のタイトル集合は上書きではなく合流する。大小を区別しない FS では
	// 2 つの MkdirAll が同じカテゴリに落ちるだけで、期待結果は変わらない
	for _, rel := range []string{"Props/Chair", "props/Table"} {
		if err := os.MkdirAll(filepath.Join(dir, "source", rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	chair := seedAssetCache(t, dir, "Props", "Chair")
	table := seedAssetCache(t, dir, "Props", "Table")

	result, err := PruneCache(dir)
	if err != nil {
		t.Fatalf("PruneCache: %v", err)
	}
	if len(result.RemovedCategories) != 0 || result.RemovedFileCount != 0 {
		t.Fatalf("result = %+v, want nothing removed", result)
	}
	assertKept(t, chair)
	assertKept(t, table)
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
