package server

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/e601201/3DLibrary/internal/index"
	"github.com/e601201/3DLibrary/internal/library"
)

// fileEntry はアセットディレクトリ直下の 1 エントリ。ディレクトリは
// 再帰的な合計サイズとファイル数を持つ。
type fileEntry struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	IsDir     bool   `json:"isDir"`
	FileCount int    `json:"fileCount"`
}

type filesResponse struct {
	Entries []fileEntry `json:"entries"`
	// GlbSize は GLB キャッシュのサイズ(未生成なら null)。
	GlbSize *int64 `json:"glbSize"`
}

// handleAssetFiles は GET /api/assets/{category}/{title}/files を処理する。
func handleAssetFiles(lib *libraryState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
			return
		}
		asset, dir, ok := findAsset(w, r, lib)
		if !ok {
			return
		}
		assetDir := filepath.Join(dir, "source", asset.Category, asset.Title)
		dirEntries, err := os.ReadDir(assetDir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "files_list_failed", err.Error())
			return
		}
		resp := filesResponse{Entries: []fileEntry{}}
		for _, e := range dirEntries {
			if library.IsHidden(e.Name()) {
				continue
			}
			entry := fileEntry{Name: e.Name(), IsDir: e.IsDir()}
			if e.IsDir() {
				entry.Size, entry.FileCount = dirSize(filepath.Join(assetDir, e.Name()))
			} else if info, err := e.Info(); err == nil {
				entry.Size = info.Size()
			}
			resp.Entries = append(resp.Entries, entry)
		}
		if info, err := os.Stat(library.CachePaths(dir, asset.Category, asset.Title).GLB); err == nil {
			size := info.Size()
			resp.GlbSize = &size
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// handleOpenInBlender は POST /api/assets/{category}/{title}/open を処理する。
// 設定の Blender 実行ファイルで model.blend を開く(起動するだけで待たない)。
func handleOpenInBlender(lib *libraryState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
			return
		}
		asset, _, ok := findAsset(w, r, lib)
		if !ok {
			return
		}
		if asset.IsIncomplete {
			writeError(w, http.StatusConflict, "asset_incomplete",
				"asset has no model.blend")
			return
		}
		cfg, err := lib.store.Load()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "config_load_failed", err.Error())
			return
		}
		if cfg.BlenderPath == "" {
			writeError(w, http.StatusConflict, "blender_not_configured",
				"set blenderPath in settings first")
			return
		}
		cmd := exec.Command(cfg.BlenderPath, asset.Path)
		if err := cmd.Start(); err != nil {
			writeError(w, http.StatusInternalServerError, "blender_launch_failed", err.Error())
			return
		}
		// 終了は待たないが、ゾンビプロセスにしないため回収だけはする
		go cmd.Wait()
		w.WriteHeader(http.StatusNoContent)
	}
}

// findAsset はパスパラメータのアセットをインデックスから引く。
// 見つからない場合はエラーレスポンスを書いて ok=false を返す。
func findAsset(w http.ResponseWriter, r *http.Request, lib *libraryState) (index.Asset, string, bool) {
	idx, dir, err := lib.resolve()
	if err != nil {
		writeLibraryError(w, err, "index_open_failed")
		return index.Asset{}, "", false
	}
	asset, err := idx.Find(r.PathValue("category"), r.PathValue("title"))
	if err != nil {
		if errors.Is(err, index.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "index_query_failed", err.Error())
		}
		return index.Asset{}, "", false
	}
	return asset, dir, true
}

// dirSize は表示用の概算値。読めないエントリは黙って飛ばす
// (権限エラー等で一覧全体を失敗させない)。
func dirSize(root string) (int64, int) {
	var size int64
	var count int
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		// 一覧に出さないファイルは件数・サイズにも数えない
		// (空の textures が「1 ファイル」と表示されてしまう)
		if library.IsHidden(d.Name()) {
			return nil
		}
		if info, err := d.Info(); err == nil {
			size += info.Size()
			count++
		}
		return nil
	})
	return size, count
}

// cacheFileHandler は cache/{subdir} 配下のファイルを配信する
// (サムネイル・GLB・抽出メタデータ共通)。
func cacheFileHandler(lib *libraryState, prefix, subdir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
			return
		}
		_, dir, err := lib.resolve()
		if err != nil {
			writeLibraryError(w, err, "index_open_failed")
			return
		}
		root := filepath.Join(dir, "cache", subdir)
		serveContainedFile(w, r, root, strings.TrimPrefix(r.URL.Path, prefix))
	}
}

// serveContainedFile は root 配下の rel を配信する。root の外に出る
// パスや存在しないファイルは 404(パストラバーサル対策)。
func serveContainedFile(w http.ResponseWriter, r *http.Request, root, rel string) {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if rel == "" || !strings.HasPrefix(path, root+string(filepath.Separator)) {
		writeError(w, http.StatusNotFound, "not_found", "no such file")
		return
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "not_found", "no such file")
		return
	}
	http.ServeFile(w, r, path)
}
