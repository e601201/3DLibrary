package server

import (
	"encoding/json"
	"net/http"

	"github.com/e601201/3DLibrary/internal/config"
	"github.com/e601201/3DLibrary/internal/library"
)

// configResponse は保存された設定に、この要求がリモート閲覧か(CONTEXT.md)を
// 添えたもの。リモート閲覧かは要求ごとの性質で、設定ファイルには持たない。
// フロントエンドはこのフラグで操作要素を出すかどうかを決める。
type configResponse struct {
	config.Config
	RemoteViewing bool `json:"remoteViewing"`
}

// handleConfig は GET /api/config(取得)と PUT /api/config(保存)を処理する。
// PUT で libraryDir が指定されていればライブラリ初期化も行う
// (空ディレクトリのみ骨格を作成。既存ライブラリには書き込まない)。
func handleConfig(store *config.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getConfig(w, r, store)
		case http.MethodPut:
			putConfig(w, r, store)
		default:
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "use GET or PUT")
		}
	}
}

func getConfig(w http.ResponseWriter, r *http.Request, store *config.Store) {
	c, err := store.Load()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "config_load_failed", err.Error())
		return
	}
	if isRemoteViewing(r) {
		c = remoteConfig(c)
	}
	writeJSON(w, http.StatusOK, configResponse{Config: c, RemoteViewing: isRemoteViewing(r)})
}

// remoteConfig はリモート閲覧へ返してよい設定だけを組み立て直す。
//
// libraryDir と blenderPath はローカルの絶対パスで、リモート閲覧には出さない
// (requirements.md §7)。読み手がいないのも確かで、設定画面はリモートでは
// 開けず、libraryDir を使う唯一の表示は空なら消える。
//
// 落とすものを列挙するのではなく返すものを列挙しているので、設定に項目が
// 増えてもここへ書き足さない限りリモートには出ない。thumbnailSize と theme は
// 表示に要る上、値そのものは何も明かさないので検討の上で通している。
func remoteConfig(c config.Config) config.Config {
	return config.Config{
		ThumbnailSize: c.ThumbnailSize,
		Theme:         c.Theme,
	}
}

func putConfig(w http.ResponseWriter, r *http.Request, store *config.Store) {
	var c config.Config
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_failed", "invalid JSON body: "+err.Error())
		return
	}
	if err := c.Validate(); err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	// macOS のアプリバンドル指定を実行ファイルへ読み替える。誤ったパスは
	// 生成時ではなく保存時に弾く(生成時の "permission denied" は原因が伝わらない)。
	blenderPath, err := config.NormalizeBlenderPath(c.BlenderPath)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "blender_path_invalid", err.Error())
		return
	}
	c.BlenderPath = blenderPath
	if c.LibraryDir != "" {
		if err := library.Ensure(c.LibraryDir); err != nil {
			writeError(w, r, http.StatusBadRequest, "library_init_failed", err.Error())
			return
		}
	}
	if err := store.Save(c); err != nil {
		writeError(w, r, http.StatusInternalServerError, "config_save_failed", err.Error())
		return
	}
	// 返すのは保存した設定そのもの。リモート閲覧フラグを持たせないのは、
	// 保存が成り立った時点でこの要求がリモート閲覧でないことが確定するため
	writeJSON(w, http.StatusOK, c)
}
