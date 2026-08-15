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
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "use PUT")
			return
		}
		asset, dir, ok := findAsset(w, r, lib)
		if !ok {
			return
		}
		var req tagsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_failed", "invalid JSON body: "+err.Error())
			return
		}
		if err := library.WriteTags(dir, asset.Category, asset.Title, req.Tags); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeError(w, r, http.StatusNotFound, "not_found", err.Error())
				return
			}
			writeError(w, r, http.StatusInternalServerError, "tags_save_failed", err.Error())
			return
		}
		if _, err := lib.runScan(); err != nil {
			writeError(w, r, http.StatusInternalServerError, "scan_failed",
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
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
			return
		}
		idx, _, err := lib.resolve()
		if err != nil {
			writeLibraryError(w, r, err, "index_open_failed")
			return
		}
		// リモート閲覧には非公開アセットの分を数えない(非公開にしか
		// 付いていないタグは名前ごと消える)
		tags, err := idx.TagCounts(isRemoteViewing(r))
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "index_query_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, tags)
	}
}
