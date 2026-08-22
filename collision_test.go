package vrclog_adapters_test

import (
	"testing"
	"time"

	vrclog "github.com/vrclog/vrclog-go"

	"github.com/vrclog/vrclog-adapters/iwasync3"
	"github.com/vrclog/vrclog-adapters/yamaplayer"
)

var fixedTime = time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

func makeRecord(msg string) vrclog.Record {
	return vrclog.Record{
		ID:      "test-record",
		Time:    fixedTime,
		Level:   vrclog.LevelLog,
		Message: msg,
		Raw:     msg,
	}
}

func communityAdapters() []vrclog.Adapter {
	return []vrclog.Adapter{
		yamaplayer.New(),
		iwasync3.New(),
	}
}

// TestNoCrossAdapterCollision verifies that a log line owned by one
// community project is not also claimed by another community adapter.
func TestNoCrossAdapterCollision(t *testing.T) {
	cases := map[string]struct {
		msg          string
		wantAdapter  vrclog.AdapterID
		wantEmission bool
	}{
		"yamaplayer_url": {
			msg:          "[YamaStream] Resolve youtube url: https://www.youtube.com/watch?v=TESTVIDEO01",
			wantAdapter:  "community.yamaplayer",
			wantEmission: true,
		},
		"yamaplayer_error": {
			msg:          "[YamaStream] [1] Video error: 500.",
			wantAdapter:  "community.yamaplayer",
			wantEmission: true,
		},
		"iwasync3_error": {
			msg:          "[iwaSync3] There was a `PlayerError` error in the video.",
			wantAdapter:  "community.iwasync3",
			wantEmission: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			record := makeRecord(tc.msg)

			for _, a := range communityAdapters() {
				emissions, err := a.Decode(record)
				if err != nil {
					t.Fatalf("%s: Decode() error = %v", a.ID(), err)
				}

				if a.ID() == tc.wantAdapter {
					if tc.wantEmission && len(emissions) == 0 {
						t.Errorf("%s: expected an emission, got none", a.ID())
					}
					continue
				}

				if len(emissions) != 0 {
					t.Errorf("%s: unexpectedly emitted for a line owned by %s: %+v", a.ID(), tc.wantAdapter, emissions)
				}
			}
		})
	}
}

// TestGenericCoreLineNoCommunityEmission verifies that lines belonging to
// vrclog-go's built-in vrchat.core adapter are not also claimed by any
// community adapter.
func TestGenericCoreLineNoCommunityEmission(t *testing.T) {
	cases := []string{
		"[Video Playback] Attempting to resolve URL 'https://relay.example.invalid/TESTVIDEO01'",
		"[Video Playback] URL 'https://relay.example.invalid/TESTVIDEO01' resolved to 'https://cdn.example.invalid/TESTVIDEO01.mp4'",
		"[AVProVideo] Opening https://relay.example.invalid/TESTVIDEO01",
	}

	for _, msg := range cases {
		record := makeRecord(msg)
		for _, a := range communityAdapters() {
			emissions, err := a.Decode(record)
			if err != nil {
				t.Fatalf("%s: Decode() error = %v", a.ID(), err)
			}
			if len(emissions) != 0 {
				t.Errorf("%s: unexpectedly emitted for a generic core line %q: %+v", a.ID(), msg, emissions)
			}
		}
	}
}

// TestNoAdapterIDCollision verifies that community adapters have distinct,
// stable Adapter IDs and that constructing an Engine from them succeeds.
func TestNoAdapterIDCollision(t *testing.T) {
	adapters := communityAdapters()
	seen := make(map[vrclog.AdapterID]bool)
	for _, a := range adapters {
		if seen[a.ID()] {
			t.Fatalf("duplicate Adapter ID: %s", a.ID())
		}
		seen[a.ID()] = true
	}

	if _, err := vrclog.NewEngine(adapters...); err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
}

// TestObservationIDStableAcrossRepeatedProcessing verifies that processing
// the same Record through the same Adapter twice produces an identical
// Observation ID, i.e. Decode is deterministic and ID generation is a pure
// function of its inputs.
func TestObservationIDStableAcrossRepeatedProcessing(t *testing.T) {
	record := makeRecord("[YamaStream] Resolve youtube url: https://www.youtube.com/watch?v=TESTVIDEO01")

	engine, err := vrclog.NewEngine(yamaplayer.New())
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	first := engine.Process(record)
	second := engine.Process(record)

	if len(first.Observations) != 1 || len(second.Observations) != 1 {
		t.Fatalf("expected 1 observation each run, got %d and %d", len(first.Observations), len(second.Observations))
	}
	if first.Observations[0].ID != second.Observations[0].ID {
		t.Errorf("Observation ID not stable: %q != %q", first.Observations[0].ID, second.Observations[0].ID)
	}
}

// reorderedEmissionAdapter is a minimal test-only Adapter that emits two
// rules for any non-zero-time Record, in an order controlled by the
// caller. It exists solely to prove Observation ID order-independence
// below; it is not one of this module's community adapters.
type reorderedEmissionAdapter struct {
	reversed bool
}

func (a reorderedEmissionAdapter) ID() vrclog.AdapterID { return "test.reordered" }

func (a reorderedEmissionAdapter) Decode(record vrclog.Record) ([]vrclog.Emission, error) {
	x := vrclog.Emission{Rule: "rule_x", Event: vrclog.PlayerJoined{Player: vrclog.Player{DisplayName: "x"}}}
	y := vrclog.Emission{Rule: "rule_y", Event: vrclog.PlayerJoined{Player: vrclog.Player{DisplayName: "y"}}}
	if a.reversed {
		return []vrclog.Emission{y, x}, nil
	}
	return []vrclog.Emission{x, y}, nil
}

// TestObservationIDIndependentOfEmissionOrder verifies that a given
// Adapter+Rule's Observation ID does not depend on the position of that
// emission within the slice Decode returns, matching the hardened
// vrclog-go contract (SHA-256 of record/adapter/rule IDs only, no emission
// index).
func TestObservationIDIndependentOfEmissionOrder(t *testing.T) {
	record := makeRecord("irrelevant for this synthetic adapter")

	forward, err := vrclog.NewEngine(reorderedEmissionAdapter{reversed: false})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	reversed, err := vrclog.NewEngine(reorderedEmissionAdapter{reversed: true})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	forwardResult := forward.Process(record)
	reversedResult := reversed.Process(record)

	if len(forwardResult.Observations) != 2 || len(reversedResult.Observations) != 2 {
		t.Fatalf("expected 2 observations each run, got %d and %d", len(forwardResult.Observations), len(reversedResult.Observations))
	}

	idByRule := func(result vrclog.Result) map[vrclog.RuleID]vrclog.ObservationID {
		m := make(map[vrclog.RuleID]vrclog.ObservationID)
		for _, obs := range result.Observations {
			m[obs.RuleID] = obs.ID
		}
		return m
	}

	forwardByRule := idByRule(forwardResult)
	reversedByRule := idByRule(reversedResult)

	for _, rule := range []vrclog.RuleID{"rule_x", "rule_y"} {
		if forwardByRule[rule] != reversedByRule[rule] {
			t.Errorf("rule %q: Observation ID depends on emission order: %q (forward) != %q (reversed)", rule, forwardByRule[rule], reversedByRule[rule])
		}
	}
}
