package server

import (
	"errors"
	"path/filepath"
	"sync"

	"github.com/e601201/3DLibrary/internal/config"
	"github.com/e601201/3DLibrary/internal/index"
	"github.com/e601201/3DLibrary/internal/scan"
)

var errLibraryNotConfigured = errors.New("library directory is not configured")

// libraryState は設定に追従して現在のライブラリのインデックスを保持する。
// libraryDir が変わったら古い DB を閉じ、新しい {libraryDir}/database.db を
// 開き直す。
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
//
// NOTE: 返した index はロック外で使われるため、直後に別リクエストが
// libraryDir を切り替えると理論上は close 済みの index に触りうる。
// ローカル単一ユーザーのアプリで設定変更は稀なため許容している。
func (l *libraryState) resolveFor(cfg config.Config) (*index.Index, error) {
	if cfg.LibraryDir == "" {
		return nil, errLibraryNotConfigured
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.idx != nil && l.dir == cfg.LibraryDir {
		return l.idx, nil
	}
	if l.idx != nil {
		l.idx.Close()
		l.idx = nil
	}
	idx, err := index.Open(filepath.Join(cfg.LibraryDir, index.DBFileName))
	if err != nil {
		return nil, err
	}
	l.dir = cfg.LibraryDir
	l.idx = idx
	return idx, nil
}

// runScan は source をスキャンしてインデックスを置き換え、件数を返す。
// 設定は 1 回だけ読む(libraryDir と thumbnailSize がずれないように)。
func (l *libraryState) runScan() (int, error) {
	cfg, err := l.store.Load()
	if err != nil {
		return 0, err
	}
	idx, err := l.resolveFor(cfg)
	if err != nil {
		return 0, err
	}
	assets, err := scan.Scan(cfg.LibraryDir, cfg.ThumbnailSize)
	if err != nil {
		return 0, err
	}
	if err := idx.ReplaceAll(assets); err != nil {
		return 0, err
	}
	return len(assets), nil
}

func (l *libraryState) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.idx != nil {
		l.idx.Close()
		l.idx = nil
		l.dir = ""
	}
}
