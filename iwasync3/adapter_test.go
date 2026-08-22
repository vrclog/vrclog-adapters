package iwasync3_test

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

	"github.com/vrclog/vrclog-adapters/iwasync3"
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
// no validation diagnostics.
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
	a := iwasync3.New()
	if a.ID() != "community.iwasync3" {
		t.Fatalf("ID() = %q, want %q", a.ID(), "community.iwasync3")
	}
}

func TestPlayerError(t *testing.T) {
	a := iwasync3.New()
	record := makeRecord(fixedTime, "[iwaSync3] There was a `PlayerError` error in the video.")

	obs := requireEngineObservation(t, a, record)
	if obs.RuleID != "player_error" {
		t.Errorf("RuleID = %q, want %q", obs.RuleID, "player_error")
	}
	ev, ok := obs.Event.(vrclog.MediaErrorObserved)
	if !ok {
		t.Fatalf("Event type = %T, want MediaErrorObserved", obs.Event)
	}
	if ev.Stage != vrclog.MediaStagePlayback {
		t.Errorf("Stage = %q, want playback", ev.Stage)
	}
	if ev.Code != "" {
		t.Errorf("Code = %q, want empty (no numeric code in this message)", ev.Code)
	}
	wantMessage := "There was a `PlayerError` error in the video."
	if ev.Message != wantMessage {
		t.Errorf("Message = %q, want %q", ev.Message, wantMessage)
	}
	if ev.Resource != nil {
		t.Errorf("Resource = %+v, want nil", ev.Resource)
	}
	if ev.Target == nil {
		t.Fatalf("Target is nil")
	}
	if ev.Target.Component != "iwasync3" {
		t.Errorf("Target.Component = %q, want iwasync3", ev.Target.Component)
	}
	if ev.Target.Backend != vrclog.MediaBackendUnknown {
		t.Errorf("Target.Backend = %q, want unknown", ev.Target.Backend)
	}
}

func TestPlayerErrorURLInMessageIsRedacted(t *testing.T) {
	a := iwasync3.New()
	record := makeRecord(fixedTime, "[iwaSync3] There was a `PlayerError` error. url: https://example.com/foo")

	obs := requireEngineObservation(t, a, record)
	ev := obs.Event.(vrclog.MediaErrorObserved)
	if ev.Message != "There was a `PlayerError` error. url: <url>" {
		t.Errorf("Message = %q, want %q", ev.Message, "There was a `PlayerError` error. url: <url>")
	}
	for _, forbidden := range []string{"http://", "https://"} {
		if strings.Contains(ev.Message, forbidden) {
			t.Errorf("Message %q leaks URL scheme %q", ev.Message, forbidden)
		}
	}
}

func TestPlayerErrorOversizedMessageTruncated(t *testing.T) {
	a := iwasync3.New()
	longMessage := "PlayerError: " + strings.Repeat("a", 3000)
	record := makeRecord(fixedTime, "[iwaSync3] "+longMessage)

	obs := requireEngineObservation(t, a, record)
	ev := obs.Event.(vrclog.MediaErrorObserved)
	if len(ev.Message) != 2048 {
		t.Errorf("len(Message) = %d, want 2048", len(ev.Message))
	}
}

func TestRepeatedDecodeIsDeterministic(t *testing.T) {
	a := iwasync3.New()
	record := makeRecord(fixedTime, "[iwaSync3] There was a `PlayerError` error in the video.")

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
		"unprefixed_continuation_line": "-736347996: url: https://www.youtube.com/watch?v=TESTVIDEO01",
		"bare_url":                     "https://www.youtube.com/watch?v=TESTVIDEO01",
		"other_component_player_error": "[OtherComponent] PlayerError detected",
		"prefix_without_player_error":  "[iwaSync3] Playback started: https://www.youtube.com/watch?v=TESTVIDEO01",
		"no_space_after_prefix":        "[iwaSync3]PlayerError occurred",
		"empty_message":                "",
		"embedded_tag_other_component": "[Behaviour] Udon Debug.Log: [iwaSync3] There was a `PlayerError` error in the video.",
		"not_player_error_substring":   "[iwaSync3] NotPlayerError occurred",
		"player_error_count_substring": "[iwaSync3] PlayerErrorCount incremented",
		"embedded_in_longer_word":      "[iwaSync3] handlePlayerErrorCallback started",
	}

	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			a := iwasync3.New()
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
	a := iwasync3.New()
	record := makeRecord(time.Time{}, "[iwaSync3] There was a `PlayerError` error in the video.")

	emissions, err := a.Decode(record)
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if emissions != nil {
		t.Fatalf("Decode() emissions = %+v, want nil for zero-time record", emissions)
	}
}

func TestConcurrentDecode(t *testing.T) {
	a := iwasync3.New()
	record := makeRecord(fixedTime, "[iwaSync3] There was a `PlayerError` error in the video.")

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
//	go test -count=1 ./iwasync3/ -update
func TestFixtureGolden(t *testing.T) {
	scenarios := []string{"player_error", "player_error_with_continuation"}

	for _, scenario := range scenarios {
		t.Run(scenario, func(t *testing.T) {
			dir := filepath.Join("testdata", scenario)
			inputPath := filepath.Join(dir, "input.log")
			expectedPath := filepath.Join(dir, "expected.json")

			engine, err := vrclog.NewEngine(iwasync3.New())
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
