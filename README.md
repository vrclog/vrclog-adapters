# vrclog-adapters

`vrclog-adapters` は、YamaPlayer、iwaSync3等、VRChat上で利用されるコミュニティ製ワールドアセット固有のログ形式を解釈し、[`vrclog-go`](https://github.com/vrclog/vrclog-go) のcanonical Eventを返すAdapterを提供するcompile-timeライブラリである。

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
```

## 責務分離

このリポジトリは、VRChatクライアント自体のログ（built-in Adapter）ではなく、コミュニティ製ワールドアセットが独自に出力するログ形式だけを扱う。ログのread/follow、cursor管理、Adapter実行順序、Observation ID生成は `vrclog-go` の責務であり、本リポジトリでは扱わない。

## 使用例

Adapterは個別にimportし、明示的に組み込む。暗黙的に有効化される aggregate API は提供しない。

```go
import (
    vrclog "github.com/vrclog/vrclog-go"

    "github.com/vrclog/vrclog-adapters/iwasync3"
    "github.com/vrclog/vrclog-adapters/yamaplayer"
)

engine, err := vrclog.NewEngine(
    vrclog.NewVRChatAdapter(),
    yamaplayer.New(),
    iwasync3.New(),
)
```

## Compatibility matrix

| Project | Adapter ID | Observed canonical event | Verification level |
|---|---|---|---|
| YamaPlayer | `community.yamaplayer` | `resource.url_observed` (source URL), `media.error_observed` (video error) | fixture_verified |
| iwaSync3 | `community.iwasync3` | `media.error_observed` (PlayerError) | fixture_verified |
| VizVid | none (core-only) | not evaluated | unverified |

`fixture_verified` は、実ログ由来fixtureが `vrclog.NewEngine` を通して少なくとも1 scenario 通過したことのみを示す。特定projectの全version、全機能を保証するものではない。詳細と既知の制限は [`compatibility/README.md`](compatibility/README.md) を参照。

## Non-goals

- ログファイルのread/follow/rotation
- timestamp/header decode
- cursor管理
- Observation ID生成
- Adapterの実行順序管理
- runtime Adapter enable/disable、remote catalog、auto-update、plugin機構

## Privacy / redaction

Adapterが観測するURLには、unlisted content、private relay、signed token等が含まれ得る。Adapterはこれらを外部送信せず、error messageにURLを再出力しない。テストfixtureは実ログ由来だが、機密情報はredactionしたうえでcommitする。
