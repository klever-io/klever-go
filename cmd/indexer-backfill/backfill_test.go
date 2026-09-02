package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/indexer"
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
// the invoked contract (reachable only through the log's own address here) and the inner
// contract, sorted; the wallet and the empty system address left out; a transaction whose
// log names no contract absent.
func TestDerivation_IsTheIndexersOwn(t *testing.T) {
	derive, err := indexerDerivation()
	require.NoError(t, err)

	invoked, inner := bech32(t, contractBytes(1)), bech32(t, contractBytes(2))
	wallet, empty := bech32(t, walletBytes(9)), bech32(t, make([]byte, 32))

	derived := derive(map[string]logDocument{
		hashOf("swap"):   {Address: invoked, Events: eventsOf(inner, wallet, empty)},
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
		`{"update":{"_id":"aa","_index":"transactions-000001","retry_on_conflict":3}}`,
		`{"doc":{"scAddresses":["klv1one","klv1three"]}}`,
		`{"update":{"_id":"bb","_index":"transactions-000001","retry_on_conflict":3}}`,
		`{"doc":{"scAddresses":["klv1two"]}}`,
		``,
	}, "\n"), string(body), "a version conflict with the live indexer must be retried, not failed")
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

func TestSearchBody_BoundsTheScrollOnlyWhenAsked(t *testing.T) {
	unbounded := (&backfiller{opts: options{}}).searchBody()
	require.Equal(t, map[string]interface{}{"match_all": map[string]interface{}{}}, unbounded["query"])

	lower := (&backfiller{opts: options{timestampFrom: 100}}).searchBody()
	require.Equal(t, map[string]interface{}{"range": map[string]interface{}{"timestamp": map[string]interface{}{"gte": int64(100)}}}, lower["query"])

	both := (&backfiller{opts: options{timestampFrom: 100, timestampTo: 200}}).searchBody()
	require.Equal(t, map[string]interface{}{"range": map[string]interface{}{"timestamp": map[string]interface{}{"gte": int64(100), "lte": int64(200)}}}, both["query"])
}

// fakeES is the slice of Elasticsearch the tool talks to, recording every request so a test
// asserts afterwards, on the test goroutine, what was sent and in which order. The product
// header is what the client checks on the first response before trusting a server.
type fakeES struct {
	mu          sync.Mutex
	aliases     []string
	fieldType   string // "" means scAddresses is not mapped on the target
	logs        map[string]logDocument
	counts      []int64
	failBulk    bool
	failCount   bool
	requests    []string // "METHOD path?query"
	bulkBody    bytes.Buffer
	scrollNext  []byte
	scrollClear []byte
}

func (f *fakeES) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")

		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)

		switch {
		case r.URL.Path == "/":
			_, _ = io.WriteString(w, `{"version":{"number":"8.4.0"},"tagline":"You Know, for Search"}`)
		case r.URL.Path == "/_alias/"+transactionsAlias:
			indices := map[string]interface{}{}
			for _, name := range f.aliases {
				indices[name] = map[string]interface{}{"aliases": map[string]interface{}{transactionsAlias: map[string]interface{}{}}}
			}
			_ = json.NewEncoder(w).Encode(indices)
		case strings.HasSuffix(r.URL.Path, "/_mapping/field/"+fieldName):
			if f.fieldType == "" {
				_, _ = io.WriteString(w, `{"transactions-000001":{"mappings":{}}}`)
				return
			}
			_, _ = io.WriteString(w, `{"transactions-000001":{"mappings":{"scAddresses":{"full_name":"scAddresses","mapping":{"scAddresses":{"type":"`+f.fieldType+`"}}}}}}`)
		case strings.HasSuffix(r.URL.Path, "/_mapping") && r.Method == http.MethodPut:
			f.fieldType = "keyword"
			_, _ = io.WriteString(w, `{"acknowledged":true}`)
		case r.URL.Path == "/"+logsIndex+"/_search":
			hits := make([]map[string]interface{}, 0, len(f.logs))
			for hash, doc := range f.logs {
				hits = append(hits, map[string]interface{}{"_id": hash, "_source": doc})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"_scroll_id": "cursor-1", "hits": map[string]interface{}{"hits": hits}})
		case r.URL.Path == "/_search/scroll" && r.Method == http.MethodDelete:
			f.scrollClear, _ = io.ReadAll(r.Body)
			_, _ = io.WriteString(w, `{"succeeded":true}`)
		case r.URL.Path == "/_search/scroll":
			f.scrollNext, _ = io.ReadAll(r.Body)
			_, _ = io.WriteString(w, `{"_scroll_id":"cursor-2","hits":{"hits":[]}}`)
		case r.URL.Path == "/_bulk" && f.failBulk:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":{"type":"cluster_block_exception"}}`)
		case r.URL.Path == "/_bulk":
			raw, _ := io.ReadAll(r.Body)
			f.bulkBody.Write(raw)
			var items []map[string]interface{}
			for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
				if bytes.HasPrefix(line, []byte(`{"update"`)) {
					items = append(items, map[string]interface{}{"update": map[string]interface{}{"status": 200, "result": "updated"}})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"items": items})
		case r.URL.Path == "/"+transactionsAlias+"/_count" && f.failCount:
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":{"type":"search_phase_execution_exception"}}`)
		case r.URL.Path == "/"+transactionsAlias+"/_count":
			count := f.counts[0]
			f.counts = f.counts[1:]
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"count": count})
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":{"type":"resource_not_found_exception"}}`)
		}
	})
}

// paths returns the recorded requests without their query strings, in order.
func (f *fakeES) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	paths := make([]string, 0, len(f.requests))
	for _, request := range f.requests {
		// "METHOD path?query" -> "path"
		withoutMethod := request[strings.Index(request, " ")+1:]
		paths = append(paths, strings.SplitN(withoutMethod, "?", 2)[0])
	}

	return paths
}

func (f *fakeES) countPath(path string) int {
	n := 0
	for _, p := range f.paths() {
		if p == path {
			n++
		}
	}

	return n
}

func (f *fakeES) request(prefix string) string {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, request := range f.requests {
		if strings.HasPrefix(request, prefix) {
			return request
		}
	}

	return ""
}

type fakeCluster struct {
	*fakeES
	server *httptest.Server
	client *elasticsearch.Client
	admin  indexer.DatabaseClientHandler
}

func startFake(t *testing.T, fake *fakeES) *fakeCluster {
	t.Helper()

	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)

	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
	require.NoError(t, err)
	admin, err := indexer.NewElasticClient(elasticsearch.Config{Addresses: []string{server.URL}})
	require.NoError(t, err)

	return &fakeCluster{fakeES: fake, server: server, client: client, admin: admin}
}

func (c *fakeCluster) backfiller(t *testing.T, opts options, out io.Writer) *backfiller {
	t.Helper()

	derive, err := indexerDerivation()
	require.NoError(t, err)

	if opts.batchSize == 0 {
		opts.batchSize = 10
	}
	if opts.scrollKeepAlive == 0 {
		opts.scrollKeepAlive = time.Minute
	}

	return newBackfiller(c.client, c.client, c.admin, derive, opts, out)
}

// TestBackfill_WritesWhatTheIndexerDerives is the whole tool against a fake cluster: the
// mapping is put before anything is written, the logs come in, the indexer's derivation
// runs, and exactly one update per transaction with contracts reaches the target, addressed
// to the concrete index behind the alias.
func TestBackfill_WritesWhatTheIndexerDerives(t *testing.T) {
	invoked, inner, wallet := bech32(t, contractBytes(1)), bech32(t, contractBytes(2)), bech32(t, walletBytes(3))
	cluster := startFake(t, &fakeES{aliases: []string{"transactions-000001"}, logs: map[string]logDocument{
		hashOf("swap"):  {Address: invoked, Events: eventsOf(inner)},
		hashOf("plain"): {Address: wallet},
	}})

	var out bytes.Buffer
	b := cluster.backfiller(t, options{}, &out)
	require.NoError(t, b.backfill(context.Background()))

	derive, err := indexerDerivation()
	require.NoError(t, err)
	want, err := bulkBody("transactions-000001", derive(cluster.logs))
	require.NoError(t, err)
	require.Equal(t, string(want), cluster.bulkBody.String(), "the bulk body must be exactly the derived updates")

	paths := cluster.paths()
	require.Equal(t, 1, cluster.countPath("/_bulk"))
	mappingAt, bulkAt := indexOf(paths, "/transactions-000001/_mapping"), indexOf(paths, "/_bulk")
	require.True(t, mappingAt >= 0 && mappingAt < bulkAt, "the mapping must be put before the first write; requests: %v", paths)

	search := cluster.request("POST /logs/_search")
	require.Contains(t, search, "scroll=60000ms", "the first page must open a scroll with the keepalive")
	require.Contains(t, search, "size=10", "the page size must follow the batch size")
	require.Contains(t, search, "_source=address%2Cevents.address")
	require.JSONEq(t, `{"scroll_id":"cursor-1","scroll":"60000ms"}`, string(cluster.scrollNext),
		"the follow-up must carry the cursor and the keepalive in the body, in the unit Elasticsearch accepts")
	require.JSONEq(t, `{"scroll_id":["cursor-2"]}`, string(cluster.scrollClear), "the last cursor must be cleared through the body")
	require.Contains(t, out.String(), "logs read: 2, with contracts: 1")
	require.Contains(t, out.String(), "updated: 1, already set: 0, no such transaction: 0, failed: 0")
}

func indexOf(paths []string, path string) int {
	for i, p := range paths {
		if p == path {
			return i
		}
	}

	return -1
}

// TestBackfill_LeavesAnUpToDateMappingAlone is the other side of the mapping guard: a target
// the new node already started against gets no mapping write, which is also what keeps the
// tool usable with a user that lacks the manage privilege on such a target.
func TestBackfill_LeavesAnUpToDateMappingAlone(t *testing.T) {
	cluster := startFake(t, &fakeES{aliases: []string{"transactions-000001"}, fieldType: "keyword", logs: map[string]logDocument{}})

	require.NoError(t, cluster.backfiller(t, options{}, io.Discard).backfill(context.Background()))
	require.Equal(t, 0, cluster.countPath("/transactions-000001/_mapping"), "requests: %v", cluster.paths())
}

// TestBackfill_RefusesATargetMappedWithAnotherType covers the state no run can repair: the
// field already mapped as text. Writing into it would make every node built with the field
// refuse to start, so the run must stop before the first bulk, and so must a dry run.
func TestBackfill_RefusesATargetMappedWithAnotherType(t *testing.T) {
	invoked := bech32(t, contractBytes(1))
	for _, dryRun := range []bool{false, true} {
		cluster := startFake(t, &fakeES{aliases: []string{"transactions-000001"}, fieldType: "text", logs: map[string]logDocument{
			hashOf("swap"): {Address: invoked},
		}})

		err := cluster.backfiller(t, options{dryRun: dryRun}, io.Discard).backfill(context.Background())
		require.ErrorContains(t, err, `maps scAddresses as "text"`, "dryRun=%v", dryRun)
		require.Equal(t, 0, cluster.countPath("/_bulk"), "dryRun=%v: nothing may be written", dryRun)
	}
}

func TestBackfill_DryRunWritesNothingAndReportsTheMapping(t *testing.T) {
	invoked := bech32(t, contractBytes(1))
	cluster := startFake(t, &fakeES{aliases: []string{"transactions-000001"}, logs: map[string]logDocument{
		hashOf("swap"): {Address: invoked},
	}})

	var out bytes.Buffer
	require.NoError(t, cluster.backfiller(t, options{dryRun: true}, &out).backfill(context.Background()))

	require.Equal(t, 0, cluster.countPath("/_bulk"), "a dry run must not touch the documents")
	require.Equal(t, 0, cluster.countPath("/transactions-000001/_mapping"), "a dry run must not touch the mapping either")
	require.Contains(t, out.String(), "mapping: scAddresses not mapped on transactions-000001 yet; a real run puts it before writing")
	require.Contains(t, out.String(), "with contracts: 1 (dry run, nothing written)")
}

// TestBackfill_RefusesAnAliasSpanningSeveralIndices covers the rollover case: a bulk update
// addressed to such an alias fails per item, so the run must stop before the first batch
// and name the indices.
func TestBackfill_RefusesAnAliasSpanningSeveralIndices(t *testing.T) {
	cluster := startFake(t, &fakeES{aliases: []string{"transactions-000002", "transactions-000001"}})

	err := cluster.backfiller(t, options{}, io.Discard).backfill(context.Background())

	require.ErrorContains(t, err, "exactly one index")
	require.ErrorContains(t, err, "transactions-000001, transactions-000002")
	require.Equal(t, 0, cluster.countPath("/_bulk"))
}

func TestBackfill_SurfacesABulkFailure(t *testing.T) {
	invoked := bech32(t, contractBytes(1))
	cluster := startFake(t, &fakeES{failBulk: true, aliases: []string{"transactions-000001"}, logs: map[string]logDocument{
		hashOf("swap"): {Address: invoked},
	}})

	require.ErrorContains(t, cluster.backfiller(t, options{}, io.Discard).backfill(context.Background()), "bulk update")
}

func TestPause_StopsWhenTheRunIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	require.ErrorIs(t, pause(ctx, time.Hour), context.Canceled)
	require.Less(t, time.Since(started), time.Second, "a cancelled run must not sit out the pause")
	require.NoError(t, pause(context.Background(), 0))
}

// TestReport_CountsWhatIsLeftDirectly pins the three counts and, above all, that "without
// it" is its own query rather than a difference: the two totals are over one population,
// but a transaction whose log names no contract sits in the first and never in the second.
func TestReport_CountsWhatIsLeftDirectly(t *testing.T) {
	cluster := startFake(t, &fakeES{counts: []int64{480483, 479460, 1023}})

	var out bytes.Buffer
	require.NoError(t, report(context.Background(), cluster.client, &out))
	require.Contains(t, out.String(), "smart contract transactions with logs: 480483, of which carrying scAddresses: 479460, without it: 1023")
	require.Equal(t, 3, cluster.countPath("/transactions/_count"), "the remainder must be counted, not computed")
}

func TestReport_SurfacesACountFailure(t *testing.T) {
	cluster := startFake(t, &fakeES{failCount: true})

	require.ErrorContains(t, report(context.Background(), cluster.client, io.Discard), "count")
}

func TestParseFlags_ReadsEveryOption(t *testing.T) {
	opts, err := parseFlags([]string{"-source-url", "http://src:9200", "-target-url", "http://dst:9200", "-batch-size", "25",
		"-pause", "2s", "-scroll-keepalive", "3m", "-timestamp-from", "5", "-timestamp-to", "9", "-dry-run"})
	require.NoError(t, err)

	require.Equal(t, options{sourceURL: "http://src:9200", targetURL: "http://dst:9200", batchSize: 25, pause: 2 * time.Second,
		scrollKeepAlive: 3 * time.Minute, timestampFrom: 5, timestampTo: 9, dryRun: true}, opts)

	_, err = parseFlags([]string{"-no-such-flag"})
	require.Error(t, err)
}

// TestRun_RefusesToStartOnBadOptions covers the validation that runs before any client is
// built: the clusters, the values that would read nothing or fail late, and a URL that
// smuggles a password past the environment.
func TestRun_RefusesToStartOnBadOptions(t *testing.T) {
	valid := options{sourceURL: "http://src:9200", targetURL: "http://dst:9200", batchSize: 10, scrollKeepAlive: time.Minute}

	for name, broken := range map[string]func(o *options){
		"-target-url is required":                     func(o *options) { o.targetURL = "" },
		"-source-url is required unless -verify-only": func(o *options) { o.sourceURL = "" },
		"-batch-size must be positive":                func(o *options) { o.batchSize = 0 },
		"-scroll-keepalive must be positive":          func(o *options) { o.scrollKeepAlive = 0 },
		"-target-url carries credentials":             func(o *options) { o.targetURL = "http://user:secret@dst:9200" },
		"-source-url carries credentials":             func(o *options) { o.sourceURL = "http://user:secret@src:9200" },
	} {
		opts := valid
		broken(&opts)
		require.ErrorContains(t, run(opts), name)
	}
}

// TestRun_EndToEnd drives main's run against the fake cluster in both modes that need no
// real chain: a dry run over the logs, and a verify-only report.
func TestRun_EndToEnd(t *testing.T) {
	invoked := bech32(t, contractBytes(1))
	cluster := startFake(t, &fakeES{aliases: []string{"transactions-000001"}, counts: []int64{3, 2, 1, 3, 2, 1}, logs: map[string]logDocument{
		hashOf("swap"): {Address: invoked},
	}})

	require.NoError(t, run(options{sourceURL: cluster.server.URL, targetURL: cluster.server.URL, batchSize: 10, scrollKeepAlive: time.Minute, dryRun: true}))
	require.Equal(t, 0, cluster.countPath("/_bulk"))

	require.NoError(t, run(options{targetURL: cluster.server.URL, batchSize: 10, scrollKeepAlive: time.Minute, verifyOnly: true}))
	require.Equal(t, 6, cluster.countPath("/transactions/_count"), "both modes end with the report")
}
