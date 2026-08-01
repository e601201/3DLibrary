package library

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PruneResult は掃除した内容(呼び出し元のログ表示用)。
type PruneResult struct {
	Categories []string // ディレクトリごと消したカテゴリ
	Files      int      // 消した孤児キャッシュファイルの数
}

// PruneCache は source から消えたカテゴリ・アセットのキャッシュを削除する
// (requirements.md §7 スキャン)。
//   - 消えたカテゴリ: cache/*/{カテゴリ}/ をディレクトリごと削除
//   - 消えたアセット: cache/*/{カテゴリ}/{タイトル}{拡張子} を削除
//
// source を読めなければ何も削除しない。生きているカテゴリ・アセットを
// 消えたと誤判定して、まだ使えるキャッシュを捨ててしまわないため。
// 個々の削除に失敗しても残りは片付け、起きたエラーはまとめて返す
// (キャッシュは再生成可能で、次のスキャンでも再試行される)。
//
// 生死の判定はスキャンと同じ Categories/AssetTitles に委ねている。
// つまり「スキャンに現れないもののキャッシュ」がちょうど掃除対象になる。
func PruneCache(libDir string) (PruneResult, error) {
	live, err := liveAssets(libDir)
	if err != nil {
		return PruneResult{}, err
	}

	var result PruneResult
	var errs []error
	removedCategories := map[string]bool{}

	for _, kind := range cacheKinds {
		root := filepath.Join(libDir, "cache", kind.dir)
		entries, err := os.ReadDir(root)
		if os.IsNotExist(err) {
			continue // その種別のキャッシュが未作成なら消すものは無い
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, e := range entries {
			// 不可視エントリと cache 直下の迷子ファイルはカテゴリの
			// キャッシュではないので触らない(全削除は ClearCache の担当)
			if !e.IsDir() || IsHidden(e.Name()) {
				continue
			}
			titles, ok := live[e.Name()]
			if !ok {
				if err := os.RemoveAll(filepath.Join(root, e.Name())); err != nil {
					errs = append(errs, err)
					continue
				}
				removedCategories[e.Name()] = true
				continue
			}
			removed, err := pruneOrphanFiles(filepath.Join(root, e.Name()), kind.ext, titles)
			result.Files += removed
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	result.Categories = sortedKeys(removedCategories)
	return result, errors.Join(errs...)
}

// pruneOrphanFiles はカテゴリのキャッシュディレクトリから、source に
// 対応するアセットが無いファイルを削除して削除数を返す。
func pruneOrphanFiles(dir, ext string, live map[string]bool) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	removed := 0
	var errs []error
	for _, e := range entries {
		name := e.Name()
		// その種別の拡張子を持つファイルだけがキャッシュ。
		// ディレクトリ・不可視ファイル・見知らぬ拡張子には触らない
		if e.IsDir() || IsHidden(name) || !strings.HasSuffix(name, ext) {
			continue
		}
		if live[strings.TrimSuffix(name, ext)] {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			errs = append(errs, err)
			continue
		}
		removed++
	}
	return removed, errors.Join(errs...)
}

// liveAssets は source に実在するカテゴリ → タイトル集合を返す。
func liveAssets(libDir string) (map[string]map[string]bool, error) {
	categories, err := Categories(libDir)
	if err != nil {
		return nil, err
	}
	live := make(map[string]map[string]bool, len(categories))
	for _, category := range categories {
		titles, err := AssetTitles(libDir, category)
		if err != nil {
			return nil, err
		}
		set := make(map[string]bool, len(titles))
		for _, title := range titles {
			set[title] = true
		}
		live[category] = set
	}
	return live, nil
}

func sortedKeys(set map[string]bool) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
