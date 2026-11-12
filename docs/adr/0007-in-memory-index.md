# インデックスは SQLite をやめてインメモリで持つ

ADR-0001 でインデックスは「source からいつでも再構築できる派生物」と決めており、実際に起動時スキャンとスキャンのたびに全行を置き換えていた(`ReplaceAll`)。永続化して得るものが無いため、SQLite(`database.db`)と GORM をやめ、スキャン結果をプロセスのメモリ上に持つことにした。`internal/index` の公開 API(`List` / `Find` / `ReplaceAll` / `Categories` / `TagCounts`)はそのまま。

## Considered Options

- **SQLite のまま残す** — 却下。中身は毎回作り直しており、ライブラリ直下に派生ファイルが増え、依存(GORM・SQLite ドライバ)も抱えるだけになっていた。

## Consequences

- ライブラリ直下に `database.db` を作らない。旧版が残したものは空判定で無視する(`library` の `legacyDBFileName`)。削除してよい。
- ライブラリを初めて開いた(切り替えた)時点でスキャンして中身を詰める。メモリ上にしか無いので、空のまま返すと次のスキャンまで一覧が空に見えるため。
- 検索の部分一致は従来の SQLite `LIKE` と同じく、ASCII だけ大文字小文字を区別しない。
- 5,000 アセット規模の一覧・検索 1 秒以内(requirements.md)は `TestListAndSearch5000AssetsUnderOneSecond` で引き続き確かめる。
