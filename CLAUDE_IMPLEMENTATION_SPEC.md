# vrclog-adapters 新規実装仕様書

作成日: 2026-08-18  
対象リポジトリ: `github.com/vrclog/vrclog-adapters`  
対象実装者: Claude Code  
仕様状態: **Superseded — 初期実装完了後は [`CLAUDE_HARDENING_SPEC.md`](CLAUDE_HARDENING_SPEC.md) が最上位仕様である**

> **注意**: 本文書は初回実装（2026-08-18）時点の仕様であり、root packageの `All()` aggregate API 等、その後 `CLAUDE_HARDENING_SPEC.md` により削除・変更された内容を含む。現行の実装契約は `CLAUDE_HARDENING_SPEC.md` を参照すること。本文書は経緯の記録として保持する。

---

## 0. Claude Codeへの最上位指示

この文書を、対象リポジトリにおける最上位の実装仕様として扱うこと。

- このリポジトリは、VRChatのコミュニティ製プロジェクトが `output_log` へ出す固有ログを、`vrclog-go` のcanonical Eventへ変換するcompile-time Adapter集である。
- 汎用plugin framework、YAML pattern集、runtime downloader、WASM、remote update機構へ拡張しないこと。
- 旧来のParser、ParserChain、RegexParserという名称・概念を導入しないこと。
- upstreamプロジェクトの内部状態を推測し、fixtureに存在しないEventを作らないこと。
- 対象ログの実fixtureが不足している場合、推測実装で埋めず、未対応のまま明記すること。
- `vrclog-go` のcanonical Eventが不足する場合、このリポジトリで独自Eventを作らず、上流 `vrclog-go` の契約変更として扱うこと。
- 実装完了時、READMEとCLAUDE.mdに実際の対応範囲だけを記載し、未検証のFuture一覧を作らないこと。
- フェーズごとにテストし、最後に全体test、race、vet、Windows buildを実行すること。

---

# 1. このリポジトリの役割

`vrclog-adapters` は、YamaPlayer、iwaSync3等、VRChat上で利用されるコミュニティ製ワールドアセット固有のログ形式を解釈し、`vrclog-go` のcanonical Eventを返すAdapterを提供する。

```text
VRChat output_log
        │
        ▼
vrclog-go: Record
        │
        ├──────────────────────────────────────┐
        │                                      │
        ▼                                      ▼
vrclog-go built-in Adapter            vrclog-adapters
VRChat client log                      community project log
        │                                      │
        └──────────────────┬───────────────────┘
                           ▼
                 canonical Observation
                           ▼
                  vrclog-companion
```

このリポジトリの価値は、対応数を増やすことではなく、変化の速いcommunity project固有実装を `vrclog-go` coreから隔離し、実ログfixtureとともに検証可能に管理することにある。

---

# 2. リポジトリ間契約

依存方向を厳守する。

```text
vrclog-go ← vrclog-adapters ← vrclog-companion
```

## 2.1 依存してよいもの

- `github.com/vrclog/vrclog-go`
- Go標準ライブラリ

## 2.2 依存してはならないもの

- `vrclog-companion`
- SQLite
- Web framework
- VRChat API client
- yt-dlp
- HTTP client library
- runtime plugin framework
- YAML parser
- scripting engine

## 2.3 下流との契約

`vrclog-companion` は次のように構成する。

```go
allAdapters := []vrclog.Adapter{
    vrclog.NewVRChatAdapter(),
}
allAdapters = append(allAdapters, adapters.All()...)

engine, err := vrclog.NewEngine(allAdapters...)
```

- built-in AdapterはCompanionが先頭に登録する。
- `All()` はcommunity Adapterだけを返す。
- 本リポジトリはbuilt-in Adapterを再exportしない。
- hidden global registrationや`init()` registrationを使わない。

---

# 3. `vrclog-go` 契約の前提

このリポジトリは、刷新後の `vrclog-go` が次の契約を持つ前提で実装する。

```go
type AdapterID string
type RuleID string

type Emission struct {
    Rule  RuleID
    Event Event
}

type Adapter interface {
    ID() AdapterID
    Decode(record Record) ([]Emission, error)
}
```

Engineは全Adapterを登録順に実行し、Adapter errorを隔離する。

Adapterは次のcanonical Eventだけを返す。初期community対応で主に使うものは以下である。

```text
resource.url_observed
resource.resolved
media.error_observed
```

関連payload:

```go
type RemoteResource struct {
    URL  string
    Kind ResourceKind
    Role ResourceRole
}

type MediaTarget struct {
    Component string
    Key       string
    Backend   MediaBackend
}

type ResourceURLObserved struct {
    Resource RemoteResource
    Target   *MediaTarget
}

type ResourceResolved struct {
    Input  RemoteResource
    Output RemoteResource
    Target *MediaTarget
}

type MediaErrorObserved struct {
    Stage    MediaStage
    Code     string
    Message  string
    Resource *RemoteResource
    Target   *MediaTarget
}
```

本リポジトリ側で、この契約の代替型やcopyを作らないこと。

ローカル同時開発ではリポジトリ外の一時 `go.work` を利用してよいが、ローカルpathの `replace` をcommitしない。

---

# 4. 非目標

以下はこのリポジトリの責務ではない。

- ログファイルのread/follow/rotation
- timestamp/header decode
- cursor管理
- Observation ID生成
- Adapterの実行順序管理
- Player/Worldのbuilt-in log parsing
- semantic media correlation
- Current Video判定
- URL canonicalization
- YouTube video ID抽出
- title/thumbnail取得
- HTTP availability check
- browser起動
- DB/API/UI
- runtime Adapter enable/disable
- remote Adapter catalog
- auto-update
- plugin signing
- arbitrary user regex
- arbitrary custom Event
- upstream projectのバージョン自動検出

---

# 5. package構成

初期構成は次とする。

```text
vrclog-adapters/
├── adapters.go
├── yamaplayer/
│   ├── adapter.go
│   ├── adapter_test.go
│   └── testdata/
│       └── <scenario>/
│           ├── input.log
│           ├── expected.json
│           └── metadata.json
├── iwasync3/
│   ├── adapter.go
│   ├── adapter_test.go
│   └── testdata/
│       └── <scenario>/
│           ├── input.log
│           ├── expected.json
│           └── metadata.json
├── compatibility/
│   └── README.md
├── README.md
├── CLAUDE.md
├── CHANGELOG.md
├── go.mod
└── LICENSE
```

package root名は `adapters` とする。

```go
import adapters "github.com/vrclog/vrclog-adapters"
```

各project packageは明示import可能にする。

```go
import "github.com/vrclog/vrclog-adapters/yamaplayer"
import "github.com/vrclog/vrclog-adapters/iwasync3"
```

以下を作らないこと。

- `internal/parser`
- generic regex package
- YAML pattern directory
- runtime registry
- generated plugin catalog
- empty placeholder package
- future project名だけを並べたdirectory

---

# 6. Root API

## 6.1 `All`

```go
func All() []vrclog.Adapter
```

要件:

- 呼び出しごとにfreshなAdapter instanceを返す。
- community Adapterだけを返す。
- 順序は決定的にする。
- 初期順序は次とする。

```text
1. community.yamaplayer
2. community.iwasync3
```

- global mutable sliceをそのまま返さない。
- Adapterがstatelessでも、呼び出し側がsliceを変更しても他の呼び出しへ影響しないようにする。
- `All()` はerrorを返さない。compile-timeで構成が確定しているためである。

## 6.2 個別constructor

各packageは次を提供する。

```go
func New() vrclog.Adapter
```

下流が一部Adapterだけを選択できるようにするが、runtime pluginや設定file loaderは作らない。

---

# 7. Adapter共通実装規則

## 7.1 Pure / stateless

全Adapterは次を満たす。

- immutableまたはzero mutable state
- 1つのRecordだけで判定
- deterministic
- thread-safeであることが自然に成立
- clock、random、environmentに依存しない
- goroutineを作らない

## 7.2 禁止I/O

`Decode` 内で次を行わない。

- HTTP GET/HEAD
- DNS
- file read/write
- subprocess
- VRChat API
- YouTube API
- yt-dlp
- DB
- logging side effect
- metrics side effect

## 7.3 Parse strategy

各ruleは次の順序で実装する。

1. project固有prefixを `strings.Contains` / `HasPrefix` で安価に確認
2. rule固有の固定文言を確認
3. 必要な場合だけcompile済みregexpでcapture
4. captureをvalidate
5. canonical Eventを返す

無関係な全Recordへ複雑なregexpを走らせない。

## 7.4 Anchor

ログ全体からURLを一般検索してはならない。

禁止例:

```go
regexp.MustCompile(`https?://\S+`)
```

を無条件に全Recordへ適用すること。

許可されるのは、次のようにproject固有contextへanchorされたruleである。

```text
[YamaStream] Resolve youtube url: <URL>
```

## 7.5 URL

- ログに現れたURL文字列をそのまま保存する。
- query stringを削除しない。
- fragmentを削除しない。
- `youtu.be` を`youtube.com`へ変えない。
- percent encodingをdecode/re-encodeしない。
- provider判定をpayloadへ追加しない。
- HTTP/HTTPSのみを初期event対象にする。
- non-HTTP URLは無視する。errorにしない。
- signed URLやtokenをtest failure messageへ不用意に出さない。

## 7.6 Adapter error

Adapter errorは次の場合だけ返す。

- project固有prefixとrule開始文言が明確にmatchした
- しかし必須fieldの形式が壊れており、Eventを安全に作れない

単なる非該当は `nil, nil`。

誤った使い方:

- prefixがない全Recordへerror
- optional fieldがないだけでerror
- non-HTTP URLでerror
- upstreamの未知メッセージをすべてerror

## 7.7 Emission

- 全Emissionにstable RuleIDを設定する。
- 一つのlog lineから意味の異なる複数canonical Eventが明示されている場合だけ複数emitしてよい。
- 同じ意味のEventを「raw vendor event」と「canonical event」で二重emitしない。
- vendor-specific event typeをemitしない。

---

# 8. Adapter IDとRule ID

## 8.1 Adapter ID

初期IDは固定する。

```text
community.yamaplayer
community.iwasync3
```

IDをupstream display nameの大文字小文字に依存させない。

## 8.2 Rule ID

snake_caseの安定した意味名を使う。

YamaPlayer初期rule:

```text
youtube_resolve_url
video_error
```

iwaSync3初期rule:

```text
player_error
```

将来ruleを追加する場合も、内部regexp番号やupstream line numberをIDにしない。

Rule IDはObservation IDの一部になるため、意味を変えずに安易にrenameしない。

互換性を一般に無視する刷新であっても、実装後のfixture再現性のためstable identifierとして扱う。

---

# 9. YamaPlayer Adapter

## 9.1 Package

```text
github.com/vrclog/vrclog-adapters/yamaplayer
```

constructor:

```go
func New() vrclog.Adapter
```

Adapter ID:

```text
community.yamaplayer
```

Target component:

```text
yamaplayer
```

## 9.2 初期対応rule 1: original YouTube URL

実ログで確認された文脈:

```text
[YamaStream] Resolve youtube url: https://www.youtube.com/watch?v=...
```

出力:

```text
RuleID:   youtube_resolve_url
Event:    resource.url_observed
Kind:     video
Role:     source
Target:   component=yamaplayer
URL:      exact observed string
```

要件:

- `[YamaStream]` と `Resolve youtube url:` の両方へanchorする。
- URLは行末またはfixtureで確認したdelimiterまで抽出する。
- trailing punctuationを一般規則で勝手に削らない。fixtureのformatに合わせる。
- HTTP/HTTPS以外はemitしない。
- URLが欠落・壊れている明確なmatchはAdapter error。
- YouTube provider metadataやvideo IDは追加しない。

## 9.3 初期対応rule 2: player-specific video error

実ログで確認された文脈例:

```text
[YamaStream] [1] Video error: 500.
```

出力:

```text
RuleID:   video_error
Event:    media.error_observed
Stage:    playback
Code:     captured code
Message:  concise observed error message
Target:
  component=yamaplayer
  key=<captured player index/key>
  backend=unknown unless same line proves otherwise
```

要件:

- keyを文字列として保持する。
- codeの末尾punctuationをfixtureに従い正規化してよいが、意味を変えない。
- backendをYamaPlayerの一般知識だけでAVProへ固定しない。同じRecordが明示していない場合はunknown。
- 別のAVPro行との相関はCompanionが行う。

## 9.4 初期非対応

次は実fixtureで「generic coreだけでは不足し、ユーザー価値がある」と確認するまで追加しない。

- `Play track:`
- queue操作
- playlist index
- owner
- synchronized position
- volume
- title
- thumbnail
- URL resolver internal state
- relay URLの再emit

特に、generic `vrchat.core` Adapterが既にresolver/playback URLを抽出する場合、YamaPlayer Adapterで同じrelay URLを重複emitしない。

## 9.5 YamaPlayer integration expectation

同じ再生試行で次のObservation群が生じ得る。

```text
community.yamaplayer
  resource.url_observed
  role=source
  URL=original YouTube URL

vrchat.core
  resource.url_observed
  role=resolver_input or playback_input
  URL=YamaPlayer relay URL

vrchat.core / community.yamaplayer
  media.error_observed
```

この複数Observationを一つのMedia Attemptへまとめ、original URLを選ぶのはCompanionの責務である。YamaPlayer Adapter内で他Adapterの結果を見てはならない。

---

# 10. iwaSync3 Adapter

## 10.1 Package

```text
github.com/vrclog/vrclog-adapters/iwasync3
```

constructor:

```go
func New() vrclog.Adapter
```

Adapter ID:

```text
community.iwasync3
```

Target component:

```text
iwasync3
```

## 10.2 初期対応rule: PlayerError

実ログで確認された文脈:

```text
[iwaSync3] There was a `PlayerError` error in the video.
```

出力:

```text
RuleID:   player_error
Event:    media.error_observed
Stage:    playback
Message:  observed PlayerError message
Target:
  component=iwasync3
  backend=unknown
```

要件:

- `[iwaSync3]` prefixとPlayerError文言へanchorする。
- codeが同じRecordにない場合は空にする。
- URLが同じRecordにない場合はResourceを設定しない。
- generic VRChat Video Playback行からsource URLが別Observationとして得られる前提でよい。

## 10.3 unprefixed continuation lineをparseしない

公開ログ例では、PlayerErrorの次に次のような詳細行が現れる場合がある。

```text
-736347996: url: https://...
```

初期Adapterはstateless / record-localであるため、`[iwaSync3]` prefixのないこの行をiwaSync3のものとしてparseしてはならない。

理由:

- 別componentの同形式行と区別できない
- 直前行とのstateful correlationが必要
- URL一般検索に近づく
- generic VRChat Adapterが元URLを抽出できるケースがある

同じ物理Record内に `[iwaSync3]` とURLが共存するfixtureが得られた場合だけ、anchored ruleとして追加する。

## 10.4 初期非対応

- unprefixed error detail
- owner/state sync
- playlist
- video progress
- title
- URL queue
- backend推定
- other iwaSync versionsの推測pattern

---

# 11. VizVidの扱い

初期リリースではVizVid Adapter packageを作らない。

手順:

1. VizVidを利用する実ログfixtureを取得する。
2. `vrclog.NewVRChatAdapter()` だけでoriginal source URLまたはブラウザで開けるURLが抽出できるか確認する。
3. generic coreだけで要件を満たすなら、READMEのcompatibility matrixへ「core-only verified」と記載する。
4. VizVid固有ログがなければ取得できない有用情報が確認された場合だけ `vizvid` packageを追加する。

禁止:

- 空のVizVid Adapter
- project名だけのplaceholder
- GitHub source codeからログ文言を推測しfixtureなしで実装
- 「多分対応」とREADMEへ記載

---

# 12. VideoTXL / USharpVideo / その他の扱い

初期scope外とする。

追加条件:

1. user-facingな具体的利用目的がある
2. real output log fixtureがある
3. generic coreで不足する
4. canonical Eventへ意味を正確に対応できる
5. negative corpusを用意できる
6. upstream変化を追跡する保守意思がある

条件を満たさないproject名をFuture一覧へ並べない。

issueを作る場合も、project名ではなく次を記載する。

- 欲しい観測事実
- 実ログ
- generic coreで不足する理由
- canonical Eventへのmapping
- privacy/redaction状況

---

# 13. Fixture設計

Adapterの品質はコード量ではなくfixtureで決める。

## 13.1 Directory

各scenario:

```text
testdata/<scenario>/
├── input.log
├── expected.json
└── metadata.json
```

## 13.2 `input.log`

- VRChat `output_log` の実形式を保つ。
- timestamp/headerを含める。
- 対象行だけでなく、前後のgeneric video行やerror行を適度に含める。
- testが`vrclog.ReadFile`を通してRecordを生成できる形にする。
- line ending別testが必要なら明示scenarioを分ける。

## 13.3 `expected.json`

そのAdapter単独、またはintegration Engineが生成すべきObservationの期待値を記載する。

volatile fieldは固定fixtureから決定できるため、Observation IDもgolden対象に含めてよい。

最低限:

- Adapter ID
- Rule ID
- EventKind
- payload
- Record line/offset
- exact URL

## 13.4 `metadata.json`

例:

```json
{
  "project": "YamaPlayer",
  "project_version": "unknown",
  "project_commit": "unknown",
  "vrchat_version": "unknown",
  "captured_at": "2026-08-18",
  "scenario": "youtube source URL followed by relay failure",
  "source": "public_issue",
  "redactions": [
    "windows_username",
    "user_display_name",
    "video_id"
  ]
}
```

要件:

- 不明値を推測しない。`unknown`を使う。
- capture dateとupstream versionを混同しない。
- 「supports version range」ではなく「verified against」を表現する。

## 13.5 Redaction

削除・置換対象:

- Windows user name
- private user IDs
- display names
- private instance IDs
- local path
- signed token
- authorization query
- private/unlisted content URL（公開fixtureへ置けない場合）

置換後もparserの構造テストが成立するよう、文字種とdelimiterを保つ。

例:

```text
https://www.youtube.com/watch?v=TESTVIDEO01
```

実在する動画へアクセスする必要はない。

## 13.6 Fixtureを捏造しない

synthetic negative testは作ってよいが、positive compatibility fixtureは実ログに由来すること。

positive fixtureのmetadataにはsourceを記録する。

---

# 14. Test strategy

## 14.1 Adapter単体test

各packageで最低限:

- exact positive fixture
- malformed anchored line → error
- prefixだけ似たline → no match
- unrelated URL → no match
- non-http URL → no match
- Unicode/spacing variationがfixture上有効なら保持
- exact URL preservation
- stable Adapter ID
- stable Rule ID
- repeated Decode gives identical result
- concurrent Decodeがdata raceを起こさない

## 14.2 Engine integration test

`vrclog.ReadFile` と `vrclog.NewEngine` を使い、実際のpipelineを検証する。

YamaPlayer scenario:

```go
engine := vrclog.NewEngine(
    vrclog.NewVRChatAdapter(),
    yamaplayer.New(),
)
```

期待:

- original YouTube URLは`community.yamaplayer`のsource resource
- relay/resolver URLは`vrchat.core`
- error Eventが別Observationとして残る
- first-matchによる欠落がない
- exact deterministic ordering

IwaSync3 scenario:

```go
engine := vrclog.NewEngine(
    vrclog.NewVRChatAdapter(),
    iwasync3.New(),
)
```

期待:

- generic coreがsource URLを観測
- iwaSync3 AdapterがPlayerErrorを観測
- unprefixed detail lineをiwaSync3 Adapterが誤parseしない

## 14.3 Negative corpus

複数Adapterが同じ行へ誤matchしないことを確認する。

最低限:

- Udon Debug.Logに`[YamaStream]`という文字列を説明文として含むだけの行
- YouTube URLだけの行
- `[iwaSync3]`以外のPlayerError
- YamaPlayer errorに似た別component
- command line/path中のURL
- malformed quote
- non-http media scheme

## 14.4 Collision test

同じRecordを全community Adapterへ通し、意図しない重複がないことを検査する。

許容:

- 同じ再生試行について、異なるRecordから異なる意味のObservationが出る
- coreとcommunityがそれぞれsource URLとresolver URLを出す

不許容:

- 同じRecord・同じURL・同じ意味を複数community Adapterがemit
- community Adapterがgeneric core Eventをcopyするだけ
- vendor raw Eventとcanonical Eventの二重emit

## 14.5 Quality gate

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
GOOS=windows GOARCH=amd64 go build ./...
```

library moduleなのでWindows cross-buildで全packageがbuildできることを確認する。

---

# 15. Compatibility matrix

READMEに実際のfixtureに基づくmatrixを置く。

形式例:

| Project | Adapter | Verified observation | Fixture provenance | Status |
|---|---|---|---|---|
| YamaPlayer | `community.yamaplayer` | original YouTube URL, player error | version/dateを記載 | verified |
| iwaSync3 | `community.iwasync3` | PlayerError; URLはcore経由 | version/dateを記載 | verified |
| VizVid | none/core-only | source URL | fixtureがある場合のみ | verified / unverified |

## 15.1 Status語彙

初期READMEでは次だけを使う。

```text
verified
unverified
```

`experimental`、`supported >=x`等の曖昧なtierをコードへ持ち込まない。

## 15.2 表現

良い表現:

```text
Verified against YamaPlayer X.Y.Z fixture captured on YYYY-MM-DD.
```

悪い表現:

```text
Supports all YamaPlayer versions.
```

バージョンがログから判定できない場合、runtimeで誤ったsupport判定をしない。

---

# 16. Documentation

## README

次を含める。

- このrepoがcommunity固有Adapterだけを持つこと
- coreとの責務分離
- compile-time composition
- `All()` と個別constructorの例
- current compatibility matrix
- fixture由来のverified範囲
- non-goals
- privacy/redaction policy

次を含めない。

- plugin marketplace構想
- runtime update構想
- YAML pattern
- WASM
- project名だけのFuture list
- 未検証support claim

## CLAUDE.md

実装後の短い永続ルール:

- adapters are pure/stateless/record-local
- no I/O
- prefix anchored
- canonical Event only
- no generic URL scan
- fixture first
- no placeholder adapters
- All order is deterministic
- core changes are required for new Event meanings

## CHANGELOG

新規repoとして初期対応を記載する。

- YamaPlayer verified rules
- iwaSync3 verified rule
- root `All()`
- fixture/test infrastructure

---

# 17. Securityとprivacy

## 17.1 Passive only

Adapterは既に読み取られたRecordをCPU上でdecodeするだけである。

しないこと:

- VRChat process attach
- memory read
- DLL injection
- network interception
- EAC interaction
- VRChat API authentication
- URL access

## 17.2 URL confidentiality

URLには次が含まれ得る。

- unlisted content
- private relay
- signed token
- temporary CDN signature

そのため:

- AdapterはURLを外部送信しない
- test logへsecretを残さない
- error messageでURLを不用意に再出力しない
- exact URLはEvent payloadだけに保持する
- metadata取得を行わない

## 17.3 ReDoS

- regexpはcompile-time固定
- catastrophic backtrackingがないGo regexpを使用
- line sizeはcoreで上限管理される
- それでも必要以上に広いregexpを使わない
- prefix prefilterを行う

---

# 18. 実装フェーズ

## Phase 1: Repository bootstrap

実装:

- module `github.com/vrclog/vrclog-adapters`
- Go versionを`vrclog-go`と揃える
- root package
- README/CLAUDE/CHANGELOG skeleton
- CIまたは既存org標準workflow
- licenseをvrclog organizationの方針へ合わせる

完了条件:

- empty moduleとしてbuild/testできる
- Companionや別moduleへの依存なし

## Phase 2: Root composition

実装:

- `All()`
- deterministic fresh slice test
- package import layout

完了条件:

- Yama/iwa constructor追加前でも構造が明確
- global registrationなし

## Phase 3: YamaPlayer

実装:

- fixture metadata
- source URL rule
- video error rule
- negative corpus
- Engine integration test

完了条件:

- original YouTube URLをexact source resourceとしてemit
- relay URLを重複emitしない
- generic coreと併用して全Observationが残る

## Phase 4: iwaSync3

実装:

- fixture metadata
- PlayerError rule
- unprefixed continuation negative test
- Engine integration test

完了条件:

- iwa errorを正確にemit
- source URLはgeneric coreと組み合わせて得られる
- URL一般検索をしない

## Phase 5: Compatibility verification

実装:

- VizVid実fixtureがある場合のみcore-only test
- README matrix
- unsupported/unverifiedを明確化

完了条件:

- fixtureなしのsupport claimなし
- empty Adapterなし

## Phase 6: Cleanup and release readiness

実装:

- gofmt/test/race/vet/build
- stale comments削除
- no network/dependency review
- fixture redaction review

完了条件:

- `go mod tidy`
- external dependencyは`vrclog-go`だけ
- root `All()`がYama→iwa順

---

# 19. 完了条件

- [ ] module pathが `github.com/vrclog/vrclog-adapters`
- [ ] `vrclog-go` と標準ライブラリ以外のdependencyがない
- [ ] runtime plugin/YAML/WASM機構がない
- [ ] `All()` がfresh community Adapter sliceを決定的順序で返す
- [ ] YamaPlayer Adapter IDが `community.yamaplayer`
- [ ] iwaSync3 Adapter IDが `community.iwasync3`
- [ ] 全Adapterがstateless/record-local/deterministic
- [ ] Decode内I/Oがない
- [ ] URL一般regexがない
- [ ] YamaPlayer original URL ruleがfixtureで検証されている
- [ ] YamaPlayer error ruleがfixtureで検証されている
- [ ] iwaSync3 PlayerError ruleがfixtureで検証されている
- [ ] iwaSync3 unprefixed detailを誤parseしない
- [ ] exact URLを変形せずEventへ保存する
- [ ] vendor-specific Event typeがない
- [ ] generic coreとのEngine integration testがある
- [ ] negative corpusがある
- [ ] fixture metadata/redactionがある
- [ ] VizVid等をfixtureなしで対応済みにしていない
- [ ] placeholder Adapter packageがない
- [ ] README compatibility matrixが事実だけを記載する
- [ ] `go test ./...` 成功
- [ ] `go test -race ./...` 成功
- [ ] `go vet ./...` 成功
- [ ] Windows amd64 build成功

---

# 20. 実装時にしてはならない妥協

- YAML RegexParserを「簡単だから」と再導入する
- AdapterをParserと呼ぶ
- first-match chainを作る
- Adapter descriptor/catalog/plugin loaderを先回りで作る
- upstream source codeにあるDebug.Log文字列だけでpositive supportを実装する
- fixture不足を緩いURL regexで補う
- unprefixed continuation lineを直前行のものと推測する
- Adapter内にmutable correlation stateを持つ
- `Data map[string]string`へ戻す
- provider/title/video IDを勝手に追加する
- generic coreが出す同じURLをcommunity Adapterでもemitする
- Empty VizVid/VideoTXL packageを置く
- READMEに「今後対応予定」のproject名を大量に並べる
- support version rangeを根拠なく断言する
- privacy-sensitive URLをfixtureへそのままcommitする

このリポジトリは「多機能な拡張基盤」ではなく、「実ログに裏付けられた小さなAdapterの検証済み集合」であることを維持する。
