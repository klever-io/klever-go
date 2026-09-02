package indexer

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/klever-io/klever-go/indexer/templates"
	"github.com/stretchr/testify/require"
)

// mappingCluster is the slice of Elasticsearch the mapping bootstrap talks to: the field
// mapping read and the mapping write. It records what it was asked, in order, so a test
// can assert the read happened before the write and that the write carries the property.
// The product header is what the client checks before trusting a server.
type mappingCluster struct {
	mu        sync.Mutex
	fieldType string // "" means the field is not mapped
	rejectPut bool
	requests  []string
	putBody   []byte
}

func (c *mappingCluster) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")

		c.mu.Lock()
		defer c.mu.Unlock()
		c.requests = append(c.requests, r.Method+" "+r.URL.Path)

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/"+txIndex+"/_mapping/field/scAddresses":
			if c.fieldType == "" {
				_, _ = io.WriteString(w, `{"transactions-000001":{"mappings":{}}}`)
				return
			}
			_, _ = io.WriteString(w, `{"transactions-000001":{"mappings":{"scAddresses":{"full_name":"scAddresses","mapping":{"scAddresses":{"type":"`+c.fieldType+`"}}}}}}`)
		case r.Method == http.MethodPut && r.URL.Path == "/"+txIndex+"/_mapping":
			c.putBody, _ = io.ReadAll(r.Body)
			if c.rejectPut {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":{"type":"illegal_argument_exception","reason":"mapper [scAddresses] cannot be changed"}}`)
				return
			}
			_, _ = io.WriteString(w, `{"acknowledged":true}`)
		case r.Method == http.MethodPut && r.URL.Path == "/_template/"+txIndex:
			if c.rejectPut {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":{"type":"illegal_argument_exception","reason":"bad template"}}`)
				return
			}
			_, _ = io.WriteString(w, `{"acknowledged":true}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":{"type":"resource_not_found_exception"}}`)
		}
	})
}

func (c *mappingCluster) seen() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]string(nil), c.requests...)
}

func newMappingClient(t *testing.T, url string) *elasticClient {
	t.Helper()

	ec, err := NewElasticClient(elasticsearch.Config{Addresses: []string{url}})
	require.NoError(t, err)

	return ec
}

var addedProperties = templates.Object{"properties": templates.TransactionsAddedProperties}

// TestCheckAndUpdateMapping covers the three states a live index can be in and the one
// thing the bootstrap must never do: report success for a mapping the cluster rejected,
// or write when nothing is missing.
func TestCheckAndUpdateMapping(t *testing.T) {
	t.Parallel()

	t.Run("puts only what the index does not map yet, after reading", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{}
		server := httptest.NewServer(cluster.handler())
		defer server.Close()

		require.NoError(t, newMappingClient(t, server.URL).CheckAndUpdateMapping(txIndex, addedProperties))

		require.Equal(t, []string{"GET /transactions/_mapping/field/scAddresses", "PUT /transactions/_mapping"}, cluster.seen(),
			"the live mapping is read first, then the missing property is written")
		require.True(t, bytes.Contains(cluster.putBody, []byte(`"scAddresses":{"type":"keyword"}`)), "body: %s", cluster.putBody)
	})

	t.Run("an index that already maps the property gets no write", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{fieldType: "keyword"}
		server := httptest.NewServer(cluster.handler())
		defer server.Close()

		require.NoError(t, newMappingClient(t, server.URL).CheckAndUpdateMapping(txIndex, addedProperties))
		require.Equal(t, []string{"GET /transactions/_mapping/field/scAddresses"}, cluster.seen(),
			"a node on an up-to-date index must not need the manage privilege at start-up")
	})

	t.Run("a property mapped with another type is an error, not a write", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{fieldType: "text"}
		server := httptest.NewServer(cluster.handler())
		defer server.Close()

		err := newMappingClient(t, server.URL).CheckAndUpdateMapping(txIndex, addedProperties)
		require.ErrorIs(t, err, ErrCouldNotUpdateMapping)
		require.ErrorContains(t, err, `maps scAddresses as "text"`, "the operator must learn which type the index carries")
		require.Equal(t, []string{"GET /transactions/_mapping/field/scAddresses"}, cluster.seen(), "no write may follow")
	})

	t.Run("a rejected write is an error, not a closed response", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{rejectPut: true}
		server := httptest.NewServer(cluster.handler())
		defer server.Close()

		err := newMappingClient(t, server.URL).CheckAndUpdateMapping(txIndex, addedProperties)
		require.ErrorIs(t, err, ErrCouldNotUpdateMapping)
		require.ErrorContains(t, err, "cannot be changed", "the cluster's reason must reach the operator")
	})

	t.Run("an unreachable cluster is an error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer((&mappingCluster{}).handler())
		server.Close()

		require.Error(t, newMappingClient(t, server.URL).CheckAndUpdateMapping(txIndex, addedProperties))
	})
}

func TestCheckFieldMapping_ReportsWhatIsMissing(t *testing.T) {
	t.Parallel()

	cluster := &mappingCluster{}
	server := httptest.NewServer(cluster.handler())
	defer server.Close()

	missing, err := newMappingClient(t, server.URL).CheckFieldMapping(txIndex, addedProperties)
	require.NoError(t, err)
	require.Equal(t, []string{"scAddresses"}, missing)
	require.Equal(t, []string{"GET /transactions/_mapping/field/scAddresses"}, cluster.seen(), "a check reads and never writes")
}

func TestPutTemplate_WritesWhetherOrNotOneExists(t *testing.T) {
	t.Parallel()

	t.Run("writes", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{}
		server := httptest.NewServer(cluster.handler())
		defer server.Close()

		require.NoError(t, newMappingClient(t, server.URL).PutTemplate(txIndex, addedProperties.ToBuffer()))
		require.Equal(t, []string{"PUT /_template/transactions"}, cluster.seen(), "no existence check may short-circuit the write")
	})

	t.Run("a rejected template is an error", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{rejectPut: true}
		server := httptest.NewServer(cluster.handler())
		defer server.Close()

		require.ErrorIs(t, newMappingClient(t, server.URL).PutTemplate(txIndex, addedProperties.ToBuffer()), ErrCouldNotCreateTemplate)
	})
}
