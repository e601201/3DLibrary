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
