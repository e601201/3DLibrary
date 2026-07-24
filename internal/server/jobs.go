package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/e601201/3DLibrary/internal/generate"
	"github.com/e601201/3DLibrary/internal/index"
)

type enqueueRequest struct {
	Category string `json:"category"`
	Title    string `json:"title"`
}

// handleJobs は GET /api/jobs(キュー状態、ADR-0002 のポーリング窓口)と
// POST /api/jobs(生成ジョブ投入)を処理する。
func handleJobs(lib *libraryState, queue *generate.Queue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, queue.Status())
		case http.MethodPost:
			enqueueJob(w, r, lib, queue)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET or POST")
		}
	}
}

func enqueueJob(w http.ResponseWriter, r *http.Request, lib *libraryState, queue *generate.Queue) {
	var req enqueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid JSON body: "+err.Error())
		return
	}
	idx, dir, err := lib.resolve()
	if err != nil {
		writeLibraryError(w, err, "index_open_failed")
		return
	}
	asset, err := idx.Find(req.Category, req.Title)
	if err != nil {
		if errors.Is(err, index.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "index_query_failed", err.Error())
		return
	}
	if asset.IsIncomplete {
		// 不完全アセットは生成できない(model.blend が無い)
		writeError(w, http.StatusConflict, "asset_incomplete",
			"asset has no model.blend and cannot be generated")
		return
	}
	// 既に実行中・待機中なら黙って現在状態を返す(再投入はしない)
	queue.Enqueue(generate.Job{
		Category:  asset.Category,
		Title:     asset.Title,
		BlendPath: asset.Path,
		LibDir:    dir,
	})
	writeJSON(w, http.StatusAccepted, queue.Status())
}

// handleThumbnails は GET /api/thumbnails/{category}/{title}.png を
// cache/thumbnails から配信する。
func handleThumbnails(lib *libraryState) http.HandlerFunc {
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
		root := filepath.Join(dir, "cache", "thumbnails")
		rel := strings.TrimPrefix(r.URL.Path, "/api/thumbnails/")
		path := filepath.Join(root, filepath.FromSlash(rel))
		// cache/thumbnails 配下以外へは出さない(パストラバーサル対策)
		if rel == "" || !strings.HasPrefix(path, root+string(filepath.Separator)) {
			writeError(w, http.StatusNotFound, "not_found", "no such thumbnail")
			return
		}
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			writeError(w, http.StatusNotFound, "not_found", "no such thumbnail")
			return
		}
		http.ServeFile(w, r, path)
	}
}
