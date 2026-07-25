# 3DLibrary

Blender で制作した 3D アセットをローカル PC 上で一元管理する Web アプリケーション。
要件は [requirements.md](./requirements.md)、用語は [CONTEXT.md](./CONTEXT.md)、API 規約は [docs/api-conventions.md](./docs/api-conventions.md) を参照。

UI は [design/Design.pen](./design/Design.pen)(Pencil)を正とする。配色・書体・余白は `frontend/src/index.css` のトークンに、共通パーツは `frontend/src/ui.tsx` に写してあるので、画面を足すときはまずそこを見る。

## 必要環境

- Go 1.26+
- Node.js 24+ / npm
- Blender 5.2 LTS 以上(生成機能に必須)

対応 OS は Windows と macOS(Apple Silicon / Intel)。CGO 不要なので、どちらもそのままネイティブビルドできる。

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

起動:

```sh
./bin/3dlibrary
```

- `http://127.0.0.1:8765` で待ち受け、デフォルトブラウザが自動で開く
- `127.0.0.1` のみにバインドし、LAN には露出しない
- `--no-browser` でブラウザの自動起動を抑止できる(開発用)
- 設定は OS 標準の設定ディレクトリ(`os.UserConfigDir()`)に保存される
  - macOS: `~/Library/Application Support/3DLibrary/config.json`
  - Windows: `%AppData%\3DLibrary\config.json`

## macOS

上の「ビルド」の手順がそのまま使える(`darwin/arm64` でネイティブビルドされる)。設定画面で指定する 2 つのパスだけ注意する。

### ダブルクリックで起動する

`scripts/3DLibrary.command` を Finder でダブルクリックするとターミナルが開いて起動する。Dock に入れたい場合はこのファイルを Dock へドラッグする(エイリアスやシンボリックリンク経由でも動く)。

- 終了はターミナルウィンドウを閉じる
- すでに起動中なら二重起動せず、ブラウザを開き直すだけにする(ポート 8765 は固定なので 2 つ目は必ず失敗するため)
- `bin/3dlibrary` が無ければ、その場でビルドするか尋ねる

### 設定するパス


- `blenderPath` は **アプリバンドルの中の実行ファイル**:
  `/Applications/Blender.app/Contents/MacOS/Blender`
  Finder 上は `Blender.app` が 1 個のアプリに見えるが実体はディレクトリで、そのまま実行すると `permission denied` になる。`/Applications/Blender.app` を入力した場合は保存時に自動で読み替える
- `libraryDir` は POSIX パス(例: `/Users/<user>/3DLibrary-data`)

ビルド済みバイナリを他の Mac へ渡す場合、未署名のため Gatekeeper に止められる。受け取った側で「システム設定 → プライバシーとセキュリティ → このまま開く」を選ぶか、`xattr -d com.apple.quarantine ./3dlibrary` を実行する(自分で `go build` したものは対象外)。

## Windows ネイティブ実行(WSL でビルド → Windows で実行)

Blender を Windows 側で使う環境では、WSL 上の Linux バイナリではなく **Windows ネイティブの `.exe`** として動かすと、WSL↔Windows のパス変換が不要になる。SQLite ドライバは pure-Go(CGO 不要)のため、WSL からそのままクロスコンパイルできる。

```sh
scripts/build.sh windows                  # 既定の出力先は下記
scripts/build.sh windows path/to/out.exe  # 出力先を指定
```

既定の出力先は WSL かどうかで変わる。

- **WSL**: `%USERPROFILE%\3DLibrary\3dlibrary.exe`(Windows 側なので、エクスプローラーからそのまま実行できる)。ユーザー名は `%USERPROFILE%` から取るので環境に依存しない
- **それ以外(macOS 等)**: `bin/3dlibrary.exe`

スクリプトは「フロントの `npm run build` → `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build`」を実行する。生成した `.exe` を Windows 側で起動すると `http://127.0.0.1:8765` が開く。クロスコンパイルは CGO 不要なので、WSL でも macOS でも同じように動く。

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
