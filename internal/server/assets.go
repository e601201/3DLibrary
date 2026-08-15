package server

import (
	"errors"
	"net/http"
	"time"

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
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "use GET or POST")
			return
		}
		opts, ok := parseListOptions(w, r)
		if !ok {
			return
		}
		// リモート閲覧にとって非公開アセットは存在しない(CONTEXT.md「非公開」)
		opts.PublicOnly = isRemoteViewing(r)
		idx, _, err := lib.resolve()
		if err != nil {
			writeLibraryError(w, r, err, "index_open_failed")
			return
		}
		assets, err := idx.List(opts)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "index_query_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, newAssetResponses(assets))
	}
}

// assetResponse は index.Asset のうちブラウザへ返してよいものだけの表現。
//
// インデックスは生成と配信のために絶対パスを持つが、ブラウザはそれを一切
// 必要としない。配信 URL はカテゴリとタイトルから組み立てられ、キャッシュの
// パスは「生成済みか」の判定にしか使われていなかった。渡さずに済むものを
// 渡さないのが一番確実なので、有無の真偽値だけを返す(requirements.md §6)。
//
// index.Asset を埋め込まずに項目を並べているのは、返すものを列挙するため。
// インデックスに列が増えても、ここへ書き足さない限り外には出ない。
type assetResponse struct {
	ID           uint        `json:"id"`
	Title        string      `json:"title"`
	Category     string      `json:"category"`
	PolygonCount *int        `json:"polygonCount"`
	Size         int64       `json:"size"`
	IsIncomplete bool        `json:"isIncomplete"`
	IsStale      bool        `json:"isStale"`
	IsPrivate    bool        `json:"isPrivate"`
	UpdatedAt    time.Time   `json:"updatedAt"`
	CreatedAt    time.Time   `json:"createdAt"`
	Tags         []index.Tag `json:"tags"`

	HasThumbnail bool `json:"hasThumbnail"`
	HasGlb       bool `json:"hasGlb"`
	HasSprite    bool `json:"hasSprite"`
}

func newAssetResponse(a index.Asset) assetResponse {
	return assetResponse{
		ID:           a.ID,
		Title:        a.Title,
		Category:     a.Category,
		PolygonCount: a.PolygonCount,
		Size:         a.Size,
		IsIncomplete: a.IsIncomplete,
		IsStale:      a.IsStale,
		IsPrivate:    a.IsPrivate,
		UpdatedAt:    a.UpdatedAt,
		CreatedAt:    a.CreatedAt,
		Tags:         a.Tags,
		HasThumbnail: a.ThumbnailPath != nil,
		HasGlb:       a.GlbPath != nil,
		HasSprite:    a.SpritePath != nil,
	}
}

func newAssetResponses(assets []index.Asset) []assetResponse {
	out := make([]assetResponse, 0, len(assets))
	for _, a := range assets {
		out = append(out, newAssetResponse(a))
	}
	return out
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
		writeError(w, r, http.StatusBadRequest, "validation_failed",
			"sort must be one of: title, updated_desc, updated_asc")
		return opts, false
	}
	return opts, true
}

// handleCategories は GET /api/categories(件数付きカテゴリ一覧)を処理する。
func handleCategories(lib *libraryState) http.HandlerFunc {
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
		// リモート閲覧には非公開アセットの分を数えない(非公開しか
		// 居ないカテゴリは名前ごと消える)
		categories, err := idx.Categories(isRemoteViewing(r))
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "index_query_failed", err.Error())
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
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
			return
		}
		n, err := lib.runScan()
		if err != nil {
			writeLibraryError(w, r, err, "scan_failed")
			return
		}
		writeJSON(w, http.StatusOK, scanResponse{AssetCount: n})
	}
}

// writeLibraryError はライブラリ未設定を 409、それ以外を 500 で返す。
func writeLibraryError(w http.ResponseWriter, r *http.Request, err error, code string) {
	if errors.Is(err, errLibraryNotConfigured) {
		writeError(w, r, http.StatusConflict, "library_not_configured",
			"set libraryDir in settings first")
		return
	}
	writeError(w, r, http.StatusInternalServerError, code, err.Error())
}
