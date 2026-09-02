package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/stretchr/testify/require"
)

// contractBytes builds a 32-byte address core.IsSmartContractAddress accepts.
func contractBytes(tail byte) []byte {
	address := make([]byte, 32)
	copy(address[core.NumInitCharactersForScAddress-core.VMTypeLen:], common.WasmVirtualMachine)
	address[31] = tail

	return address
}

func walletBytes(tail byte) []byte {
	address := bytes.Repeat([]byte{0xAB}, 32)
	address[31] = tail

	return address
}

func bech32(t *testing.T, address []byte) string {
	t.Helper()

	converter, err := pubkeyConverter.NewBech32PubkeyConverter(addressLength)
	require.NoError(t, err)

	return converter.Encode(address)
}

func hashOf(raw string) string { return hex.EncodeToString([]byte(raw)) }

func eventsOf(addresses ...string) []struct {
	Address string `json:"address"`
} {
	events := make([]struct {
		Address string `json:"address"`
	}, 0, len(addresses))
	for _, address := range addresses {
		events = append(events, struct {
			Address string `json:"address"`
		}{Address: address})
	}

	return events
}

// TestDerivation_IsTheIndexersOwn feeds log documents through the real indexer processor,
// wired exactly as main wires it, and checks the field comes out as the indexer writes it:
// the invoked contract and the inner contract, once each, sorted; the wallet and the empty
// system address left out; a transaction whose log names no contract absent.
func TestDerivation_IsTheIndexersOwn(t *testing.T) {
	derive, err := indexerDerivation()
	require.NoError(t, err)

	invoked, inner := bech32(t, contractBytes(1)), bech32(t, contractBytes(2))
	wallet, empty := bech32(t, walletBytes(9)), bech32(t, make([]byte, 32))

	derived := derive(map[string]logDocument{
		hashOf("swap"):   {Address: invoked, Events: eventsOf(inner, invoked, wallet, empty)},
		hashOf("plain"):  {Address: wallet, Events: eventsOf(wallet)},
		"not-hex-at-all": {Address: invoked},
	})

	want := []string{invoked, inner}
	if invoked > inner {
		want = []string{inner, invoked}
	}
	require.Equal(t, map[string][]string{hashOf("swap"): want}, derived,
		"only the transaction with contracts gets an entry, and it lists the contracts sorted")
}

func TestBulkBody_IsOneUpdatePerTransactionInHashOrder(t *testing.T) {
	body, err := bulkBody("transactions-000001", map[string][]string{
		"bb": {"klv1two"},
		"aa": {"klv1one", "klv1three"},
	})
	require.NoError(t, err)

	require.Equal(t, strings.Join([]string{
		`{"update":{"_id":"aa","_index":"transactions-000001"}}`,
		`{"doc":{"scAddresses":["klv1one","klv1three"]}}`,
		`{"update":{"_id":"bb","_index":"transactions-000001"}}`,
		`{"doc":{"scAddresses":["klv1two"]}}`,
		``,
	}, "\n"), string(body))
}

func TestReadBulkResult_TalliesEveryOutcome(t *testing.T) {
	var out bytes.Buffer
	result, err := readBulkResult(strings.NewReader(`{"items":[
		{"update":{"_id":"a","status":200,"result":"updated"}},
		{"update":{"_id":"b","status":200,"result":"noop"}},
		{"update":{"_id":"c","status":404,"error":{"type":"document_missing_exception","reason":"[c]: document missing"}}},
		{"update":{"_id":"d","status":400,"error":{"type":"mapper_parsing_exception","reason":"bad"}}}
	]}`), &out)
	require.NoError(t, err)

	require.Equal(t, applyResult{updated: 1, unchanged: 1, missing: 1, failed: 1}, result)
	require.Contains(t, out.String(), "failed d: mapper_parsing_exception: bad")
}

// fakeES is the slice of Elasticsearch the tool talks to. The product header is what the
// client checks before trusting a server.
type fakeES struct {
	t        *testing.T
	aliases  []string
	logs     map[string]logDocument
	counts   []int64
	bulkBody *bytes.Buffer
	bulkHits int
}

func (f *fakeES) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/":
			_, _ = io.WriteString(w, `{"version":{"number":"8.4.0"},"tagline":"You Know, for Search"}`)
		case r.URL.Path == "/_alias/"+transactionsAlias:
			indices := map[string]interface{}{}
			for _, name := range f.aliases {
				indices[name] = map[string]interface{}{"aliases": map[string]interface{}{transactionsAlias: map[string]interface{}{}}}
			}
			_ = json.NewEncoder(w).Encode(indices)
		case r.URL.Path == "/"+logsIndex+"/_search":
			require.Contains(f.t, r.URL.Query().Get("_source"), "events.address")
			hits := make([]map[string]interface{}, 0, len(f.logs))
			for hash, doc := range f.logs {
				hits = append(hits, map[string]interface{}{"_id": hash, "_source": doc})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"_scroll_id": "cursor-1", "hits": map[string]interface{}{"hits": hits}})
		case strings.HasPrefix(r.URL.Path, "/_search/scroll") && r.Method == http.MethodDelete:
			_, _ = io.WriteString(w, `{"succeeded":true}`)
		case r.URL.Path == "/_search/scroll":
			_, _ = io.WriteString(w, `{"_scroll_id":"cursor-2","hits":{"hits":[]}}`)
		case r.URL.Path == "/_bulk":
			f.bulkHits++
			raw, _ := io.ReadAll(r.Body)
			f.bulkBody.Write(raw)
			var items []map[string]interface{}
			for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
				if bytes.HasPrefix(line, []byte(`{"update"`)) {
					items = append(items, map[string]interface{}{"update": map[string]interface{}{"status": 200, "result": "updated"}})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"items": items})
		case r.URL.Path == "/"+transactionsAlias+"/_count":
			count := f.counts[0]
			f.counts = f.counts[1:]
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"count": count})
		default:
			f.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func clientFor(t *testing.T, url string) *elasticsearch.Client {
	t.Helper()

	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{url}})
	require.NoError(t, err)

	return client
}

// TestRun_WritesWhatTheIndexerDerives is the whole tool against a fake cluster: the logs
// come in, the indexer's derivation runs, and exactly one update per transaction with
// contracts reaches the target, addressed to the concrete index behind the alias.
func TestRun_WritesWhatTheIndexerDerives(t *testing.T) {
	invoked, inner, wallet := bech32(t, contractBytes(1)), bech32(t, contractBytes(2)), bech32(t, walletBytes(3))
	fake := &fakeES{t: t, aliases: []string{"transactions-000001"}, bulkBody: &bytes.Buffer{}, logs: map[string]logDocument{
		hashOf("swap"):  {Address: invoked, Events: eventsOf(inner)},
		hashOf("plain"): {Address: wallet},
	}}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	derive, err := indexerDerivation()
	require.NoError(t, err)

	var out bytes.Buffer
	b := newBackfiller(clientFor(t, server.URL), clientFor(t, server.URL), derive, options{batchSize: 10}, &out)
	require.NoError(t, b.run(t.Context()))

	want, err := bulkBody("transactions-000001", derive(fake.logs))
	require.NoError(t, err)
	require.Equal(t, string(want), fake.bulkBody.String(), "the bulk body must be exactly the derived updates")
	require.Equal(t, 1, fake.bulkHits)
	require.Contains(t, out.String(), "logs read: 2, with contracts: 1")
	require.Contains(t, out.String(), "updated: 1, already set: 0, no such transaction: 0, failed: 0")
}

func TestRun_DryRunWritesNothing(t *testing.T) {
	invoked := bech32(t, contractBytes(1))
	fake := &fakeES{t: t, aliases: []string{"transactions-000001"}, bulkBody: &bytes.Buffer{}, logs: map[string]logDocument{
		hashOf("swap"): {Address: invoked},
	}}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	derive, err := indexerDerivation()
	require.NoError(t, err)

	var out bytes.Buffer
	b := newBackfiller(clientFor(t, server.URL), clientFor(t, server.URL), derive, options{batchSize: 10, dryRun: true}, &out)
	require.NoError(t, b.run(t.Context()))

	require.Equal(t, 0, fake.bulkHits, "a dry run must not touch the target")
	require.Contains(t, out.String(), "with contracts: 1 (dry run, nothing written)")
}

// TestRun_RefusesAnAliasSpanningSeveralIndices covers the rollover case: a bulk update
// addressed to such an alias fails per item, so the run must stop before the first batch
// and name the indices.
func TestRun_RefusesAnAliasSpanningSeveralIndices(t *testing.T) {
	fake := &fakeES{t: t, aliases: []string{"transactions-000002", "transactions-000001"}, bulkBody: &bytes.Buffer{}}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	b := newBackfiller(clientFor(t, server.URL), clientFor(t, server.URL), func(map[string]logDocument) map[string][]string { return nil }, options{batchSize: 10}, io.Discard)
	err := b.run(t.Context())

	require.ErrorContains(t, err, "exactly one index")
	require.ErrorContains(t, err, "transactions-000001, transactions-000002")
	require.Equal(t, 0, fake.bulkHits)
}

func TestReport_PrintsWhatIsStillMissing(t *testing.T) {
	fake := &fakeES{t: t, counts: []int64{480483, 479460}}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	var out bytes.Buffer
	require.NoError(t, report(t.Context(), clientFor(t, server.URL), &out))
	require.Contains(t, out.String(), "smart contract transactions with logs: 480483, carrying scAddresses: 479460, missing: 1023")
}
