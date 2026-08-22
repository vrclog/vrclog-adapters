package errtext

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParse_SpecExamples(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantCode    string
		wantMessage string
	}{
		{"numeric_code_only", "-736347996.", "-736347996", ""},
		{"numeric_code_with_message", "-736347996: Loading failed.", "-736347996", "Loading failed."},
		{"human_message_only", "Connection timeout.", "", "Connection timeout."},
		{"url_in_message", "Error for https://example.invalid/video", "", "Error for <url>"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.raw)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tc.raw, err)
			}
			if got.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", got.Code, tc.wantCode)
			}
			if got.Message != tc.wantMessage {
				t.Errorf("Message = %q, want %q", got.Message, tc.wantMessage)
			}
		})
	}
}

func TestParse_CodeDelimiters(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantCode    string
		wantMessage string
	}{
		{"trailing_dot_only", "500.", "500", ""},
		{"bare_number", "123", "123", ""},
		{"negative_zero", "-0", "-0", ""},
		{"space_delimiter", "123 foo", "123", "foo"},
		{"colon_delimiter", "123:foo", "123", "foo"},
		{"dot_delimiter", "123.foo", "123", "foo"},
		{"leading_zeros", "00123: foo", "00123", "foo"},
		{"letters_not_delimiter", "123abc", "", "123abc"},
		{"plus_not_recognized", "+123", "", "+123"},
		{"double_minus_not_recognized", "--1", "", "--1"},
		{"hex_like_not_recognized", "0x1F", "", "0x1F"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.raw)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tc.raw, err)
			}
			if got.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", got.Code, tc.wantCode)
			}
			if got.Message != tc.wantMessage {
				t.Errorf("Message = %q, want %q", got.Message, tc.wantMessage)
			}
		})
	}
}

func TestParse_EmptyAfterNormalizationIsError(t *testing.T) {
	cases := map[string]string{
		"empty_string":     "",
		"whitespace_only":  "   ",
		"control_only":     "\t\r\n",
		"tab_cr_lf_spaces": "\t \r \n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(raw)
			if err == nil {
				t.Fatalf("Parse(%q) error = nil, want error", raw)
			}
		})
	}
}

func TestParse_ControlCharNormalization(t *testing.T) {
	got, err := Parse("\t\r\nfoo")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Code != "" {
		t.Errorf("Code = %q, want empty", got.Code)
	}
	if got.Message != "foo" {
		t.Errorf("Message = %q, want %q", got.Message, "foo")
	}
}

func TestParse_MultipleURLRedaction(t *testing.T) {
	got, err := Parse("42: https://a.com and https://b.com")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Code != "42" {
		t.Errorf("Code = %q, want %q", got.Code, "42")
	}
	if got.Message != "<url> and <url>" {
		t.Errorf("Message = %q, want %q", got.Message, "<url> and <url>")
	}
	for _, forbidden := range []string{"http://", "https://"} {
		if strings.Contains(got.Message, forbidden) {
			t.Errorf("Message %q leaks URL scheme %q", got.Message, forbidden)
		}
	}
}

func TestParse_URLOnlyMessage(t *testing.T) {
	got, err := Parse("https://example.com/foo")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Code != "" {
		t.Errorf("Code = %q, want empty", got.Code)
	}
	if got.Message != "<url>" {
		t.Errorf("Message = %q, want %q", got.Message, "<url>")
	}
}

// TestParse_AngleBracketsAreHardURLBoundaries documents that a literal '<'
// or '>' inside raw text stops URL redaction, unlike every other
// character. This is intentional: '<' and '>' are not valid unescaped
// RFC 3986 URI characters, so a legitimate URL never contains one -- any
// occurrence marks adversarial or malformed input, not a real URL
// continuation. The trade-off is that text following an errant angle
// bracket is not redacted; this is accepted rather than treated as a bug.
func TestParse_AngleBracketsAreHardURLBoundaries(t *testing.T) {
	got, err := Parse("error https://a.com/path<token=secret rest")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Message != "error <url><token=secret rest" {
		t.Errorf("Message = %q, want %q", got.Message, "error <url><token=secret rest")
	}
}

func TestParse_TrailingPunctuationAfterURL(t *testing.T) {
	got, err := Parse("failed for https://example.com/video.")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Message != "failed for <url>" {
		t.Errorf("Message = %q, want %q", got.Message, "failed for <url>")
	}
}

// TestParse_ControlCharSplitURLFullyRedacted verifies that a non-whitespace
// control character embedded inside a URL (e.g. a NUL byte inserted to
// evade naive redaction) does not cause any fragment of the URL --
// including trailing query parameters -- to leak into the normalized
// Message. This documents the redact-first, over-redaction-is-safe design.
func TestParse_ControlCharSplitURLFullyRedacted(t *testing.T) {
	raw := "error https://a.com/path\x00?token=secret rest"
	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Message != "error <url> rest" {
		t.Errorf("Message = %q, want %q", got.Message, "error <url> rest")
	}
	if strings.Contains(got.Message, "token") || strings.Contains(got.Message, "secret") {
		t.Errorf("Message %q leaks query token", got.Message)
	}
}

// TestParse_WhitespaceControlCharSplitURLFullyRedacted verifies that a
// whitespace-classified control character (tab, CR, LF, FF, VT) embedded
// inside a URL also does not split the redaction and leak a trailing
// fragment. Unlike \x00, these ARE members of regexp's \s class, so a
// naive [^\s<>]+ URL pattern stops matching at them -- this is the
// regression the redact-first design must guard against explicitly.
func TestParse_WhitespaceControlCharSplitURLFullyRedacted(t *testing.T) {
	cases := map[string]string{
		"tab":             "\t",
		"carriage_return": "\r",
		"line_feed":       "\n",
		"form_feed":       "\f",
		"vertical_tab":    "\v",
	}
	for name, ctrl := range cases {
		t.Run(name, func(t *testing.T) {
			raw := "error https://a.com/path" + ctrl + "?token=secret rest"
			got, err := Parse(raw)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if got.Message != "error <url> rest" {
				t.Errorf("Message = %q, want %q", got.Message, "error <url> rest")
			}
			if strings.Contains(got.Message, "token") || strings.Contains(got.Message, "secret") {
				t.Errorf("Message %q leaks query token", got.Message)
			}
		})
	}
}

// TestParse_BidiFormatCharsNormalized verifies that Unicode bidi formatting
// characters (e.g. U+202E RIGHT-TO-LEFT OVERRIDE) are normalized to spaces
// like control characters. Upstream vrclog-go rejects these in Code and
// Message; leaving them unnormalized would let an otherwise-valid rule
// match produce an event the Engine silently drops as invalid.
func TestParse_BidiFormatCharsNormalized(t *testing.T) {
	got, err := Parse("foo‮bar")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if ContainsUnsafeControlOrBidi(got.Message) {
		t.Errorf("Message %q still contains unsafe control/bidi characters", got.Message)
	}
	if got.Message != "foo bar" {
		t.Errorf("Message = %q, want %q", got.Message, "foo bar")
	}
}

func TestContainsUnsafeControlOrBidi(t *testing.T) {
	cases := map[string]bool{
		"plain ascii":      false,
		"with\ttab":        true,
		"with‮bidi":        true,
		"with space is ok": false,
		"":                 false,
	}
	for input, want := range cases {
		if got := ContainsUnsafeControlOrBidi(input); got != want {
			t.Errorf("ContainsUnsafeControlOrBidi(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestParse_InvalidUTF8Repaired(t *testing.T) {
	raw := "\xff\xfe error message"
	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !utf8.ValidString(got.Message) {
		t.Errorf("Message %q is not valid UTF-8", got.Message)
	}
	if !strings.Contains(got.Message, "error message") {
		t.Errorf("Message = %q, want it to contain %q", got.Message, "error message")
	}
}

func TestParse_MessageTruncatedToByteLimit(t *testing.T) {
	raw := "Connection timeout: " + strings.Repeat("a", 3000)
	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(got.Message) != maxMessageBytes {
		t.Errorf("len(Message) = %d, want %d", len(got.Message), maxMessageBytes)
	}
}

func TestParse_MessageTruncationIsUTF8SafeMultiByte(t *testing.T) {
	raw := strings.Repeat("あ", 1000) // 3 bytes per rune, 3000 bytes total
	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(got.Message) > maxMessageBytes {
		t.Fatalf("len(Message) = %d, want <= %d", len(got.Message), maxMessageBytes)
	}
	if !utf8.ValidString(got.Message) {
		t.Errorf("Message is not valid UTF-8 after truncation: %q", got.Message)
	}
}

func TestParse_MessageTruncationIsUTF8SafeEmoji(t *testing.T) {
	raw := strings.Repeat("\U0001F600", 600) // 4 bytes per rune, 2400 bytes total
	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(got.Message) > maxMessageBytes {
		t.Fatalf("len(Message) = %d, want <= %d", len(got.Message), maxMessageBytes)
	}
	if !utf8.ValidString(got.Message) {
		t.Errorf("Message is not valid UTF-8 after truncation: %q", got.Message)
	}
}

func TestParse_CodeTruncatedToByteLimit(t *testing.T) {
	raw := strings.Repeat("9", 200)
	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(got.Code) != maxCodeBytes {
		t.Errorf("len(Code) = %d, want %d", len(got.Code), maxCodeBytes)
	}
	if got.Message != "" {
		t.Errorf("Message = %q, want empty", got.Message)
	}
}

func TestParse_ErrorMessageDoesNotLeakURL(t *testing.T) {
	_, err := Parse("   ")
	if err == nil {
		t.Fatalf("Parse() error = nil, want error")
	}
	for _, forbidden := range []string{"http://", "https://"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Errorf("error message %q leaks URL scheme %q", err.Error(), forbidden)
		}
	}
}
