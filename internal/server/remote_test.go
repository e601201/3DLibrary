package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func remoteViewingFlag(t *testing.T, rec *httptest.ResponseRecorder) bool {
	t.Helper()
	var body struct {
		RemoteViewing bool `json:"remoteViewing"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v (%s)", err, rec.Body.String())
	}
	return body.RemoteViewing
}

func TestRemoteViewingRejectsEverythingButReads(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)
	remote := srv.RemoteViewingHandler()

	for _, req := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/scan", ""},
		{http.MethodPost, "/api/assets", `{"title":"New","category":"Props","template":"empty"}`},
		{http.MethodPost, "/api/jobs", `{"category":"Props","title":"Chair"}`},
		{http.MethodPost, "/api/jobs/bulk", ""},
		{http.MethodDelete, "/api/cache", ""},
		{http.MethodPut, "/api/config", `{"thumbnailSize":512,"theme":"dark"}`},
		{http.MethodPut, "/api/assets/Props/Chair/tags", `{"tags":["wood"]}`},
		{http.MethodPost, "/api/assets/Props/Chair/open", ""},
		{http.MethodPost, "/api/assets/Props/Chair/reveal", ""},
		// 判定はメソッドだけを見る。経路を列挙しないので、後から足した
		// エンドポイントも(GET でない限り)最初から閉じている
		{http.MethodPost, "/api/not-yet-invented", ""},
		{http.MethodPost, "/", ""},
	} {
		rec := doRequest(t, remote, req.method, req.path, req.body)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s = %d, want 403: %s", req.method, req.path, rec.Code, rec.Body.String())
			continue
		}
		if code := errorCode(t, rec); code != "remote_viewing_read_only" {
			t.Errorf("%s %s error.code = %q", req.method, req.path, code)
		}
	}
}

func TestRemoteViewingScanDoesNotRun(t *testing.T) {
	// 403 はハンドラの手前で返すので、スキャンは走らない(インデックスは
	// リモート閲覧の前後で変わらない)
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)
	addAsset(t, libDir, "Props", "Table", true)

	if rec := doRequest(t, srv.RemoteViewingHandler(), http.MethodPost, "/api/scan", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if assets := listAssets(t, srv); len(assets) != 1 {
		t.Errorf("assets = %d, want 1(スキャンが走ってしまっている)", len(assets))
	}
}

func TestRemoteViewingAllowsReads(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	writeFileIn(t, libDir, "cache/glb/Props/Chair.glb", "glb-bytes")
	rescan(t, srv)
	remote := srv.RemoteViewingHandler()

	for _, path := range []string{
		"/api/health",
		"/api/config",
		"/api/assets",
		"/api/categories",
		"/api/tags",
		"/api/jobs",
		"/api/cache",
		"/api/templates",
		"/api/assets/Props/Chair/files",
		"/api/assets/Props/Chair/dir/textures",
		"/api/glb/Props/Chair.glb",
	} {
		if rec := doRequest(t, remote, http.MethodGet, path, ""); rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200: %s", path, rec.Code, rec.Body.String())
		}
	}

	// SPA も届く(Mac のブラウザが開くのはこの口)
	if rec := doRequest(t, remote, http.MethodGet, "/", ""); rec.Code != http.StatusOK {
		t.Errorf("GET / = %d, want 200", rec.Code)
	}
}

func TestRemoteViewingAllowsBlendDownload(t *testing.T) {
	// .blend の取得は許す(リモートで「開けない」のは Blender 起動の話)
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)

	rec := doRequest(t, srv.RemoteViewingHandler(), http.MethodGet,
		"/api/assets/Props/Chair/raw/model.blend", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "blend" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "blend")
	}
}

func TestConfigReportsWhichPortTheRequestArrivedOn(t *testing.T) {
	srv, _ := newLibraryServer(t)

	local := doRequest(t, srv, http.MethodGet, "/api/config", "")
	if local.Code != http.StatusOK {
		t.Fatalf("local status = %d", local.Code)
	}
	if remoteViewingFlag(t, local) {
		t.Error("ローカルの口は remoteViewing=false であるべき")
	}

	remote := doRequest(t, srv.RemoteViewingHandler(), http.MethodGet, "/api/config", "")
	if remote.Code != http.StatusOK {
		t.Fatalf("remote status = %d", remote.Code)
	}
	if !remoteViewingFlag(t, remote) {
		t.Error("閲覧専用の口は remoteViewing=true であるべき")
	}
	// 設定そのものは同じものが返る
	if got := decodeConfig(t, remote); got != decodeConfig(t, local) {
		t.Errorf("config = %+v, want %+v", got, decodeConfig(t, local))
	}
}
