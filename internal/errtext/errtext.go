// Package errtext provides shared, privacy-preserving normalization and
// Code/Message extraction for community adapter error text. It is used by
// adapters after they have already matched a project-specific prefix and
// extracted the error text to decode; it does not scan arbitrary log
// content for URLs.
package errtext

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxCodeBytes    = 128
	maxMessageBytes = 2048
)

var (
	// urlPattern stops a match only at a literal ASCII space or angle
	// bracket, not at every \s-class character. A raw URL must not contain
	// an un-encoded space, so a space reliably marks a real word boundary
	// (e.g. between two URLs in prose). Any other byte -- including tab,
	// CR, LF, FF, and non-whitespace control characters -- is treated as
	// possibly part of the URL and swallowed into the redacted span. This
	// is required for redact-first to actually prevent leaks: tab/CR/LF/FF
	// are members of regexp's \s class, so a [^\s<>]+ pattern would stop
	// matching at them and let an attacker smuggle a trailing query
	// string (e.g. a signed token) past redaction by embedding one of
	// them mid-URL instead of a real space.
	urlPattern  = regexp.MustCompile(`(?i)https?://[^ <>]+`)
	codePattern = regexp.MustCompile(`^-?\d+`)
)

// isBidiFormat reports whether r is a Unicode bidirectional formatting
// character (category Cf): the embedding/override controls U+202A-202E
// and the isolate controls U+2066-2069. unicode.IsControl does not treat
// these as control characters, but they can visually spoof displayed
// text, and vrclog-go's canonical event validation rejects them in Code,
// Message, and MediaTarget fields.
func isBidiFormat(r rune) bool {
	// Escaped rune literals are used instead of the literal bidi
	// characters themselves: embedding bidi/invisible formatting
	// characters directly in source text is a Trojan-Source-style
	// source-audit hazard (they can render misleadingly in editors and
	// trip security scanners), independent of this function's own
	// correctness.
	return (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069')
}

// ContainsUnsafeControlOrBidi reports whether s contains a control
// character or a bidi formatting character. It matches the definition
// vrclog-go's canonical event validation uses to reject Code, Message,
// and MediaTarget.Component/Key, so adapters can check target fields
// they build themselves (e.g. a regex-captured key) before emitting an
// event upstream would otherwise silently drop as invalid.
func ContainsUnsafeControlOrBidi(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || isBidiFormat(r)
	})
}

// Parsed holds the machine-readable Code and human-readable Message
// extracted from a raw error text fragment.
type Parsed struct {
	Code    string
	Message string
}

// Parse normalizes raw error text for use in vrclog-go canonical events.
//
// The pipeline runs, in order: HTTP(S) URL redaction (applied first, and
// stopping only at a literal space or angle bracket, so that any control
// character embedded inside a URL -- including \s-class ones like tab or
// CR/LF -- cannot split it and leak a fragment past redaction), invalid
// UTF-8 repair, control/bidi character normalization to spaces, whitespace
// collapsing, trimming, leading numeric Code extraction, and UTF-8-safe
// truncation to the upstream length limits.
//
// Parse returns an error if, after normalization, both Code and Message
// are empty.
func Parse(raw string) (Parsed, error) {
	s := urlPattern.ReplaceAllString(raw, "<url>")
	s = strings.ToValidUTF8(s, string(utf8.RuneError))
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || isBidiFormat(r) {
			return ' '
		}
		return r
	}, s)
	s = collapseSpaces(s)
	s = strings.TrimSpace(s)

	code, message := splitCode(s)

	code = truncateUTF8(code, maxCodeBytes)
	message = truncateUTF8(message, maxMessageBytes)

	if code == "" && message == "" {
		return Parsed{}, fmt.Errorf("errtext: empty after normalization")
	}

	return Parsed{Code: code, Message: message}, nil
}

// splitCode extracts a leading signed/unsigned integer as Code when it is
// followed by end-of-string, a space, a period, or a colon. Any other
// following character means the leading digits are not a machine-readable
// code and the entire string is returned as Message.
func splitCode(s string) (code, message string) {
	loc := codePattern.FindStringIndex(s)
	if loc == nil {
		return "", s
	}

	candidate := s[:loc[1]]
	rest := s[loc[1]:]

	switch {
	case rest == "":
		return candidate, ""
	case rest[0] == ' ':
		return candidate, strings.TrimLeft(rest, " ")
	case rest[0] == '.' || rest[0] == ':':
		return candidate, strings.TrimLeft(rest[1:], " ")
	default:
		return "", s
	}
}

// collapseSpaces reduces runs of consecutive ASCII spaces to a single
// space. It assumes other whitespace/control characters have already been
// normalized to spaces.
func collapseSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r == ' ' {
			if prevSpace {
				continue
			}
			prevSpace = true
		} else {
			prevSpace = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// truncateUTF8 truncates s to at most maxBytes bytes without splitting a
// multi-byte rune.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	trunc := s[:maxBytes]
	for len(trunc) > 0 {
		r, size := utf8.DecodeLastRuneInString(trunc)
		if r != utf8.RuneError || size != 1 {
			break
		}
		trunc = trunc[:len(trunc)-1]
	}
	return trunc
}
