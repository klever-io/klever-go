package converters

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJsonEscape(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", JsonEscape(""))
	require.Equal(t, "nil", JsonEscape("nil"))
	require.Equal(t, "hello", JsonEscape("hello"))
	require.Equal(t, "\\\\", JsonEscape("\\"))
	require.Equal(t, `\"'\\/.,\u003c\u003e'\"`, JsonEscape(`"'\/.,<>'"`))
	require.Equal(t, `tag\u003e`, JsonEscape(`tag>`))
	require.Equal(t, ",.\\u003c.\\u003e\\u003c\\u003c\\u003c\\u003c\\u003e\\u003e\\u003e\\u003e\\u003e", JsonEscape(",.<.><<<<>>>>>"))
}

func TestPrepareHashesForQueryRemove(t *testing.T) {
	t.Parallel()

	res := PrepareHashesForQueryRemove([]string{"1", "2"})
	require.Equal(t, `{"query": {"ids": {"values": ["1","2"]}}}`, res.String())

	res = PrepareHashesForQueryRemove(nil)
	require.Equal(t, `{"query": {"ids": {"values": []}}}`, res.String())

	res = PrepareHashesForQueryRemove([]string{})
	require.Equal(t, `{"query": {"ids": {"values": []}}}`, res.String())

	res = PrepareHashesForQueryRemove([]string{`"""`, "1111", `~''`})
	require.Equal(t, `{"query": {"ids": {"values": ["\"\"\"","1111","~''"]}}}`, res.String())

	res = PrepareHashesForQueryRemove([]string{""})
	require.Equal(t, `{"query": {"ids": {"values": [""]}}}`, res.String())
}
