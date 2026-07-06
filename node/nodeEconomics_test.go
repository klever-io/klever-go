package node

import (
	"errors"
	"math"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/kapp"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/keyValStorage"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
)

// marketKappEscrowStub stubs only GetMarketEscrowTotal; other MarketKapp calls are not expected.
type marketKappEscrowStub struct {
	kapp.MarketKapp
	total int64
	err   error
}

func (m *marketKappEscrowStub) GetMarketEscrowTotal() (int64, error) { return m.total, m.err }

// trieWithKeys returns a trie stub whose leaves channel yields the given keys (values via GetStorage).
func trieWithKeys(keys ...string) *mock.TrieStub {
	return &mock.TrieStub{
		GetAllLeavesOnChannelCalled: func(_ []byte) (chan data.KeyValueHolder, error) {
			ch := make(chan data.KeyValueHolder, len(keys))
			for _, k := range keys {
				ch <- keyValStorage.NewKeyValStorage([]byte(k), nil)
			}
			close(ch)
			return ch, nil
		},
	}
}

// newEconomicsTestNode builds a minimal Node whose KApp adapter returns an account whose data
// trie yields the given key->value records (values delivered via GetStorage, already trimmed).
func newEconomicsTestNode(records map[string][]byte) *Node {
	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	trieStub := trieWithKeys(keys...)
	app := &mock.KAppAccountHandlerStub{
		DataTrieCalled:    func() data.Trie { return trieStub },
		GetRootHashCalled: func() []byte { return []byte("root") },
		GetStorageCalled:  func(key []byte) []byte { return records[string(key)] },
	}
	return &Node{
		kapps: &mock.AccountsStub{
			LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) { return app, nil },
		},
		internalMarshalizer: marshal.NewProtoMarshalizer(),
	}
}

func TestNode_loadFeesPoolKLVTotal(t *testing.T) {
	t.Parallel()

	marshalizer := marshal.NewProtoMarshalizer()
	records := make(map[string][]byte)
	// NEG exercises the negative guard: a corrupt balance is skipped, not summed.
	for id, klv := range map[string]int64{"KLV": 1000, "ABC-1q2w": 2500, "ZERO": 0, "NEG": -50} {
		raw, err := marshalizer.Marshal(&kdafeespool.KDAFeesPoolData{KLVBalance: klv})
		require.NoError(t, err)
		records[id] = raw
	}

	n := newEconomicsTestNode(records)
	total, err := n.loadFeesPoolKLVTotal()
	require.NoError(t, err)
	require.Equal(t, int64(3500), total) // 1000 + 2500 + 0, NEG skipped
}

func TestNode_loadFPRPoolKLVTotal(t *testing.T) {
	t.Parallel()

	marshalizer := marshal.NewProtoMarshalizer()
	// total per asset = CurrentFPRAmount + Σ(TotalAmount-TotalClaimed) where the diff is > 0.
	// TotalAmount/CurrentFPRAmount are KLV; the KDAS map (non-KLV rewards) must be excluded.
	stakings := map[string]*kapps.StakingData{
		"A": {CurrentFPRAmount: 100, FPR: []*kapps.FPRData{{
			TotalAmount: 500, TotalClaimed: 200,
			// non-KLV reward (9999) must NOT be added to the KLV total.
			KDAS: map[string]*kapps.KDAFPRData{"OTHER-1q2w": {TotalAmount: 9999, TotalClaimed: 0}},
		}}}, // 100 + 300 = 400 (KDAS excluded)
		"B": {CurrentFPRAmount: 50, FPR: []*kapps.FPRData{{TotalAmount: 1000, TotalClaimed: 1000}}}, // 50 + 0 = 50
		"C": {CurrentFPRAmount: 0, FPR: []*kapps.FPRData{{TotalAmount: 100, TotalClaimed: 150}}},    // 0 + skip(neg) = 0
		"D": {CurrentFPRAmount: 25, FPR: []*kapps.FPRData{
			{TotalAmount: 300, TotalClaimed: 100},
			{TotalAmount: 200, TotalClaimed: 200},
		}}, // 25 + 200 + 0 = 225
	}
	records := make(map[string][]byte)
	for id, s := range stakings {
		raw, err := marshalizer.Marshal(s)
		require.NoError(t, err)
		records[string(kdautils.ToKDAKey([]byte(id), nil))] = raw
	}
	// a non-KDA-prefixed leaf must be filtered out, not decoded into the total
	records["garbage-key"] = []byte{0xff, 0xfe, 0xfd}

	n := newEconomicsTestNode(records)
	total, err := n.loadFPRPoolKLVTotal()
	require.NoError(t, err)
	require.Equal(t, int64(675), total) // 400 + 50 + 0 + 225
}

func TestNode_scanKAppDataTrie_nilTrie(t *testing.T) {
	t.Parallel()

	app := &mock.KAppAccountHandlerStub{
		DataTrieCalled: func() data.Trie { return nil },
	}
	n := &Node{
		kapps: &mock.AccountsStub{
			LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) { return app, nil },
		},
		internalMarshalizer: marshal.NewProtoMarshalizer(),
	}

	called := false
	err := n.scanKAppDataTrie([]byte("addr"), nil, func(_ []byte) error { called = true; return nil })
	require.NoError(t, err)
	require.False(t, called)
}

func TestNode_scanKAppDataTrie_channelError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("walk failed")
	app := &mock.KAppAccountHandlerStub{
		DataTrieCalled: func() data.Trie {
			return &mock.TrieStub{GetAllLeavesOnChannelCalled: func(_ []byte) (chan data.KeyValueHolder, error) {
				return nil, expectedErr
			}}
		},
		GetRootHashCalled: func() []byte { return []byte("root") },
	}
	n := &Node{
		kapps: &mock.AccountsStub{
			LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) { return app, nil },
		},
		internalMarshalizer: marshal.NewProtoMarshalizer(),
	}

	err := n.scanKAppDataTrie([]byte("addr"), nil, func(_ []byte) error { return nil })
	require.ErrorIs(t, err, expectedErr)
}

func TestNode_loadKAppAccount_errors(t *testing.T) {
	t.Parallel()

	t.Run("load error propagates", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("load failed")
		n := &Node{kapps: &mock.AccountsStub{
			LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) { return nil, expectedErr },
		}}
		app, err := n.loadKAppAccount([]byte("addr"))
		require.ErrorIs(t, err, expectedErr)
		require.Nil(t, app)
	})

	t.Run("non-KApp account is a wrong type assertion", func(t *testing.T) {
		t.Parallel()
		n := &Node{kapps: &mock.AccountsStub{
			LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) { return state.NewEmptyPeerAccount(), nil },
		}}
		app, err := n.loadKAppAccount([]byte("addr"))
		require.ErrorIs(t, err, common.ErrWrongTypeAssertion)
		require.Nil(t, app)
	})
}

func TestNode_loadSystemAccountKLV(t *testing.T) {
	t.Parallel()

	t.Run("returns the system-account KLV balance", func(t *testing.T) {
		t.Parallel()
		app := &mock.KAppAccountHandlerStub{
			GetUserKDACalled: func(_ []byte, _ []byte) (*kapps.UserKDA, error) {
				return &kapps.UserKDA{Balance: 77}, nil
			},
		}
		n := &Node{kapps: &mock.AccountsStub{
			LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) { return app, nil },
		}}
		balance, err := n.loadSystemAccountKLV()
		require.NoError(t, err)
		require.Equal(t, int64(77), balance)
	})

	t.Run("propagates GetUserKDA error", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("kda read failed")
		app := &mock.KAppAccountHandlerStub{
			GetUserKDACalled: func(_ []byte, _ []byte) (*kapps.UserKDA, error) { return nil, expectedErr },
		}
		n := &Node{kapps: &mock.AccountsStub{
			LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) { return app, nil },
		}}
		balance, err := n.loadSystemAccountKLV()
		require.ErrorIs(t, err, expectedErr)
		require.Zero(t, balance)
	})
}

func TestNode_loadKAppHeldTotals_errors(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("kapp read failed")

	t.Run("pending rewards error propagates", func(t *testing.T) {
		t.Parallel()
		n := &Node{kappController: &mock.KappsControllerMock{
			ValidatorsKapp: &mock.ValidatorsKAppStub{
				GetPendingRewardsTotalCalled: func() (int64, error) { return 0, expectedErr },
			},
		}}
		_, _, err := n.loadKAppHeldTotals()
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("market escrow error propagates", func(t *testing.T) {
		t.Parallel()
		n := &Node{kappController: &mock.KappsControllerMock{
			ValidatorsKapp: &mock.ValidatorsKAppStub{
				GetPendingRewardsTotalCalled: func() (int64, error) { return 1, nil },
			},
			MarketKapp: &marketKappEscrowStub{err: expectedErr},
		}}
		_, _, err := n.loadKAppHeldTotals()
		require.ErrorIs(t, err, expectedErr)
	})
}

func TestAddGuarded(t *testing.T) {
	t.Parallel()

	total := int64(math.MaxInt64 - 1)
	addGuarded(&total, 5, "test") // would overflow, skipped
	require.Equal(t, int64(math.MaxInt64-1), total)
	addGuarded(&total, -1, "test") // negative, skipped
	require.Equal(t, int64(math.MaxInt64-1), total)
	addGuarded(&total, 1, "test")
	require.Equal(t, int64(math.MaxInt64), total)
}

// newFullEconomicsNode builds a Node where every economics dependency succeeds. The returned apps
// map is captured by the KApp adapter, so tests can swap per-address behavior before calling.
func newFullEconomicsNode(t *testing.T) (*Node, map[string]state.AccountHandler) {
	marshalizer := marshal.NewProtoMarshalizer()
	klvKey := string(kdautils.ToKDAKey(kdautils.KLVIdentifier, nil))

	kdaRaw, err := marshalizer.Marshal(&kapps.KDAData{
		InitialSupply:     10_000,
		MaxSupply:         100_000,
		MintedValue:       500,
		BurnedValue:       200,
		CirculatingSupply: 9_000,
	})
	require.NoError(t, err)
	kdaApp := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{RetrieveValueCalled: func(key []byte) ([]byte, error) {
				require.Equal(t, klvKey, string(key))
				return kdaRaw, nil
			}}
		},
	}

	feesRaw, err := marshalizer.Marshal(&kdafeespool.KDAFeesPoolData{KLVBalance: 300})
	require.NoError(t, err)
	feesApp := &mock.KAppAccountHandlerStub{
		DataTrieCalled:    func() data.Trie { return trieWithKeys("KLV") },
		GetRootHashCalled: func() []byte { return []byte("root") },
		GetStorageCalled:  func(_ []byte) []byte { return feesRaw },
	}

	sysApp := &mock.KAppAccountHandlerStub{
		GetUserKDACalled: func(_ []byte, _ []byte) (*kapps.UserKDA, error) {
			return &kapps.UserKDA{Balance: 77}, nil
		},
	}

	// one record serves both the FPR-pool scan (leaf) and loadStakingData (tracker retrieve)
	stakingRaw, err := marshalizer.Marshal(&kapps.StakingData{
		TotalStaked:      5_000,
		CurrentFPRAmount: 40,
		FPR:              []*kapps.FPRData{{TotalAmount: 100, TotalClaimed: 60}},
	})
	require.NoError(t, err)
	stakingApp := &mock.KAppAccountHandlerStub{
		DataTrieCalled:    func() data.Trie { return trieWithKeys(klvKey) },
		GetRootHashCalled: func() []byte { return []byte("root") },
		GetStorageCalled:  func(_ []byte) []byte { return stakingRaw },
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{RetrieveValueCalled: func(_ []byte) ([]byte, error) {
				return stakingRaw, nil
			}}
		},
	}

	apps := map[string]state.AccountHandler{
		string(kapps.KDAKAppAddress):           kdaApp,
		string(kapps.KDAFeesPoolKAppAddress):   feesApp,
		string(kapps.SystemAccountKAppAddress): sysApp,
		string(kapps.StakingKAppAddress):       stakingApp,
	}

	mkPeer := func(fees int64) state.PeerAccountHandler {
		p := state.NewEmptyPeerAccount()
		p.AddToAccumulatedFees(fees)
		return p
	}

	n := &Node{
		blkc: &mock.BlockChainMock{GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return nil }},
		kapps: &mock.AccountsStub{LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
			app, ok := apps[string(address)]
			require.True(t, ok, "unexpected KApp address")
			return app, nil
		}},
		kappController: &mock.KappsControllerMock{
			ValidatorsKapp: &mock.ValidatorsKAppStub{
				GetPendingRewardsTotalCalled: func() (int64, error) { return 111, nil },
			},
			MarketKapp: &marketKappEscrowStub{total: 222},
		},
		validatorStatistics: &mock.ValidatorStatisticsProcessorStub{
			LastFinalizedRootHashCalled: func() []byte { return []byte("root") },
			ListPeerAccountsCalled: func(_ []byte) ([]state.PeerAccountHandler, error) {
				return []state.PeerAccountHandler{mkPeer(25), mkPeer(15)}, nil
			},
		},
		internalMarshalizer: marshalizer,
	}
	return n, apps
}

func TestNode_GetEconomics(t *testing.T) {
	t.Parallel()

	n, _ := newFullEconomicsNode(t)

	resp, err := n.GetEconomics()
	require.NoError(t, err)
	require.Equal(t, &models.EconomicsResponse{
		InitialSupply:           10_000,
		MaxSupply:               100_000,
		MintedValue:             500,
		BurnedValue:             200,
		CirculatingSupply:       9_000,
		TotalStaked:             5_000,
		PendingRewardsTotal:     111,
		MarketEscrowTotal:       222,
		FeesPoolKLVTotal:        300,
		FPRPoolTotal:            80, // 40 current + (100-60) unclaimed
		AccumulatedFeesTotal:    40, // 25 + 15
		SystemAccountKLVBalance: 77,
	}, resp)

	cached, err := n.GetEconomics()
	require.NoError(t, err)
	require.Same(t, resp, cached) // memoized within the block
}

func TestNode_computeEconomics_paths(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("dependency failed")
	klvKey := string(kdautils.ToKDAKey(kdautils.KLVIdentifier, nil))

	t.Run("asset load error propagates", func(t *testing.T) {
		t.Parallel()
		n, _ := newFullEconomicsNode(t)
		n.kapps = &mock.AccountsStub{
			LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) { return nil, expectedErr },
		}
		_, err := n.computeEconomics()
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("held totals error propagates", func(t *testing.T) {
		t.Parallel()
		n, _ := newFullEconomicsNode(t)
		n.kappController = &mock.KappsControllerMock{
			ValidatorsKapp: &mock.ValidatorsKAppStub{
				GetPendingRewardsTotalCalled: func() (int64, error) { return 0, expectedErr },
			},
		}
		_, err := n.computeEconomics()
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("fees pool decode error propagates", func(t *testing.T) {
		t.Parallel()
		n, apps := newFullEconomicsNode(t)
		apps[string(kapps.KDAFeesPoolKAppAddress)] = &mock.KAppAccountHandlerStub{
			DataTrieCalled:    func() data.Trie { return trieWithKeys("KLV") },
			GetRootHashCalled: func() []byte { return []byte("root") },
			GetStorageCalled:  func(_ []byte) []byte { return []byte{0xff, 0xfe, 0xfd} },
		}
		_, err := n.computeEconomics()
		require.Error(t, err)
	})

	t.Run("system account error propagates", func(t *testing.T) {
		t.Parallel()
		n, apps := newFullEconomicsNode(t)
		apps[string(kapps.SystemAccountKAppAddress)] = &mock.KAppAccountHandlerStub{
			GetUserKDACalled: func(_ []byte, _ []byte) (*kapps.UserKDA, error) { return nil, expectedErr },
		}
		_, err := n.computeEconomics()
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("accumulated fees error propagates", func(t *testing.T) {
		t.Parallel()
		n, _ := newFullEconomicsNode(t)
		n.validatorStatistics = &mock.ValidatorStatisticsProcessorStub{
			LastFinalizedRootHashCalled: func() []byte { return []byte("root") },
			ListPeerAccountsCalled: func(_ []byte) ([]state.PeerAccountHandler, error) {
				return nil, expectedErr
			},
		}
		_, err := n.computeEconomics()
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("staking read error propagates", func(t *testing.T) {
		t.Parallel()
		n, apps := newFullEconomicsNode(t)
		marshalizer := marshal.NewProtoMarshalizer()
		stakingRaw, err := marshalizer.Marshal(&kapps.StakingData{CurrentFPRAmount: 40})
		require.NoError(t, err)
		apps[string(kapps.StakingKAppAddress)] = &mock.KAppAccountHandlerStub{
			DataTrieCalled:    func() data.Trie { return trieWithKeys(klvKey) },
			GetRootHashCalled: func() []byte { return []byte("root") },
			GetStorageCalled:  func(_ []byte) []byte { return stakingRaw },
			DataTrieTrackerCalled: func() state.DataTrieTracker {
				return &mock.DataTrieTrackerStub{RetrieveValueCalled: func(_ []byte) ([]byte, error) {
					return nil, expectedErr
				}}
			},
		}
		_, err = n.computeEconomics()
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("missing staking record means zero staked, not an error", func(t *testing.T) {
		t.Parallel()
		n, apps := newFullEconomicsNode(t)
		marshalizer := marshal.NewProtoMarshalizer()
		stakingRaw, err := marshalizer.Marshal(&kapps.StakingData{CurrentFPRAmount: 40})
		require.NoError(t, err)
		apps[string(kapps.StakingKAppAddress)] = &mock.KAppAccountHandlerStub{
			DataTrieCalled:    func() data.Trie { return trieWithKeys(klvKey) },
			GetRootHashCalled: func() []byte { return []byte("root") },
			GetStorageCalled:  func(_ []byte) []byte { return stakingRaw },
			DataTrieTrackerCalled: func() state.DataTrieTracker {
				return &mock.DataTrieTrackerStub{RetrieveValueCalled: func(_ []byte) ([]byte, error) {
					return nil, nil // no record stored
				}}
			},
		}
		resp, err := n.computeEconomics()
		require.NoError(t, err)
		require.Zero(t, resp.TotalStaked)
	})

	t.Run("nil validator statistics yields zero accumulated fees", func(t *testing.T) {
		t.Parallel()
		n, _ := newFullEconomicsNode(t)
		n.validatorStatistics = nil
		resp, err := n.computeEconomics()
		require.NoError(t, err)
		require.Zero(t, resp.AccumulatedFeesTotal)
	})
}
