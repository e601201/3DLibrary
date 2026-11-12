# 開発ガイド

起動後の設定や OS ごとの注意は [setup.md](./setup.md) を参照。

## ビルド(配布用の単一バイナリ)

フロントエンドをビルドして `internal/web/static/` に出力し、それを `go:embed` で埋め込んだ単一バイナリを作る。

```sh
scripts/build.sh     # → bin/3dlibrary
```

スクリプトは以下と同じことをする(手動でやる場合はこちら):

```sh
cd frontend
npm install
npm run build        # → ../internal/web/static/ に出力
cd ..
go build -o bin/3dlibrary ./cmd/3dlibrary
```

> `npm run build` を飛ばして `go build` だけ実行しても**ビルドは通る**が、UI が入っていないバイナリができる(`internal/web/static/.gitkeep` があるため `go:embed` が空にならず、エラーにならない)。`scripts/build.sh` は必ず両方を順に実行する。

### バージョン

バージョンの正は git タグ(`v0.1.0` など)。`go build` がタグとコミットからモジュールのバージョンを自動でバイナリに書き込むので、`-ldflags` は使わない。どのビルド方法でも同じように入り、`./bin/3dlibrary --version` で確かめられる。

- タグの付いたコミット: `v0.1.0`
- タグの無いコミット: `v0.1.1-0.20261112110442-a222302eb186` のような疑似バージョン
- 作業ツリーに変更がある: 末尾に `+dirty`
- `go run`: `dev`

リリースするときは、main のコミットにタグを付けて push する。README のバージョンは最新のタグを表示するバッジなので、手で直すところは無い。

> Go の決まりで、`v2.0.0` 以上のタグは `go.mod` のモジュールパスが `/v2` で終わっていないと無視され、疑似バージョンになる。v2 に上げるときはモジュールパスも合わせて変える。

## Windows 向けビルド(WSL でビルド → Windows で実行)

Blender を Windows 側で使う環境では、WSL 上の Linux バイナリではなく **Windows ネイティブの `.exe`** として動かすと、WSL↔Windows のパス変換が不要になる。SQLite ドライバは pure-Go(CGO 不要)のため、WSL からそのままクロスコンパイルできる。

```sh
scripts/build.sh windows                  # 既定の出力先は下記
scripts/build.sh windows path/to/out.exe  # 出力先を指定
```

既定の出力先は WSL かどうかで変わる。

- **WSL**: `%USERPROFILE%\3DLibrary\3dlibrary.exe`(Windows 側なので、エクスプローラーからそのまま実行できる)。ユーザー名は `%USERPROFILE%` から取るので環境に依存しない
- **それ以外(macOS 等)**: `bin/3dlibrary.exe`

スクリプトは「フロントの `npm run build` → `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build`」を実行する。クロスコンパイルは CGO 不要なので、WSL でも macOS でも同じように動く。Windows 側で設定するパスは [setup.md の「Windows」](./setup.md#windows) を参照。

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

## UI

UI は [design/Design.pen](../design/Design.pen)(Pencil)を正とする。配色・書体・余白は `frontend/src/index.css` のトークンに、共通パーツは `frontend/src/ui.tsx` に写してあるので、画面を足すときはまずそこを見る。
