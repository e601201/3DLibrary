# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 3DLibrary

A web application for managing Blender-authored 3D assets on a local machine. It ships as a **single binary** — a Go HTTP server with the React SPA embedded via `go:embed` — and binds to loopback only (there is no authentication): `127.0.0.1:8765` for normal use, plus `127.0.0.1:8766` for remote viewing, which is read-only (ADR-0004).

## Architecture

### Data flow

```
source/ (source of truth) --[scan: read-only]--> index (in-memory) --> /api/assets --> React SPA
        |                                        ^
        +--[generate: one Blender CLI run]--> cache/ (5 artifacts) --(picked up by next scan)--+
```

**`source/` is the single source of truth.** `cache/` (GLB, thumbnail, extracted metadata, sprite, USDZ) can be deleted at any time and rebuilt from it, and the in-memory index is rebuilt by every scan (there is no database, ADR-0007). This invariant shapes the whole design — don't break it.

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical triage labels are used as-is (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.


### internal配下の各ディレクトリの役割

config/
役割: アプリ設定の読み書き。
設定はライブラリの外、OS標準の設定ディレクトリに JSONで保存されます。
Blender実行パスの解決(blenderpath.go)もここです。

library/
役割: ライブラリディレクトリの構造と規則を一手に担う中核。
骨格(source// cache/ /templates/)の初期化、カテゴリ・アセットの列挙規則、アセット作成とメタ(meta.jsonのタグ・公開/非公開)の読み書き、キャッシュのパス決定・全削除(ClearCache)・孤児キャッシュ掃除(PruneCache)を提供します。
sourceへ書き込む経路はこのパッケージの 2経路(アセット作成とメタ書き込み)のみという規約になっています。配下のtemplates/ には新規アセット用の empty.blendが同梱されています。

scan/
役割: source/を歩いてアセット一覧を組み立てるスキャナー。
sourceに対して完全に読み取り専用で、Blenderは起動しません。
結果は indexに流し込まれます。

index/
役割: 検索・高速表示のためのインメモリ・インデックス(DB は使わない。ADR-0007)。
純粋なインデックスであり、ユーザーデータは一切持たず、sourceからいつでも再構築できます(ADR-0001)。

generate/
役割: 生成ジョブ(GLB・サムネイル・抽出メタデータ)の直列キュー(queue.go)とBlender CLI の実行(runner.go)。
成果物はcache/に書き出され、次のスキャンで拾われます。ジョブ状態の通知はポーリング方式(ADR-0002)。

server/
役割: HTTPハンドラ層。
機能ごとにファイルが分かれています(アセット一覧assets.go、詳細 detail.go、作成create.go、ジョブ jobs.go、タグtags.go、公開/非公開private.go、読み取り専用のリモート閲覧remote.go など)。
API 規約はdocs/api-conventions.md 参照。

web/
役割: フロントエンドのビルド成果物(frontend/ のVite ビルド)を go:embedでバイナリに埋め込むだけの薄いパッケージ。
static/の中身は cd frontend && npm run buildで生成されます。

### 依存の向き

server → generate / scan / index / library / config
scan → index
generate → library / config
「source/ が唯一の真実で、cache/ とインデックスは使い捨て」という不変条件を library(書き込み経路の限定)、scan(読み取り専用)、index(再構築可能)がれぞれの立場で守る設計になっています。