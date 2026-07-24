# ローカル3Dアセットマネージャー 要件定義書

## 1. 概要

### 目的

Blenderで制作した3DアセットをローカルPC上で一元管理するためのWebアプリケーションを開発する。

本システムでは以下を実現する。

- アセットの一覧表示
- 検索・フィルタリング
- Three.jsによるプレビュー
- Blenderとの連携
- GLB生成
- サムネイル生成
- メタデータ管理

本システムはローカル環境で動作し、インターネット接続を必要としない。

---

# 2. システム構成

```
React + Vite
      │
      │ HTTP API
      ▼
Go Backend
      │
      ├── SQLite
      ├── Blender CLI
      └── File System
```

---

# 3. 使用技術

|項目|技術|
|------|------|
|Frontend|React + Vite|
|UI|Tailwind CSS|
|3D表示|Three.js|
|Backend|Go|
|Database|SQLite|
|ORM|GORM|
|3D変換|Blender CLI|
|画像生成|Blender Python Script|

---

# 4. ディレクトリ構成

```
3DLibrary/

├── source/
│   ├── Characters/
│   ├── Props/
│   ├── Environment/
│   └── Vehicles/
│
├── cache/
│   ├── glb/
│   ├── thumbnails/
│   └── metadata/
│
├── exports/
│   ├── glb/
│   ├── fbx/
│   ├── obj/
│   └── usd/
│
├── templates/
│   ├── empty.blend
│   ├── character.blend
│   ├── environment.blend
│   └── vehicle.blend
│
└── database.db
```

### ディレクトリ役割

|ディレクトリ|用途|
|------------|------|
|source|編集対象となる.blendファイル|
|cache|GLB・サムネイルなど再生成可能なデータ|
|exports|ユーザーが書き出した成果物|
|templates|新規作成用テンプレート|
|database.db|SQLiteデータベース|

---

# 5. アセット構成

```
Props/

└── Wooden Chair/
    ├── model.blend
    ├── textures/
    ├── references/
    ├── renders/
    └── notes.md
```

アセットごとにディレクトリを作成する。

---

# 6. データベース設計

## assets

|項目|説明|
|------|------|
|id|ID|
|title|タイトル|
|category|カテゴリ|
|path|Blendファイル|
|thumbnail_path|サムネイル|
|glb_path|GLBファイル|
|size|サイズ|
|updated_at|更新日時|
|created_at|作成日時|

---

## tags

|項目|
|------|
|id|
|name|

---

## asset_tags

|項目|
|------|
|asset_id|
|tag_id|

---

## favorites

|項目|
|------|
|asset_id|

---

## history

|項目|
|------|
|asset_id|
|opened_at|

---

# 7. 機能一覧

## アセット一覧

- カード表示
- リスト表示
- サムネイル表示

---

## 検索

- タイトル検索
- タグ検索
- カテゴリ検索
- 更新日検索

---

## プレビュー

Three.jsによるGLB表示

対応操作

- 回転
- パン
- ズーム

---

## Blender起動

「Blenderで開く」ボタンからmodel.blendを起動する。

---

## GLB生成

Blender CLIを利用して

```
model.blend
↓

model.glb
```

を生成する。

保存先

```
cache/glb/
```

---

## サムネイル生成

Blender CLIによりサムネイル画像を生成する。

保存先

```
cache/thumbnails/
```

---

## メタデータ取得

Blender Python Scriptにより以下を取得する。

- Object数
- Collection数
- Material数
- Polygon数
- Texture数
- Animation有無

---

# 8. 新規アセット作成

ユーザーは以下を入力する。

- タイトル
- カテゴリ
- テンプレート

作成時に

```
source/

└── Props/
    └── Wooden Chair/
        ├── model.blend
        ├── textures/
        ├── references/
        ├── renders/
        └── notes.md
```

を自動生成する。

テンプレート.blendをコピーしてmodel.blendを生成する。

---

# 9. テンプレート

テンプレート一覧

```
templates/

empty.blend
character.blend
environment.blend
vehicle.blend
```

テンプレートには以下を保持できる。

- Collection
- Camera
- Lighting
- Render設定
- World設定

---

# 10. キャッシュ

```
cache/

glb/

thumbnails/

metadata/
```

キャッシュは削除しても問題ない。

必要に応じて再生成する。

---

# 11. エクスポート

対応予定フォーマット

- GLB
- FBX
- OBJ
- USD

エクスポート先

```
exports/
```

---

# 12. 設定

設定画面で変更可能とする。

- Blender実行ファイル
- ライブラリディレクトリ
- サムネイルサイズ
- GLB出力先
- テーマ

---

# 13. 将来機能

## ライブラリ管理

- 複数ライブラリ対応
- NAS対応
- 外付けSSD対応

---

## 自動同期

- ファイル監視
- 自動インデックス更新
- 自動GLB生成
- 自動サムネイル生成

---

## 検索強化

- AIタグ付け
- 類似アセット検索
- 全文検索

---

## Blender連携

- Blender Asset Browser連携
- Blenderアドオン
- ワンクリックインポート

---

## エクスポート

- Unity向け
- Unreal向け
- 一括変換

---

## その他

- お気に入り
- 最近開いたアセット
- レーティング
- コメント
- プラグイン機構

---

# 14. 非機能要件

|項目|内容|
|------|------|
|対応OS|Windows / macOS（Linuxは将来対応）|
|インターネット接続|不要|
|オフライン動作|必須|
|DB|SQLite|
|キャッシュ|再生成可能|
|レスポンス|一覧・検索は1秒以内を目標|
|バックアップ|sourceディレクトリのみで復元可能|
|データ保全|sourceを唯一の正しいデータ（ソース・オブ・トゥルース）とし、cacheとDBは再構築可能とする|

---

# 基本設計方針

本システムでは以下の考え方を採用する。

- **source** が唯一の正しいデータ（Source of Truth）
- **cache** はいつでも削除・再生成可能
- **SQLite** は検索・高速表示のためのインデックスとして利用
- **GLB** はThree.js表示用のキャッシュデータ
- **Blender** は編集専用
- **Three.js** は閲覧専用
