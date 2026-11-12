# 3DLibrary

Blender で制作した 3D アセットをローカル PC 上で一元管理する Web アプリケーション。
Go の HTTP サーバーに React の画面を埋め込んだ単一バイナリで動き、`127.0.0.1` でだけ待ち受ける。

- バージョン: [![version](https://img.shields.io/github/v/tag/e601201/3DLibrary?sort=semver&label=version)](https://github.com/e601201/3DLibrary/tags)(最新の git タグを表示。手元のバイナリは `--version` で確かめる)
- 対応 OS: Windows / macOS(Apple Silicon / Intel)

## 必要環境

- Go 1.27+
- Node.js 24+ / npm
- Blender 5.2 LTS 以上(生成機能に必須)

## 主要ライブラリ

| 領域 | ライブラリ |
|------|-----------|
| バックエンド | GORM 1 / glebarez/sqlite 1(pure-Go の SQLite。CGO 不要) |
| フロントエンド | React 19 / three.js 0.186 / Tailwind CSS 4 |
| ビルド | Vite 8 / TypeScript 7 |

細かいバージョンは [go.mod](./go.mod) と [frontend/package.json](./frontend/package.json) を参照。

## クイックスタート

```sh
scripts/build.sh   # → bin/3dlibrary
./bin/3dlibrary    # http://127.0.0.1:8765 がブラウザで開く
```

OS ごとの起動方法や設定は [docs/setup.md](./docs/setup.md) を参照。

## ドキュメント

使う人向け

- [docs/manual.md](./docs/manual.md) — 操作マニュアル
- [docs/setup.md](./docs/setup.md) — インストールと起動(macOS / Windows / リモート閲覧)

開発する人向け

- [docs/development.md](./docs/development.md) — ビルド・開発モード・テスト・UI の正
- [requirements.md](./requirements.md) — 要件
- [CONTEXT.md](./CONTEXT.md) — 用語
- [docs/api-conventions.md](./docs/api-conventions.md) — API 規約
- [docs/adr/](./docs/adr/) — 設計判断の記録(ADR)
