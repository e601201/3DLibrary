// Package server は 3DLibrary の HTTP ハンドラを提供する。
// API 規約は docs/api-conventions.md を参照。
package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"

	"github.com/e601201/3DLibrary/internal/config"
	"github.com/e601201/3DLibrary/internal/generate"
)

// Server は API と埋め込みフロントエンドを配信する。
type Server struct {
	http.Handler
	lib   *libraryState
	queue *generate.Queue
}

// New は API と埋め込みフロントエンド(static)を配信するサーバーを返す。
func New(static fs.FS, store *config.Store) *Server {
	lib := newLibraryState(store)
	runner := generate.NewRunner(store)
	// ジョブ完了ごとに再スキャンし、キャッシュをインデックスへ取り込む
	queue := generate.NewQueue(runner.Run, func(job generate.Job, jobErr error) {
		if jobErr != nil {
			log.Printf("生成に失敗しました %s/%s: %v", job.Category, job.Title, jobErr)
		}
		if _, err := lib.runScan(); err != nil {
			log.Printf("生成後の再スキャンに失敗しました: %v", err)
		}
	})
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/config", handleConfig(store))
	mux.HandleFunc("/api/assets", handleAssets(lib))
	mux.HandleFunc("/api/scan", handleScan(lib))
	mux.HandleFunc("/api/templates", handleTemplates(lib))
	mux.HandleFunc("/api/categories", handleCategories(lib))
	mux.HandleFunc("/api/jobs", handleJobs(lib, queue))
	mux.HandleFunc("/api/thumbnails/", handleThumbnails(lib))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such API endpoint")
	})
	mux.Handle("/", spaHandler(static))
	return &Server{Handler: mux, lib: lib, queue: queue}
}

// StartupScan は起動時の自動スキャンを実行する(requirements.md §7)。
// ライブラリ未設定の場合は何もしない。
func (s *Server) StartupScan() (int, error) {
	n, err := s.lib.runScan()
	if errors.Is(err, errLibraryNotConfigured) {
		return 0, nil
	}
	return n, err
}

// CloseLibrary はジョブキューを止め、インデックス DB を閉じる
// (終了時・テスト用)。
func (s *Server) CloseLibrary() {
	s.queue.Stop()
	s.lib.close()
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
