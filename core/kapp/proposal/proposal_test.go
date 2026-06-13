package proposal

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
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

func TestProposalKApp_isVoterLimitExceeded(t *testing.T) {
	t.Parallel()

	t.Run("returns false when EpochRewardsV2 is disabled", func(t *testing.T) {
		proposalKApp, _, forkController := createTestProposalKApp(t)
		forkController.EpochRewardsV2Value = false

		proposal := &kapps.ProposalData{
			Voters: make(map[string]*kapps.ProposalData_VoteDetail),
		}
		// Fill to max voters
		for i := range MaxVotersPerProposal {
			addr := makeAddress("voter" + string(rune(i)))
			proposal.Voters[string(addr)] = &kapps.ProposalData_VoteDetail{Amount: 100}
		}

		result := proposalKApp.IsVoterLimitExceeded(proposal, "newvoter")
		require.False(t, result)
	})

	t.Run("returns false when voter already exists", func(t *testing.T) {
		proposalKApp, _, forkController := createTestProposalKApp(t)
		forkController.EpochRewardsV2Value = true

		existingVoter := "existingvoter"
		proposal := &kapps.ProposalData{
			Voters: make(map[string]*kapps.ProposalData_VoteDetail),
		}
		// Fill to max voters including the existing voter
		for i := range MaxVotersPerProposal {
			addr := "voter" + string(rune(i))
			proposal.Voters[addr] = &kapps.ProposalData_VoteDetail{Amount: 100}
		}
		proposal.Voters[existingVoter] = &kapps.ProposalData_VoteDetail{Amount: 200}

		result := proposalKApp.IsVoterLimitExceeded(proposal, existingVoter)
		require.False(t, result)
	})

	t.Run("returns false when under limit", func(t *testing.T) {
		proposalKApp, _, forkController := createTestProposalKApp(t)
		forkController.EpochRewardsV2Value = true

		proposal := &kapps.ProposalData{
			Voters: make(map[string]*kapps.ProposalData_VoteDetail),
		}
		// Add fewer than max voters
		for i := range 10 {
			addr := "voter" + string(rune(i))
			proposal.Voters[addr] = &kapps.ProposalData_VoteDetail{Amount: 100}
		}

		result := proposalKApp.IsVoterLimitExceeded(proposal, "newvoter")
		require.False(t, result)
	})

	t.Run("returns true when at limit and new voter", func(t *testing.T) {
		proposalKApp, _, forkController := createTestProposalKApp(t)
		forkController.EpochRewardsV2Value = true

		proposal := &kapps.ProposalData{
			Voters: make(map[string]*kapps.ProposalData_VoteDetail),
		}
		// Fill to exactly max voters
		for i := range MaxVotersPerProposal {
			addr := "voter" + string(rune(i))
			proposal.Voters[addr] = &kapps.ProposalData_VoteDetail{Amount: 100}
		}

		result := proposalKApp.IsVoterLimitExceeded(proposal, "newvoter")
		require.True(t, result)
	})
}

func TestProposalKApp_processExistingVote(t *testing.T) {
	t.Parallel()

	t.Run("returns 0 for non-existing voter", func(t *testing.T) {
		proposalKApp, _, _ := createTestProposalKApp(t)

		proposal := &kapps.ProposalData{
			Voters: make(map[string]*kapps.ProposalData_VoteDetail),
			Votes:  make(map[int32]int64),
		}

		result := proposalKApp.ProcessExistingVote(proposal, "nonexistent", kapps.ProposalData_VoteDetail_Yes)
		require.Equal(t, int64(0), result)
	})

	t.Run("returns 0 for nil voter detail", func(t *testing.T) {
		proposalKApp, _, _ := createTestProposalKApp(t)

		proposal := &kapps.ProposalData{
			Voters: map[string]*kapps.ProposalData_VoteDetail{
				"voter1": nil,
			},
			Votes: make(map[int32]int64),
		}

		result := proposalKApp.ProcessExistingVote(proposal, "voter1", kapps.ProposalData_VoteDetail_Yes)
		require.Equal(t, int64(0), result)
	})

	t.Run("subtracts from old type when changing vote type", func(t *testing.T) {
		proposalKApp, _, _ := createTestProposalKApp(t)

		proposal := &kapps.ProposalData{
			Voters: map[string]*kapps.ProposalData_VoteDetail{
				"voter1": {Type: kapps.ProposalData_VoteDetail_Yes, Amount: 500},
			},
			Votes: map[int32]int64{
				int32(kapps.ProposalData_VoteDetail_Yes): 1000,
				int32(kapps.ProposalData_VoteDetail_No):  200,
			},
		}

		// Change from Yes to No
		result := proposalKApp.ProcessExistingVote(proposal, "voter1", kapps.ProposalData_VoteDetail_No)
		require.Equal(t, int64(0), result)
		// Yes votes should be reduced by the old amount
		require.Equal(t, int64(500), proposal.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
	})

	t.Run("returns old amount when same vote type", func(t *testing.T) {
		proposalKApp, _, _ := createTestProposalKApp(t)

		proposal := &kapps.ProposalData{
			Voters: map[string]*kapps.ProposalData_VoteDetail{
				"voter1": {Type: kapps.ProposalData_VoteDetail_Yes, Amount: 500},
			},
			Votes: map[int32]int64{
				int32(kapps.ProposalData_VoteDetail_Yes): 1000,
			},
		}

		// Same vote type (Yes -> Yes)
		result := proposalKApp.ProcessExistingVote(proposal, "voter1", kapps.ProposalData_VoteDetail_Yes)
		require.Equal(t, int64(500), result)
		// Votes should remain unchanged
		require.Equal(t, int64(1000), proposal.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
	})
}

// setupVoteTest creates a proposal kapp with KAppController and necessary dependencies for Vote tests
func setupVoteTest(t *testing.T) (*proposalKapp, state.AccountsCacher, *mock.ForkControllerStub, *stub.KAppControllerStub) {
	proposalKApp, accCacher, forkController := createTestProposalKApp(t)

	// Create KAppController stub
	kappController := &stub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return &mock.KAppContextStub{
				BlockCalled: func() *block.Block {
					return &block.Block{
						Header: &block.BlockHeader{
							Epoch:     5,
							Timestamp: 1234567890,
						},
					}
				},
				ContractIDCalled: func() int { return 1 },
				ReceiptsCalled: func() kapp.ReceiptsContext {
					return mock.NewReceiptsContextStub()
				},
			}
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &stub.KDAKappStub{
				GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
					return nil, &kapps.StakingData{TotalStaked: 10000000}, nil
				},
			}
		},
		GetProposalControllerCalled: func() kapps.ActiveProposalController {
			return &mock.ProposalControllerStub{
				GetParameterIntCalled: func(param kapps.EnumParameter) int64 {
					if param == kapps.EnumParameter_MinKFIStakedToEnableProposals {
						return 1000 // Low threshold for testing
					}
					return 0
				},
			}
		},
	}

	err := proposalKApp.SetKAppController(kappController)
	require.NoError(t, err)

	return proposalKApp, accCacher, forkController, kappController
}

// createActiveProposal creates and stores an active proposal with the given voters
func createActiveProposal(t *testing.T, proposalKApp *proposalKapp, accCacher state.AccountsCacher, proposalID uint64, voters map[string]*kapps.ProposalData_VoteDetail) {
	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	controller := &kapps.ProposalController{
		ProposalCount:   proposalID,
		ActiveProposals: make(map[uint32]*kapps.ActiveProposals),
	}

	proposal := &kapps.ProposalData{
		Proposer:       proposerAddr,
		TXHash:         []byte("tx-hash"),
		ProposalStatus: kapps.ProposalData_ActiveProposal,
		Parameters:     make(map[int32][]byte),
		Votes:          make(map[int32]int64),
		Voters:         voters,
		EpochStart:     1,
		EpochEnd:       100,
		TotalStaked:    1000000,
	}

	err = proposalKApp.SetProposal(proposalKappAcc, proposalID, proposal, controller)
	require.NoError(t, err)

	err = accCacher.UpdateKapp(proposalKappAcc)
	require.NoError(t, err)
}

// createVoterAccount creates a user account with KFI frozen balance for voting
func createVoterAccount(t *testing.T, accCacher state.AccountsCacher, voterAddr []byte, frozenBalance int64) {
	userAcc, err := accCacher.LoadUser(voterAddr)
	require.NoError(t, err)

	// Set KFI frozen balance for voting power
	userKDA := &kapps.UserKDA{
		Balance:       0,
		FrozenBalance: frozenBalance,
	}
	err = userAcc.SetUserKDA(kdautils.KFIIdentifier, nil, userKDA)
	require.NoError(t, err)

	err = accCacher.UpdateUser(userAcc)
	require.NoError(t, err)
}

func TestProposalKApp_Vote(t *testing.T) {
	t.Parallel()

	t.Run("fails with invalid vote type", func(t *testing.T) {
		proposalKApp, _, _, _ := setupVoteTest(t)

		tc := &transaction.VoteContract{
			ProposalID: 1,
			Amount:     100,
			Type:       99, // Invalid type
		}

		resultCode, err := proposalKApp.Vote(proposerAddr, tc)
		require.Equal(t, transaction.Transaction_ParameterInvalid, resultCode)
		require.Equal(t, common.ErrInvalidValue, err)
	})

	t.Run("fails with zero proposal ID", func(t *testing.T) {
		proposalKApp, _, _, _ := setupVoteTest(t)

		tc := &transaction.VoteContract{
			ProposalID: 0,
			Amount:     100,
			Type:       transaction.VoteContract_Yes,
		}

		resultCode, err := proposalKApp.Vote(proposerAddr, tc)
		require.Equal(t, transaction.Transaction_ParameterInvalid, resultCode)
		require.Equal(t, common.ErrInvalidValue, err)
	})

	t.Run("fails with zero amount", func(t *testing.T) {
		proposalKApp, _, _, _ := setupVoteTest(t)

		tc := &transaction.VoteContract{
			ProposalID: 1,
			Amount:     0,
			Type:       transaction.VoteContract_Yes,
		}

		resultCode, err := proposalKApp.Vote(proposerAddr, tc)
		require.Equal(t, transaction.Transaction_ParameterInvalid, resultCode)
		require.Equal(t, common.ErrInvalidValue, err)
	})

	t.Run("fails when max voters reached (EpochRewardsV2 enabled)", func(t *testing.T) {
		proposalKApp, accCacher, forkController, _ := setupVoteTest(t)
		forkController.EpochRewardsV2Value = true

		// Create voters map at max capacity
		voters := make(map[string]*kapps.ProposalData_VoteDetail)
		for i := range MaxVotersPerProposal {
			voterAddr := makeAddress(fmt.Sprintf("existingvoter%d", i))
			encodedAddr := hex.EncodeToString(voterAddr)
			voters[encodedAddr] = &kapps.ProposalData_VoteDetail{
				Type:   kapps.ProposalData_VoteDetail_Yes,
				Amount: 100,
			}
		}

		createActiveProposal(t, proposalKApp, accCacher, 1, voters)

		// Create new voter account
		newVoter := makeAddress("newvoter")
		createVoterAccount(t, accCacher, newVoter, 1000)

		tc := &transaction.VoteContract{
			ProposalID: 1,
			Amount:     100,
			Type:       transaction.VoteContract_Yes,
		}

		resultCode, err := proposalKApp.Vote(newVoter, tc)
		require.Equal(t, transaction.Transaction_ParameterInvalid, resultCode)
		require.Equal(t, common.ErrProposalMaxVotersReached, err)
	})

	t.Run("succeeds when max voters reached but voter already exists (update vote)", func(t *testing.T) {
		proposalKApp, accCacher, forkController, _ := setupVoteTest(t)
		forkController.EpochRewardsV2Value = true

		existingVoter := makeAddress("existingvoter0")
		encodedExistingVoter := hex.EncodeToString(existingVoter)

		// Create voters map at max capacity including existing voter
		voters := make(map[string]*kapps.ProposalData_VoteDetail)
		for i := range MaxVotersPerProposal {
			voterAddr := makeAddress(fmt.Sprintf("existingvoter%d", i))
			encodedAddr := hex.EncodeToString(voterAddr)
			voters[encodedAddr] = &kapps.ProposalData_VoteDetail{
				Type:   kapps.ProposalData_VoteDetail_Yes,
				Amount: 100,
			}
		}

		createActiveProposal(t, proposalKApp, accCacher, 1, voters)

		// Create voter account with more frozen balance
		createVoterAccount(t, accCacher, existingVoter, 5000)

		tc := &transaction.VoteContract{
			ProposalID: 1,
			Amount:     500, // Update with higher amount
			Type:       transaction.VoteContract_Yes,
		}

		resultCode, err := proposalKApp.Vote(existingVoter, tc)
		require.Equal(t, transaction.Transaction_Ok, resultCode)
		require.NoError(t, err)

		// Verify vote was updated
		_, updatedProposal, _, err := proposalKApp.GetProposal(1)
		require.NoError(t, err)
		require.Equal(t, int64(500), updatedProposal.Voters[encodedExistingVoter].Amount)
	})

	t.Run("succeeds when EpochRewardsV2 disabled (no voter limit)", func(t *testing.T) {
		proposalKApp, accCacher, forkController, _ := setupVoteTest(t)
		forkController.EpochRewardsV2Value = false

		// Create voters map at max capacity
		voters := make(map[string]*kapps.ProposalData_VoteDetail)
		for i := range MaxVotersPerProposal {
			voterAddr := makeAddress(fmt.Sprintf("existingvoter%d", i))
			encodedAddr := hex.EncodeToString(voterAddr)
			voters[encodedAddr] = &kapps.ProposalData_VoteDetail{
				Type:   kapps.ProposalData_VoteDetail_Yes,
				Amount: 100,
			}
		}

		createActiveProposal(t, proposalKApp, accCacher, 1, voters)

		// Create new voter account
		newVoter := makeAddress("newvoter")
		createVoterAccount(t, accCacher, newVoter, 1000)

		tc := &transaction.VoteContract{
			ProposalID: 1,
			Amount:     100,
			Type:       transaction.VoteContract_Yes,
		}

		resultCode, err := proposalKApp.Vote(newVoter, tc)
		require.Equal(t, transaction.Transaction_Ok, resultCode)
		require.NoError(t, err)

		// Verify vote was added (exceeding max voters since fork is disabled)
		_, updatedProposal, _, err := proposalKApp.GetProposal(1)
		require.NoError(t, err)
		require.Len(t, updatedProposal.Voters, MaxVotersPerProposal+1)
	})

	t.Run("vote type change updates vote counts correctly", func(t *testing.T) {
		proposalKApp, accCacher, forkController, _ := setupVoteTest(t)
		forkController.EpochRewardsV2Value = true

		voterAddr := makeAddress("voter")
		encodedVoter := hex.EncodeToString(voterAddr)

		// Create proposal with initial vote
		voters := map[string]*kapps.ProposalData_VoteDetail{
			encodedVoter: {
				Type:   kapps.ProposalData_VoteDetail_Yes,
				Amount: 500,
			},
		}

		// Create proposal with initial Yes votes
		proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		controller := &kapps.ProposalController{
			ProposalCount:   1,
			ActiveProposals: make(map[uint32]*kapps.ActiveProposals),
		}

		proposal := &kapps.ProposalData{
			Proposer:       proposerAddr,
			TXHash:         []byte("tx-hash"),
			ProposalStatus: kapps.ProposalData_ActiveProposal,
			Parameters:     make(map[int32][]byte),
			Votes: map[int32]int64{
				int32(kapps.ProposalData_VoteDetail_Yes): 500,
				int32(kapps.ProposalData_VoteDetail_No):  0,
			},
			Voters:      voters,
			EpochStart:  1,
			EpochEnd:    100,
			TotalStaked: 1000000,
		}

		err = proposalKApp.SetProposal(proposalKappAcc, 1, proposal, controller)
		require.NoError(t, err)
		err = accCacher.UpdateKapp(proposalKappAcc)
		require.NoError(t, err)

		// Create voter account
		createVoterAccount(t, accCacher, voterAddr, 1000)

		// Change vote from Yes to No
		tc := &transaction.VoteContract{
			ProposalID: 1,
			Amount:     300,
			Type:       transaction.VoteContract_No,
		}

		resultCode, err := proposalKApp.Vote(voterAddr, tc)
		require.Equal(t, transaction.Transaction_Ok, resultCode)
		require.NoError(t, err)

		// Verify vote counts were updated correctly
		_, updatedProposal, _, err := proposalKApp.GetProposal(1)
		require.NoError(t, err)

		// Yes votes should be reduced by old amount (500) to 0
		require.Equal(t, int64(0), updatedProposal.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
		// No votes should be set to new amount (300)
		require.Equal(t, int64(300), updatedProposal.Votes[int32(kapps.ProposalData_VoteDetail_No)])
		// Voter should have new type and amount
		require.Equal(t, kapps.ProposalData_VoteDetail_No, updatedProposal.Voters[encodedVoter].Type)
		require.Equal(t, int64(300), updatedProposal.Voters[encodedVoter].Amount)
	})
}

func TestProposalKApp_ValidateScriptTrigger(t *testing.T) {
	t.Parallel()

	scriptParam := int32(kapps.EnumParameter_ExecuteScript)

	newCtx := func() kapp.KappContext {
		return &mock.KAppContextStub{
			ContractIDCalled: func() int { return 1 },
			ReceiptsCalled: func() kapp.ReceiptsContext {
				return mock.NewReceiptsContextStub()
			},
		}
	}

	t.Run("no-op when trigger not present", func(t *testing.T) {
		proposalKApp, _, _ := createTestProposalKApp(t)
		controller := &kapps.ProposalController{}

		code, err := proposalKApp.ValidateScriptTrigger(newCtx(),
			map[int32][]byte{int32(kapps.EnumParameter_FeePerDataByte): []byte("4000")}, controller)

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})

	t.Run("allows known script when fork on and never executed", func(t *testing.T) {
		proposalKApp, _, forkController := createTestProposalKApp(t)
		forkController.SetFork("ProposalScriptExecution", true)

		controller := &kapps.ProposalController{}

		code, err := proposalKApp.ValidateScriptTrigger(newCtx(),
			map[int32][]byte{scriptParam: []byte(kapps.ScriptBurnKLV)}, controller)

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})

	t.Run("rejects unknown script name", func(t *testing.T) {
		proposalKApp, _, forkController := createTestProposalKApp(t)
		forkController.SetFork("ProposalScriptExecution", true)

		controller := &kapps.ProposalController{}

		code, err := proposalKApp.ValidateScriptTrigger(newCtx(),
			map[int32][]byte{scriptParam: []byte("NotARealScript")}, controller)

		require.Equal(t, common.ErrInvalidParameter, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
	})

	t.Run("rejects script when fork off", func(t *testing.T) {
		proposalKApp, _, forkController := createTestProposalKApp(t)
		forkController.SetFork("ProposalScriptExecution", false)

		controller := &kapps.ProposalController{}

		code, err := proposalKApp.ValidateScriptTrigger(newCtx(),
			map[int32][]byte{scriptParam: []byte(kapps.ScriptBurnKLV)}, controller)

		require.Equal(t, common.ErrInvalidParameter, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
	})

	t.Run("rejects one-time script already executed in history", func(t *testing.T) {
		proposalKApp, accCacher, forkController := createTestProposalKApp(t)
		forkController.SetFork("ProposalScriptExecution", true)

		// Persist an approved proposal that already carried the BurnKLV trigger.
		proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		executed := &kapps.ProposalData{
			ProposalStatus: kapps.ProposalData_ApprovedProposal,
			Parameters:     map[int32][]byte{scriptParam: []byte(kapps.ScriptBurnKLV)},
		}
		require.NoError(t, proposalKApp.SetProposal(proposalKappAcc, 1, executed, nil))
		require.NoError(t, accCacher.UpdateKapp(proposalKappAcc))

		controller := &kapps.ProposalController{ProposalCount: 1}

		code, err := proposalKApp.ValidateScriptTrigger(newCtx(),
			map[int32][]byte{scriptParam: []byte(kapps.ScriptBurnKLV)}, controller)

		require.Equal(t, common.ErrScriptAlreadyProposed, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
	})

	t.Run("rejects one-time script with a pending active proposal", func(t *testing.T) {
		proposalKApp, accCacher, forkController := createTestProposalKApp(t)
		forkController.SetFork("ProposalScriptExecution", true)

		// Persist a still-ACTIVE proposal carrying BurnKLV — e.g. one created earlier in the
		// same block/epoch that has not been resolved yet. A second one must be refused.
		proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		pending := &kapps.ProposalData{
			ProposalStatus: kapps.ProposalData_ActiveProposal,
			Parameters:     map[int32][]byte{scriptParam: []byte(kapps.ScriptBurnKLV)},
		}
		require.NoError(t, proposalKApp.SetProposal(proposalKappAcc, 1, pending, nil))
		require.NoError(t, accCacher.UpdateKapp(proposalKappAcc))

		controller := &kapps.ProposalController{ProposalCount: 1}

		code, err := proposalKApp.ValidateScriptTrigger(newCtx(),
			map[int32][]byte{scriptParam: []byte(kapps.ScriptBurnKLV)}, controller)

		require.Equal(t, common.ErrScriptAlreadyProposed, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
	})

	t.Run("allows one-time script after a denied proposal", func(t *testing.T) {
		proposalKApp, accCacher, forkController := createTestProposalKApp(t)
		forkController.SetFork("ProposalScriptExecution", true)

		// A prior proposal carried BurnKLV but was DENIED — it never ran, so the script may
		// be proposed again.
		proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		denied := &kapps.ProposalData{
			ProposalStatus: kapps.ProposalData_DeniedProposal,
			Parameters:     map[int32][]byte{scriptParam: []byte(kapps.ScriptBurnKLV)},
		}
		require.NoError(t, proposalKApp.SetProposal(proposalKappAcc, 1, denied, nil))
		require.NoError(t, accCacher.UpdateKapp(proposalKappAcc))

		controller := &kapps.ProposalController{ProposalCount: 1}

		code, err := proposalKApp.ValidateScriptTrigger(newCtx(),
			map[int32][]byte{scriptParam: []byte(kapps.ScriptBurnKLV)}, controller)

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})
}
