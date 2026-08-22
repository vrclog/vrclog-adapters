package yamaplayer_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	vrclog "github.com/vrclog/vrclog-go"

	"github.com/vrclog/vrclog-adapters/yamaplayer"
)

var update = flag.Bool("update", false, "update golden fixture files")

func makeRecord(t time.Time, msg string) vrclog.Record {
	return vrclog.Record{
		ID:      "test-record",
		Time:    t,
		Level:   vrclog.LevelLog,
		Message: msg,
		Raw:     msg,
	}
}

var fixedTime = time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

// requireEngineObservation decodes record through a fresh Engine wrapping
// adapter a and asserts it produces exactly one canonical Observation with
// no validation diagnostics. This exercises the same upstream
// MediaErrorObserved/ResourceURLObserved validation that Companion relies
// on, rather than only checking the Emission struct in isolation.
func requireEngineObservation(t *testing.T, a vrclog.Adapter, record vrclog.Record) vrclog.Observation {
	t.Helper()
	engine, err := vrclog.NewEngine(a)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	result := engine.Process(record)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	if len(result.Observations) != 1 {
		t.Fatalf("len(Observations) = %d, want 1", len(result.Observations))
	}
	return result.Observations[0]
}

func TestID(t *testing.T) {
	a := yamaplayer.New()
	if a.ID() != "community.yamaplayer" {
		t.Fatalf("ID() = %q, want %q", a.ID(), "community.yamaplayer")
	}
}

func TestYoutubeResolveURL(t *testing.T) {
	a := yamaplayer.New()
	record := makeRecord(fixedTime, "[YamaStream] Resolve youtube url: https://www.youtube.com/watch?v=TESTVIDEO01")

	emissions, err := a.Decode(record)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(emissions) != 1 {
		t.Fatalf("len(emissions) = %d, want 1", len(emissions))
	}

	em := emissions[0]
	if em.Rule != "youtube_resolve_url" {
		t.Errorf("Rule = %q, want %q", em.Rule, "youtube_resolve_url")
	}
	ev, ok := em.Event.(vrclog.ResourceURLObserved)
	if !ok {
		t.Fatalf("Event type = %T, want ResourceURLObserved", em.Event)
	}
	if ev.Resource.URL != "https://www.youtube.com/watch?v=TESTVIDEO01" {
		t.Errorf("Resource.URL = %q, want exact preserved URL", ev.Resource.URL)
	}
	if ev.Resource.Kind != vrclog.ResourceKindVideo {
		t.Errorf("Resource.Kind = %q, want video", ev.Resource.Kind)
	}
	if ev.Resource.Role != vrclog.ResourceRoleSource {
		t.Errorf("Resource.Role = %q, want source", ev.Resource.Role)
	}
	if ev.Target == nil || ev.Target.Component != "yamaplayer" {
		t.Errorf("Target.Component = %+v, want yamaplayer", ev.Target)
	}
	if ev.Target != nil && ev.Target.Backend != vrclog.MediaBackendUnknown {
		t.Errorf("Target.Backend = %q, want unknown", ev.Target.Backend)
	}
}

func TestYoutubeResolveURLWithQueryAndFragment(t *testing.T) {
	a := yamaplayer.New()
	url := "https://www.youtube.com/watch?v=TESTVIDEO01&t=30s#frag"
	record := makeRecord(fixedTime, "[YamaStream] Resolve youtube url: "+url)

	emissions, err := a.Decode(record)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(emissions) != 1 {
		t.Fatalf("len(emissions) = %d, want 1", len(emissions))
	}
	ev := emissions[0].Event.(vrclog.ResourceURLObserved)
	if ev.Resource.URL != url {
		t.Errorf("Resource.URL = %q, want %q (exact preservation)", ev.Resource.URL, url)
	}
}

func TestVideoError(t *testing.T) {
	a := yamaplayer.New()
	record := makeRecord(fixedTime, "[YamaStream] [1] Video error: 500.")

	obs := requireEngineObservation(t, a, record)
	if obs.RuleID != "video_error" {
		t.Errorf("RuleID = %q, want %q", obs.RuleID, "video_error")
	}
	ev, ok := obs.Event.(vrclog.MediaErrorObserved)
	if !ok {
		t.Fatalf("Event type = %T, want MediaErrorObserved", obs.Event)
	}
	if ev.Stage != vrclog.MediaStagePlayback {
		t.Errorf("Stage = %q, want playback", ev.Stage)
	}
	if ev.Code != "500" {
		t.Errorf("Code = %q, want %q", ev.Code, "500")
	}
	if ev.Message != "" {
		t.Errorf("Message = %q, want empty", ev.Message)
	}
	if ev.Target == nil {
		t.Fatalf("Target is nil")
	}
	if ev.Target.Component != "yamaplayer" {
		t.Errorf("Target.Component = %q, want yamaplayer", ev.Target.Component)
	}
	if ev.Target.Key != "1" {
		t.Errorf("Target.Key = %q, want %q", ev.Target.Key, "1")
	}
	if ev.Target.Backend != vrclog.MediaBackendUnknown {
		t.Errorf("Target.Backend = %q, want unknown", ev.Target.Backend)
	}
}

func TestVideoErrorDifferentIndex(t *testing.T) {
	for _, key := range []string{"0", "2", "player1"} {
		t.Run(key, func(t *testing.T) {
			a := yamaplayer.New()
			record := makeRecord(fixedTime, "[YamaStream] ["+key+"] Video error: Connection timeout")

			obs := requireEngineObservation(t, a, record)
			ev := obs.Event.(vrclog.MediaErrorObserved)
			if ev.Target.Key != key {
				t.Errorf("Target.Key = %q, want %q", ev.Target.Key, key)
			}
			if ev.Code != "" {
				t.Errorf("Code = %q, want empty (human text is not a machine code)", ev.Code)
			}
			if ev.Message != "Connection timeout" {
				t.Errorf("Message = %q, want %q", ev.Message, "Connection timeout")
			}
		})
	}
}

func TestVideoErrorURLInMessageIsRedacted(t *testing.T) {
	a := yamaplayer.New()
	record := makeRecord(fixedTime, "[YamaStream] [1] Video error: failed for https://example.com/video")

	obs := requireEngineObservation(t, a, record)
	ev := obs.Event.(vrclog.MediaErrorObserved)
	if ev.Message != "failed for <url>" {
		t.Errorf("Message = %q, want %q", ev.Message, "failed for <url>")
	}
	for _, forbidden := range []string{"http://", "https://"} {
		if strings.Contains(ev.Message, forbidden) {
			t.Errorf("Message %q leaks URL scheme %q", ev.Message, forbidden)
		}
	}
}

func TestVideoErrorControlCharsNormalized(t *testing.T) {
	// A Record.Message is already single-line by the time an adapter sees
	// it, so this exercises a control character (tab) that can plausibly
	// appear mid-line, rather than \r\n which would never reach Decode as
	// part of one Record.
	a := yamaplayer.New()
	record := makeRecord(fixedTime, "[YamaStream] [1] Video error: \tfoo")

	obs := requireEngineObservation(t, a, record)
	ev := obs.Event.(vrclog.MediaErrorObserved)
	if ev.Code != "" {
		t.Errorf("Code = %q, want empty", ev.Code)
	}
	if ev.Message != "foo" {
		t.Errorf("Message = %q, want %q", ev.Message, "foo")
	}
}

func TestVideoErrorOversizedMessageTruncated(t *testing.T) {
	a := yamaplayer.New()
	longMessage := "Connection timeout: " + strings.Repeat("a", 3000)
	record := makeRecord(fixedTime, "[YamaStream] [1] Video error: "+longMessage)

	obs := requireEngineObservation(t, a, record)
	ev := obs.Event.(vrclog.MediaErrorObserved)
	if len(ev.Message) != 2048 {
		t.Errorf("len(Message) = %d, want 2048", len(ev.Message))
	}
}

// TestResolveAnchorMustBeAtExpectedPosition verifies that
// youtube_resolve_url only fires when "Resolve youtube url: " appears
// immediately after the "[YamaStream] " prefix, per
// CLAUDE_HARDENING_SPEC.md 5.4 ("期待位置に存在すること"). A video_error
// line whose human-readable message happens to contain that anchor text
// must not be misclassified as a source URL observation.
func TestResolveAnchorMustBeAtExpectedPosition(t *testing.T) {
	a := yamaplayer.New()
	record := makeRecord(fixedTime, "[YamaStream] [1] Video error: Resolve youtube url: https://evil.example/x")

	obs := requireEngineObservation(t, a, record)
	if obs.RuleID != "video_error" {
		t.Errorf("RuleID = %q, want %q (must not be misclassified as youtube_resolve_url)", obs.RuleID, "video_error")
	}
	ev, ok := obs.Event.(vrclog.MediaErrorObserved)
	if !ok {
		t.Fatalf("Event type = %T, want MediaErrorObserved", obs.Event)
	}
	if strings.Contains(ev.Message, "https://") {
		t.Errorf("Message %q leaks URL scheme", ev.Message)
	}
}

func TestResolveAnchorNotAtStartIsNoMatch(t *testing.T) {
	a := yamaplayer.New()
	record := makeRecord(fixedTime, "[YamaStream] extra text Resolve youtube url: https://www.youtube.com/watch?v=TESTVIDEO01")

	emissions, err := a.Decode(record)
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if emissions != nil {
		t.Fatalf("Decode() emissions = %+v, want nil", emissions)
	}
}

func TestRepeatedDecodeIsDeterministic(t *testing.T) {
	a := yamaplayer.New()
	record := makeRecord(fixedTime, "[YamaStream] Resolve youtube url: https://www.youtube.com/watch?v=TESTVIDEO01")

	first, err1 := a.Decode(record)
	second, err2 := a.Decode(record)
	if err1 != nil || err2 != nil {
		t.Fatalf("Decode() errors = %v, %v", err1, err2)
	}

	b1, _ := json.Marshal(first)
	b2, _ := json.Marshal(second)
	if string(b1) != string(b2) {
		t.Errorf("repeated Decode produced different results:\n%s\n%s", b1, b2)
	}
}

func TestNoMatch(t *testing.T) {
	cases := map[string]string{
		"different_component_prefix":      "[iwaSync3] Resolve youtube url: https://www.youtube.com/watch?v=TESTVIDEO01",
		"prefix_without_rule_anchor":      "[YamaStream] Play track: something",
		"bare_url":                        "https://www.youtube.com/watch?v=TESTVIDEO01",
		"empty_message":                   "",
		"quoted_prefix_other_component":   "[Behaviour] Udon Debug.Log: saw string \"[YamaStream]\" in chat",
		"embedded_anchor_other_component": "[Behaviour] Udon Debug.Log: [YamaStream] Resolve youtube url: https://evil.example/malicious",
		"embedded_error_other_component":  "[Behaviour] Udon Debug.Log: [YamaStream] [1] Video error: 500.",
		"ftp_url_after_anchor":            "[YamaStream] Resolve youtube url: ftp://example.invalid/file",
		"userinfo_url_after_anchor":       "[YamaStream] Resolve youtube url: http://user:pass@example.invalid/file",
		"not_a_url_after_anchor":          "[YamaStream] Resolve youtube url: not-a-url",
		"scheme_only_after_anchor":        "[YamaStream] Resolve youtube url: https://",
		"oversized_url_after_anchor":      "[YamaStream] Resolve youtube url: https://example.invalid/" + strings.Repeat("a", 16*1024),
		"bidi_char_in_url":                "[YamaStream] Resolve youtube url: https://example.invalid/‮video",
		"percent_encoded_control_in_url":  "[YamaStream] Resolve youtube url: https://example.invalid/watch?v=%0aSECRET",
	}

	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			a := yamaplayer.New()
			record := makeRecord(fixedTime, msg)
			emissions, err := a.Decode(record)
			if err != nil {
				t.Fatalf("Decode() error = %v, want nil", err)
			}
			if emissions != nil {
				t.Fatalf("Decode() emissions = %+v, want nil", emissions)
			}
		})
	}
}

func TestZeroTimeRecord(t *testing.T) {
	a := yamaplayer.New()
	record := makeRecord(time.Time{}, "[YamaStream] Resolve youtube url: https://www.youtube.com/watch?v=TESTVIDEO01")

	emissions, err := a.Decode(record)
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if emissions != nil {
		t.Fatalf("Decode() emissions = %+v, want nil for zero-time record", emissions)
	}
}

func TestMissingURLIsError(t *testing.T) {
	cases := map[string]string{
		"empty_after_anchor":     "[YamaStream] Resolve youtube url: ",
		"whitespace_only_anchor": "[YamaStream] Resolve youtube url:   ",
	}
	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			a := yamaplayer.New()
			record := makeRecord(fixedTime, msg)
			emissions, err := a.Decode(record)
			if err == nil {
				t.Fatalf("Decode() error = nil, want error")
			}
			if emissions != nil {
				t.Fatalf("Decode() emissions = %+v, want nil alongside error", emissions)
			}
		})
	}
}

func TestMalformedVideoErrorIsError(t *testing.T) {
	cases := map[string]string{
		"empty_key":                 "[YamaStream] [] Video error: 500",
		"space_in_key":              "[YamaStream] [1 2] Video error: 500",
		"key_exceeds_byte_limit":    "[YamaStream] [" + strings.Repeat("a", 300) + "] Video error: 500",
		"key_contains_control_char": "[YamaStream] [\x01] Video error: 500",
	}
	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			a := yamaplayer.New()
			record := makeRecord(fixedTime, msg)

			emissions, err := a.Decode(record)
			if err == nil {
				t.Fatalf("Decode() error = nil, want error")
			}
			if emissions != nil {
				t.Fatalf("Decode() emissions = %+v, want nil alongside error", emissions)
			}
		})
	}
}

func TestErrorMessageDoesNotLeakURL(t *testing.T) {
	a := yamaplayer.New()
	record := makeRecord(fixedTime, "[YamaStream] Resolve youtube url: ")
	_, err := a.Decode(record)
	if err == nil {
		t.Fatalf("Decode() error = nil, want error")
	}
	msg := err.Error()
	if len(msg) == 0 {
		t.Fatalf("error message is empty")
	}
	for _, forbidden := range []string{"http://", "https://"} {
		if strings.Contains(msg, forbidden) {
			t.Errorf("error message %q leaks URL scheme %q", msg, forbidden)
		}
	}
}

func TestConcurrentDecode(t *testing.T) {
	a := yamaplayer.New()
	record := makeRecord(fixedTime, "[YamaStream] Resolve youtube url: https://www.youtube.com/watch?v=TESTVIDEO01")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := a.Decode(record); err != nil {
				t.Errorf("Decode() error = %v", err)
			}
		}()
	}
	wg.Wait()
}

// normalizeGoldenObservation zeroes/replaces fields that are inherently
// machine-dependent so golden fixtures are portable across developer
// machines and CI runners: Observation.ID, Record.ID, and Record.SourceID
// are SHA-256 hashes of the fixture file's absolute path, and Time
// reflects vrclog-go's time.Local-based parsing of a timezone-naive
// VRChat timestamp (a different absolute instant on every machine). This
// does not touch vrclog-go's parsing/hashing behavior -- only what this
// repo's golden comparison treats as significant. Working with the typed
// Observation/RecordRef struct (rather than rewriting the JSON tree)
// means a future field that happens to be named "id" elsewhere in an
// Event payload can never be clobbered by this normalization.
func normalizeGoldenObservation(obs vrclog.Observation) vrclog.Observation {
	obs.ID = "_"
	obs.Time = time.Time{}
	obs.Record.ID = "_"
	obs.Record.SourceID = "_"
	return obs
}

// TestFixtureGolden verifies the adapter's output against golden files
// generated from real VRChat log fixtures using vrclog.ReadFile +
// vrclog.EncodeObservationJSON. Machine-dependent fields (see
// normalizeGoldenObservation) are normalized before comparison, so no
// special TZ handling is needed. Run with -update to regenerate:
//
//	go test -count=1 ./yamaplayer/ -update
func TestFixtureGolden(t *testing.T) {
	scenarios := []string{"youtube_source_url", "video_error"}

	for _, scenario := range scenarios {
		t.Run(scenario, func(t *testing.T) {
			dir := filepath.Join("testdata", scenario)
			inputPath := filepath.Join(dir, "input.log")
			expectedPath := filepath.Join(dir, "expected.json")

			engine, err := vrclog.NewEngine(yamaplayer.New())
			if err != nil {
				t.Fatalf("NewEngine() error = %v", err)
			}

			var observations []vrclog.Observation
			for record, err := range vrclog.ReadFile(context.Background(), vrclog.ReadFileConfig{Path: inputPath}) {
				if err != nil {
					t.Fatalf("ReadFile() error = %v", err)
				}
				result := engine.Process(record)
				if len(result.Diagnostics) != 0 {
					t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
				}
				observations = append(observations, result.Observations...)
			}

			var encoded []json.RawMessage
			for _, obs := range observations {
				b, err := vrclog.EncodeObservationJSON(normalizeGoldenObservation(obs))
				if err != nil {
					t.Fatalf("EncodeObservationJSON() error = %v", err)
				}
				encoded = append(encoded, b)
			}
			got, err := json.MarshalIndent(encoded, "", "  ")
			if err != nil {
				t.Fatalf("MarshalIndent() error = %v", err)
			}
			got = append(got, '\n')

			if *update {
				if err := os.WriteFile(expectedPath, got, 0o644); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
				return
			}

			want, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("ReadFile(expected) error = %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("golden mismatch for %s\ngot:\n%s\nwant:\n%s", scenario, got, want)
			}
		})
	}
}
