package server

import (
	"errors"
	"log"
	"strings"
	"sync"

	"github.com/e601201/3DLibrary/internal/config"
	"github.com/e601201/3DLibrary/internal/index"
	"github.com/e601201/3DLibrary/internal/library"
	"github.com/e601201/3DLibrary/internal/scan"
)

var errLibraryNotConfigured = errors.New("library directory is not configured")

// libraryState は設定に追従して現在のライブラリのインデックスを保持する。
// libraryDir が変わったら古いインデックスを捨て、新しいライブラリを
// スキャンしてインメモリ・インデックスを作り直す。
type libraryState struct {
	store *config.Store

	mu  sync.Mutex
	dir string
	idx *index.Index
}

func newLibraryState(store *config.Store) *libraryState {
	return &libraryState{store: store}
}

// resolve は現在設定されているライブラリの index とディレクトリを返す。
func (l *libraryState) resolve() (*index.Index, string, error) {
	cfg, err := l.store.Load()
	if err != nil {
		return nil, "", err
	}
	idx, err := l.resolveFor(cfg)
	return idx, cfg.LibraryDir, err
}

// resolveFor は cfg のライブラリの index を返す(設定は読み直さない)。
func (l *libraryState) resolveFor(cfg config.Config) (*index.Index, error) {
	idx, _, _, err := l.indexFor(cfg)
	return idx, err
}

// indexFor は cfg のライブラリの index を返す。ライブラリを初めて開いた
// (切り替えた)ときはその場でスキャンして中身を詰め、件数と scanned=true を
// 返す。インデックスはメモリ上にしか無いため、空のまま返すと次のスキャン
// まで一覧が空に見えてしまう。
//
// NOTE: 返した index はロック外で使われるため、直後に別リクエストが
// libraryDir を切り替えると理論上は捨てた index に触りうる(古い一覧が
// 見えるだけで壊れはしない)。ローカル単一ユーザーのアプリで設定変更は
// 稀なため許容している。
func (l *libraryState) indexFor(cfg config.Config) (idx *index.Index, n int, scanned bool, err error) {
	if cfg.LibraryDir == "" {
		return nil, 0, false, errLibraryNotConfigured
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.idx != nil && l.dir == cfg.LibraryDir {
		return l.idx, 0, false, nil
	}
	idx = index.New()
	n, err = scanInto(idx, cfg)
	if err != nil {
		return nil, 0, false, err
	}
	l.dir = cfg.LibraryDir
	l.idx = idx
	return idx, n, true, nil
}

// scanInto は source をスキャンして idx の中身を置き換え、件数を返す。
func scanInto(idx *index.Index, cfg config.Config) (int, error) {
	assets, err := scan.Scan(cfg.LibraryDir, cfg.ThumbnailSize)
	if err != nil {
		return 0, err
	}
	if err := idx.ReplaceAll(assets); err != nil {
		return 0, err
	}
	return len(assets), nil
}

// runScan は source をスキャンしてインデックスを置き換え、件数を返す。
// 設定は 1 回だけ読む(libraryDir と thumbnailSize がずれないように)。
// 併せて孤児キャッシュを掃除する。
func (l *libraryState) runScan() (int, error) {
	cfg, err := l.store.Load()
	if err != nil {
		return 0, err
	}
	idx, n, scanned, err := l.indexFor(cfg)
	if err != nil {
		return 0, err
	}
	// 初めて開いたライブラリなら indexFor が今スキャンしたばかり
	if !scanned {
		if n, err = scanInto(idx, cfg); err != nil {
			return 0, err
		}
	}
	pruneCache(cfg.LibraryDir)
	return n, nil
}

// pruneCache は孤児キャッシュを掃除する。失敗してもスキャン自体は成功
// 扱いにする(インデックスは既に現実と一致しており、キャッシュは再生成
// 可能な派生データで、次のスキャンでも再試行される)。
func pruneCache(libDir string) {
	result, err := library.PruneCache(libDir)
	if len(result.RemovedCategories) > 0 {
		log.Printf("孤児キャッシュを掃除しました(カテゴリごと削除: %s)", strings.Join(result.RemovedCategories, ", "))
	}
	if result.RemovedFileCount > 0 {
		log.Printf("孤児キャッシュを掃除しました(ファイル %d 件を削除)", result.RemovedFileCount)
	}
	if err != nil {
		log.Printf("孤児キャッシュの掃除に失敗しました: %v", err)
	}
}

func (l *libraryState) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.idx = nil
	l.dir = ""
}
