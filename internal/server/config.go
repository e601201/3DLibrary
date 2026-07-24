package server

import (
	"encoding/json"
	"net/http"

	"github.com/e601201/3DLibrary/internal/config"
	"github.com/e601201/3DLibrary/internal/library"
)

// handleConfig は GET /api/config(取得)と PUT /api/config(保存)を処理する。
// PUT で libraryDir が指定されていればライブラリ初期化も行う
// (空ディレクトリのみ骨格を作成。既存ライブラリには書き込まない)。
func handleConfig(store *config.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getConfig(w, store)
		case http.MethodPut:
			putConfig(w, r, store)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET or PUT")
		}
	}
}

func getConfig(w http.ResponseWriter, store *config.Store) {
	c, err := store.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "config_load_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func putConfig(w http.ResponseWriter, r *http.Request, store *config.Store) {
	var c config.Config
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid JSON body: "+err.Error())
		return
	}
	if err := c.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if c.LibraryDir != "" {
		if err := library.Ensure(c.LibraryDir); err != nil {
			writeError(w, http.StatusBadRequest, "library_init_failed", err.Error())
			return
		}
	}
	if err := store.Save(c); err != nil {
		writeError(w, http.StatusInternalServerError, "config_save_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}
