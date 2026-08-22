// Package iwasync3 decodes iwaSync3 log lines from VRChat output_log into
// vrclog-go canonical Events.
package iwasync3

import (
	"fmt"
	"regexp"
	"strings"

	vrclog "github.com/vrclog/vrclog-go"

	"github.com/vrclog/vrclog-adapters/internal/errtext"
)

const prefixWithSpace = "[iwaSync3] "

var rePlayerError = regexp.MustCompile(`\bPlayerError\b`)

type adapter struct{}

// New returns a stateless Adapter for the iwaSync3 community project.
func New() vrclog.Adapter { return adapter{} }

func (a adapter) ID() vrclog.AdapterID { return "community.iwasync3" }

func (a adapter) Decode(record vrclog.Record) ([]vrclog.Emission, error) {
	if record.Time.IsZero() {
		return nil, nil
	}

	msg := record.Message
	if !strings.HasPrefix(msg, prefixWithSpace) {
		return nil, nil
	}
	if !rePlayerError.MatchString(msg) {
		return nil, nil
	}

	raw := strings.TrimRight(msg[len(prefixWithSpace):], " \t")

	parsed, err := errtext.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("iwasync3: player_error: %w", err)
	}

	return []vrclog.Emission{{
		Rule: "player_error",
		Event: vrclog.MediaErrorObserved{
			Stage:   vrclog.MediaStagePlayback,
			Code:    parsed.Code,
			Message: parsed.Message,
			Target: &vrclog.MediaTarget{
				Component: "iwasync3",
				Backend:   vrclog.MediaBackendUnknown,
			},
		},
	}}, nil
}
