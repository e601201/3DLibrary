package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/e601201/3DLibrary/internal/library"
)

type tagsRequest struct {
	Tags []string `json:"tags"`
}

// handleAssetTags は PUT /api/assets/{category}/{title}/tags を処理する。
// meta.json に保存してから再スキャンし、インデックスへ射影する。
func handleAssetTags(lib *libraryState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use PUT")
			return
		}
		asset, dir, ok := findAsset(w, r, lib)
		if !ok {
			return
		}
		var req tagsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid JSON body: "+err.Error())
			return
		}
		if err := library.WriteTags(dir, asset.Category, asset.Title, req.Tags); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "tags_save_failed", err.Error())
			return
		}
		if _, err := lib.runScan(); err != nil {
			writeError(w, http.StatusInternalServerError, "scan_failed",
				"tags were saved but rescan failed: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, tagsRequest{Tags: library.NormalizeTags(req.Tags)})
	}
}

// handleTags は GET /api/tags(件数付きタグ一覧)を処理する。
func handleTags(lib *libraryState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
			return
		}
		idx, _, err := lib.resolve()
		if err != nil {
			writeLibraryError(w, err, "index_open_failed")
			return
		}
		tags, err := idx.TagCounts()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "index_query_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, tags)
	}
}
