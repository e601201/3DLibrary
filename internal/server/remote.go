package server

import (
	"context"
	"net/http"
)

// リモート閲覧(CONTEXT.md)の入口。ネットワーク越しの代理はバイナリではなく
// この閲覧専用ハンドラへ転送され、届いた要求は読み取り専用として扱う。
//
// 判定材料は「どのハンドラに届いたか」だけで、要求の中身は一切見ない。
// ヘッダやアドレスを信用しないので偽装で覆せず、通す条件を GET/HEAD に
// 限っているので、後から足したエンドポイントも最初から閉じている(ADR-0004)。

type remoteViewingKey struct{}

// RemoteViewingHandler はリモート閲覧用のハンドラを返す。
func (s *Server) RemoteViewingHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusForbidden, "remote_viewing_read_only",
				"remote viewing is read-only")
			return
		}
		ctx := context.WithValue(r.Context(), remoteViewingKey{}, true)
		s.Handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isRemoteViewing はこの要求がリモート閲覧として届いたかを返す。
func isRemoteViewing(r *http.Request) bool {
	remote, _ := r.Context().Value(remoteViewingKey{}).(bool)
	return remote
}
