# インストールと起動

ビルドの手順は [development.md](./development.md) を参照。画面の操作は [manual.md](./manual.md) を参照。

## 起動

```sh
./bin/3dlibrary
```

- `http://127.0.0.1:8765` で待ち受け、デフォルトブラウザが自動で開く
- 併せて `http://127.0.0.1:8766` でも待ち受ける(リモート閲覧の入口。下記)
- `127.0.0.1` のみにバインドし、LAN には露出しない
- `--no-browser` でブラウザの自動起動を抑止できる(開発用)
- 設定は OS 標準の設定ディレクトリ(`os.UserConfigDir()`)に保存される
  - macOS: `~/Library/Application Support/3DLibrary/config.json`
  - Windows: `%AppData%\3DLibrary\config.json`

## macOS

ビルドの手順はそのまま使える(`darwin/arm64` でネイティブビルドされる)。設定画面で指定する 2 つのパスだけ注意する。

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

### 他の Mac へ渡す

ビルド済みバイナリを他の Mac へ渡す場合、未署名のため Gatekeeper に止められる。受け取った側で「システム設定 → プライバシーとセキュリティ → このまま開く」を選ぶか、`xattr -d com.apple.quarantine ./3dlibrary` を実行する(自分で `go build` したものは対象外)。

## Windows

Windows ネイティブの `.exe` の作り方は [development.md の「Windows 向けビルド」](./development.md#windows-向けビルドwsl-でビルド--windows-で実行) を参照。生成した `.exe` を Windows 側で起動すると `http://127.0.0.1:8765` が開く。

- 設定ファイルは Windows の `%AppData%\3DLibrary\config.json`(`os.UserConfigDir()`)に保存される
- `blenderPath` は変換不要で Windows パスを直接指定する(例: `C:\Program Files\Blender Foundation\Blender 5.2\blender.exe`)
- `libraryDir` も Windows パスを指定する(例: `C:\Users\<user>\3DLibrary-data`)

## 別の PC から見る(リモート閲覧)

アプリを動かしている PC はそのままに、手元の別マシン(例: Mac)からライブラリを**読み取り専用**で見られる。仕組みと判断の経緯は [ADR-0004](./adr/0004-remote-viewing-auth-delegated-to-tailscale.md) を参照。

アプリは 2 つのポートで待ち受ける。どちらも `127.0.0.1` にしかバインドしない。

|ポート|用途|
|------|------|
|8765|手元での通常利用。全操作が可能|
|8766|リモート閲覧の入口。**GET / HEAD 以外はすべて 403**|

外へ見せるのは [Tailscale](https://tailscale.com/) に任せる。アプリを動かしている PC で、閲覧専用ポートを tailnet に出す。

```sh
tailscale serve --bg 8766     # https://<マシン名>.<tailnet>.ts.net → 127.0.0.1:8766
tailscale serve status        # 転送先の確認
tailscale serve reset         # 転送をやめる
```

- 同じ tailnet のデバイスからは `https://<マシン名>.<tailnet>.ts.net` で開ける。インターネットからは届かない(`tailscale funnel` は使わない)
- 事前に Tailscale の管理画面で **MagicDNS と HTTPS 証明書**を有効にしておく
- ログインは無い。本人確認は Tailscale のデバイス認証に委任している
- リモート閲覧では操作要素(新規作成・タグ編集・生成・一括生成・再スキャン・キャッシュ削除・Blenderで開く・Finderで表示・設定)が**表示されない**。UI から消えるだけでなく、サーバー側が経路を問わず 403 で止める
- `.blend` を含むアセット内のファイルはダウンロードできる。「開けない」のは Blender の起動操作
- **見せている間はアプリを動かしている PC を起動したままにする**(表示しているのは手元のライブラリそのもので、どこかに複製は置かない)
- 8766 を別のプロセスが使っていると、アプリはそのポートを開けずに起動を続ける(手元での利用は何も変わらない)。リモート閲覧が繋がらないときは、起動ログに `リモート閲覧: 127.0.0.1:8766` が出ているかを先に確認する
