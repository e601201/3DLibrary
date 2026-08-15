package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		{http.MethodPut, "/api/assets/Props/Chair/private", `{"private":true}`},
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

// TestAssetListHasNoLocalPaths はローカル・リモートのどちらの口でも、
// 一覧 JSON にローカルの絶対パスが 1 つも出ないことを確かめる。
// ブラウザは配信 URL をカテゴリとタイトルから組み立てるので、パスは要らない。
func TestAssetListHasNoLocalPaths(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	writeFileIn(t, libDir, "cache/glb/Props/Chair.glb", "glb-bytes")
	rescan(t, srv)

	for name, h := range map[string]http.Handler{
		"local":  srv,
		"remote": srv.RemoteViewingHandler(),
	} {
		rec := doRequest(t, h, http.MethodGet, "/api/assets", "")
		body := rec.Body.String()
		if strings.Contains(body, libDir) {
			t.Errorf("%s: 一覧にライブラリの絶対パスが載っている: %s", name, body)
		}
		for _, field := range []string{`"path"`, `"thumbnailPath"`, `"glbPath"`, `"spritePath"`} {
			if strings.Contains(body, field) {
				t.Errorf("%s: %s が応答に残っている: %s", name, field, body)
			}
		}
		// 有無は真偽値で伝わる(生成済みかの判定はこれで足りる)
		if !strings.Contains(body, `"hasGlb":true`) || !strings.Contains(body, `"hasSprite":false`) {
			t.Errorf("%s: 有無の真偽値が正しくない: %s", name, body)
		}
	}
}

// TestRemoteViewingHidesFailureReason はリモート閲覧への失敗応答が
// code だけを返し、理由(ローカルの絶対パスが乗りうる)を伏せることを確かめる。
func TestRemoteViewingHidesFailureReason(t *testing.T) {
	srv, _ := newLibraryServer(t)
	remote := srv.RemoteViewingHandler()

	for _, req := range []struct{ method, path, wantCode string }{
		// 読み取りの 404(存在しないアセット)
		{http.MethodGet, "/api/assets/Props/Nope/files", "not_found"},
		// 書き込みを弾く 403。印はメソッド判定より先に付くのでここも伏せる
		{http.MethodPost, "/api/scan", "remote_viewing_read_only"},
	} {
		rec := doRequest(t, remote, req.method, req.path, "")
		if code := errorCode(t, rec); code != req.wantCode {
			t.Errorf("%s %s error.code = %q, want %q", req.method, req.path, code, req.wantCode)
		}
		if msg := errorMessage(t, rec); msg != remoteFailureMessage {
			t.Errorf("%s %s error.message = %q, want 伏せられた文言", req.method, req.path, msg)
		}
	}

	// ローカルの口は従来どおり理由を返す
	rec := doRequest(t, srv, http.MethodGet, "/api/assets/Props/Nope/files", "")
	if msg := errorMessage(t, rec); msg == remoteFailureMessage || msg == "" {
		t.Errorf("local error.message = %q(ローカルでは理由を返すべき)", msg)
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
}

func TestRemoteViewingConfigOmitsLocalPaths(t *testing.T) {
	srv, libDir := newLibraryServer(t)

	// ローカルには設定がそのまま返る
	got := decodeConfig(t, doRequest(t, srv, http.MethodGet, "/api/config", ""))
	if got.LibraryDir != libDir {
		t.Fatalf("local libraryDir = %q, want %q", got.LibraryDir, libDir)
	}

	// リモートには表示に要るものだけ。ローカルの絶対パスは出さない
	got = decodeConfig(t, doRequest(t, srv.RemoteViewingHandler(), http.MethodGet, "/api/config", ""))
	if got.LibraryDir != "" || got.BlenderPath != "" {
		t.Errorf("remote config = %+v(ローカルの絶対パスが漏れている)", got)
	}
	if got.Theme == "" || got.ThumbnailSize == 0 {
		t.Errorf("remote config = %+v(表示に要る項目まで落ちている)", got)
	}
}
