package server

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// browseEntry はサブディレクトリ内の 1 ファイル。
type browseEntry struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// handleAssetDir は GET /api/assets/{category}/{title}/dir/{subdir} を処理する。
// アセット直下のサブディレクトリのファイル一覧(名前順)。ディレクトリが
// 無ければ空一覧(Finder 作成のアセットは textures 等を持たないことがある)。
func handleAssetDir(lib *libraryState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
			return
		}
		asset, dir, ok := findAsset(w, r, lib)
		if !ok {
			return
		}
		subdir := r.PathValue("subdir")
		if subdir == "" || subdir == "." || subdir == ".." || strings.ContainsAny(subdir, `/\`) {
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid directory name")
			return
		}
		entries, err := os.ReadDir(filepath.Join(dir, "source", asset.Category, asset.Title, subdir))
		if os.IsNotExist(err) {
			// ディレクトリが無いのは空状態(Finder 作成のアセット等)
			writeJSON(w, http.StatusOK, []browseEntry{})
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "files_list_failed", err.Error())
			return
		}
		// os.ReadDir は名前順を保証する
		files := []browseEntry{}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			entry := browseEntry{Name: e.Name()}
			if info, err := e.Info(); err == nil {
				entry.Size = info.Size()
			}
			files = append(files, entry)
		}
		writeJSON(w, http.StatusOK, files)
	}
}

// handleAssetRaw は GET /api/assets/{category}/{title}/raw/{path...} を処理する。
// アセットディレクトリ内のファイルを読み取り専用で配信する(画像・notes.md)。
func handleAssetRaw(lib *libraryState) http.HandlerFunc {
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
		serveContainedFile(w, r, assetDir, r.PathValue("path"))
	}
}

// execStart はテストから差し替えるための実行シーム。
var execStart = func(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // ゾンビプロセスにしない
	return nil
}

// revealCommand は OS のファイルマネージャで dir を開くコマンドを返す。
func revealCommand(goos, dir string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{dir} // Finder
	case "windows":
		return "explorer", []string{dir} // エクスプローラー
	default:
		return "xdg-open", []string{dir}
	}
}

// handleReveal は POST /api/assets/{category}/{title}/reveal を処理する。
// OS のファイルマネージャでアセットディレクトリを開く(requirements.md §7)。
func handleReveal(lib *libraryState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
			return
		}
		asset, dir, ok := findAsset(w, r, lib)
		if !ok {
			return
		}
		assetDir := filepath.Join(dir, "source", asset.Category, asset.Title)
		name, args := revealCommand(runtime.GOOS, assetDir)
		if err := execStart(name, args...); err != nil {
			writeError(w, http.StatusInternalServerError, "reveal_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
