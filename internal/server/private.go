package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/e601201/3DLibrary/internal/library"
)

type privateRequest struct {
	Private bool `json:"private"`
}

// handleAssetPrivate は PUT /api/assets/{category}/{title}/private を処理する。
// 公開状態を meta.json に保存してから再スキャンし、インデックスへ射影する
// (タグ編集と同じ流れ。CONTEXT.md「非公開」)。
func handleAssetPrivate(lib *libraryState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use PUT")
			return
		}
		asset, dir, ok := findAsset(w, r, lib)
		if !ok {
			return
		}
		var req privateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid JSON body: "+err.Error())
			return
		}
		if err := library.WritePrivate(dir, asset.Category, asset.Title, req.Private); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "private_save_failed", err.Error())
			return
		}
		if _, err := lib.runScan(); err != nil {
			writeError(w, http.StatusInternalServerError, "scan_failed",
				"private flag was saved but rescan failed: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, privateRequest{Private: req.Private})
	}
}
