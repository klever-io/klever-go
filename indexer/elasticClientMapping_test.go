package indexer

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/klever-io/klever-go/indexer/templates"
	"github.com/stretchr/testify/require"
)

// TestCheckAndUpdateMapping drives the mapping update against a fake cluster, because the
// one thing it must never do is report success for a mapping the cluster rejected: that is
// the silent drift the method exists to prevent. The product header is what the client
// checks before trusting a server.
func TestCheckAndUpdateMapping(t *testing.T) {
	t.Parallel()

	newServer := func(status int, body string, seen *[]byte, path *string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Elastic-Product", "Elasticsearch")
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/" {
				_, _ = io.WriteString(w, `{"version":{"number":"8.4.0"},"tagline":"You Know, for Search"}`)
				return
			}
			*path = r.Method + " " + r.URL.Path
			*seen, _ = io.ReadAll(r.Body)
			w.WriteHeader(status)
			_, _ = io.WriteString(w, body)
		}))
	}

	properties := templates.Object{"properties": templates.TransactionsAddedProperties}

	t.Run("puts the properties on the index", func(t *testing.T) {
		t.Parallel()

		var seen []byte
		var path string
		server := newServer(http.StatusOK, `{"acknowledged":true}`, &seen, &path)
		defer server.Close()

		ec, err := NewElasticClient(elasticsearch.Config{Addresses: []string{server.URL}})
		require.NoError(t, err)

		require.NoError(t, ec.CheckAndUpdateMapping(txIndex, properties.ToBuffer()))
		require.Equal(t, "PUT /"+txIndex+"/_mapping", path)
		require.True(t, bytes.Contains(seen, []byte(`"scAddresses":{"type":"keyword"}`)), "body: %s", seen)
	})

	t.Run("a rejected mapping is an error, not a closed response", func(t *testing.T) {
		t.Parallel()

		var seen []byte
		var path string
		server := newServer(http.StatusBadRequest,
			`{"error":{"type":"illegal_argument_exception","reason":"mapper [scAddresses] cannot be changed from type [text] to [keyword]"}}`,
			&seen, &path)
		defer server.Close()

		ec, err := NewElasticClient(elasticsearch.Config{Addresses: []string{server.URL}})
		require.NoError(t, err)

		err = ec.CheckAndUpdateMapping(txIndex, properties.ToBuffer())
		require.ErrorIs(t, err, ErrCouldNotUpdateMapping)
		require.ErrorContains(t, err, "cannot be changed from type [text] to [keyword]",
			"the cluster's reason must reach the operator")
	})

	t.Run("an unreachable cluster is an error", func(t *testing.T) {
		t.Parallel()

		var seen []byte
		var path string
		server := newServer(http.StatusOK, `{}`, &seen, &path)
		server.Close()

		ec, err := NewElasticClient(elasticsearch.Config{Addresses: []string{server.URL}})
		require.NoError(t, err)

		require.Error(t, ec.CheckAndUpdateMapping(txIndex, properties.ToBuffer()))
	})
}
