# API 規約

すべての HTTP API はこの規約に従う。最初の準拠例はヘルス API(`GET /api/health`)。

## パス

- プレフィックスは **`/api`**(バージョン番号は付けない。ローカル専用アプリでフロントとバックは常に同一バイナリから配布されるため、互換性維持のためのバージョニングは不要)
- リソースは複数形のケバブケースなし・小文字: `/api/assets`, `/api/assets/{id}`, `/api/health`
- `/api` 以外のパスはすべて SPA(埋め込みフロントエンド)の配信に使う

## JSON 命名規則

- リクエスト・レスポンスのフィールド名は **camelCase**(例: `polygonCount`, `isIncomplete`)
- Go 側は構造体の `json` タグで camelCase を明示する

## 成功レスポンス

- リソースをそのまま JSON で返す(エンベロープなし)
- `Content-Type: application/json; charset=utf-8`

```json
{ "status": "ok" }
```

## エラーレスポンス

- HTTP ステータスコードで大分類を表す(400 / 404 / 409 / 500 など)
- ボディは必ず次の形:

```json
{
  "error": {
    "code": "not_found",
    "message": "asset not found"
  }
}
```

- `code`: プログラムが分岐に使う安定した snake_case の識別子
- `message`: 人間向けの説明(英語)。UI 表示用の翻訳はフロント側で `code` を基に行う
- 存在するパスへのメソッド不一致 → `405` + `code: "method_not_allowed"`
- リモート閲覧(CONTEXT.md)に届いた GET / HEAD 以外 → `403` + `code: "remote_viewing_read_only"`。パスを問わずハンドラの手前で返すため、**副作用のある操作は必ず POST / PUT / DELETE で足す**(ADR-0004)

## ヘルス API(準拠例)

- `GET /api/health` → `200 OK`

```json
{ "status": "ok" }
```

- 未知の `/api/*` パス → `404` + 上記エラー形式(`code: "not_found"`)
