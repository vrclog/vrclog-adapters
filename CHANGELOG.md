# Changelog

## Unreleased

### Breaking

- Removed the root `All()` aggregate API. Companion (and any consumer) must explicitly import and construct each adapter: `yamaplayer.New()`, `iwasync3.New()`. This prevents new community adapters from being implicitly activated by a dependency bump.
- Updated the `vrclog-go` dependency to the hardened contract (`v0.0.0-20260821143906-536e1d2ffda3`). Observation IDs are recomputed (the emission index was dropped from the ID formula); all golden fixtures were regenerated.

### Fixed

- YamaPlayer `video_error`: human-readable error text is no longer copied into the machine-readable `Code` field. Only a leading numeric code (e.g. `500`, `-736347996`) is extracted; everything else goes to `Message` only.
- YamaPlayer `youtube_resolve_url`: `MediaTarget.Backend` is now explicitly set to `unknown`, satisfying the hardened upstream's required-backend validation.
- iwaSync3 `player_error`: HTTP(S) URLs embedded in error text are now redacted to `<url>` before being placed in `Message`, instead of being preserved verbatim.
- iwaSync3 `player_error`: matching narrowed from `strings.Contains("PlayerError")` to a word-boundary match (`\bPlayerError\b`), rejecting substrings such as `NotPlayerError` or `handlePlayerErrorCallback`.
- **[Security]** `internal/errtext`: fixed a redaction bypass where a whitespace-classified control character (tab, CR, LF, FF) embedded mid-URL split the URL-redaction match, letting a trailing query string (e.g. a signed token) survive control-character normalization and leak into `Message`. The URL pattern now stops only at a literal ASCII space or `<`/`>`, so any other byte is treated as possibly part of the URL and redacted along with it.
- `internal/errtext`: Unicode bidi formatting characters (U+202A-202E, U+2066-2069) are now normalized to spaces alongside control characters, matching upstream `vrclog-go` validation and preventing an otherwise-valid rule match from being silently dropped as an invalid canonical event.
- YamaPlayer `youtube_resolve_url`: dispatch now requires `Resolve youtube url: ` to appear immediately after the `[YamaStream] ` prefix (exact position), rather than anywhere in the message, preventing a `video_error` line whose text happens to contain that phrase from being misclassified as a source URL observation.
- YamaPlayer `video_error`: the regex-captured target key is now validated (≤ 256 bytes, no control/bidi characters) before use; an invalid key now produces an explicit decode error instead of an event the Engine would silently drop.
- YamaPlayer `youtube_resolve_url`: the local URL validator now also rejects oversized URLs (> 16 KiB), bidi-formatted URLs, and URLs with percent-encoded control characters in the path/query/fragment, mirroring upstream `vrclog-go`'s `RemoteResource` validation so the adapter no longer emits events the Engine would silently drop as invalid.

### Added

- `internal/errtext`: shared, privacy-preserving error text normalization used by both adapters — URL redaction (redact-first, before control-character normalization, so a control character embedded inside a URL cannot split it and leak a fragment), invalid UTF-8 repair, control-character-to-space normalization, whitespace collapsing, leading-numeric Code extraction, and UTF-8-safe truncation (Code ≤ 128 bytes, Message ≤ 2048 bytes).
- `yamaplayer` package: `community.yamaplayer` adapter.
  - `youtube_resolve_url` rule — observes the original YouTube source URL from `[YamaStream] Resolve youtube url: ...`.
  - `video_error` rule — observes player-specific video errors from `[YamaStream] [<key>] Video error: ...`.
- `iwasync3` package: `community.iwasync3` adapter.
  - `player_error` rule — observes PlayerError messages from `[iwaSync3] ...PlayerError...`.
- Fixture-based test infrastructure (`testdata/<scenario>/{input.log,expected.json,metadata.json}`) with golden-file generation via `vrclog.EncodeObservationJSON`.
- Negative corpus and cross-adapter collision tests, including explicit Adapter ID collision and Observation ID order-independence checks.
- `vrclog.NewVRChatAdapter()` + community adapter Engine integration tests.
- `requireEngineObservation` test helper in each adapter package: every positive unit test now decodes through a fresh `vrclog.Engine` and asserts zero validation diagnostics, rather than only inspecting the raw `Emission` struct.
- Expanded `compatibility/README.md` status vocabulary (`fixture_verified` / `field_tested` / `version_verified` / `unverified`) replacing the previous binary `verified`/`unverified`.
- `.github/workflows/ci.yml`: gofmt, `go vet`, `go test` (Ubuntu + Windows), race detector and coverage (Ubuntu), `go.work`/`replace`-directive rejection (whitespace-tolerant), and `go mod tidy` cleanliness, all with least-privilege `permissions: contents: read`.
