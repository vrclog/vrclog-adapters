# Compatibility matrix

| Project | Adapter ID | Observed canonical event | Upstream version/commit | Captured date | Fixture source | Verification level | Known limitations |
|---|---|---|---|---|---|---|---|
| YamaPlayer | `community.yamaplayer` | `resource.url_observed` (original YouTube source URL, `youtube_resolve_url`) | unknown | 2026-08-18 | public_issue | fixture_verified | Version unknown; non-HTTP(S) URLs are treated as non-match, not decode errors |
| YamaPlayer | `community.yamaplayer` | `media.error_observed` (player-specific video error, `video_error`) | unknown | 2026-08-18 | public_issue | fixture_verified | Only leading numeric error codes are extracted as `Code`; human-readable-only errors carry an empty `Code` |
| iwaSync3 | `community.iwasync3` | `media.error_observed` (`player_error`) | unknown | 2026-08-18 | public_issue | fixture_verified | Source URL is observed via `vrchat.core`, not this adapter; continuation lines without the `[iwaSync3] ` prefix are not parsed |
| VizVid | none (core-only) | not evaluated — no real log fixture available yet | n/a | n/a | no fixture | unverified | No support claim; `vrchat.core` alone has not been confirmed insufficient |

## Status vocabulary

- `fixture_verified`: at least one real-log-derived fixture exercises the rule end-to-end through `vrclog.NewEngine`.
- `field_tested`: verified against a live VRChat environment by a developer.
- `version_verified`: multiple scenarios confirmed against a specific upstream project version/commit.
- `unverified`: no fixture exists; no support claim is made.

No claim of "supports the entire project" or "works on all versions" is made anywhere in this table. Each row describes only what a specific fixture scenario has demonstrated.
