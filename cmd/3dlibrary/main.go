// 3DLibrary — Blender 3D アセットのローカル管理アプリ。
// 127.0.0.1 の固定ポートで待ち受け、デフォルトブラウザを自動で開く。
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/e601201/3DLibrary/internal/config"
	"github.com/e601201/3DLibrary/internal/server"
	"github.com/e601201/3DLibrary/internal/web"
)

const (
	// 認証を持たないローカル専用アプリのため、127.0.0.1 以外にはバインドしない。
	listenAddr = "127.0.0.1:8765"
	// リモート閲覧(CONTEXT.md)の口。ネットワーク越しの代理はここへ転送する。
	// こちらも loopback のみで、届いた要求は読み取り専用として扱う(ADR-0004)。
	remoteViewingAddr = "127.0.0.1:8766"
)

func main() {
	noBrowser := flag.Bool("no-browser", false, "起動時にブラウザを開かない(開発用)")
	flag.Parse()

	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("設定ディレクトリを取得できません: %v", err)
	}
	store := config.NewStore(filepath.Join(configDir, "3DLibrary", "config.json"))

	srv := server.New(web.Static(), store)
	if n, err := srv.StartupScan(); err != nil {
		log.Printf("起動時スキャンに失敗しました: %v", err)
	} else {
		log.Printf("起動時スキャン完了: %d 件のアセット", n)
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", listenAddr, err)
	}
	url := fmt.Sprintf("http://%s", listenAddr)
	log.Printf("3DLibrary: %s で待ち受けています", url)

	serveRemoteViewing(srv)

	if !*noBrowser {
		go func() {
			time.Sleep(100 * time.Millisecond)
			if err := openBrowser(url); err != nil {
				log.Printf("ブラウザを開けませんでした(%v)。手動で %s を開いてください", err, url)
			}
		}()
	}

	if err := http.Serve(ln, srv); err != nil {
		log.Fatal(err)
	}
}

// serveRemoteViewing はリモート閲覧の口を開く。開けなくてもアプリ本体は
// 動かす(手元での利用は何も変わらず、リモート閲覧だけが使えなくなる)。
func serveRemoteViewing(srv *server.Server) {
	ln, err := net.Listen("tcp", remoteViewingAddr)
	if err != nil {
		log.Printf("リモート閲覧の口 %s を開けませんでした(リモート閲覧は使えません): %v",
			remoteViewingAddr, err)
		return
	}
	log.Printf("リモート閲覧: %s(読み取り専用)", remoteViewingAddr)
	go func() {
		if err := http.Serve(ln, srv.RemoteViewingHandler()); err != nil {
			log.Printf("リモート閲覧の待ち受けが止まりました: %v", err)
		}
	}()
}

// openBrowser は OS のデフォルトブラウザで url を開く。
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
