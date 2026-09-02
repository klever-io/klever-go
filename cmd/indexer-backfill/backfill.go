package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/klever-io/klever-go/core"
	nodeData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
)

const (
	logsIndex         = "logs"
	transactionsAlias = "transactions"
	fieldName         = "scAddresses"
)

type options struct {
	sourceURL, sourceUser string
	targetURL, targetUser string
	batchSize             int
	pause                 time.Duration
	scrollKeepAlive       time.Duration
	timestampFrom         int64
	timestampTo           int64
	dryRun                bool
	verifyOnly            bool
}

// logDocument is the slice of a logs document the derivation needs.
type logDocument struct {
	Address string `json:"address"`
	Events  []struct {
		Address string `json:"address"`
	} `json:"events"`
}

// deriveFunc turns log documents, keyed by transaction hash, into the scAddresses each
// transaction gets. Hashes whose logs name no contract are absent from the result.
type deriveFunc func(docs map[string]logDocument) map[string][]string

// logsExtractor is the one indexer method the derivation relies on.
type logsExtractor interface {
	ExtractDataFromLogs(pool *indexer.Pool, txs []*data.Transaction, timestamp int64) *data.PreparedLogsResults
}

// newDerivation feeds each log document to the indexer exactly as the indexer would have
// received it from the node: addresses decoded back to bytes, the hash decoded back to the
// raw bytes ExtractDataFromLogs hex-encodes to find the transaction. Whatever the indexer
// derives is what gets written. An address that does not decode is skipped as if it were
// not a contract, which is also what the indexer does with a wallet.
func newDerivation(extractor logsExtractor, converter core.PubkeyConverter) deriveFunc {
	decode := func(address string) []byte {
		decoded, err := converter.Decode(address)
		if err != nil {
			return nil
		}

		return decoded
	}

	return func(docs map[string]logDocument) map[string][]string {
		txs := make([]*data.Transaction, 0, len(docs))
		pool := &indexer.Pool{Logs: make([]*nodeData.LogData, 0, len(docs))}

		for hash, doc := range docs {
			rawHash, err := hex.DecodeString(hash)
			if err != nil {
				continue
			}

			events := make([]*transaction.Event, 0, len(doc.Events))
			for _, event := range doc.Events {
				events = append(events, &transaction.Event{Address: decode(event.Address)})
			}

			txs = append(txs, &data.Transaction{Hash: hash})
			pool.Logs = append(pool.Logs, &nodeData.LogData{
				LogHandler: &transaction.Log{Address: decode(doc.Address), Events: events},
				TxHash:     string(rawHash),
			})
		}

		extractor.ExtractDataFromLogs(pool, txs, 0)

		derived := make(map[string][]string, len(txs))
		for _, tx := range txs {
			if len(tx.SCAddresses) > 0 {
				derived[tx.Hash] = tx.SCAddresses
			}
		}

		return derived
	}
}

type backfiller struct {
	source, target *elasticsearch.Client
	derive         deriveFunc
	opts           options
	out            io.Writer
}

// logf writes progress; a failed write to the progress writer is not a reason to stop a backfill.
func (b *backfiller) logf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(b.out, format, args...)
}

func newBackfiller(source, target *elasticsearch.Client, derive deriveFunc, opts options, out io.Writer) *backfiller {
	return &backfiller{source: source, target: target, derive: derive, opts: opts, out: out}
}

// tally is what a run reports at the end.
type tally struct {
	read, withContracts, updated, unchanged, missing, failed int
}

func (b *backfiller) run(ctx context.Context) error {
	index, err := resolveSingleIndex(ctx, b.target, transactionsAlias)
	if err != nil {
		return err
	}
	b.logf("target: %s (alias %s)\n", index, transactionsAlias)

	var totals tally
	err = b.scroll(ctx, func(docs map[string]logDocument) error {
		derived := b.derive(docs)
		totals.read += len(docs)
		totals.withContracts += len(derived)

		if b.opts.dryRun {
			return nil
		}

		result, err := b.apply(ctx, index, derived)
		if err != nil {
			return err
		}
		totals.updated += result.updated
		totals.unchanged += result.unchanged
		totals.missing += result.missing
		totals.failed += result.failed

		if b.opts.pause > 0 {
			time.Sleep(b.opts.pause)
		}

		return nil
	})
	if err != nil {
		return err
	}

	mode := "written"
	if b.opts.dryRun {
		mode = "dry run, nothing written"
	}
	b.logf("logs read: %d, with contracts: %d (%s)\n", totals.read, totals.withContracts, mode)
	if !b.opts.dryRun {
		b.logf("updated: %d, already set: %d, no such transaction: %d, failed: %d\n",
			totals.updated, totals.unchanged, totals.missing, totals.failed)
		if totals.failed > 0 {
			return fmt.Errorf("%d updates failed; rerun after fixing the cause, the run is idempotent", totals.failed)
		}
	}

	return nil
}

// resolveSingleIndex returns the one concrete index behind alias. A bulk update addressed
// to an alias needs a single index behind it, so an alias spanning several (a rollover)
// is refused up front with the list, rather than failing item by item.
func resolveSingleIndex(ctx context.Context, client *elasticsearch.Client, alias string) (string, error) {
	res, err := client.Indices.GetAlias(client.Indices.GetAlias.WithContext(ctx), client.Indices.GetAlias.WithName(alias))
	if err != nil {
		return "", fmt.Errorf("resolve alias %s: %w", alias, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return "", fmt.Errorf("resolve alias %s: %s", alias, res.String())
	}

	var indices map[string]json.RawMessage
	if err := json.NewDecoder(res.Body).Decode(&indices); err != nil {
		return "", fmt.Errorf("resolve alias %s: %w", alias, err)
	}

	names := make([]string, 0, len(indices))
	for name := range indices {
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) != 1 {
		return "", fmt.Errorf("alias %s must point at exactly one index, found %d: %s", alias, len(names), strings.Join(names, ", "))
	}

	return names[0], nil
}

// scroll walks the whole source logs index in batches and hands each batch to visit.
func (b *backfiller) scroll(ctx context.Context, visit func(map[string]logDocument) error) error {
	body, err := json.Marshal(b.searchBody())
	if err != nil {
		return err
	}

	res, err := b.source.Search(
		b.source.Search.WithContext(ctx),
		b.source.Search.WithIndex(logsIndex),
		b.source.Search.WithBody(bytes.NewReader(body)),
		b.source.Search.WithSize(b.opts.batchSize),
		b.source.Search.WithScroll(b.opts.scrollKeepAlive),
		b.source.Search.WithSource("address", "events.address"),
	)
	if err != nil {
		return fmt.Errorf("search %s: %w", logsIndex, err)
	}

	scrollID := ""
	defer func() {
		if scrollID == "" {
			return
		}
		clear, err := b.source.ClearScroll(b.source.ClearScroll.WithScrollID(scrollID))
		if err == nil {
			_ = clear.Body.Close()
		}
	}()

	for {
		page, err := readPage(res)
		if err != nil {
			return err
		}
		scrollID = page.scrollID

		if len(page.docs) == 0 {
			return nil
		}
		if err := visit(page.docs); err != nil {
			return err
		}

		res, err = b.source.Scroll(
			b.source.Scroll.WithContext(ctx),
			b.source.Scroll.WithScrollID(scrollID),
			b.source.Scroll.WithScroll(b.opts.scrollKeepAlive),
		)
		if err != nil {
			return fmt.Errorf("scroll %s: %w", logsIndex, err)
		}
	}
}

func (b *backfiller) searchBody() map[string]interface{} {
	query := map[string]interface{}{"match_all": map[string]interface{}{}}
	if b.opts.timestampFrom > 0 || b.opts.timestampTo > 0 {
		bounds := map[string]interface{}{}
		if b.opts.timestampFrom > 0 {
			bounds["gte"] = b.opts.timestampFrom
		}
		if b.opts.timestampTo > 0 {
			bounds["lte"] = b.opts.timestampTo
		}
		query = map[string]interface{}{"range": map[string]interface{}{"timestamp": bounds}}
	}

	return map[string]interface{}{
		"query": query,
		"sort":  []interface{}{"_doc"},
	}
}

type page struct {
	scrollID string
	docs     map[string]logDocument
}

func readPage(res *esapi.Response) (page, error) {
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return page{}, fmt.Errorf("search %s: %s", logsIndex, res.String())
	}

	var body struct {
		ScrollID string `json:"_scroll_id"`
		Hits     struct {
			Hits []struct {
				ID     string      `json:"_id"`
				Source logDocument `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return page{}, fmt.Errorf("search %s: %w", logsIndex, err)
	}

	docs := make(map[string]logDocument, len(body.Hits.Hits))
	for _, hit := range body.Hits.Hits {
		docs[hit.ID] = hit.Source
	}

	return page{scrollID: body.ScrollID, docs: docs}, nil
}

type applyResult struct {
	updated, unchanged, missing, failed int
}

// apply writes one batch of derived fields as partial updates. A transaction the target
// does not hold is counted, not failed: a logs index can hold a hash its transactions
// index never received, and a missing document is not something this tool should create.
func (b *backfiller) apply(ctx context.Context, index string, derived map[string][]string) (applyResult, error) {
	if len(derived) == 0 {
		return applyResult{}, nil
	}

	body, err := bulkBody(index, derived)
	if err != nil {
		return applyResult{}, err
	}

	res, err := b.target.Bulk(bytes.NewReader(body), b.target.Bulk.WithContext(ctx))
	if err != nil {
		return applyResult{}, fmt.Errorf("bulk update: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return applyResult{}, fmt.Errorf("bulk update: %s", res.String())
	}

	return readBulkResult(res.Body, b.out)
}

// bulkBody builds the NDJSON for one batch: an update action per transaction, each carrying
// only the field, in hash order so a body is reproducible.
func bulkBody(index string, derived map[string][]string) ([]byte, error) {
	hashes := make([]string, 0, len(derived))
	for hash := range derived {
		hashes = append(hashes, hash)
	}
	sort.Strings(hashes)

	var body bytes.Buffer
	for _, hash := range hashes {
		action, err := json.Marshal(map[string]interface{}{"update": map[string]string{"_index": index, "_id": hash}})
		if err != nil {
			return nil, err
		}
		doc, err := json.Marshal(map[string]interface{}{"doc": map[string]interface{}{fieldName: derived[hash]}})
		if err != nil {
			return nil, err
		}
		body.Write(action)
		body.WriteByte('\n')
		body.Write(doc)
		body.WriteByte('\n')
	}

	return body.Bytes(), nil
}

func readBulkResult(body io.Reader, out io.Writer) (applyResult, error) {
	var response struct {
		Items []struct {
			Update struct {
				ID     string `json:"_id"`
				Status int    `json:"status"`
				Result string `json:"result"`
				Error  *struct {
					Type   string `json:"type"`
					Reason string `json:"reason"`
				} `json:"error"`
			} `json:"update"`
		} `json:"items"`
	}
	if err := json.NewDecoder(body).Decode(&response); err != nil {
		return applyResult{}, fmt.Errorf("bulk response: %w", err)
	}

	var result applyResult
	for _, item := range response.Items {
		switch {
		case item.Update.Error != nil && item.Update.Error.Type == "document_missing_exception":
			result.missing++
		case item.Update.Error != nil:
			result.failed++
			_, _ = fmt.Fprintf(out, "failed %s: %s: %s\n", item.Update.ID, item.Update.Error.Type, item.Update.Error.Reason)
		case item.Update.Result == "noop":
			result.unchanged++
		default:
			result.updated++
		}
	}

	return result, nil
}

// report prints the two counts that say whether the backfill is complete on the target:
// smart contract transactions the indexer saw logs for, against transactions carrying the
// field. The first minus the second is what is still missing.
func report(ctx context.Context, target *elasticsearch.Client, out io.Writer) error {
	withLogs, err := count(ctx, target, map[string]interface{}{
		"query": map[string]interface{}{"bool": map[string]interface{}{"filter": []interface{}{
			map[string]interface{}{"term": map[string]interface{}{"contract.type": 63}},
			map[string]interface{}{"term": map[string]interface{}{"hasLogs": true}},
		}}},
	})
	if err != nil {
		return err
	}

	withField, err := count(ctx, target, map[string]interface{}{
		"query": map[string]interface{}{"exists": map[string]interface{}{"field": fieldName}},
	})
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "target %s: smart contract transactions with logs: %d, carrying %s: %d, missing: %d\n",
		transactionsAlias, withLogs, fieldName, withField, withLogs-withField)

	return nil
}

func count(ctx context.Context, client *elasticsearch.Client, query map[string]interface{}) (int64, error) {
	body, err := json.Marshal(query)
	if err != nil {
		return 0, err
	}

	res, err := client.Count(
		client.Count.WithContext(ctx),
		client.Count.WithIndex(transactionsAlias),
		client.Count.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return 0, fmt.Errorf("count: %s", res.String())
	}

	var response struct {
		Count int64 `json:"count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}

	return response.Count, nil
}
