# vrclog-adapters — 永続ルール

- Adapterはpure/stateless/record-local。1つのRecordだけで判定し、goroutineを作らない。
- Decode内でI/Oを行わない（HTTP、DNS、file、subprocess、DB、logging/metrics side effect）。
- ruleはproject固有prefixへanchorする。ログ全体からの一般URL検索は行わない。
- Adapterはcanonical Event（`vrclog-go` の7型）のみを返す。独自Event型を作らない。
- `All()` aggregate APIは削除済み。下流は個別constructor（`yamaplayer.New()`, `iwasync3.New()`）を明示的にimportする。
- placeholder Adapterを作らない。fixtureで検証できない対応は追加しない。
- コア（`vrclog-go`）の変更が必要なcanonical Eventの不足は、このリポジトリで代替実装しない。
