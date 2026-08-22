// Package yamaplayer decodes YamaPlayer log lines from VRChat output_log
// into vrclog-go canonical Events.
package yamaplayer

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	vrclog "github.com/vrclog/vrclog-go"

	"github.com/vrclog/vrclog-adapters/internal/errtext"
)

const (
	prefix        = "[YamaStream]"
	resolveAnchor = "Resolve youtube url: "
	resolvePrefix = prefix + " " + resolveAnchor
	errorAnchor   = "Video error:"

	// maxTargetKeyBytes mirrors vrclog-go's MediaTarget.Key length limit.
	// Checked here so a malformed key produces an explicit decode error
	// instead of an event the Engine would silently drop as invalid.
	maxTargetKeyBytes = 256

	// maxURLBytes mirrors vrclog-go's RemoteResource.URL length limit.
	// Checked here so an oversized URL is treated as a non-match rather
	// than an event the Engine would silently drop as invalid.
	maxURLBytes = 16 * 1024
)

var reVideoError = regexp.MustCompile(`^\[YamaStream\] \[(\S+)\] Video error: (.+)$`)

type adapter struct{}

// New returns a stateless Adapter for the YamaPlayer community project.
func New() vrclog.Adapter { return adapter{} }

func (a adapter) ID() vrclog.AdapterID { return "community.yamaplayer" }

func (a adapter) Decode(record vrclog.Record) ([]vrclog.Emission, error) {
	if record.Time.IsZero() {
		return nil, nil
	}

	msg := record.Message
	if !strings.HasPrefix(msg, prefix) {
		return nil, nil
	}

	if strings.HasPrefix(msg, resolvePrefix) {
		return decodeYoutubeResolveURL(msg)
	}

	if strings.Contains(msg, errorAnchor) {
		return decodeVideoError(msg)
	}

	return nil, nil
}

func decodeYoutubeResolveURL(msg string) ([]vrclog.Emission, error) {
	rest := strings.TrimRight(msg[len(resolvePrefix):], " \t")

	if rest == "" {
		return nil, fmt.Errorf("yamaplayer: youtube_resolve_url: missing URL after anchor")
	}

	if !isHTTPURL(rest) {
		return nil, nil
	}

	return []vrclog.Emission{{
		Rule: "youtube_resolve_url",
		Event: vrclog.ResourceURLObserved{
			Resource: vrclog.RemoteResource{
				URL:  rest,
				Kind: vrclog.ResourceKindVideo,
				Role: vrclog.ResourceRoleSource,
			},
			Target: &vrclog.MediaTarget{
				Component: "yamaplayer",
				Backend:   vrclog.MediaBackendUnknown,
			},
		},
	}}, nil
}

func decodeVideoError(msg string) ([]vrclog.Emission, error) {
	m := reVideoError.FindStringSubmatch(msg)
	if m == nil {
		return nil, fmt.Errorf("yamaplayer: video_error: malformed error line")
	}

	key := m[1]
	if len(key) > maxTargetKeyBytes || errtext.ContainsUnsafeControlOrBidi(key) {
		return nil, fmt.Errorf("yamaplayer: video_error: invalid target key")
	}

	raw := m[2]

	parsed, err := errtext.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("yamaplayer: video_error: %w", err)
	}

	return []vrclog.Emission{{
		Rule: "video_error",
		Event: vrclog.MediaErrorObserved{
			Stage:   vrclog.MediaStagePlayback,
			Code:    parsed.Code,
			Message: parsed.Message,
			Target: &vrclog.MediaTarget{
				Component: "yamaplayer",
				Key:       key,
				Backend:   vrclog.MediaBackendUnknown,
			},
		},
	}}, nil
}

func isHTTPURL(rawURL string) bool {
	if len(rawURL) > maxURLBytes {
		return false
	}
	if strings.ContainsFunc(rawURL, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r)
	}) || errtext.ContainsUnsafeControlOrBidi(rawURL) {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.User != nil {
		return false
	}
	if u.Host == "" {
		return false
	}
	// The checks above only inspect the raw (percent-encoded) string, so a
	// control character can still slip through if it was percent-encoded
	// (e.g. %0d%0a). Re-check the decoded path, query, and fragment, as
	// vrclog-go's own RemoteResource validation does.
	if strings.ContainsFunc(u.Path, unicode.IsControl) {
		return false
	}
	decodedQuery, err := url.QueryUnescape(u.RawQuery)
	if err != nil || strings.ContainsFunc(decodedQuery, unicode.IsControl) {
		return false
	}
	if strings.ContainsFunc(u.Fragment, unicode.IsControl) {
		return false
	}
	return true
}
