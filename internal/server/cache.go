package server

import (
	"net/http"

	"github.com/e601201/3DLibrary/internal/library"
)

type cacheInfo struct {
	SizeBytes int64 `json:"sizeBytes"`
	FileCount int   `json:"fileCount"`
}

// handleCache は GET /api/cache(容量の実測)と DELETE /api/cache
// (キャッシュ全削除。requirements.md §11)を処理する。
func handleCache(lib *libraryState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET or DELETE")
			return
		}
		// resolve はライブラリ未設定の 409 判定を兼ねる
		_, dir, err := lib.resolve()
		if err != nil {
			writeLibraryError(w, err, "index_open_failed")
			return
		}
		if r.Method == http.MethodGet {
			size, count, err := library.CacheSize(dir)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "cache_size_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, cacheInfo{SizeBytes: size, FileCount: count})
			return
		}
		if err := library.ClearCache(dir); err != nil {
			writeError(w, http.StatusInternalServerError, "cache_clear_failed", err.Error())
			return
		}
		// インデックスのキャッシュ参照(サムネイル・GLB・ポリゴン数)を
		// 現実と一致させる
		if _, err := lib.runScan(); err != nil {
			writeError(w, http.StatusInternalServerError, "scan_failed",
				"cache was cleared but rescan failed: "+err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
