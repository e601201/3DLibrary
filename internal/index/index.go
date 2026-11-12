package index

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// Index は 1 つのライブラリのインメモリ・インデックス。
// 中身はスキャン結果の写しで、プロセス終了とともに消える(起動時スキャンで
// 作り直される)。複数のリクエストから並行に使われるため排他を取る。
type Index struct {
	mu     sync.RWMutex
	assets []Asset
}

// New は空のインデックスを返す。
func New() *Index {
	return &Index{}
}

// ReplaceAll はインデックスの全行をスキャン結果で置き換える。
// 消えたアセットの行はこれにより削除される。ID はスキャンごとに変わりうる。
//
// インデックスへの書き込みはスキャン経由のこの全置換のみ。キャッシュ由来の
// 値(thumbnailPath / polygonCount 等)を持たせる場合も、別経路で行を
// 書き換えるのではなくスキャンの取り込みに載せること
// (でなければ次の再スキャンで消える)。
func (i *Index) ReplaceAll(assets []Asset) error {
	now := time.Now()
	next := make([]Asset, len(assets))
	for n, a := range assets {
		a.ID = uint(n + 1)
		if a.CreatedAt.IsZero() {
			a.CreatedAt = now
		}
		a.Tags = dedupTags(a.Tags)
		next[n] = a
	}
	i.mu.Lock()
	i.assets = next
	i.mu.Unlock()
	return nil
}

// dedupTags は同名タグを 1 つにまとめた新しいスライスを返す(順序は保つ)。
func dedupTags(tags []Tag) []Tag {
	out := make([]Tag, 0, len(tags))
	seen := map[string]bool{}
	for _, t := range tags {
		if seen[t.Name] {
			continue
		}
		seen[t.Name] = true
		out = append(out, t)
	}
	return out
}

// Sort は一覧の並び順。
type Sort string

const (
	// SortTitle はカテゴリ→タイトル順(既定)。
	SortTitle Sort = ""
	// SortUpdatedDesc は更新日の新しい順。
	SortUpdatedDesc Sort = "updated_desc"
	// SortUpdatedAsc は更新日の古い順。
	SortUpdatedAsc Sort = "updated_asc"
)

// ListOptions は一覧の絞り込み・並び順(requirements.md §7 検索)。
type ListOptions struct {
	// Query はタイトルまたはタグ名の部分一致。ASCII は大文字小文字を
	// 区別しない(非 ASCII は区別する)。空なら全件。
	Query string
	// Category はカテゴリの完全一致。空なら全カテゴリ。
	Category string
	// Tag はタグの完全一致。空なら全タグ。
	Tag  string
	Sort Sort
	// PublicOnly は非公開アセットを除外する(リモート閲覧。CONTEXT.md「非公開」)。
	PublicOnly bool
}

// List は条件に合うアセットを返す。返り値は呼び出し側が自由に変更してよい。
func (i *Index) List(opts ListOptions) ([]Asset, error) {
	query := asciiLower(opts.Query)
	i.mu.RLock()
	out := []Asset{}
	for _, a := range i.assets {
		if opts.Category != "" && a.Category != opts.Category {
			continue
		}
		if opts.PublicOnly && a.IsPrivate {
			continue
		}
		if opts.Tag != "" && !hasTag(a, opts.Tag) {
			continue
		}
		if query != "" && !matchesQuery(a, query) {
			continue
		}
		out = append(out, cloneAsset(a))
	}
	i.mu.RUnlock()

	byTitle := func(a, b Asset) int {
		if c := strings.Compare(a.Category, b.Category); c != 0 {
			return c
		}
		return strings.Compare(a.Title, b.Title)
	}
	switch opts.Sort {
	case SortUpdatedDesc:
		slices.SortStableFunc(out, func(a, b Asset) int {
			if c := b.UpdatedAt.Compare(a.UpdatedAt); c != 0 {
				return c
			}
			return byTitle(a, b)
		})
	case SortUpdatedAsc:
		slices.SortStableFunc(out, func(a, b Asset) int {
			if c := a.UpdatedAt.Compare(b.UpdatedAt); c != 0 {
				return c
			}
			return byTitle(a, b)
		})
	default:
		slices.SortStableFunc(out, byTitle)
	}
	return out, nil
}

func hasTag(a Asset, name string) bool {
	for _, t := range a.Tags {
		if t.Name == name {
			return true
		}
	}
	return false
}

// matchesQuery はタイトルかタグ名に lowerQuery(asciiLower 済み)を含むか。
func matchesQuery(a Asset, lowerQuery string) bool {
	if strings.Contains(asciiLower(a.Title), lowerQuery) {
		return true
	}
	for _, t := range a.Tags {
		if strings.Contains(asciiLower(t.Name), lowerQuery) {
			return true
		}
	}
	return false
}

// asciiLower は ASCII の英大文字だけを小文字にする(非 ASCII は区別したまま)。
func asciiLower(s string) string {
	b := []byte(s)
	for n, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[n] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// cloneAsset は呼び出し側の変更がインデックスに波及しないよう写しを作る。
func cloneAsset(a Asset) Asset {
	a.Tags = slices.Clone(a.Tags)
	if a.Tags == nil {
		a.Tags = []Tag{}
	}
	return a
}

// ErrNotFound は Find で該当アセットが無い場合のエラー。
var ErrNotFound = errors.New("asset not found")

// Find はカテゴリとタイトルでアセットを 1 件引く。
func (i *Index) Find(category, title string) (Asset, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	for _, a := range i.assets {
		if a.Category == category && a.Title == title {
			return cloneAsset(a), nil
		}
	}
	return Asset{}, fmt.Errorf("%w: %s/%s", ErrNotFound, category, title)
}

// CategoryCount はカテゴリ名とそのアセット件数。
type CategoryCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TagCount はタグ名とそのアセット件数。
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TagCounts はタグ一覧(名前順)を件数付きで返す。publicOnly なら非公開
// アセットを数えず、非公開にしか付いていないタグは名前ごと消える
// (タグ名自体が情報漏れになるため。CONTEXT.md「非公開」)。
func (i *Index) TagCounts(publicOnly bool) ([]TagCount, error) {
	counts := map[string]int{}
	i.mu.RLock()
	for _, a := range i.assets {
		if publicOnly && a.IsPrivate {
			continue
		}
		for _, t := range a.Tags {
			counts[t.Name]++
		}
	}
	i.mu.RUnlock()
	out := []TagCount{}
	for name, n := range counts {
		out = append(out, TagCount{Name: name, Count: n})
	}
	slices.SortFunc(out, func(a, b TagCount) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// Categories はカテゴリ一覧(名前順)を件数付きで返す。publicOnly なら
// 非公開アセットを数えず、非公開しか居ないカテゴリは名前ごと消える。
func (i *Index) Categories(publicOnly bool) ([]CategoryCount, error) {
	counts := map[string]int{}
	i.mu.RLock()
	for _, a := range i.assets {
		if publicOnly && a.IsPrivate {
			continue
		}
		counts[a.Category]++
	}
	i.mu.RUnlock()
	out := []CategoryCount{}
	for name, n := range counts {
		out = append(out, CategoryCount{Name: name, Count: n})
	}
	slices.SortFunc(out, func(a, b CategoryCount) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}
