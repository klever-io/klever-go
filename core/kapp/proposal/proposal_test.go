package proposal

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
)

var (
	proposerAddr = makeAddress("proposer")
)

func makeAddress(prefix string) []byte {
	addr := make([]byte, 32)
	copy(addr, []byte(prefix))
	return addr
}

func createMemUnit() storage.Storer {
	capacity := uint32(10)
	shards := uint32(1)
	sizeInBytes := uint64(0)
	cache, _ := storageUnit.NewCache(storageUnit.CacheConfig{
		Type:        storageUnit.LRUCache,
		Capacity:    capacity,
		Shards:      shards,
		SizeInBytes: sizeInBytes,
	})
	persist, _ := memorydb.NewlruDB(100000)
	unit, _ := storageUnit.NewStorageUnit(cache, persist)
	return unit
}

func createAccountsDB(marshalizer marshal.Marshalizer, accountFactory state.AccountFactory) *state.AccountsDB {
	hasher := &sha256.Sha256{}
	trieStorageManager, _ := trie.NewTrieStorageManagerWithoutPruning(createMemUnit())
	tr, _ := trie.NewTrie(trieStorageManager, marshalizer, hasher, 5)
	adb, _ := state.NewAccountsDB(tr, hasher, marshalizer, accountFactory, core.Normal)
	return adb
}

func createTestProposalKApp(t *testing.T) (*proposalKapp, state.AccountsCacher, *mock.ForkControllerStub) {
	marshalizer := marshal.NewProtoMarshalizer()
	hasher := &sha256.Sha256{}
	forkController := mock.NewForkControllerStub()
	pubkeyConv := mock.NewPubkeyConverterMock(32)

	// Create accounts databases
	userAccountsDB := createAccountsDB(marshalizer, factory.NewAccountCreator())
	kappAccountsDB := createAccountsDB(marshalizer, factory.NewKAppAccountCreator())
	peerAccountsDB := createAccountsDB(marshalizer, factory.NewPeerAccountCreator())

	// Create accounts cacher
	accCacher, err := state.NewAccountsCacher(state.ArgsAcccountCacher{
		Accounts: userAccountsDB,
		Kapps:    kappAccountsDB,
		Peers:    peerAccountsDB,
	})
	require.NoError(t, err)
	accCacher.ResetAll(true)

	// Create proposal KApp
	args := &ArgsNewProposalKApp{
		Hasher:         hasher,
		Marshalizer:    marshalizer,
		PubkeyConv:     pubkeyConv,
		ForkController: forkController,
	}

	proposalKApp, err := NewProposalKApp(args)
	require.NoError(t, err)
	require.NotNil(t, proposalKApp)

	err = proposalKApp.SetAccountsCacher(accCacher)
	require.NoError(t, err)

	return proposalKApp, accCacher, forkController
}

func TestNewProposalKApp(t *testing.T) {
	t.Parallel()

	t.Run("NilMarshalizer", func(t *testing.T) {
		args := &ArgsNewProposalKApp{
			Hasher:         &sha256.Sha256{},
			Marshalizer:    nil,
			PubkeyConv:     mock.NewPubkeyConverterMock(32),
			ForkController: mock.NewForkControllerStub(),
		}

		proposalKApp, err := NewProposalKApp(args)
		require.Error(t, err)
		require.Nil(t, proposalKApp)
	})

	t.Run("NilPubkeyConverter", func(t *testing.T) {
		args := &ArgsNewProposalKApp{
			Hasher:         &sha256.Sha256{},
			Marshalizer:    marshal.NewProtoMarshalizer(),
			PubkeyConv:     nil,
			ForkController: mock.NewForkControllerStub(),
		}

		proposalKApp, err := NewProposalKApp(args)
		require.Error(t, err)
		require.Nil(t, proposalKApp)
	})

	t.Run("Success", func(t *testing.T) {
		args := &ArgsNewProposalKApp{
			Hasher:         &sha256.Sha256{},
			Marshalizer:    marshal.NewProtoMarshalizer(),
			PubkeyConv:     mock.NewPubkeyConverterMock(32),
			ForkController: mock.NewForkControllerStub(),
		}

		proposalKApp, err := NewProposalKApp(args)
		require.NoError(t, err)
		require.NotNil(t, proposalKApp)
		require.False(t, proposalKApp.IsInterfaceNil())
	})
}

func TestProposalKApp_SetAccountsCacher(t *testing.T) {
	t.Parallel()

	t.Run("NilCacher", func(t *testing.T) {
		proposalKApp, _, _ := createTestProposalKApp(t)

		err := proposalKApp.SetAccountsCacher(nil)
		require.Error(t, err)
	})

	t.Run("Success", func(t *testing.T) {
		proposalKApp, accCacher, _ := createTestProposalKApp(t)

		err := proposalKApp.SetAccountsCacher(accCacher)
		require.NoError(t, err)

		retrievedCacher := proposalKApp.GetAccountsCacher()
		require.Equal(t, accCacher, retrievedCacher)
	})
}

func Test_copyProposalDetails(t *testing.T) {
	t.Parallel()

	proposal := &kapps.ProposalData{}
	totalStaked := int64(1000000)

	tc := &transaction.ProposalContract{
		EpochsDuration: 10,
		Parameters: map[int32][]byte{
			1: []byte("value"),
		},
		Description: []byte("Test proposal description"),
	}

	ctx := &mock.KAppContextStub{
		TxHashCalled: func() []byte {
			return []byte("tx-hash-123")
		},
		BlockCalled: func() *block.Block {
			return &block.Block{
				Header: &block.BlockHeader{
					Epoch: 5,
				},
			}
		},
	}

	CopyProposalDetails(ctx, proposal, totalStaked, tc, proposerAddr)

	require.Equal(t, proposerAddr, proposal.Proposer)
	require.Equal(t, []byte("tx-hash-123"), proposal.TXHash)
	require.Equal(t, kapps.ProposalData_ActiveProposal, proposal.ProposalStatus)
	require.Equal(t, tc.GetParameters(), proposal.Parameters)
	require.Equal(t, tc.GetDescription(), proposal.Description)
	require.Equal(t, totalStaked, proposal.TotalStaked)
	require.Equal(t, uint32(5), proposal.EpochStart)
	require.Equal(t, uint32(15), proposal.EpochEnd)
	require.NotNil(t, proposal.Voters)
	require.NotNil(t, proposal.Votes)
}

func TestProposalKApp_finalizeProposal(t *testing.T) {
	t.Parallel()

	proposalKApp, accCacher, _ := createTestProposalKApp(t)

	// Load proposal kapp account
	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	// Initialize controller
	controller := &kapps.ProposalController{
		ProposalCount:   0,
		ActiveProposals: make(map[uint32]*kapps.ActiveProposals),
	}

	// Create a proposal data
	proposal := &kapps.ProposalData{}
	totalStaked := int64(2000000)

	tc := &transaction.ProposalContract{
		EpochsDuration: 15,
		Parameters: map[int32][]byte{
			int32(kapps.EnumParameter_MinKFIStakedToEnableProposals): []byte("500000"),
		},
		Description: []byte("Finalize proposal test"),
	}

	// Create staking data
	staking := &kapps.StakingData{
		TotalStaked: totalStaked,
	}

	receiptsStub := mock.NewReceiptsContextStub()
	returnData := [][]byte{}

	ctx := &mock.KAppContextStub{
		TxHashCalled: func() []byte {
			return []byte("tx-hash-finalize")
		},
		BlockCalled: func() *block.Block {
			return &block.Block{
				Header: &block.BlockHeader{
					Epoch: 10,
				},
			}
		},
		ContractIDCalled: func() int {
			return 1
		},
		SetReturnDataCalled: func(data [][]byte) {
			returnData = data
		},
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsStub
		},
	}

	// Call FinalizeProposal
	resultCode, err := proposalKApp.FinalizeProposal(ctx, proposalKappAcc, proposal, controller, staking, tc, proposerAddr)
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resultCode)

	// Verify proposal details were copied correctly (same as copyProposalDetails test)
	require.Equal(t, proposerAddr, proposal.Proposer)
	require.Equal(t, []byte("tx-hash-finalize"), proposal.TXHash)
	require.Equal(t, kapps.ProposalData_ActiveProposal, proposal.ProposalStatus)
	require.Equal(t, tc.GetParameters(), proposal.Parameters)
	require.Equal(t, tc.GetDescription(), proposal.Description)
	require.Equal(t, totalStaked, proposal.TotalStaked)
	require.Equal(t, uint32(10), proposal.EpochStart)
	require.Equal(t, uint32(25), proposal.EpochEnd) // 10 + 15

	// Verify controller was updated
	require.Equal(t, uint64(1), controller.ProposalCount)

	// Verify active proposals were updated
	require.Len(t, controller.ActiveProposals, 1)
	require.Contains(t, controller.ActiveProposals, uint32(25))
	require.Equal(t, []uint64{1}, controller.ActiveProposals[25].ProposalIDs)

	// Verify return data was set
	require.Len(t, returnData, 1)
	require.Equal(t, []byte("1"), returnData[0])

	// Verify receipt was added
	receipts := receiptsStub.Get()
	require.Len(t, receipts, 1)
}

func Test_copyProposalDetails_and_finalizeProposal_consistency(t *testing.T) {
	t.Parallel()

	// This test verifies that copyProposalDetails behaves the same when called
	// directly vs when called through finalizeProposal

	proposalKApp, accCacher, _ := createTestProposalKApp(t)

	// Setup common data
	totalStaked := int64(3000000)
	tc := &transaction.ProposalContract{
		EpochsDuration: 20,
		Parameters: map[int32][]byte{
			int32(kapps.EnumParameter_MinKFIStakedToEnableProposals): []byte("1000000"),
		},
		Description: []byte("Consistency test proposal"),
	}

	ctx := &mock.KAppContextStub{
		TxHashCalled: func() []byte {
			return []byte("tx-hash-consistency")
		},
		BlockCalled: func() *block.Block {
			return &block.Block{
				Header: &block.BlockHeader{
					Epoch: 7,
				},
			}
		},
		ContractIDCalled: func() int {
			return 2
		},
		SetReturnDataCalled: func(data [][]byte) {},
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return mock.NewReceiptsContextStub()
		},
	}

	// Test 1: Call copyProposalDetails directly
	proposal1 := &kapps.ProposalData{}
	CopyProposalDetails(ctx, proposal1, totalStaked, tc, proposerAddr)

	// Test 2: Call finalizeProposal (which internally calls copyProposalDetails)
	proposal2 := &kapps.ProposalData{}
	controller := &kapps.ProposalController{
		ProposalCount:   0,
		ActiveProposals: make(map[uint32]*kapps.ActiveProposals),
	}
	staking := &kapps.StakingData{TotalStaked: totalStaked}
	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	resultCode, err := proposalKApp.FinalizeProposal(ctx, proposalKappAcc, proposal2, controller, staking, tc, proposerAddr)
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resultCode)

	// Compare: Both proposals should have identical core fields
	require.Equal(t, proposal1.Proposer, proposal2.Proposer, "Proposer should match")
	require.Equal(t, proposal1.TXHash, proposal2.TXHash, "TXHash should match")
	require.Equal(t, proposal1.ProposalStatus, proposal2.ProposalStatus, "ProposalStatus should match")
	require.Equal(t, proposal1.Parameters, proposal2.Parameters, "Parameters should match")
	require.Equal(t, proposal1.Description, proposal2.Description, "Description should match")
	require.Equal(t, proposal1.TotalStaked, proposal2.TotalStaked, "TotalStaked should match")
	require.Equal(t, proposal1.EpochStart, proposal2.EpochStart, "EpochStart should match")
	require.Equal(t, proposal1.EpochEnd, proposal2.EpochEnd, "EpochEnd should match")
	require.NotNil(t, proposal1.Voters)
	require.NotNil(t, proposal2.Voters)
	require.NotNil(t, proposal1.Votes)
	require.NotNil(t, proposal2.Votes)
}

func Test_updateActiveProposals(t *testing.T) {
	t.Parallel()

	controller := &kapps.ProposalController{
		ProposalCount:   1,
		ActiveProposals: make(map[uint32]*kapps.ActiveProposals),
	}

	proposal := &kapps.ProposalData{
		EpochStart: 1,
		EpochEnd:   10,
	}

	UpdateActiveProposals(controller, proposal)

	require.Len(t, controller.ActiveProposals, 1)
	require.Contains(t, controller.ActiveProposals, uint32(10))
	require.NotNil(t, controller.ActiveProposals[10])
	require.Equal(t, []uint64{1}, controller.ActiveProposals[10].ProposalIDs)
}

func TestProposalKApp_SetAndGetProposal(t *testing.T) {
	t.Parallel()

	proposalKApp, accCacher, _ := createTestProposalKApp(t)

	// Load proposal kapp account
	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	// Initialize controller
	controller := &kapps.ProposalController{
		ProposalCount:   1,
		ActiveProposals: make(map[uint32]*kapps.ActiveProposals),
	}

	// Create a proposal
	proposalID := uint64(1)
	proposal := &kapps.ProposalData{
		Proposer:       proposerAddr,
		TXHash:         []byte("tx-hash"),
		ProposalStatus: kapps.ProposalData_ActiveProposal,
		Parameters:     make(map[int32][]byte),
		Votes:          make(map[int32]int64),
		Voters:         make(map[string]*kapps.ProposalData_VoteDetail),
		EpochStart:     1,
		EpochEnd:       10,
		TotalStaked:    1000000,
	}

	// Set proposal
	err = proposalKApp.SetProposal(proposalKappAcc, proposalID, proposal, controller)
	require.NoError(t, err)

	// Save kapp account
	err = accCacher.UpdateKapp(proposalKappAcc)
	require.NoError(t, err)

	// Get proposal back
	retrievedKappAcc, retrievedProposal, retrievedController, err := proposalKApp.GetProposal(proposalID)
	require.NoError(t, err)
	require.NotNil(t, retrievedKappAcc)
	require.NotNil(t, retrievedProposal)
	require.NotNil(t, retrievedController)
	require.Equal(t, proposal.Proposer, retrievedProposal.Proposer)
	require.Equal(t, proposal.ProposalStatus, retrievedProposal.ProposalStatus)
	require.Equal(t, uint64(1), retrievedController.ProposalCount)
}
