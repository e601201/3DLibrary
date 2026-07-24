package server

import (
	"errors"
	"net/http"
)

// handleAssets は GET /api/assets(インデックスの一覧)を処理する。
func handleAssets(lib *libraryState) http.HandlerFunc {
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
		assets, err := idx.List()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "index_query_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, assets)
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
