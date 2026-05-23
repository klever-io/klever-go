package process_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/klever-io/klever-go/core/process"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeBlacklistReason(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain", "hello world", "hello world"},
		{"strips CR LF TAB", "a\rb\nc\td", "abcd"},
		{"strips ANSI ESC (CSI is neutered)", "\x1b[31mred\x1b[0m", "[31mred[0m"},
		{"strips DEL", "a\x7fb", "ab"},
		{"strips C1 controls (NEL, raw CSI)", "a\u0085b\u009bc", "abc"},
		{"strips U+2028 LINE SEPARATOR", "a\u2028b", "ab"},
		{"strips U+2029 PARAGRAPH SEPARATOR", "a\u2029b", "ab"},
		{"strips zero-width chars", "a\u200bb\u200cc\u200dd\ufeffe", "abcde"},
		{"strips Trojan-Source bidi controls", "user\u202eadmin\u202cend", "useradminend"},
		{"keeps printable unicode", "héllo — wörld 日本語", "héllo — wörld 日本語"},
		{"caps at MaxSanitizedBlacklistReason (ASCII)", strings.Repeat("a", process.MaxSanitizedBlacklistReason+1) + "\n", strings.Repeat("a", process.MaxSanitizedBlacklistReason)},
		{"caps at MaxSanitizedBlacklistReason (multi-byte rune)", strings.Repeat("世", process.MaxSanitizedBlacklistReason+10), strings.Repeat("世", process.MaxSanitizedBlacklistReason)},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := process.SanitizeBlacklistReason(tc.in)
			assert.Equal(t, tc.want, got)
			assert.LessOrEqual(t, utf8.RuneCountInString(got), process.MaxSanitizedBlacklistReason)
			assert.False(t, strings.ContainsAny(got, "\r\n\t\x00\x1b\x7f\u0085\u009b\u2028\u2029\u200b\u202e"),
				"sanitized must not contain any stripped char")
		})
	}
}
