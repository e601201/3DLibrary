package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/e601201/3DLibrary/internal/library"
)

// handleTemplates は GET /api/templates(テンプレート一覧)を処理する。
func handleTemplates(lib *libraryState) http.HandlerFunc {
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
		templates, err := library.ListTemplates(dir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "templates_list_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, templates)
	}
}

// Tags は省略可。作成と同時に meta.json へ書くため、タグ付きの新規作成が
// 1 リクエストで完結する(片方だけ成功する状態を作らない)。
type createAssetRequest struct {
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Template string   `json:"template"`
	Tags     []string `json:"tags"`
}

// createAsset は POST /api/assets(新規アセット作成)を処理する。
// 作成後にスキャンを走らせ、一覧へ即時反映する。
func createAsset(w http.ResponseWriter, r *http.Request, lib *libraryState) {
	var req createAssetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid JSON body: "+err.Error())
		return
	}
	_, dir, err := lib.resolve()
	if err != nil {
		writeLibraryError(w, err, "index_open_failed")
		return
	}
	if err := library.CreateAsset(dir, req.Category, req.Title, req.Template, req.Tags); err != nil {
		switch {
		case errors.Is(err, library.ErrAssetExists):
			writeError(w, http.StatusConflict, "asset_already_exists", err.Error())
		case errors.Is(err, library.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		}
		return
	}
	if _, err := lib.runScan(); err != nil {
		writeError(w, http.StatusInternalServerError, "scan_failed",
			"asset was created but rescan failed: "+err.Error())
		return
	}
	// タグは PUT .../tags と同じく正規化後の姿を返す
	req.Tags = library.NormalizeTags(req.Tags)
	writeJSON(w, http.StatusCreated, req)
}
