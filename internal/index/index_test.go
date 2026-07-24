package index

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func openTest(t *testing.T) *Index {
	t.Helper()
	idx, err := Open(filepath.Join(t.TempDir(), "database.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func TestReplaceAllAndList(t *testing.T) {
	idx := openTest(t)
	now := time.Now().Truncate(time.Second)
	in := []Asset{
		{Title: "Chair", Category: "Props", Path: "/s/Props/Chair/model.blend", Size: 10, UpdatedAt: now},
		{Title: "Hero", Category: "Characters", Path: "/s/Characters/Hero/model.blend", Size: 20, UpdatedAt: now},
		{Title: "NoBlend", Category: "Props", IsIncomplete: true},
	}
	if err := idx.ReplaceAll(in); err != nil {
		t.Fatalf("ReplaceAll: %v", err)
	}

	got, err := idx.List(ListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	// カテゴリ→タイトル順
	wantOrder := []string{"Hero", "Chair", "NoBlend"}
	for i, w := range wantOrder {
		if got[i].Title != w {
			t.Errorf("order[%d] = %q, want %q", i, got[i].Title, w)
		}
	}
	for _, a := range got {
		if a.Title == "NoBlend" && !a.IsIncomplete {
			t.Error("NoBlend should stay incomplete")
		}
	}
}

func TestReplaceAllRemovesVanishedAssets(t *testing.T) {
	idx := openTest(t)
	if err := idx.ReplaceAll([]Asset{
		{Title: "A", Category: "C"},
		{Title: "B", Category: "C"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := idx.ReplaceAll([]Asset{{Title: "B", Category: "C"}}); err != nil {
		t.Fatal(err)
	}
	got, err := idx.List(ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "B" {
		t.Fatalf("got %+v, want only B", got)
	}
}

func TestReplaceAllEmptyClearsIndex(t *testing.T) {
	idx := openTest(t)
	if err := idx.ReplaceAll([]Asset{{Title: "A", Category: "C"}}); err != nil {
		t.Fatal(err)
	}
	if err := idx.ReplaceAll(nil); err != nil {
		t.Fatalf("ReplaceAll(nil): %v", err)
	}
	got, err := idx.List(ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d rows, want 0", len(got))
	}
}

func TestListEmptyIsNotNilError(t *testing.T) {
	idx := openTest(t)
	got, err := idx.List(ListOptions{})
	if err != nil {
		t.Fatalf("List on empty index: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d rows", len(got))
	}
}

func seedForSearch(t *testing.T, idx *Index) {
	t.Helper()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := idx.ReplaceAll([]Asset{
		{Title: "Wooden Chair", Category: "Props", UpdatedAt: base.Add(3 * time.Hour)},
		{Title: "Wooden Table", Category: "Props", UpdatedAt: base.Add(1 * time.Hour)},
		{Title: "Hero", Category: "Characters", UpdatedAt: base.Add(2 * time.Hour)},
		{Title: "wood elf", Category: "Characters", UpdatedAt: base.Add(4 * time.Hour)},
	}); err != nil {
		t.Fatal(err)
	}
}

func titles(assets []Asset) []string {
	out := make([]string, len(assets))
	for i, a := range assets {
		out[i] = a.Title
	}
	return out
}

func TestListFiltersByTitleSubstring(t *testing.T) {
	idx := openTest(t)
	seedForSearch(t, idx)

	got, err := idx.List(ListOptions{Query: "wood"})
	if err != nil {
		t.Fatal(err)
	}
	// 大文字小文字は区別しない部分一致
	if len(got) != 3 {
		t.Fatalf("titles = %v, want 3 wood* matches", titles(got))
	}
}

func TestListFiltersByCategory(t *testing.T) {
	idx := openTest(t)
	seedForSearch(t, idx)

	got, err := idx.List(ListOptions{Category: "Props"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("titles = %v, want 2 Props", titles(got))
	}
}

func TestListCombinesFiltersAndSort(t *testing.T) {
	idx := openTest(t)
	seedForSearch(t, idx)

	got, err := idx.List(ListOptions{Query: "wood", Category: "Props", Sort: SortUpdatedDesc})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Wooden Chair", "Wooden Table"}
	if len(got) != 2 || got[0].Title != want[0] || got[1].Title != want[1] {
		t.Fatalf("titles = %v, want %v", titles(got), want)
	}
}

func TestListSortUpdatedAsc(t *testing.T) {
	idx := openTest(t)
	seedForSearch(t, idx)

	got, err := idx.List(ListOptions{Sort: SortUpdatedAsc})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Wooden Table", "Hero", "Wooden Chair", "wood elf"}
	for i, w := range want {
		if got[i].Title != w {
			t.Fatalf("titles = %v, want %v", titles(got), want)
		}
	}
}

func TestListQueryEscapesLikeWildcards(t *testing.T) {
	idx := openTest(t)
	if err := idx.ReplaceAll([]Asset{
		{Title: "100% Cotton", Category: "Props"},
		{Title: "100x Cotton", Category: "Props"},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := idx.List(ListOptions{Query: "0% C"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "100% Cotton" {
		t.Fatalf("titles = %v, want only the literal %% match", titles(got))
	}
}

func TestCategories(t *testing.T) {
	idx := openTest(t)
	seedForSearch(t, idx)

	got, err := idx.Categories()
	if err != nil {
		t.Fatal(err)
	}
	want := []CategoryCount{{Name: "Characters", Count: 2}, {Name: "Props", Count: 2}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("categories = %+v, want %+v", got, want)
	}
}

func TestListAndSearch5000AssetsUnderOneSecond(t *testing.T) {
	idx := openTest(t)
	assets := make([]Asset, 5000)
	for i := range assets {
		assets[i] = Asset{
			Title:     fmt.Sprintf("Asset %04d", i),
			Category:  fmt.Sprintf("Category %d", i%20),
			UpdatedAt: time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC),
		}
	}
	if err := idx.ReplaceAll(assets); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	all, err := idx.List(ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	listDur := time.Since(start)

	start = time.Now()
	hits, err := idx.List(ListOptions{Query: "0042", Sort: SortUpdatedDesc})
	if err != nil {
		t.Fatal(err)
	}
	searchDur := time.Since(start)

	if len(all) != 5000 || len(hits) != 1 {
		t.Fatalf("len(all) = %d, len(hits) = %d", len(all), len(hits))
	}
	if listDur > time.Second || searchDur > time.Second {
		t.Fatalf("too slow: list=%v search=%v (requirement: <1s)", listDur, searchDur)
	}
	t.Logf("5000 assets: list=%v search=%v", listDur, searchDur)
}
