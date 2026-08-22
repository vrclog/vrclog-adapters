# vrclog-adapters 契約追従・精度強化 指示書兼実装仕様書

## 0. この文書の扱い

この文書は `github.com/vrclog/vrclog-adapters` を、刷新済み `vrclog-go` の新しい整合性契約へ追従させ、community Adapterの誤検出、error semantics、privacy、CIを完成させるための最上位実装仕様である。

互換性維持は不要である。aggregate API、古いgolden ID、古いpseudo-versionを残す必要はない。

ただし、このリポジトリの役割を拡大してはならない。今回、新しいcommunity projectへ対応しない。

初期サポート対象は次の2つだけである。

```text
community.yamaplayer
community.iwasync3
```

VizVidは実fixtureで`vrchat.core`だけでは不足すると確認されるまで専用Adapterを作らない。

## 1. 前提となる上流契約

この作業は、`vrclog-go` の整合性強化が完了した後に行う。

必要な上流変更：

- Observation IDからemission indexが削除されている。
- 同一Record・同一Adapter内のRule IDが一意であることをEngineが要求する。
- canonical EventのRemoteResource、MediaTarget、MediaError validationが強化されている。
- `MediaTarget.Backend`は未特定の場合も明示的な`unknown`が必要である。

上流commitが未公開の場合、下流に互換ラッパーやコピー型を作ってはならない。必要なら一時的な`go.work`で隣接checkoutを参照し、最終的な`go.mod`には公開commit/versionを記録する。commit済み`replace ../vrclog-go`は禁止する。

依存方向：

```text
vrclog-go ← vrclog-adapters ← vrclog-companion
```

## 2. リポジトリ責務

### このリポジトリが行うこと

- community project固有の既知ログprefixを認識する。
- 実ログfixtureに根拠を持つ情報だけを抽出する。
- community語彙を`vrclog-go`のcanonical Eventへ変換する。
- Adapter ID / Rule IDを安定して維持する。
- positive、negative、collision fixtureを保有する。
- fixtureが何を検証したかを正確に文書化する。

### 行わないこと

- `output_log` file I/O
- network access
- VRChat API access
- SQLite
- media correlation
- URL title/thumbnail取得
- runtime plugin / YAML pattern
- log全体に対する汎用URL抽出
- 実fixtureのないプロジェクトのplaceholder
- `vrclog-go` canonical Eventと同じ型の再定義

Adapterは純粋なCPU処理でなければならない。

```go
Decode(record vrclog.Record) ([]vrclog.Emission, error)
```

## 3. 今回解決する問題

1. YamaPlayer Adapterが人間向けerror message全体を`Code`にも設定している。
2. iwaSync3 AdapterがURLを含み得る長いログ本文をそのまま`Message`へ保存する。
3. error textのcontrol character、上限、URL redactionがAdapter間で統一されていない。
4. rootの`adapters.All()`が、依存更新だけで新Adapterを暗黙に有効化する構造になっている。
5. compatibility matrixの`verified`がfixture 1件だけの検証とversion-level保証を区別していない。
6. CI workflowが存在しない。
7. `vrclog-go`の新Observation IDとvalidationへ追従する必要がある。

## 4. 絶対に維持する不変条件

1. Adapterは既知prefixがmessage先頭にある場合だけmatchする。
2. ログ中にcommunity prefixの文字列が引用・埋め込みされているだけの行をmatchしない。
3. URLらしい文字列をログ全体から無差別に拾わない。
4. Eventは`vrclog-go`のcanonical型だけを使用する。
5. 同一Adapterが同一Recordで同じRule IDを複数emitしない。
6. Rule IDは意味ごとに安定しており、配列位置に依存しない。
7. errorの`Code`にはmachine-readableな識別子だけを入れる。
8. errorの`Message`に完全なHTTP(S) URLを残さない。
9. Adapterはpanicせず、malformedな対象行は明示的なerrorまたは非matchとして扱う。
10. 外部通信を一切行わない。

## 5. 実装仕様

## 5.1 vrclog-go依存を更新する

`go.mod`を完成済みcore commit/versionへ更新する。

- 新しいObservation IDに伴いgolden出力を更新する。
- canonical Event validationを満たすよう全payloadを修正する。
- `MediaTarget.Backend`を必ず明示する。
- Adapter IDと既存Rule IDは、意味が変わらない限り維持する。

現行の主要ID：

```text
Adapter: community.yamaplayer
Rule:    youtube_resolve_url
Rule:    video_error

Adapter: community.iwasync3
Rule:    player_error
```

Ruleの意味を分割する場合のみ新Rule IDを追加する。

## 5.2 aggregate APIを削除する

root packageの次のAPIを削除する。

```go
func All() []vrclog.Adapter
```

`adapters.go`と専用testが不要になれば削除する。root packageに他の公開責務がなければ、module rootにGo packageを残す必要はない。

理由：

- 新Adapter追加がCompanionで暗黙に有効化される。
- support setの変更がdependency updateへ隠れる。
- Companionでのセキュリティ・機能レビュー単位が曖昧になる。

Companionは次のように明示的に組み込む。

```go
community := []vrclog.Adapter{
    yamaplayer.New(),
    iwasync3.New(),
}
```

別の`All`、`Default`、global registry、init自動登録を作ってはならない。

## 5.3 error textの共通正規化を実装する

`internal/errtext`等の非公開packageを作り、YamaPlayerとiwaSync3で共用する。

推奨API例：

```go
type Parsed struct {
    Code    string
    Message string
}

func Parse(raw string) Parsed
```

名前は変更してよいが、次の規則を満たすこと。

### 正規化

1. 前後空白を除去する。
2. CR/LFとunsafe control characterをspaceへ置換する。
3. 連続した不要な空白を正規化する。ただし意味のあるASCII tokenを壊さない。
4. HTTP(S) URLを`<url>`へredactする。
5. UTF-8を壊さず、Messageを最大2048 bytesへtruncateする。
6. Codeを最大128 bytesへ制限する。
7. 正規化後にCodeもMessageも空なら、対象行のdecode errorとする。

### Code抽出

`Code`へ入れてよいもの：

- 符号付き/符号なし整数error code
- upstreamが明確にmachine-readable IDとして出力する固定token

人間向け文章をCodeへ複製してはならない。

最低限、以下を扱う。

```text
-736347996.
  → Code: -736347996
  → Message: ""

-736347996: Loading failed.
  → Code: -736347996
  → Message: Loading failed.

Connection timeout.
  → Code: ""
  → Message: Connection timeout.

Error for https://example.invalid/video
  → Code: ""
  → Message: Error for <url>
```

曖昧なtokenを無理にCode化しない。

### URL抽出との関係

URLを`Message`からredactしただけで`ResourceURLObserved`を新規生成してはならない。

- 既存fixtureとupstreamログの意味から、そのURLがsource/resolver/playback resourceだと明確な場合だけ別RuleとしてEvent化する。
- 今回は新しいiwaSync3 URL Ruleを必須にしない。
- `vrchat.core`が既にURLを観測する場合は重複実装しない。

## 5.4 YamaPlayer Adapterを修正する

### URL Rule

`youtube_resolve_url`は維持する。

- messageが`[YamaStream]`で始まること。
- `Resolve youtube url: `が期待位置に存在すること。
- URLはcoreのRemoteResource validationを満たすこと。
- Resource:

```text
Kind: video
Role: source
Target.Component: yamaplayer
Target.Backend: unknown
```

- URLが欠落した対象らしい行はdecode error。
- URLがHTTP(S)でない場合は非matchまたは明示的な方針をtestで固定する。危険schemeをEvent化しない。

### Error Rule

`video_error`は維持する。

- regexは完全anchorする。
- keyはTarget.Keyへ設定する。
- Target:

```text
Component: yamaplayer
Key:       upstream key
Backend:   unknown
```

- `Code`と`Message`は共通errtext parserの結果を使う。
- 人間向けmessageをCodeへ入れない。
- URLをMessageへ残さない。
- key、messageがcore上限を超える場合は安全に正規化するかdecode errorにする。

## 5.5 iwaSync3 Adapterを修正する

`player_error`を維持する。

- messageは`[iwaSync3] `で始まること。
- `PlayerError`が既知contextにあること。
- embedded prefixを認識しない。
- Error:

```text
Stage: playback
Target.Component: iwasync3
Target.Backend: unknown
```

- Code/Messageは共通errtext parserを使用する。
- URLをMessageへ残さない。
- 数値codeが明確でなければCodeは空でよい。
- 完全な元URLをcanonical Resourceとして扱う根拠がfixtureにない場合、URL Eventを追加しない。

単に`strings.Contains("PlayerError")`だけで広い形式を受理する場合は、positive fixtureとnegative fixtureで許容範囲を固定する。より狭いregexへできるなら狭くする。

## 5.6 fixtureと互換性表現を精密化する

`compatibility/README.md`のstatus vocabularyを次のように分ける。

推奨値：

```text
fixture_verified
field_tested
version_verified
unverified
```

意味：

- `fixture_verified`: 実ログ由来fixtureが少なくとも1 scenarioを通過した。
- `field_tested`: 現行VRChat実環境で開発者が動作確認した。
- `version_verified`: upstreamの特定version/commitに対して複数scenarioを確認した。
- `unverified`: fixtureなし。support claimなし。

現在version不明・public issue由来fixtureだけなら`fixture_verified`とする。

表には最低限次を記載する。

```text
Project
Adapter ID
Observed canonical event
Upstream version/commit
Captured date
Fixture source
Verification level
Known limitations
```

「プロジェクト全体をサポート」「全version対応」と読める表現を避ける。

## 5.7 CIを追加する

`.github/workflows/ci.yml`を追加する。

最低構成：

```text
push main
pull_request main

Test matrix:
- ubuntu-latest
- windows-latest
- Go 1.25

gofmt check
go vet ./...
go test -count=1 ./...

Ubuntu additional:
go test -race -count=1 ./...
go test -coverprofile=coverage.out ./...
```

必要ならCodeCov uploadを追加してよいが、外部サービス失敗を必須gateにしなくてよい。

CI内でnetwork fixture取得を行わない。fixtureはrepositoryへ保存する。

## 6. テスト仕様

## 6.1 YamaPlayer

最低限：

- 正常なYouTube source URL。
- URL欠落行。
- 非HTTP(S) URL。
- embedded `[YamaStream]`。
- 数値error codeのみ。
- 数値code + human message。
- human messageのみ。
- URL入りmessageがredactされる。
- control character正規化。
- oversized message truncate。
- Target backendが`unknown`。
- Engine経由でcanonical Event validationを通る。

## 6.2 iwaSync3

最低限：

- 正常なPlayerError。
- embedded prefix。
- `PlayerError`を含まないiwaSync3 line。
- URL入りmessageのredaction。
- 明確な数値code。
- human messageのみ。
- oversized message。
- Engine経由validation。

## 6.3 cross-adapter

同じRecordをcore + YamaPlayer + iwaSync3へ通した場合：

- panicしない。
- Adapter ID衝突なし。
- 同一Adapter内Rule ID重複なし。
- 無関係なAdapterが誤matchしない。
- 出力はdeterministic。
- emission順序に依存せずObservation IDが安定する。

## 6.4 golden fixture

- Observation ID変更に伴いgoldenを更新する。
- golden更新コマンドをREADMEまたはtest commentに記載する。
- `UPDATE_GOLDEN=1`等の仕組みを使う場合、CIでは自動更新しない。
- golden差分は人間がレビュー可能なJSON/JSONLにする。

## 7. 推奨実装フェーズ

### Phase 1: 上流追従

- `vrclog-go` dependency更新
- compile error解消
- backend明示
- golden ID更新前の失敗確認

### Phase 2: aggregate削除

- `All()`削除
- root package整理
- README import例更新

### Phase 3: errtext

- 共通parser/sanitizer
- unit test
- YamaPlayer追従
- iwaSync3追従

### Phase 4: fixture / compatibility

- negative fixture追加
- status vocabulary更新
- limitation明記

### Phase 5: CI / cleanup

- workflow追加
- `go mod tidy`
- stale code/comment削除

## 8. 禁止事項

- `All()`の別名再導入
- initによる自動登録
- global mutable registry
- runtime plugin
- YAML parser
- HTTP request
- yt-dlp
- YouTube API
- `output_log` file read
- DB依存
- VizVid/VideoTXL等の空package
- fixtureのないRule
- log全体の`https?://`抽出
- Message内URLの保存
- human textのCode複製

## 9. 受け入れコマンド

```bash
gofmt -w .
go vet ./...
go test -count=1 ./...
go test -race -count=1 ./...
go test -run 'Test.*Yama|Test.*Iwa|Test.*Collision|Test.*Golden' -count=20 ./...
go mod tidy
git diff --exit-code -- go.mod go.sum
```

workflow YAMLのsyntaxと、Windows/Linux両方のCIが通ること。

## 10. 完了条件

- YamaPlayerのhuman messageがCodeへ複製されない。
- iwaSync3/YamaPlayerのMessageへ完全なHTTP(S) URLが残らない。
- Target backendが常に明示される。
- `adapters.All()`が存在しない。
- Companionがconstructorを明示的に選択できる。
- compatibility表がfixture-levelの保証を正確に表す。
- CIが存在し、Windows/Linux/test/raceを実行する。
- 新しいcore契約で全fixture/goldenが通る。
- 新しいcommunity projectを追加していない。

## 11. Companionへの完了報告

実装完了時に次を通知する。

```text
- 参照すべきvrclog-adapters commit/version
- 参照中のvrclog-go commit/version
- 明示的にimportすべきconstructor:
  - yamaplayer.New()
  - iwasync3.New()
- Adapter ID / Rule ID一覧
- All()削除
- error Code/Message semantics
- compatibility verification level
```

明示的な指示なしにcommit、push、tag、releaseは行わない。
