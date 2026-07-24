# 3DLibrary

Blender で制作した 3D アセットをローカル PC 上で一元管理する Web アプリケーション。
要件は [requirements.md](./requirements.md)、用語は [CONTEXT.md](./CONTEXT.md)、API 規約は [docs/api-conventions.md](./docs/api-conventions.md) を参照。

## 必要環境

- Go 1.26+
- Node.js 24+ / npm

## ビルド(配布用の単一バイナリ)

フロントエンドをビルドして `internal/web/static/` に出力し、それを `go:embed` で埋め込んだ単一バイナリを作る。

```sh
cd frontend
npm install
npm run build        # → ../internal/web/static/ に出力
cd ..
go build -o bin/3dlibrary ./cmd/3dlibrary
```

起動:

```sh
./bin/3dlibrary
```

- `http://127.0.0.1:8765` で待ち受け、デフォルトブラウザが自動で開く
- `127.0.0.1` のみにバインドし、LAN には露出しない
- `--no-browser` でブラウザの自動起動を抑止できる(開発用)
- 設定は OS 標準の設定ディレクトリ(例: Linux は `~/.config/3DLibrary/config.json`)に保存される

## Windows ネイティブ実行(WSL でビルド → Windows で実行)

Blender を Windows 側で使う環境では、WSL 上の Linux バイナリではなく **Windows ネイティブの `.exe`** として動かすと、WSL↔Windows のパス変換が不要になる。SQLite ドライバは pure-Go(CGO 不要)のため、WSL からそのままクロスコンパイルできる。

```sh
scripts/build-windows.sh                  # 既定: C:\Users\<user>\3DLibrary\3dlibrary.exe へ出力
scripts/build-windows.sh path/to/out.exe  # 出力先を指定
```

スクリプトは「フロントの `npm run build` → `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build`」を実行する。生成した `.exe` を Windows 側で起動すると `http://127.0.0.1:8765` が開く。

- 設定ファイルは Windows の `%AppData%\3DLibrary\config.json`(`os.UserConfigDir()`)に保存される
- `blenderPath` は変換不要で Windows パスを直接指定する(例: `C:\Program Files\Blender Foundation\Blender 5.2\blender.exe`)
- `libraryDir` も Windows パスを指定する(例: `C:\Users\<user>\3DLibrary-data`)

## 開発モード(Vite + Go の分離構成)

ターミナル 1: Go バックエンド

```sh
go run ./cmd/3dlibrary --no-browser
```

ターミナル 2: Vite dev server(`/api` は 127.0.0.1:8765 へプロキシされる)

```sh
cd frontend
npm install
npm run dev
```

Vite が表示する URL(通常 `http://localhost:5173`)を開く。フロントの変更は HMR で即時反映される。

## テスト・型チェック

```sh
go test ./...                     # バックエンド
cd frontend && npm run typecheck  # フロントエンド(tsc)
```
