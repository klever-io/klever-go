package indexer

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/klever-io/klever-go/indexer/templates"
	"github.com/stretchr/testify/require"
)

// mappingCluster is the slice of Elasticsearch start-up talks to: the existence checks,
// which all answer 404, the template, index and alias creates, and the field mapping read
// and write. It records what it was asked, in order, so a test can assert the read happened
// before the write and that the write carries the property. The product header is what the
// client checks before trusting a server.
type mappingCluster struct {
	mu             sync.Mutex
	fieldType      string // "" means the field is not mapped
	secondIndex    string // "" means one backing index; "unmapped" adds one without the field; "empty" adds one with an empty mapping object
	rejectPut      bool   // every write is refused with 400
	indexTaken     bool   // an index create answers that the index already exists
	requests       []string
	putBody        []byte
	templateBodies [][]byte // every body written to the transactions template, in order
	ioErr          error    // the first read or write that failed inside the handler
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
			first := `"transactions-000001":{"mappings":{}}`
			if c.fieldType != "" {
				first = `"transactions-000001":{"mappings":{"scAddresses":{"full_name":"scAddresses","mapping":{"scAddresses":{"type":"` + c.fieldType + `"}}}}}`
			}
			switch c.secondIndex {
			case "unmapped":
				c.write(w, `{`+first+`,"transactions-000002":{"mappings":{}}}`)
			case "empty":
				c.write(w, `{`+first+`,"transactions-000002":{"mappings":{"scAddresses":{"full_name":"scAddresses","mapping":{}}}}}`)
			default:
				c.write(w, `{`+first+`}`)
			}
		case r.Method == http.MethodPut && r.URL.Path == "/"+txIndex+"/_mapping":
			c.putBody = c.read(r)
			if c.rejectPut {
				w.WriteHeader(http.StatusBadRequest)
				c.write(w, `{"error":{"type":"illegal_argument_exception","reason":"mapper [scAddresses] cannot be changed"}}`)
				return
			}
			c.write(w, `{"acknowledged":true}`)
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/_template/"):
			if r.URL.Path == "/_template/"+txIndex {
				body := c.read(r)
				c.templateBodies = append(c.templateBodies, body)
			}
			if c.rejectPut {
				w.WriteHeader(http.StatusBadRequest)
				c.write(w, `{"error":{"type":"illegal_argument_exception","reason":"bad template"}}`)
				return
			}
			c.write(w, `{"acknowledged":true}`)
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/_alias"):
			if c.rejectPut {
				w.WriteHeader(http.StatusBadRequest)
				c.write(w, `{"error":{"type":"illegal_argument_exception","reason":"bad alias"}}`)
				return
			}
			c.write(w, `{"acknowledged":true}`)
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "-000001"):
			if c.indexTaken {
				w.WriteHeader(http.StatusBadRequest)
				c.write(w, `{"error":{"type":"resource_already_exists_exception","reason":"index already exists"}}`)
				return
			}
			if c.rejectPut {
				w.WriteHeader(http.StatusBadRequest)
				c.write(w, `{"error":{"type":"illegal_argument_exception","reason":"bad index"}}`)
				return
			}
			c.write(w, `{"acknowledged":true}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			c.write(w, `{"error":{"type":"resource_not_found_exception"}}`)
		}
	})
}

// write and read record the first I/O failure inside the handler, which holds c.mu, so a
// partial fixture surfaces as a test failure rather than as a puzzling client error.
func (c *mappingCluster) write(w io.Writer, s string) {
	if _, err := io.WriteString(w, s); err != nil && c.ioErr == nil {
		c.ioErr = err
	}
}

func (c *mappingCluster) read(r *http.Request) []byte {
	body, err := io.ReadAll(r.Body)
	if err != nil && c.ioErr == nil {
		c.ioErr = err
	}

	return body
}

// serve starts the fake and, when the test ends, closes it and fails the test on any I/O
// error the handler recorded.
func serve(t *testing.T, cluster *mappingCluster) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(cluster.handler())
	t.Cleanup(func() {
		server.Close()
		cluster.mu.Lock()
		defer cluster.mu.Unlock()
		require.NoError(t, cluster.ioErr, "the fake must have served every request in full")
	})

	return server
}

func (c *mappingCluster) seen() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]string(nil), c.requests...)
}

func (c *mappingCluster) mappingPutBody() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.putBody
}

func (c *mappingCluster) templatesWritten() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([][]byte(nil), c.templateBodies...)
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
		server := serve(t, cluster)

		require.NoError(t, newMappingClient(t, server.URL).CheckAndUpdateMapping(txIndex, addedProperties))

		require.Equal(t, []string{"GET /transactions/_mapping/field/scAddresses", "PUT /transactions/_mapping"}, cluster.seen(),
			"the live mapping is read first, then the missing property is written")
		require.True(t, bytes.Contains(cluster.mappingPutBody(), []byte(`"scAddresses":{"type":"keyword"}`)), "body: %s", cluster.mappingPutBody())
	})

	t.Run("an index that already maps the property gets no write", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{fieldType: "keyword"}
		server := serve(t, cluster)

		require.NoError(t, newMappingClient(t, server.URL).CheckAndUpdateMapping(txIndex, addedProperties))
		require.Equal(t, []string{"GET /transactions/_mapping/field/scAddresses"}, cluster.seen(),
			"a node on an up-to-date index must not need the manage privilege at start-up")
	})

	t.Run("a property mapped with another type is an error, not a write", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{fieldType: "text"}
		server := serve(t, cluster)

		err := newMappingClient(t, server.URL).CheckAndUpdateMapping(txIndex, addedProperties)
		require.ErrorIs(t, err, ErrCouldNotUpdateMapping)
		require.ErrorContains(t, err, `maps scAddresses as "text"`, "the operator must learn which type the index carries")
		require.Equal(t, []string{"GET /transactions/_mapping/field/scAddresses"}, cluster.seen(), "no write may follow")
	})

	t.Run("a rejected write is an error, not a closed response", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{rejectPut: true}
		server := serve(t, cluster)

		err := newMappingClient(t, server.URL).CheckAndUpdateMapping(txIndex, addedProperties)
		require.ErrorIs(t, err, ErrCouldNotUpdateMapping)
		require.ErrorContains(t, err, "cannot be changed", "the cluster's reason must reach the operator")
	})

	t.Run("an unreachable cluster is an error", func(t *testing.T) {
		t.Parallel()

		server := serve(t, &mappingCluster{})
		server.Close()

		require.Error(t, newMappingClient(t, server.URL).CheckAndUpdateMapping(txIndex, addedProperties))
	})
}

// TestCheckAndUpdateMapping_WritesWhenAnyBackingIndexLacksTheField covers an alias with
// several indices behind it: the field mapped on one of them says nothing about the others,
// and an entry with an empty mapping object is not a mapping.
func TestCheckAndUpdateMapping_WritesWhenAnyBackingIndexLacksTheField(t *testing.T) {
	t.Parallel()

	for _, second := range []string{"unmapped", "empty"} {
		t.Run(second, func(t *testing.T) {
			t.Parallel()

			cluster := &mappingCluster{fieldType: "keyword", secondIndex: second}
			server := serve(t, cluster)

			require.NoError(t, newMappingClient(t, server.URL).CheckAndUpdateMapping(txIndex, addedProperties))
			require.Equal(t, []string{"GET /transactions/_mapping/field/scAddresses", "PUT /transactions/_mapping"}, cluster.seen(),
				"one index carrying the field must not suppress the write for the one that lacks it")
		})
	}
}

func TestCheckFieldMapping_ReportsWhatIsMissing(t *testing.T) {
	t.Parallel()

	cluster := &mappingCluster{}
	server := serve(t, cluster)

	missing, err := newMappingClient(t, server.URL).CheckFieldMapping(txIndex, addedProperties)
	require.NoError(t, err)
	require.Equal(t, []string{"scAddresses"}, missing)
	require.Equal(t, []string{"GET /transactions/_mapping/field/scAddresses"}, cluster.seen(), "a check reads and never writes")
}

// TestCheckAndCreateTemplate_LeavesTheBufferForTheNextRequest pins the contract the start-up
// path relies on: the create step and the rewrite share one buffer out of the templates map,
// so the create request must read a copy of it and leave the buffer intact for the rewrite.
func TestCheckAndCreateTemplate_LeavesTheBufferForTheNextRequest(t *testing.T) {
	t.Parallel()

	cluster := &mappingCluster{}
	server := serve(t, cluster)

	client := newMappingClient(t, server.URL)
	template := addedProperties.ToBuffer()
	want := append([]byte(nil), template.Bytes()...)

	require.NoError(t, client.CheckAndCreateTemplate(txIndex, template))
	require.Equal(t, want, template.Bytes(), "the create request must not consume the caller's buffer")
	require.NoError(t, client.PutTemplate(txIndex, template.Bytes()))

	require.Equal(t, []string{"HEAD /_template/transactions", "PUT /_template/transactions", "PUT /_template/transactions"}, cluster.seen())
	require.Equal(t, [][]byte{want, want}, cluster.templatesWritten(), "both writes must carry the full template")
}

// TestNewElasticProcessor_OnAFreshClusterWritesTheTransactionsTemplateTwiceInFull is the
// start-up on a cluster that has nothing yet: every existence check answers 404, so the
// create step sends the templates, and the rewrite that follows must send the transactions
// template again in full, not the empty remainder of a buffer the create request read. On a
// cluster that already carries the template the create step never reads the buffer, which
// is why only a fresh cluster shows this.
func TestNewElasticProcessor_OnAFreshClusterWritesTheTransactionsTemplateTwiceInFull(t *testing.T) {
	cluster := &mappingCluster{}
	server := serve(t, cluster)

	indexTemplates, indexPolicies, err := GetElasticTemplatesAndPolicies(false)
	require.NoError(t, err)
	want := append([]byte(nil), indexTemplates[txIndex].Bytes()...)

	args := createMockElasticProcessorArgs()
	args.DBClient = newMappingClient(t, server.URL)
	args.IndexTemplates, args.IndexPolicies = indexTemplates, indexPolicies

	_, err = NewElasticProcessor(args)
	require.NoError(t, err)

	require.Contains(t, cluster.seen(), "HEAD /_template/transactions", "the premise: the create step checked, found nothing, and wrote")
	require.Equal(t, [][]byte{want, want}, cluster.templatesWritten(), "the create step and the rewrite must both carry the full template")
	require.Equal(t, want, indexTemplates[txIndex].Bytes(), "start-up must leave the shared buffer intact")
}

// TestCreateSteps_ARefusalIsAnError pins that a template, index or alias the cluster refuses
// stops start-up instead of being closed and forgotten, and that an index which exists by
// the time the create arrives is not an error, since that is what the call wanted.
func TestCreateSteps_ARefusalIsAnError(t *testing.T) {
	t.Parallel()

	t.Run("template", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{rejectPut: true}
		server := serve(t, cluster)

		err := newMappingClient(t, server.URL).CheckAndCreateTemplate(txIndex, addedProperties.ToBuffer())
		require.ErrorIs(t, err, ErrCouldNotCreateTemplate)
		require.ErrorContains(t, err, "bad template", "the cluster's reason must reach the operator")
	})

	t.Run("index", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{rejectPut: true}
		server := serve(t, cluster)

		err := newMappingClient(t, server.URL).CheckAndCreateIndex("transactions-000001")
		require.ErrorIs(t, err, ErrCouldNotCreateIndex)
		require.ErrorContains(t, err, "bad index")
	})

	t.Run("index that exists by the time the create arrives", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{indexTaken: true}
		server := serve(t, cluster)

		require.NoError(t, newMappingClient(t, server.URL).CheckAndCreateIndex("transactions-000001"))
		require.Equal(t, []string{"HEAD /transactions-000001", "PUT /transactions-000001"}, cluster.seen(),
			"the premise: the existence check said no, the create was sent, the cluster said it exists")
	})

	t.Run("alias", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{rejectPut: true}
		server := serve(t, cluster)

		err := newMappingClient(t, server.URL).CheckAndCreateAlias(txIndex, "transactions-000001")
		require.ErrorIs(t, err, ErrCouldNotCreateAlias)
		require.ErrorContains(t, err, "bad alias")
	})

	t.Run("an unreachable cluster is an error naming the resource", func(t *testing.T) {
		t.Parallel()

		server := serve(t, &mappingCluster{})
		server.Close()
		client := newMappingClient(t, server.URL)

		require.ErrorIs(t, client.CheckAndCreateTemplate(txIndex, addedProperties.ToBuffer()), ErrCouldNotCreateTemplate)
		require.ErrorIs(t, client.CheckAndCreateIndex("transactions-000001"), ErrCouldNotCreateIndex)
		require.ErrorIs(t, client.CheckAndCreateAlias(txIndex, "transactions-000001"), ErrCouldNotCreateAlias)
	})
}

func TestPutTemplate_WritesWhetherOrNotOneExists(t *testing.T) {
	t.Parallel()

	t.Run("writes", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{}
		server := serve(t, cluster)

		require.NoError(t, newMappingClient(t, server.URL).PutTemplate(txIndex, addedProperties.ToBuffer().Bytes()))
		require.Equal(t, []string{"PUT /_template/transactions"}, cluster.seen(), "no existence check may short-circuit the write")
	})

	t.Run("a rejected template is an error", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{rejectPut: true}
		server := serve(t, cluster)

		require.ErrorIs(t, newMappingClient(t, server.URL).PutTemplate(txIndex, addedProperties.ToBuffer().Bytes()), ErrCouldNotCreateTemplate)
	})

	t.Run("an empty body is refused before it reaches the cluster", func(t *testing.T) {
		t.Parallel()

		cluster := &mappingCluster{}
		server := serve(t, cluster)

		require.ErrorIs(t, newMappingClient(t, server.URL).PutTemplate(txIndex, nil), ErrCouldNotCreateTemplate)
		require.Empty(t, cluster.seen(), "a consumed buffer must not turn into an empty template on the cluster")
	})
}
