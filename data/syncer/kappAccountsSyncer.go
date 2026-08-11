package syncer

import (
	"context"
	"sync"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/epochStart"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/data/trie/statistics"
	"github.com/klever-io/klever-go/tools/check"
)

var _ epochStart.AccountsDBSyncer = (*kappAccountsSyncer)(nil)

type kappAccountsSyncer struct {
	*baseAccountsSyncer
	throttler   data.GoRoutineThrottler
	syncerMutex sync.Mutex
}

// ArgsNewKappAccountsSyncer defines the arguments needed for the new account syncer
type ArgsNewKappAccountsSyncer struct {
	ArgsNewBaseAccountsSyncer
	Throttler data.GoRoutineThrottler
}

// NewKappAccountsSyncer creates a kapp account syncer
func NewKappAccountsSyncer(args ArgsNewKappAccountsSyncer) (*kappAccountsSyncer, error) {
	err := checkArgs(args.ArgsNewBaseAccountsSyncer)
	if err != nil {
		return nil, err
	}

	if check.IfNil(args.Throttler) {
		return nil, common.ErrNilThrottler
	}

	b := &baseAccountsSyncer{
		hasher:                    args.Hasher,
		marshalizer:               args.Marshalizer,
		trieSyncers:               make(map[string]data.TrieSyncer),
		dataTries:                 make(map[string]data.Trie),
		trieStorageManager:        args.TrieStorageManager,
		requestHandler:            args.RequestHandler,
		timeout:                   args.Timeout,
		cacher:                    args.Cacher,
		rootHash:                  nil,
		maxTrieLevelInMemory:      args.MaxTrieLevelInMemory,
		name:                      "kapp accounts",
		maxHardCapForMissingNodes: args.MaxHardCapForMissingNodes,
	}

	u := &kappAccountsSyncer{
		baseAccountsSyncer: b,
		throttler:          args.Throttler,
	}

	return u, nil
}

// SyncAccounts will launch the syncing method to gather all the data needed for validatorAccounts - it is a blocking method
func (k *kappAccountsSyncer) SyncAccounts(rootHash []byte) error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tss := statistics.NewTrieSyncStatistics()
	go k.printStatistics(tss, ctx)

	err := k.syncMainTrie(rootHash, common.KappTrieNodesTopic, tss, ctx)
	if err != nil {
		return err
	}

	mainTrie := k.dataTries[string(rootHash)]
	rootHashes, err := k.findAllAccountRootHashes(mainTrie, ctx)
	if err != nil {
		return err
	}

	err = k.syncAccountDataTries(rootHashes, tss, ctx)
	if err != nil {
		return err
	}

	return nil
}

func (k *kappAccountsSyncer) syncAccountDataTries(rootHashes [][]byte, ssh data.SyncStatisticsHandler, ctx context.Context) error {
	var errFound error
	errMutex := sync.Mutex{}

	wg := sync.WaitGroup{}
	wg.Add(len(rootHashes))

	for _, rootHash := range rootHashes {
		for !k.throttler.CanProcess() {
			select {
			case <-time.After(timeBetweenRetries):
				continue
			case <-ctx.Done():
				return common.ErrTimeIsOut
			}
		}

		go func(trieRootHash []byte) {
			newErr := k.syncDataTrie(trieRootHash, ssh, ctx)
			if newErr != nil {
				errMutex.Lock()
				errFound = newErr
				errMutex.Unlock()
			}
			wg.Done()
		}(rootHash)
	}

	wg.Wait()

	errMutex.Lock()
	defer errMutex.Unlock()

	return errFound
}

func (k *kappAccountsSyncer) syncDataTrie(rootHash []byte, ssh data.SyncStatisticsHandler, ctx context.Context) error {
	k.throttler.StartProcessing()
	defer k.throttler.EndProcessing()

	k.syncerMutex.Lock()
	if _, ok := k.dataTries[string(rootHash)]; ok {
		k.syncerMutex.Unlock()
		return nil
	}

	dataTrie, err := trie.NewTrie(k.trieStorageManager, k.marshalizer, k.hasher, k.maxTrieLevelInMemory)
	if err != nil {
		k.syncerMutex.Unlock()
		return err
	}

	k.dataTries[string(rootHash)] = dataTrie
	arg := trie.ArgTrieSyncer{
		RequestHandler:                 k.requestHandler,
		InterceptedNodes:               k.cacher,
		Trie:                           dataTrie,
		Topic:                          common.KappTrieNodesTopic,
		TrieSyncStatistics:             ssh,
		TimeoutBetweenTrieNodesCommits: k.timeout,
		MaxHardCapForMissingNodes:      k.maxHardCapForMissingNodes,
	}
	trieSyncer, err := trie.NewTrieSyncer(arg)
	if err != nil {
		k.syncerMutex.Unlock()
		return err
	}
	k.trieSyncers[string(rootHash)] = trieSyncer
	// Released before the blocking StartSyncing — do NOT move to defer or
	// numConcurrentTrieSyncers parallelism collapses.
	k.syncerMutex.Unlock()

	return trieSyncer.StartSyncing(rootHash, ctx)
}

func (k *kappAccountsSyncer) findAllAccountRootHashes(mainTrie data.Trie, ctx context.Context) ([][]byte, error) {
	mainRootHash, err := mainTrie.RootHash()
	if err != nil {
		return nil, err
	}

	leavesChannels, err := mainTrie.GetAllLeavesOnChannel(mainRootHash, ctx)
	if err != nil {
		return nil, err
	}

	rootHashes := make([][]byte, 0)

	// A truncated walk would leave part of the state unsynced without anyone noticing.
	err = leavesChannels.ForEach(func(leaf data.KeyValueHolder) error {
		account := state.NewEmptyKAppAccount()
		errUnmarshal := k.marshalizer.Unmarshal(account, leaf.Value())
		if errUnmarshal != nil {
			log.Trace("this must be a leaf with code", "err", errUnmarshal)
			return nil
		}

		if len(account.RootHash) > 0 {
			rootHashes = append(rootHashes, account.RootHash)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return rootHashes, nil
}
