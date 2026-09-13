// indexer-backfill writes scAddresses onto transaction documents indexed before the
// indexer learned to derive it, reading each transaction's events from a logs index.
//
// The field is derived by the indexer's own ExtractDataFromLogs, fed with the log document
// as the indexer would have received it from the node, so a backfilled transaction carries
// what a freshly indexed one carries. Two things the logs cannot give back: a contract that
// ran without emitting an event was never in the log, and a failed deploy logs under the
// sender's address, so its transaction gets no field either way. The source and the target
// are separate clusters on purpose: a logs index can be shorter than the transactions it
// belongs to (one deployment's logs start months after its first smart contract
// transaction), so the run reads from whichever cluster holds the complete logs and writes
// wherever the transactions live.
//
// Before the first write the target's mapping is brought up to date through the same code
// the indexing node runs at start-up, so a backfill can run before or after that node is
// deployed: a field first written without a mapping would be typed as text, and the node
// would then refuse to start against it. That step needs the manage privilege on the
// transactions index when the property is absent, and only view_index_metadata otherwise.
//
// Writes are plain partial updates and the field is a set, so a run is idempotent and a
// failed run is repeated rather than resumed. Passwords come from the environment
// (BACKFILL_SOURCE_PASSWORD, BACKFILL_TARGET_PASSWORD), never from flags, and a URL
// carrying credentials is refused.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	hashingFactory "github.com/klever-io/klever-go/crypto/hashing/factory"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/indexer/logsevents"
	marshalFactory "github.com/klever-io/klever-go/tools/marshal/factory"
)

const addressLength = 32

func main() {
	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "indexer-backfill:", err)
		os.Exit(2)
	}

	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "indexer-backfill:", err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (options, error) {
	var opts options
	flags := flag.NewFlagSet("indexer-backfill", flag.ContinueOnError)
	flags.StringVar(&opts.sourceURL, "source-url", "", "cluster holding the complete logs index (read only)")
	flags.StringVar(&opts.sourceUser, "source-user", "", "user name for the source cluster")
	flags.StringVar(&opts.targetURL, "target-url", "", "cluster holding the transactions index that receives the field")
	flags.StringVar(&opts.targetUser, "target-user", "", "user name for the target cluster")
	flags.IntVar(&opts.batchSize, "batch-size", 500, "log documents read and transactions updated per round trip")
	flags.DurationVar(&opts.pause, "pause", 0, "pause between batches, to keep a busy cluster comfortable")
	flags.DurationVar(&opts.scrollKeepAlive, "scroll-keepalive", 5*time.Minute, "how long the source keeps the scroll cursor alive between batches")
	flags.Int64Var(&opts.timestampFrom, "timestamp-from", 0, "only logs with timestamp >= this value; 0 means unbounded. The unit is whatever the logs mapping stores, check it before relying on this")
	flags.Int64Var(&opts.timestampTo, "timestamp-to", 0, "only logs with timestamp <= this value; 0 means unbounded")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "derive and count, write nothing, and report whether the target mapping is ready")
	flags.BoolVar(&opts.verifyOnly, "verify-only", false, "skip the backfill and only report the target's counts")

	if err := flags.Parse(args); err != nil {
		return options{}, err
	}

	return opts, nil
}

// validate rejects what would otherwise fail late or silently: a missing cluster, a batch
// size that reads nothing and exits clean, a keepalive the client drops from the request,
// and credentials smuggled in through a URL.
func (o options) validate() error {
	if o.targetURL == "" {
		return errors.New("-target-url is required")
	}
	if !o.verifyOnly && o.sourceURL == "" {
		return errors.New("-source-url is required unless -verify-only is set")
	}
	if o.batchSize <= 0 {
		return errors.New("-batch-size must be positive")
	}
	if o.scrollKeepAlive <= 0 {
		return errors.New("-scroll-keepalive must be positive")
	}
	for name, raw := range map[string]string{"-target-url": o.targetURL, "-source-url": o.sourceURL} {
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if parsed.User != nil {
			return fmt.Errorf("%s carries credentials; pass the password through BACKFILL_SOURCE_PASSWORD or BACKFILL_TARGET_PASSWORD instead", name)
		}
	}

	return nil
}

func run(opts options) error {
	if err := opts.validate(); err != nil {
		return err
	}

	targetConfig := elasticsearch.Config{
		Addresses: []string{opts.targetURL},
		Username:  opts.targetUser,
		Password:  os.Getenv("BACKFILL_TARGET_PASSWORD"),
	}
	target, err := elasticsearch.NewClient(targetConfig)
	if err != nil {
		return fmt.Errorf("target client: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if opts.verifyOnly {
		return report(ctx, target, os.Stdout)
	}

	// The mapping goes through the indexer's own client so the tool and the node cannot
	// disagree on what the field is, or on when a write is refused.
	mapping, err := indexer.NewElasticClient(targetConfig)
	if err != nil {
		return fmt.Errorf("target mapping client: %w", err)
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

	if err := newBackfiller(source, target, mapping, derive, opts, os.Stdout).backfill(ctx); err != nil {
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
