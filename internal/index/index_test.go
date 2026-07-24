package index

import (
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

	got, err := idx.List()
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
	got, err := idx.List()
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
	got, err := idx.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d rows, want 0", len(got))
	}
}

func TestListEmptyIsNotNilError(t *testing.T) {
	idx := openTest(t)
	got, err := idx.List()
	if err != nil {
		t.Fatalf("List on empty index: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d rows", len(got))
	}
}
