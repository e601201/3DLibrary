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
	RemovedCategories []string // ディレクトリごと削除したカテゴリ
	RemovedFileCount  int      // 個別に削除した孤児キャッシュファイルの数
}

// PruneCache は孤児キャッシュ(source に対応するカテゴリ・アセットが
// もう存在しないキャッシュ)を掃除する(requirements.md §7 スキャン)。
//   - 消えたカテゴリ: cache/{種別}/{カテゴリ}/ をディレクトリごと削除
//   - 消えたアセット: cache/{種別}/{カテゴリ}/{タイトル}{拡張子} を削除
//
// source を読めなければ何も削除しない。生きているカテゴリ・アセットを
// 消えたと誤判定して、まだ使えるキャッシュを削除してしまわないため。
// 個々の削除に失敗しても残りは掃除し、起きたエラーはまとめて返す
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
			continue // その種別のキャッシュが未作成なら掃除するものは無い
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
			titles, ok := live[foldName(e.Name())]
			if !ok {
				if err := os.RemoveAll(filepath.Join(root, e.Name())); err != nil {
					errs = append(errs, err)
					continue
				}
				removedCategories[e.Name()] = true
				continue
			}
			removed, err := kind.pruneOrphanFiles(filepath.Join(root, e.Name()), titles)
			result.RemovedFileCount += removed
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	result.RemovedCategories = sortedNames(removedCategories)
	return result, errors.Join(errs...)
}

// pruneOrphanFiles はカテゴリのキャッシュディレクトリから、source に
// 対応するアセットが無いこの種別のファイルを削除して削除数を返す。
func (k cacheKind) pruneOrphanFiles(dir string, live map[string]bool) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	removed := 0
	var errs []error
	for _, e := range entries {
		name := e.Name()
		// この種別の拡張子を持つファイルだけがキャッシュ。
		// ディレクトリ・不可視ファイル・見知らぬ拡張子には触らない
		if e.IsDir() || IsHidden(name) || !strings.HasSuffix(name, k.ext) {
			continue
		}
		if live[foldName(strings.TrimSuffix(name, k.ext))] {
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
// キーは foldName 済み。大小のみ違うカテゴリが併存する場合(大小を区別する
// FS のみ起こりうる)はタイトル集合を合流させる。
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
		set := live[foldName(category)]
		if set == nil {
			set = make(map[string]bool, len(titles))
			live[foldName(category)] = set
		}
		for _, title := range titles {
			set[foldName(title)] = true
		}
	}
	return live, nil
}

// foldName は生死照合用にカテゴリ・タイトル名を正規化する。
// Windows・macOS の既定のファイルシステムは大文字小文字を区別しないため、
// 大小だけを変えたリネームの直後に生きているキャッシュを消してしまわない
// よう、照合も大小を区別しない。大小を区別する FS では大小のみ違う古い
// キャッシュが残りうるが、消しすぎるより残しすぎの安全側に倒す。
func foldName(name string) string {
	return strings.ToLower(name)
}

// sortedNames は集合の名前を表示用に安定した順(名前順)で返す。
func sortedNames(set map[string]bool) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
