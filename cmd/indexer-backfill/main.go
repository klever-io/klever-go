// indexer-backfill writes scAddresses onto transaction documents indexed before the
// indexer learned to derive it, reading each transaction's events from a logs index.
//
// The field is derived by the indexer's own ExtractDataFromLogs, fed with the log document
// as the indexer would have seen it, so a backfilled transaction carries exactly what a
// freshly indexed one carries. The source and the target are separate clusters on purpose:
// a logs index can be shorter than the transactions it belongs to (one deployment's logs
// start months after its first smart contract transaction), so the run reads from whichever
// cluster holds the complete logs and writes wherever the transactions live.
//
// Writes are plain partial updates and the field is a set, so a run is idempotent and a
// failed run is repeated rather than resumed. Passwords come from the environment
// (BACKFILL_SOURCE_PASSWORD, BACKFILL_TARGET_PASSWORD), never from flags.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	hashingFactory "github.com/klever-io/klever-go/crypto/hashing/factory"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/indexer/logsevents"
	marshalFactory "github.com/klever-io/klever-go/tools/marshal/factory"
)

const addressLength = 32

func main() {
	var opts options
	flag.StringVar(&opts.sourceURL, "source-url", "", "cluster holding the complete logs index (read only)")
	flag.StringVar(&opts.sourceUser, "source-user", "", "user name for the source cluster")
	flag.StringVar(&opts.targetURL, "target-url", "", "cluster holding the transactions index that receives the field")
	flag.StringVar(&opts.targetUser, "target-user", "", "user name for the target cluster")
	flag.IntVar(&opts.batchSize, "batch-size", 500, "log documents read and transactions updated per round trip")
	flag.DurationVar(&opts.pause, "pause", 0, "pause between batches, to keep a busy cluster comfortable")
	flag.DurationVar(&opts.scrollKeepAlive, "scroll-keepalive", 5*time.Minute, "how long the source keeps the scroll cursor alive between batches")
	flag.Int64Var(&opts.timestampFrom, "timestamp-from", 0, "only logs with timestamp >= this value; 0 means unbounded. The unit is whatever the logs mapping stores, check it before relying on this")
	flag.Int64Var(&opts.timestampTo, "timestamp-to", 0, "only logs with timestamp <= this value; 0 means unbounded")
	flag.BoolVar(&opts.dryRun, "dry-run", false, "derive and count, write nothing")
	flag.BoolVar(&opts.verifyOnly, "verify-only", false, "skip the backfill and only report the target's counts")
	flag.Parse()

	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "indexer-backfill:", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	if opts.targetURL == "" {
		return fmt.Errorf("-target-url is required")
	}
	if !opts.verifyOnly && opts.sourceURL == "" {
		return fmt.Errorf("-source-url is required unless -verify-only is set")
	}

	target, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{opts.targetURL},
		Username:  opts.targetUser,
		Password:  os.Getenv("BACKFILL_TARGET_PASSWORD"),
	})
	if err != nil {
		return fmt.Errorf("target client: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if opts.verifyOnly {
		return report(ctx, target, os.Stdout)
	}

	source, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{opts.sourceURL},
		Username:  opts.sourceUser,
		Password:  os.Getenv("BACKFILL_SOURCE_PASSWORD"),
	})
	if err != nil {
		return fmt.Errorf("source client: %w", err)
	}

	derive, err := indexerDerivation()
	if err != nil {
		return err
	}

	backfiller := newBackfiller(source, target, derive, opts, os.Stdout)
	if err := backfiller.run(ctx); err != nil {
		return err
	}

	return report(ctx, target, os.Stdout)
}

// indexerDerivation wires the indexer's own logs processor with the same address converter
// the node uses, so the field written here is the field the indexer writes.
func indexerDerivation() (deriveFunc, error) {
	converter, err := pubkeyConverter.NewBech32PubkeyConverter(addressLength)
	if err != nil {
		return nil, fmt.Errorf("address converter: %w", err)
	}

	marshalizer, err := marshalFactory.NewInternalMarshalizer()
	if err != nil {
		return nil, fmt.Errorf("marshalizer: %w", err)
	}

	hasher, err := hashingFactory.NewDefaultHasher()
	if err != nil {
		return nil, fmt.Errorf("hasher: %w", err)
	}

	processor, err := logsevents.NewLogsAndEventsProcessor(logsevents.ArgsLogsAndEventsProcessor{
		PubKeyConverter: converter,
		Marshalizer:     marshalizer,
		Hasher:          hasher,
	})
	if err != nil {
		return nil, fmt.Errorf("logs processor: %w", err)
	}

	return newDerivation(processor, converter), nil
}
