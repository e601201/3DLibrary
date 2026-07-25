package server

import (
	"errors"
	"net/http"

	"github.com/e601201/3DLibrary/internal/index"
)

// handleAssets は GET /api/assets(一覧)と POST /api/assets(新規作成)を
// 処理する。
func handleAssets(lib *libraryState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// この下の一覧処理へ
		case http.MethodPost:
			createAsset(w, r, lib)
			return
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET or POST")
			return
		}
		opts, ok := parseListOptions(w, r)
		if !ok {
			return
		}
		idx, _, err := lib.resolve()
		if err != nil {
			writeLibraryError(w, err, "index_open_failed")
			return
		}
		assets, err := idx.List(opts)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "index_query_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, assets)
	}
}

// parseListOptions は ?q= / ?category= / ?sort= を読み取る。
// 不正な sort は 400 を書き込み ok=false を返す。
func parseListOptions(w http.ResponseWriter, r *http.Request) (index.ListOptions, bool) {
	opts := index.ListOptions{
		Query:    r.URL.Query().Get("q"),
		Category: r.URL.Query().Get("category"),
		Tag:      r.URL.Query().Get("tag"),
	}
	switch sort := r.URL.Query().Get("sort"); sort {
	case "", "title":
		opts.Sort = index.SortTitle
	case string(index.SortUpdatedDesc):
		opts.Sort = index.SortUpdatedDesc
	case string(index.SortUpdatedAsc):
		opts.Sort = index.SortUpdatedAsc
	default:
		writeError(w, http.StatusBadRequest, "validation_failed",
			"sort must be one of: title, updated_desc, updated_asc")
		return opts, false
	}
	return opts, true
}

// handleCategories は GET /api/categories(件数付きカテゴリ一覧)を処理する。
func handleCategories(lib *libraryState) http.HandlerFunc {
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
		categories, err := idx.Categories()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "index_query_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, categories)
	}
}

type scanResponse struct {
	AssetCount int `json:"assetCount"`
}

// handleScan は POST /api/scan(手動再スキャン)を処理する。
func handleScan(lib *libraryState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
			return
		}
		n, err := lib.runScan()
		if err != nil {
			writeLibraryError(w, err, "scan_failed")
			return
		}
		writeJSON(w, http.StatusOK, scanResponse{AssetCount: n})
	}
}

// writeLibraryError はライブラリ未設定を 409、それ以外を 500 で返す。
func writeLibraryError(w http.ResponseWriter, err error, code string) {
	if errors.Is(err, errLibraryNotConfigured) {
		writeError(w, http.StatusConflict, "library_not_configured",
			"set libraryDir in settings first")
		return
	}
	writeError(w, http.StatusInternalServerError, code, err.Error())
}
