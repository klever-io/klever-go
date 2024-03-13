package syncer

import (
	"context"
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/epochStart"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/data/trie/statistics"
	"github.com/klever-io/klever-go/tools/check"
)

var _ epochStart.AccountsDBSyncer = (*userAccountsSyncer)(nil)

var log = logger.GetOrCreate("syncer")

const timeBetweenRetries = 100 * time.Millisecond

type userAccountsSyncer struct {
	*baseAccountsSyncer
	throttler   data.GoRoutineThrottler
	syncerMutex sync.Mutex
}

// ArgsNewUserAccountsSyncer defines the arguments needed for the new account syncer
type ArgsNewUserAccountsSyncer struct {
	ArgsNewBaseAccountsSyncer
	ShardId   uint32
	Throttler data.GoRoutineThrottler
}

// NewUserAccountsSyncer creates a user account syncer
func NewUserAccountsSyncer(args ArgsNewUserAccountsSyncer) (*userAccountsSyncer, error) {
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
		name:                      "user accounts",
		maxHardCapForMissingNodes: args.MaxHardCapForMissingNodes,
	}

	u := &userAccountsSyncer{
		baseAccountsSyncer: b,
		throttler:          args.Throttler,
	}

	return u, nil
}

// SyncAccounts will launch the syncing method to gather all the data needed for userAccounts - it is a blocking method
func (u *userAccountsSyncer) SyncAccounts(rootHash []byte) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tss := statistics.NewTrieSyncStatistics()
	go u.printStatistics(tss, ctx)

	err := u.syncMainTrie(rootHash, common.AccountTrieNodesTopic, tss, ctx)
	if err != nil {
		return err
	}

	mainTrie := u.dataTries[string(rootHash)]
	rootHashes, err := u.findAllAccountRootHashes(mainTrie, ctx)
	if err != nil {
		return err
	}

	err = u.syncAccountDataTries(rootHashes, tss, ctx)
	if err != nil {
		return err
	}

	return nil
}

func (u *userAccountsSyncer) syncAccountDataTries(rootHashes [][]byte, ssh data.SyncStatisticsHandler, ctx context.Context) error {
	var errFound error
	errMutex := sync.Mutex{}

	wg := sync.WaitGroup{}
	wg.Add(len(rootHashes))

	for _, rootHash := range rootHashes {
		for {
			if u.throttler.CanProcess() {
				break
			}

			select {
			case <-time.After(timeBetweenRetries):
				continue
			case <-ctx.Done():
				return common.ErrTimeIsOut
			}
		}

		go func(trieRootHash []byte) {
			newErr := u.syncDataTrie(trieRootHash, ssh, ctx)
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

func (u *userAccountsSyncer) syncDataTrie(rootHash []byte, ssh data.SyncStatisticsHandler, ctx context.Context) error {
	u.throttler.StartProcessing()

	u.syncerMutex.Lock()
	if _, ok := u.dataTries[string(rootHash)]; ok {
		u.syncerMutex.Unlock()
		u.throttler.EndProcessing()
		return nil
	}

	dataTrie, err := trie.NewTrie(u.trieStorageManager, u.marshalizer, u.hasher, u.maxTrieLevelInMemory)
	if err != nil {
		u.syncerMutex.Unlock()
		return err
	}

	u.dataTries[string(rootHash)] = dataTrie
	arg := trie.ArgTrieSyncer{
		RequestHandler:                 u.requestHandler,
		InterceptedNodes:               u.cacher,
		Trie:                           dataTrie,
		Topic:                          common.AccountTrieNodesTopic,
		TrieSyncStatistics:             ssh,
		TimeoutBetweenTrieNodesCommits: u.timeout,
		MaxHardCapForMissingNodes:      u.maxHardCapForMissingNodes,
	}
	trieSyncer, err := trie.NewTrieSyncer(arg)
	if err != nil {
		u.syncerMutex.Unlock()
		return err
	}
	u.trieSyncers[string(rootHash)] = trieSyncer
	u.syncerMutex.Unlock()

	err = trieSyncer.StartSyncing(rootHash, ctx)
	if err != nil {
		return err
	}

	u.throttler.EndProcessing()

	return nil
}

func (u *userAccountsSyncer) findAllAccountRootHashes(mainTrie data.Trie, ctx context.Context) ([][]byte, error) {
	mainRootHash, err := mainTrie.RootHash()
	if err != nil {
		return nil, err
	}

	leavesChannel, err := mainTrie.GetAllLeavesOnChannel(mainRootHash, ctx)
	if err != nil {
		return nil, err
	}

	rootHashes := make([][]byte, 0)
	for leaf := range leavesChannel {
		account := state.NewEmptyUserAccount()
		err = u.marshalizer.Unmarshal(account, leaf.Value())
		if err != nil {
			log.Trace("this must be a leaf with code", "err", err)
			continue
		}

		if len(account.RootHash) > 0 {
			rootHashes = append(rootHashes, account.RootHash)
		}
	}

	return rootHashes, nil
}
