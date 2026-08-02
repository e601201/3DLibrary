package server

import (
	"encoding/json"
	"errors"
	"net/http"

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

type bulkResponse struct {
	// Enqueued は今回新たにキューへ積んだ件数。
	Enqueued int             `json:"enqueued"`
	Status   generate.Status `json:"status"`
}

// handleBulkJobs は POST /api/jobs/bulk(不足分を一括生成)を処理する。
// キャッシュ未生成または陳腐化したアセットだけを対象にし、最新の
// アセットはスキップする(requirements.md §7 生成)。
func handleBulkJobs(lib *libraryState, queue *generate.Queue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
			return
		}
		idx, dir, err := lib.resolve()
		if err != nil {
			writeLibraryError(w, err, "index_open_failed")
			return
		}
		assets, err := idx.List(index.ListOptions{})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "index_query_failed", err.Error())
			return
		}
		enqueued := 0
		for _, asset := range assets {
			if !needsGeneration(asset) {
				continue
			}
			if queue.Enqueue(generate.Job{
				Category:  asset.Category,
				Title:     asset.Title,
				BlendPath: asset.Path,
				LibDir:    dir,
			}) {
				enqueued++
			}
		}
		writeJSON(w, http.StatusAccepted, bulkResponse{Enqueued: enqueued, Status: queue.Status()})
	}
}

// needsGeneration はキャッシュ未生成または陳腐化かを判定する。
func needsGeneration(asset index.Asset) bool {
	if asset.IsIncomplete {
		return false // model.blend が無いものは生成できない
	}
	missing := asset.ThumbnailPath == nil || asset.GlbPath == nil ||
		asset.SpritePath == nil || asset.PolygonCount == nil
	return missing || asset.IsStale
}
