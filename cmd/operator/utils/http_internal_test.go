package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTrimSplitRune drives the cut-repair directly on byte slices, one case per UTF-8
// sequence length and cut position, so the loop that steps back over continuation bytes
// is exercised for every width it has to handle.
func TestTrimSplitRune(t *testing.T) {
	const (
		twoByte  = "é" // c3 a9
		three    = "€" // e2 82 ac
		fourByte = "😀" // f0 9f 98 80
	)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "ascii only", in: "abc", want: "abc"},
		{name: "complete 2-byte rune at the end is kept", in: "ab" + twoByte, want: "ab" + twoByte},
		{name: "complete 3-byte rune at the end is kept", in: "ab" + three, want: "ab" + three},
		{name: "complete 4-byte rune at the end is kept", in: "ab" + fourByte, want: "ab" + fourByte},
		{name: "2-byte rune cut after 1", in: "ab" + twoByte[:1], want: "ab"},
		{name: "3-byte rune cut after 1", in: "ab" + three[:1], want: "ab"},
		{name: "3-byte rune cut after 2", in: "ab" + three[:2], want: "ab"},
		{name: "4-byte rune cut after 1", in: "ab" + fourByte[:1], want: "ab"},
		{name: "4-byte rune cut after 2", in: "ab" + fourByte[:2], want: "ab"},
		{name: "4-byte rune cut after 3", in: "ab" + fourByte[:3], want: "ab"},
		{name: "only a split rune", in: fourByte[:3], want: ""},
		{name: "split rune after a complete one", in: twoByte + three[:2], want: twoByte},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(trimSplitRune([]byte(tt.in))))
		})
	}
}
