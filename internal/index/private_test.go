package index

import "testing"

// 非公開(CONTEXT.md)の絞り込み。リモート閲覧のハンドラは PublicOnly /
// publicOnly=true で引き、非公開アセットを存在ごと消す。

func seedPrivate(t *testing.T, idx *Index) {
	t.Helper()
	if err := idx.ReplaceAll([]Asset{
		{Title: "Chair", Category: "Props", Tags: tagged("wood", "furniture")},
		{Title: "Draft", Category: "Props", IsPrivate: true, Tags: tagged("wood", "wip")},
		{Title: "Secret", Category: "Experiments", IsPrivate: true, Tags: tagged("wip")},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceAllKeepsIsPrivate(t *testing.T) {
	idx := openTest(t)
	seedPrivate(t, idx)

	got, err := idx.List(ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	private := map[string]bool{}
	for _, a := range got {
		private[a.Title] = a.IsPrivate
	}
	if len(got) != 3 || private["Chair"] || !private["Draft"] || !private["Secret"] {
		t.Fatalf("private flags = %v", private)
	}
}

func TestListPublicOnlyExcludesPrivate(t *testing.T) {
	idx := openTest(t)
	seedPrivate(t, idx)

	got, err := idx.List(ListOptions{PublicOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "Chair" {
		t.Fatalf("titles = %v, want [Chair]", titles(got))
	}

	// 検索 q・タグ絞り込みでも非公開は出ない
	got, err = idx.List(ListOptions{Query: "Draft", PublicOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("query should not surface private assets: %v", titles(got))
	}
	got, err = idx.List(ListOptions{Tag: "wip", PublicOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("tag filter should not surface private assets: %v", titles(got))
	}
}

func TestTagCountsPublicOnly(t *testing.T) {
	idx := openTest(t)
	seedPrivate(t, idx)

	got, err := idx.TagCounts(true)
	if err != nil {
		t.Fatal(err)
	}
	// wood は公開分だけ数え、非公開にしか付いていない wip は名前ごと消える
	want := []TagCount{{Name: "furniture", Count: 1}, {Name: "wood", Count: 1}}
	if len(got) != len(want) {
		t.Fatalf("got = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCategoriesPublicOnly(t *testing.T) {
	idx := openTest(t)
	seedPrivate(t, idx)

	got, err := idx.Categories(true)
	if err != nil {
		t.Fatal(err)
	}
	// 非公開しか居ないカテゴリ Experiments は名前ごと消える
	want := []CategoryCount{{Name: "Props", Count: 1}}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("categories = %+v, want %+v", got, want)
	}
}
