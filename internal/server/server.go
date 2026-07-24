// Package server は 3DLibrary の HTTP ハンドラを提供する。
// API 規約は docs/api-conventions.md を参照。
package server

import (
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/e601201/3DLibrary/internal/config"
)

// New は API と埋め込みフロントエンド(static)を配信するハンドラを返す。
func New(static fs.FS, store *config.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/config", handleConfig(store))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such API endpoint")
	})
	mux.Handle("/", spaHandler(static))
	return mux
}

type healthResponse struct {
	Status string `json:"status"`
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

// spaHandler は static 内のファイルを配信し、見つからないパスには
// index.html を返す(クライアントサイドルーティングのフォールバック)。
func spaHandler(static fs.FS) http.Handler {
	fileServer := http.FileServerFS(static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path != "/" {
			if f, err := static.Open(path[1:]); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(static, "index.html")
		if err != nil {
			writeHTML(w, []byte("<html><body><p>フロントエンドがビルドされていません。<code>cd frontend && npm install && npm run build</code> を実行してから再ビルドしてください。</p></body></html>"))
			return
		}
		writeHTML(w, index)
	})
}

func writeHTML(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeError は API 規約のエラー形式で応答する。
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
