package index

import (
	"testing"
)

func tagged(names ...string) []Tag {
	tags := make([]Tag, len(names))
	for i, n := range names {
		tags[i] = Tag{Name: n}
	}
	return tags
}

func seedTagged(t *testing.T, idx *Index) {
	t.Helper()
	if err := idx.ReplaceAll([]Asset{
		{Title: "Chair", Category: "Props", Tags: tagged("wood", "furniture")},
		{Title: "Table", Category: "Props", Tags: tagged("wood")},
		{Title: "Hero", Category: "Characters", Tags: tagged("rigged")},
		{Title: "Rock", Category: "Props"},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestListPreloadsTags(t *testing.T) {
	idx := openTest(t)
	seedTagged(t, idx)

	got, err := idx.List(ListOptions{Category: "Props", Query: "Chair"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Tags) != 2 {
		t.Fatalf("got = %+v", got)
	}
}

func TestListFiltersByTag(t *testing.T) {
	idx := openTest(t)
	seedTagged(t, idx)

	got, err := idx.List(ListOptions{Tag: "wood"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("titles = %v, want 2 wood-tagged", titles(got))
	}
	// 完全一致(部分一致ではない)
	got, err = idx.List(ListOptions{Tag: "woo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("Tag=woo should match nothing, got %v", titles(got))
	}
}

func TestQueryMatchesTagsToo(t *testing.T) {
	idx := openTest(t)
	seedTagged(t, idx)

	// "rig" はタイトルには無いがタグ rigged に部分一致する
	got, err := idx.List(ListOptions{Query: "rig"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "Hero" {
		t.Fatalf("titles = %v, want [Hero]", titles(got))
	}
}

func TestTagCounts(t *testing.T) {
	idx := openTest(t)
	seedTagged(t, idx)

	got, err := idx.TagCounts()
	if err != nil {
		t.Fatal(err)
	}
	want := []TagCount{{Name: "furniture", Count: 1}, {Name: "rigged", Count: 1}, {Name: "wood", Count: 2}}
	if len(got) != len(want) {
		t.Fatalf("got = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestReplaceAllRebuildsTags(t *testing.T) {
	idx := openTest(t)
	seedTagged(t, idx)
	// タグを変えて全置換 → 旧タグ・旧関連は消える
	if err := idx.ReplaceAll([]Asset{
		{Title: "Chair", Category: "Props", Tags: tagged("plastic")},
	}); err != nil {
		t.Fatal(err)
	}
	counts, err := idx.TagCounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 1 || counts[0].Name != "plastic" || counts[0].Count != 1 {
		t.Fatalf("counts = %+v", counts)
	}
}

func TestTagsSurviveManyAssets(t *testing.T) {
	// バッチ挿入(50 件区切り)を跨いでも ID の対応が壊れないこと
	idx := openTest(t)
	assets := make([]Asset, 120)
	for i := range assets {
		assets[i] = Asset{
			Title:    titleOf(i),
			Category: "C",
			Tags:     tagged("common", titleOf(i)+"-tag"),
		}
	}
	if err := idx.ReplaceAll(assets); err != nil {
		t.Fatal(err)
	}
	got, err := idx.List(ListOptions{Tag: "common"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 120 {
		t.Fatalf("common-tagged = %d, want 120", len(got))
	}
	got, err = idx.List(ListOptions{Tag: titleOf(77) + "-tag"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != titleOf(77) {
		t.Fatalf("got = %v", titles(got))
	}
}

func titleOf(i int) string {
	return "Asset-" + string(rune('A'+i/26)) + string(rune('A'+i%26))
}
