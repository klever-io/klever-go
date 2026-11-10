package proposal

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
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
		forkController.FixAuditChangesV7Value = false

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
		forkController.FixAuditChangesV7Value = false

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
		forkController.FixAuditChangesV7Value = false

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
		forkController.FixAuditChangesV7Value = false

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

	t.Run("a nil-valued voter counts as existing at the limit", func(t *testing.T) {
		proposalKApp, _, forkController := createTestProposalKApp(t)
		forkController.FixAuditChangesV7Value = false

		proposal := &kapps.ProposalData{
			Voters: make(map[string]*kapps.ProposalData_VoteDetail),
		}
		for i := range MaxVotersPerProposal {
			proposal.Voters[fmt.Sprintf("voter%d", i)] = nil
		}

		require.False(t, proposalKApp.IsVoterLimitExceeded(proposal, "voter0"))
		require.True(t, proposalKApp.IsVoterLimitExceeded(proposal, "newvoter"))
	})

	t.Run("the limit holds on a proposal created before the fork", func(t *testing.T) {
		proposalKApp, _, forkController := createTestProposalKApp(t)
		activateVoterKeysAfterExistingProposals(forkController)

		proposal := &kapps.ProposalData{
			EpochStart: 1,
			Voters:     make(map[string]*kapps.ProposalData_VoteDetail),
		}
		for i := range MaxVotersPerProposal {
			proposal.Voters[fmt.Sprintf("voter%d", i)] = &kapps.ProposalData_VoteDetail{Amount: 100}
		}

		require.True(t, proposalKApp.IsVoterLimitExceeded(proposal, "newvoter"))
	})

	t.Run("the limit does not apply on voter keys", func(t *testing.T) {
		proposalKApp, _, _ := createTestProposalKApp(t)

		proposal := &kapps.ProposalData{
			Voters: make(map[string]*kapps.ProposalData_VoteDetail),
		}
		for i := range MaxVotersPerProposal {
			proposal.Voters[fmt.Sprintf("voter%d", i)] = &kapps.ProposalData_VoteDetail{Amount: 100}
		}

		require.False(t, proposalKApp.IsVoterLimitExceeded(proposal, "newvoter"))
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
					if param == kapps.EnumParameter_MinKFIVoteAmount {
						return 1
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

func setMinKFIVoteAmount(kappController *stub.KAppControllerStub, minimum int64) {
	kappController.GetProposalControllerCalled = func() kapps.ActiveProposalController {
		return &mock.ProposalControllerStub{
			GetParameterIntCalled: func(param kapps.EnumParameter) int64 {
				switch param {
				case kapps.EnumParameter_MinKFIStakedToEnableProposals:
					return 1000
				case kapps.EnumParameter_MinKFIVoteAmount:
					return minimum
				default:
					return 0
				}
			},
			GetParameterUintCalled: func(kapps.EnumParameter) uint64 { return 40 },
		}
	}
}

func captureReceipts(kappController *stub.KAppControllerStub) *mock.ReceiptsContextStub {
	receipts := mock.NewReceiptsContextStub()
	kappController.GetCurrentKAppContextCalled = func() kapp.KappContext {
		return &mock.KAppContextStub{
			BlockCalled: func() *block.Block {
				return &block.Block{Header: &block.BlockHeader{Epoch: 5, Timestamp: 1234567890}}
			},
			ContractIDCalled: func() int { return 1 },
			ReceiptsCalled:   func() kapp.ReceiptsContext { return receipts },
		}
	}

	return receipts
}

func storedProposal(t *testing.T, proposalKApp *proposalKapp, accCacher state.AccountsCacher, proposalID uint64) *kapps.ProposalData {
	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	proposal, err := proposalKApp.GetProposalData(proposalKappAcc, proposalID)
	require.NoError(t, err)

	return proposal
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
		forkController.FixAuditChangesV7Value = false

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
		forkController.FixAuditChangesV7Value = false

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
		forkController.FixAuditChangesV7Value = false

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
		forkController.FixAuditChangesV7Value = false

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

func TestVoteWritesNoIndexKeyBeforeTheFork(t *testing.T) {
	for _, fixV5 := range []bool{false, true} {
		t.Run(fmt.Sprintf("fixAuditChangesV5=%v", fixV5), func(t *testing.T) {
			proposalKApp, accCacher, forkController, _ := setupVoteTest(t)
			forkController.FixAuditChangesV5Value = fixV5

			voterAddr := makeAddress("voter")
			createActiveProposal(t, proposalKApp, accCacher, 1, map[string]*kapps.ProposalData_VoteDetail{})
			createVoterAccount(t, accCacher, voterAddr, 1000)

			resultCode, err := proposalKApp.Vote(voterAddr, &transaction.VoteContract{
				ProposalID: 1,
				Amount:     100,
				Type:       transaction.VoteContract_Yes,
			})
			require.Equal(t, transaction.Transaction_Ok, resultCode)
			require.NoError(t, err)

			proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
			require.NoError(t, err)

			indexKey := string(kdautils.ToAccountProposalVotesKey(hex.EncodeToString(voterAddr)))
			_, written := proposalKappAcc.DataTrieTracker().DirtyData()[indexKey]
			require.Equal(t, fixV5, written)
		})
	}
}

func TestPreForkVoteIDBoundIsRecordedOnceAndNotExtendedByLaterProposals(t *testing.T) {
	proposalKApp, accCacher, _, _ := setupVoteTest(t)

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	controller := &kapps.ProposalController{ProposalCount: 4}

	first, err := proposalKApp.PreForkVoteIDBound(proposalKappAcc, controller)
	require.NoError(t, err)
	require.Equal(t, uint64(4), first)

	controller.ProposalCount = 900

	second, err := proposalKApp.PreForkVoteIDBound(proposalKappAcc, controller)
	require.NoError(t, err)
	require.Equal(t, uint64(4), second, "the bound is frozen at the ids that predate the fork")
}

var errScanEndUnavailable = errors.New("scan end unavailable")

func TestPreForkVoteIDBoundRecomputesOnAToleratedReadError(t *testing.T) {
	tests := []struct {
		name        string
		retrieveErr error
	}{
		{name: "data trie is missing", retrieveErr: common.ErrNilTrie},
		{name: "stored entry was emptied", retrieveErr: common.ErrNegativeValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proposalKApp, _, _, _ := setupVoteTest(t)

			var saved []byte
			acc := &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(_ []byte) ([]byte, error) {
							return nil, tt.retrieveErr
						},
						SaveKeyValueCalled: func(_ []byte, value []byte) error {
							saved = value

							return nil
						},
					}
				},
			}

			controller := &kapps.ProposalController{ProposalCount: 77}

			idBound, err := proposalKApp.PreForkVoteIDBound(acc, controller)
			require.NoError(t, err)
			require.Equal(t, uint64(77), idBound)
			require.Equal(t, uint64(77), binary.BigEndian.Uint64(saved))
		})
	}
}

func TestPreForkVoteIDBoundPropagatesStorageErrors(t *testing.T) {
	tests := []struct {
		name    string
		tracker func() state.DataTrieTracker
	}{
		{
			name: "the stored bound cannot be read",
			tracker: func() state.DataTrieTracker {
				return &mock.DataTrieTrackerStub{
					RetrieveValueCalled: func(_ []byte) ([]byte, error) {
						return nil, errScanEndUnavailable
					},
				}
			},
		},
		{
			name: "the computed bound cannot be written",
			tracker: func() state.DataTrieTracker {
				return &mock.DataTrieTrackerStub{
					RetrieveValueCalled: func(_ []byte) ([]byte, error) {
						return nil, common.ErrNilTrie
					},
					SaveKeyValueCalled: func(_ []byte, _ []byte) error {
						return errScanEndUnavailable
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proposalKApp, _, _, _ := setupVoteTest(t)

			acc := &mock.KAppAccountHandlerStub{DataTrieTrackerCalled: tt.tracker}

			idBound, err := proposalKApp.PreForkVoteIDBound(acc, &kapps.ProposalController{})
			require.ErrorIs(t, err, errScanEndUnavailable)
			require.Equal(t, uint64(0), idBound)
		})
	}
}

func TestPreForkVoteIDBoundIsZeroOnAChainWithNoProposals(t *testing.T) {
	proposalKApp, accCacher, _, _ := setupVoteTest(t)

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	idBound, err := proposalKApp.PreForkVoteIDBound(proposalKappAcc, &kapps.ProposalController{})
	require.NoError(t, err)
	require.Equal(t, uint64(0), idBound, "nothing predates the fork, so the scan reads nothing")
}

func TestAccountVoteIndexRoundTripsThroughStorage(t *testing.T) {
	proposalKApp, accCacher, _, _ := setupVoteTest(t)

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	voter := hex.EncodeToString(makeAddress("indexedvoter"))

	active := controllerWithActive(1, 2)
	require.NoError(t, proposalKApp.addVoterToIndex(proposalKappAcc, voter, 1, 100, active))
	require.NoError(t, proposalKApp.addVoterToIndex(proposalKappAcc, voter, 2, 200, active))
	require.NoError(t, proposalKApp.addVoterToIndex(proposalKappAcc, voter, 1, 100, active))

	entries, err := proposalKApp.GetAccountProposalVotes(proposalKappAcc, voter)
	require.NoError(t, err)
	require.Equal(t, []kapp.ProposalVoteIndexEntry{{ProposalID: 1, Amount: 100}, {ProposalID: 2, Amount: 200}}, entries)

	require.NoError(t, proposalKApp.SetAccountProposalVotes(proposalKappAcc, voter, []kapp.ProposalVoteIndexEntry{{ProposalID: 2, Amount: 150}}))

	entries, err = proposalKApp.GetAccountProposalVotes(proposalKappAcc, voter)
	require.NoError(t, err)
	require.Equal(t, []kapp.ProposalVoteIndexEntry{{ProposalID: 2, Amount: 150}}, entries)
}

func TestAccountVoteIndexReadsBackEmptyAfterLastEntryRemoved(t *testing.T) {
	proposalKApp, accCacher, _, _ := setupVoteTest(t)

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	voter := hex.EncodeToString(makeAddress("indexedvoter"))

	require.NoError(t, proposalKApp.addVoterToIndex(proposalKappAcc, voter, 7, 70, controllerWithActive(7)))
	require.NoError(t, proposalKApp.SetAccountProposalVotes(proposalKappAcc, voter, nil))

	entries, err := proposalKApp.GetAccountProposalVotes(proposalKappAcc, voter)
	require.NoError(t, err)
	require.Empty(t, entries)

	require.NoError(t, proposalKApp.addVoterToIndex(proposalKappAcc, voter, 8, 80, controllerWithActive(8)))

	entries, err = proposalKApp.GetAccountProposalVotes(proposalKappAcc, voter)
	require.NoError(t, err)
	require.Equal(t, []kapp.ProposalVoteIndexEntry{{ProposalID: 8, Amount: 80}}, entries)
}

func TestAccountVoteIndexTreatsMissingDataTrieAsEmpty(t *testing.T) {
	proposalKApp, _, _, _ := setupVoteTest(t)

	freshKapp, err := state.NewKAppAccount(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	voter := hex.EncodeToString(makeAddress("indexedvoter"))

	entries, err := proposalKApp.GetAccountProposalVotes(freshKapp, voter)
	require.NoError(t, err)
	require.Empty(t, entries)

	require.NoError(t, proposalKApp.SetAccountProposalVotes(freshKapp, voter, nil))
}

// TestVoteRecordsTheAmountAndAReVoteReplacesIt drives the real Vote twice: the index must carry
// the amount the proposal records for the voter, and a re-vote — which replaces the vote — must
// replace the indexed amount too, or the unfreeze skip would trust a stale figure.
func TestVoteRecordsTheAmountAndAReVoteReplacesIt(t *testing.T) {
	proposalKApp, accCacher, forkController, _ := setupVoteTest(t)
	forkController.FixAuditChangesV5Value = true

	voterAddr := makeAddress("voter")
	createActiveProposal(t, proposalKApp, accCacher, 1, map[string]*kapps.ProposalData_VoteDetail{})
	createVoterAccount(t, accCacher, voterAddr, 1000)

	for _, amount := range []int64{100, 40} {
		resultCode, err := proposalKApp.Vote(voterAddr, &transaction.VoteContract{
			ProposalID: 1,
			Amount:     amount,
			Type:       transaction.VoteContract_Yes,
		})
		require.Equal(t, transaction.Transaction_Ok, resultCode)
		require.NoError(t, err)

		proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		entries, err := proposalKApp.GetAccountProposalVotes(proposalKappAcc, hex.EncodeToString(voterAddr))
		require.NoError(t, err)
		require.Equal(t, []kapp.ProposalVoteIndexEntry{{ProposalID: 1, Amount: amount}}, entries)
	}
}

// TestPreForkVoteIDBoundFailsClosedOnAMalformedBound: a recorded bound of the wrong shape is
// corruption. Recomputing it from the count as it stands now would let a later proposal widen the
// scan this key exists to freeze, so the unfreeze must fail instead.
func TestPreForkVoteIDBoundFailsClosedOnAMalformedBound(t *testing.T) {
	proposalKApp, _, _, _ := setupVoteTest(t)

	saves := 0
	acc := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				RetrieveValueCalled: func(_ []byte) ([]byte, error) {
					return []byte{0, 0, 0, 1}, nil
				},
				SaveKeyValueCalled: func(_ []byte, _ []byte) error {
					saves++
					return nil
				},
			}
		},
	}

	controller := &kapps.ProposalController{ProposalCount: 900}

	_, err := proposalKApp.PreForkVoteIDBound(acc, controller)
	require.ErrorIs(t, err, common.ErrInvalidValue)
	require.Equal(t, 0, saves, "a malformed bound must not be recomputed and rewritten")
}

func activateVoterKeysAfterExistingProposals(forkController *mock.ForkControllerStub) {
	activation := uint32(2)
	forkController.FixAuditChangesV7Value = true
	forkController.FixAuditChangesV7Epoch = &activation
}

func storedVoterRecord(t *testing.T, accCacher state.AccountsCacher, proposalID uint64, encodedAddr string) *kapps.ProposalData_VoteDetail {
	t.Helper()

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	raw, err := retrieveOptional(proposalKappAcc.DataTrieTracker(), kdautils.ToProposalVoterKey(proposalID, encodedAddr))
	require.NoError(t, err)
	if len(raw) == 0 {
		return nil
	}

	detail := &kapps.ProposalData_VoteDetail{}
	require.NoError(t, marshal.NewProtoMarshalizer().Unmarshal(detail, raw))

	return detail
}

func dirtyVoterKeys(t *testing.T, accCacher state.AccountsCacher) []string {
	t.Helper()

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	keys := make([]string, 0)
	for key := range proposalKappAcc.DataTrieTracker().DirtyData() {
		if strings.HasPrefix(key, kapps.ProposalVoterPrefix) {
			keys = append(keys, key)
		}
	}

	return keys
}

func castVote(t *testing.T, proposalKApp *proposalKapp, voterAddr []byte, voteType transaction.VoteContract_EnumVoteType, amount int64) {
	t.Helper()

	resultCode, err := proposalKApp.Vote(voterAddr, &transaction.VoteContract{ProposalID: 1, Amount: amount, Type: voteType})
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resultCode)
}

func TestVoteStoresANewVoterUnderItsOwnKey(t *testing.T) {
	proposalKApp, accCacher, _, _ := setupVoteTest(t)

	voterAddr := makeAddress("voter")
	encodedVoter := hex.EncodeToString(voterAddr)
	createActiveProposal(t, proposalKApp, accCacher, 1, nil)
	createVoterAccount(t, accCacher, voterAddr, 1000)

	castVote(t, proposalKApp, voterAddr, transaction.VoteContract_No, 100)

	record := storedVoterRecord(t, accCacher, 1, encodedVoter)
	require.NotNil(t, record)
	require.Equal(t, kapps.ProposalData_VoteDetail_No, record.Type)
	require.Equal(t, int64(100), record.Amount)
	require.Equal(t, int64(1234567890), record.Timestamp)

	proposal := storedProposal(t, proposalKApp, accCacher, 1)
	require.Empty(t, proposal.Voters)
	require.Equal(t, int64(100), proposal.Votes[int32(kapps.ProposalData_VoteDetail_No)])
	require.Equal(t, int64(10000000), proposal.TotalStaked)

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	entries, err := proposalKApp.GetAccountProposalVotes(proposalKappAcc, encodedVoter)
	require.NoError(t, err)
	require.Equal(t, []kapp.ProposalVoteIndexEntry{{ProposalID: 1, Amount: 100}}, entries)
}

func TestVoteOnVoterKeysReplacesTheAmountOfASameTypeReVote(t *testing.T) {
	proposalKApp, accCacher, _, _ := setupVoteTest(t)

	voterAddr := makeAddress("voter")
	encodedVoter := hex.EncodeToString(voterAddr)
	otherAddr := makeAddress("other")
	createActiveProposal(t, proposalKApp, accCacher, 1, nil)
	createVoterAccount(t, accCacher, voterAddr, 1000)
	createVoterAccount(t, accCacher, otherAddr, 1000)

	castVote(t, proposalKApp, otherAddr, transaction.VoteContract_Yes, 30)
	castVote(t, proposalKApp, voterAddr, transaction.VoteContract_Yes, 100)
	castVote(t, proposalKApp, voterAddr, transaction.VoteContract_Yes, 40)

	require.Equal(t, int64(40), storedVoterRecord(t, accCacher, 1, encodedVoter).Amount)
	require.Equal(t, int64(30), storedVoterRecord(t, accCacher, 1, hex.EncodeToString(otherAddr)).Amount)

	proposal := storedProposal(t, proposalKApp, accCacher, 1)
	require.Empty(t, proposal.Voters)
	require.Equal(t, int64(70), proposal.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	entries, err := proposalKApp.GetAccountProposalVotes(proposalKappAcc, encodedVoter)
	require.NoError(t, err)
	require.Equal(t, []kapp.ProposalVoteIndexEntry{{ProposalID: 1, Amount: 40}}, entries)
}

func TestVoteOnVoterKeysMovesTheTallyWhenTheTypeChanges(t *testing.T) {
	proposalKApp, accCacher, _, _ := setupVoteTest(t)

	voterAddr := makeAddress("voter")
	encodedVoter := hex.EncodeToString(voterAddr)
	otherAddr := makeAddress("other")
	createActiveProposal(t, proposalKApp, accCacher, 1, nil)
	createVoterAccount(t, accCacher, voterAddr, 1000)
	createVoterAccount(t, accCacher, otherAddr, 1000)

	castVote(t, proposalKApp, otherAddr, transaction.VoteContract_Yes, 30)
	castVote(t, proposalKApp, voterAddr, transaction.VoteContract_Yes, 500)
	castVote(t, proposalKApp, voterAddr, transaction.VoteContract_No, 300)

	record := storedVoterRecord(t, accCacher, 1, encodedVoter)
	require.Equal(t, kapps.ProposalData_VoteDetail_No, record.Type)
	require.Equal(t, int64(300), record.Amount)

	proposal := storedProposal(t, proposalKApp, accCacher, 1)
	require.Empty(t, proposal.Voters)
	require.Equal(t, int64(30), proposal.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
	require.Equal(t, int64(300), proposal.Votes[int32(kapps.ProposalData_VoteDetail_No)])
}

func TestVoteOnVoterKeysAcceptsVotersBeyondTheInlineCap(t *testing.T) {
	proposalKApp, accCacher, _, _ := setupVoteTest(t)

	createActiveProposal(t, proposalKApp, accCacher, 1, nil)

	voters := MaxVotersPerProposal + 1
	for i := range voters {
		voterAddr := makeAddress(fmt.Sprintf("capvoter%d", i))
		createVoterAccount(t, accCacher, voterAddr, 1000)
		castVote(t, proposalKApp, voterAddr, transaction.VoteContract_Yes, 10)
	}

	proposal := storedProposal(t, proposalKApp, accCacher, 1)
	require.Empty(t, proposal.Voters)
	require.Equal(t, int64(voters)*10, proposal.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])

	lastVoter := hex.EncodeToString(makeAddress(fmt.Sprintf("capvoter%d", MaxVotersPerProposal)))
	require.Equal(t, int64(10), storedVoterRecord(t, accCacher, 1, lastVoter).Amount)
	require.Len(t, dirtyVoterKeys(t, accCacher), voters)
}

func TestVoteBelowTheMinimumLeavesAVoterKeyUntouched(t *testing.T) {
	proposalKApp, accCacher, _, kappController := setupVoteTest(t)
	setMinKFIVoteAmount(kappController, 100)

	voterAddr := makeAddress("voter")
	encodedVoter := hex.EncodeToString(voterAddr)
	createActiveProposal(t, proposalKApp, accCacher, 1, nil)
	createVoterAccount(t, accCacher, voterAddr, 1000)

	castVote(t, proposalKApp, voterAddr, transaction.VoteContract_Yes, 500)

	resultCode, err := proposalKApp.Vote(voterAddr, &transaction.VoteContract{ProposalID: 1, Amount: 99, Type: transaction.VoteContract_No})
	require.Equal(t, transaction.Transaction_ParameterInvalid, resultCode)
	require.ErrorIs(t, err, common.ErrProposalVoteBelowMinimum)

	record := storedVoterRecord(t, accCacher, 1, encodedVoter)
	require.Equal(t, kapps.ProposalData_VoteDetail_Yes, record.Type)
	require.Equal(t, int64(500), record.Amount)
}

func TestVoteFailsOnAMalformedVoterRecord(t *testing.T) {
	proposalKApp, accCacher, _, _ := setupVoteTest(t)

	voterAddr := makeAddress("voter")
	encodedVoter := hex.EncodeToString(voterAddr)
	createActiveProposal(t, proposalKApp, accCacher, 1, nil)
	createVoterAccount(t, accCacher, voterAddr, 1000)

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)
	require.NoError(t, proposalKappAcc.DataTrieTracker().SaveKeyValue(kdautils.ToProposalVoterKey(1, encodedVoter), []byte{0xff, 0xff, 0xff}))

	resultCode, err := proposalKApp.Vote(voterAddr, &transaction.VoteContract{ProposalID: 1, Amount: 100, Type: transaction.VoteContract_Yes})
	require.Error(t, err)
	require.Equal(t, transaction.Transaction_KAPPError, resultCode)

	proposal := storedProposal(t, proposalKApp, accCacher, 1)
	require.Zero(t, proposal.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
}

func TestVoterKeyIsDiscardedWithTheCachedAccount(t *testing.T) {
	proposalKApp, accCacher, _, _ := setupVoteTest(t)

	voterAddr := makeAddress("voter")
	encodedVoter := hex.EncodeToString(voterAddr)
	createActiveProposal(t, proposalKApp, accCacher, 1, nil)
	createVoterAccount(t, accCacher, voterAddr, 1000)
	require.NoError(t, accCacher.SaveAll())

	castVote(t, proposalKApp, voterAddr, transaction.VoteContract_Yes, 100)
	require.NotNil(t, storedVoterRecord(t, accCacher, 1, encodedVoter))

	accCacher.ResetAll(true)

	require.Nil(t, storedVoterRecord(t, accCacher, 1, encodedVoter))
	require.Zero(t, storedProposal(t, proposalKApp, accCacher, 1).Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
}

func TestVoteStaysInlineOnAProposalNotInTheVoterKeyLayout(t *testing.T) {
	layouts := []struct {
		name  string
		setup func(forkController *mock.ForkControllerStub)
	}{
		{"fork off", func(forkController *mock.ForkControllerStub) { forkController.FixAuditChangesV7Value = false }},
		{"created before the fork", activateVoterKeysAfterExistingProposals},
	}

	for _, layout := range layouts {
		t.Run(layout.name, func(t *testing.T) {
			proposalKApp, accCacher, forkController, _ := setupVoteTest(t)
			layout.setup(forkController)

			voterAddr := makeAddress("voter")
			encodedVoter := hex.EncodeToString(voterAddr)
			createActiveProposal(t, proposalKApp, accCacher, 1, map[string]*kapps.ProposalData_VoteDetail{})
			createVoterAccount(t, accCacher, voterAddr, 1000)

			castVote(t, proposalKApp, voterAddr, transaction.VoteContract_Yes, 100)
			castVote(t, proposalKApp, voterAddr, transaction.VoteContract_No, 40)

			proposal := storedProposal(t, proposalKApp, accCacher, 1)
			require.Equal(t, int64(40), proposal.Voters[encodedVoter].Amount)
			require.Equal(t, kapps.ProposalData_VoteDetail_No, proposal.Voters[encodedVoter].Type)
			require.Equal(t, int64(0), proposal.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
			require.Equal(t, int64(40), proposal.Votes[int32(kapps.ProposalData_VoteDetail_No)])
			require.Empty(t, dirtyVoterKeys(t, accCacher))
		})
	}
}

func TestVoteWritesTheSameBytesWithTheForkOffAndOnAProposalCreatedBeforeIt(t *testing.T) {
	stored := func(setup func(forkController *mock.ForkControllerStub)) map[string][]byte {
		proposalKApp, accCacher, forkController, _ := setupVoteTest(t)
		setup(forkController)

		voterAddr := makeAddress("voter")
		createActiveProposal(t, proposalKApp, accCacher, 1, map[string]*kapps.ProposalData_VoteDetail{})
		createVoterAccount(t, accCacher, voterAddr, 1000)
		castVote(t, proposalKApp, voterAddr, transaction.VoteContract_Yes, 100)

		proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		return proposalKappAcc.DataTrieTracker().DirtyData()
	}

	forkOff := stored(func(forkController *mock.ForkControllerStub) { forkController.FixAuditChangesV7Value = false })
	createdBefore := stored(activateVoterKeysAfterExistingProposals)

	require.Equal(t, forkOff, createdBefore)
	require.Len(t, forkOff, 3)
	require.Contains(t, forkOff, string(kdautils.ToProposalKey(1)))
	require.Contains(t, forkOff, string(kdautils.ProposalControllerKey))
	require.Contains(t, forkOff, string(kdautils.ToAccountProposalVotesKey(hex.EncodeToString(makeAddress("voter")))))
}

func TestVoteOnAProposalCreatedBeforeTheForkKeepsTheCapAndTheMinimum(t *testing.T) {
	proposalKApp, accCacher, forkController, kappController := setupVoteTest(t)
	activateVoterKeysAfterExistingProposals(forkController)
	setMinKFIVoteAmount(kappController, 100)

	voters := make(map[string]*kapps.ProposalData_VoteDetail, MaxVotersPerProposal)
	for i := range MaxVotersPerProposal {
		voters[hex.EncodeToString(makeAddress(fmt.Sprintf("existingvoter%d", i)))] = &kapps.ProposalData_VoteDetail{Amount: 100}
	}
	createActiveProposal(t, proposalKApp, accCacher, 1, voters)

	newVoter := makeAddress("newvoter")
	createVoterAccount(t, accCacher, newVoter, 1000)

	resultCode, err := proposalKApp.Vote(newVoter, &transaction.VoteContract{ProposalID: 1, Amount: 100, Type: transaction.VoteContract_Yes})
	require.Equal(t, transaction.Transaction_ParameterInvalid, resultCode)
	require.ErrorIs(t, err, common.ErrProposalMaxVotersReached)

	existingVoter := makeAddress("existingvoter0")
	createVoterAccount(t, accCacher, existingVoter, 1000)

	resultCode, err = proposalKApp.Vote(existingVoter, &transaction.VoteContract{ProposalID: 1, Amount: 99, Type: transaction.VoteContract_Yes})
	require.Equal(t, transaction.Transaction_ParameterInvalid, resultCode)
	require.ErrorIs(t, err, common.ErrProposalVoteBelowMinimum)

	require.Empty(t, dirtyVoterKeys(t, accCacher))
}

func TestVoterLayoutFollowsTheEpochTheProposalWasCreatedIn(t *testing.T) {
	proposalKApp, accCacher, forkController, _ := setupVoteTest(t)
	activateVoterKeysAfterExistingProposals(forkController)

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	encodedVoter := hex.EncodeToString(makeAddress("voter"))
	detail := &kapps.ProposalData_VoteDetail{Amount: 100}

	onTheEpochBefore := &kapps.ProposalData{EpochStart: 1}
	require.NoError(t, proposalKApp.SetVoter(proposalKappAcc, onTheEpochBefore, 1, encodedVoter, detail))
	require.Contains(t, onTheEpochBefore.Voters, encodedVoter)
	require.Nil(t, storedVoterRecord(t, accCacher, 1, encodedVoter))

	onTheActivationEpoch := &kapps.ProposalData{EpochStart: 2}
	require.NoError(t, proposalKApp.SetVoter(proposalKappAcc, onTheActivationEpoch, 2, encodedVoter, detail))
	require.Empty(t, onTheActivationEpoch.Voters)
	require.NotNil(t, storedVoterRecord(t, accCacher, 2, encodedVoter))
}

func TestVoterAccessorsOnVoterKeys(t *testing.T) {
	encodedVoter := hex.EncodeToString(makeAddress("voter"))
	detail := func() *kapps.ProposalData_VoteDetail {
		return &kapps.ProposalData_VoteDetail{Type: kapps.ProposalData_VoteDetail_No, Amount: 100, Timestamp: 7}
	}

	t.Run("a voter with no record reads as nil", func(t *testing.T) {
		proposalKApp, accCacher, _, _ := setupVoteTest(t)
		proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		voter, err := proposalKApp.GetVoter(proposalKappAcc, &kapps.ProposalData{}, 1, encodedVoter)
		require.NoError(t, err)
		require.Nil(t, voter)
	})

	t.Run("a missing data trie reads as nil", func(t *testing.T) {
		proposalKApp, _, _, _ := setupVoteTest(t)
		freshKapp, err := state.NewKAppAccount(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		voter, err := proposalKApp.GetVoter(freshKapp, &kapps.ProposalData{}, 1, encodedVoter)
		require.NoError(t, err)
		require.Nil(t, voter)
	})

	t.Run("a record round trips and leaves the map alone", func(t *testing.T) {
		proposalKApp, accCacher, _, _ := setupVoteTest(t)
		proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		proposal := &kapps.ProposalData{}
		require.NoError(t, proposalKApp.SetVoter(proposalKappAcc, proposal, 1, encodedVoter, detail()))
		require.Nil(t, proposal.Voters)

		voter, err := proposalKApp.GetVoter(proposalKappAcc, proposal, 1, encodedVoter)
		require.NoError(t, err)
		require.Equal(t, kapps.ProposalData_VoteDetail_No, voter.Type)
		require.Equal(t, int64(100), voter.Amount)
		require.Equal(t, int64(7), voter.Timestamp)
	})

	t.Run("a record is kept per proposal", func(t *testing.T) {
		proposalKApp, accCacher, _, _ := setupVoteTest(t)
		proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		proposal := &kapps.ProposalData{}
		require.NoError(t, proposalKApp.SetVoter(proposalKappAcc, proposal, 1, encodedVoter, detail()))

		voter, err := proposalKApp.GetVoter(proposalKappAcc, proposal, 12, encodedVoter)
		require.NoError(t, err)
		require.Nil(t, voter)
	})

	t.Run("a delete reads as nil in the same block and can be written again", func(t *testing.T) {
		proposalKApp, accCacher, _, _ := setupVoteTest(t)
		proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		proposal := &kapps.ProposalData{}
		require.NoError(t, proposalKApp.SetVoter(proposalKappAcc, proposal, 1, encodedVoter, detail()))
		require.NoError(t, proposalKApp.RemoveVoter(proposalKappAcc, proposal, 1, encodedVoter))

		voter, err := proposalKApp.GetVoter(proposalKappAcc, proposal, 1, encodedVoter)
		require.NoError(t, err)
		require.Nil(t, voter)

		require.NoError(t, proposalKApp.SetVoter(proposalKappAcc, proposal, 1, encodedVoter, detail()))

		voter, err = proposalKApp.GetVoter(proposalKappAcc, proposal, 1, encodedVoter)
		require.NoError(t, err)
		require.Equal(t, int64(100), voter.Amount)
	})

	t.Run("a write and a delete survive a save to the trie", func(t *testing.T) {
		proposalKApp, accCacher, _, _ := setupVoteTest(t)
		proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		proposal := &kapps.ProposalData{}
		require.NoError(t, proposalKApp.SetVoter(proposalKappAcc, proposal, 1, encodedVoter, detail()))
		require.NoError(t, accCacher.SaveAll())
		accCacher.ResetAll(true)

		proposalKappAcc, err = accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		voter, err := proposalKApp.GetVoter(proposalKappAcc, proposal, 1, encodedVoter)
		require.NoError(t, err)
		require.Equal(t, int64(100), voter.Amount)

		require.NoError(t, proposalKApp.RemoveVoter(proposalKappAcc, proposal, 1, encodedVoter))
		require.NoError(t, accCacher.SaveAll())
		accCacher.ResetAll(true)

		proposalKappAcc, err = accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)

		voter, err = proposalKApp.GetVoter(proposalKappAcc, proposal, 1, encodedVoter)
		require.NoError(t, err)
		require.Nil(t, voter)
	})

	t.Run("a malformed record is an error", func(t *testing.T) {
		proposalKApp, accCacher, _, _ := setupVoteTest(t)
		proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
		require.NoError(t, err)
		require.NoError(t, proposalKappAcc.DataTrieTracker().SaveKeyValue(kdautils.ToProposalVoterKey(1, encodedVoter), []byte{0xff, 0xff, 0xff}))

		voter, err := proposalKApp.GetVoter(proposalKappAcc, &kapps.ProposalData{}, 1, encodedVoter)
		require.Error(t, err)
		require.Nil(t, voter)
	})
}

func TestVoterAccessorsOnInlineVoters(t *testing.T) {
	encodedVoter := hex.EncodeToString(makeAddress("voter"))

	layouts := []struct {
		name  string
		setup func(forkController *mock.ForkControllerStub)
	}{
		{"fork off", func(forkController *mock.ForkControllerStub) { forkController.FixAuditChangesV7Value = false }},
		{"created before the fork", activateVoterKeysAfterExistingProposals},
	}

	for _, layout := range layouts {
		t.Run(layout.name, func(t *testing.T) {
			proposalKApp, accCacher, forkController, _ := setupVoteTest(t)
			layout.setup(forkController)

			proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
			require.NoError(t, err)

			proposal := &kapps.ProposalData{EpochStart: 1}

			voter, err := proposalKApp.GetVoter(proposalKappAcc, proposal, 1, encodedVoter)
			require.NoError(t, err)
			require.Nil(t, voter)

			detail := &kapps.ProposalData_VoteDetail{Amount: 100}
			require.NoError(t, proposalKApp.SetVoter(proposalKappAcc, proposal, 1, encodedVoter, detail))
			require.Same(t, detail, proposal.Voters[encodedVoter])

			voter, err = proposalKApp.GetVoter(proposalKappAcc, proposal, 1, encodedVoter)
			require.NoError(t, err)
			require.Same(t, detail, voter)

			proposal.Voters[encodedVoter] = nil
			voter, err = proposalKApp.GetVoter(proposalKappAcc, proposal, 1, encodedVoter)
			require.NoError(t, err)
			require.Nil(t, voter)

			require.NoError(t, proposalKApp.RemoveVoter(proposalKappAcc, proposal, 1, encodedVoter))
			require.NotContains(t, proposal.Voters, encodedVoter)

			require.NoError(t, proposalKApp.RemoveVoter(proposalKappAcc, &kapps.ProposalData{EpochStart: 1}, 1, encodedVoter))

			require.Empty(t, dirtyVoterKeys(t, accCacher))
		})
	}
}

func controllerWithActive(ids ...uint64) *kapps.ProposalController {
	return &kapps.ProposalController{
		ActiveProposals: map[uint32]*kapps.ActiveProposals{100: {ProposalIDs: ids}},
	}
}

// TestVoteDropsIndexEntriesForSettledProposals: a vote rewrites the index anyway, so it drops the
// entries whose proposal is in none of the controller's buckets. That is what keeps the index at
// the proposals still active for an account that votes for years and never unfreezes.
func TestVoteDropsIndexEntriesForSettledProposals(t *testing.T) {
	proposalKApp, accCacher, _, _ := setupVoteTest(t)

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	voter := hex.EncodeToString(makeAddress("indexedvoter"))

	require.NoError(t, proposalKApp.addVoterToIndex(proposalKappAcc, voter, 7, 70, controllerWithActive(7)))
	require.NoError(t, proposalKApp.addVoterToIndex(proposalKappAcc, voter, 1, 100, controllerWithActive(1, 7)))
	require.NoError(t, proposalKApp.addVoterToIndex(proposalKappAcc, voter, 2, 20, controllerWithActive(1, 2)))

	entries, err := proposalKApp.GetAccountProposalVotes(proposalKappAcc, voter)
	require.NoError(t, err)
	require.Equal(t, []kapp.ProposalVoteIndexEntry{{ProposalID: 1, Amount: 100}, {ProposalID: 2, Amount: 20}}, entries,
		"proposal 7 is in no bucket any more, so its entry goes with this write; 1 is still active and stays")
}

// TestCreateRejectsAZeroDurationProposalAfterTheFork: a zero-duration proposal ends in the epoch
// it is created in, whose bucket settlement already processed at that epoch's start, so it would
// never be settled — active and votable forever, and never leaving the buckets, which is what
// makes every voter's index entry for it unprunable. Pre-fork behaviour is unchanged.
// TestCreateRecordsThePreForkIDBoundBeforeTakingTheNextID is the guard on the transition window.
// A proposal created after the fork can be given any EpochEnd its proposer likes, including one
// below anything active when the bound was recorded, so an epoch bound could not keep it out of
// the unfreeze scan. Its id can only be above every pre-fork id, so Create records the bound
// before it takes the next one. Without this, creating proposals after the fork would put the
// unmetered per-proposal read back on every KFI unfreeze until they settled.
func TestCreateRecordsThePreForkIDBoundBeforeTakingTheNextID(t *testing.T) {
	proposalKApp, accCacher, forkController, _ := setupVoteTest(t)
	forkController.FixAuditChangesV5Value = true

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	ctx := proposalKApp.KAppController.GetCurrentKAppContext()
	controller := &kapps.ProposalController{ProposalCount: 4}
	staking := &kapps.StakingData{TotalStaked: 10000000}

	create := func(duration uint32) uint64 {
		resCode, err := proposalKApp.finalizeProposal(
			ctx,
			proposalKappAcc,
			&kapps.ProposalData{},
			controller,
			staking,
			&transaction.ProposalContract{EpochsDuration: duration},
			proposerAddr,
		)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, resCode)

		return controller.ProposalCount
	}

	id := create(1)
	require.Equal(t, uint64(5), id)

	bound, err := proposalKApp.PreForkVoteIDBound(proposalKappAcc, controller)
	require.NoError(t, err)
	require.Equal(t, uint64(4), bound, "the bound is the last id handed out before the fork")
	require.Greater(t, id, bound, "a proposal created after the fork is never scanned")

	second := create(40)
	require.Equal(t, uint64(6), second)

	bound, err = proposalKApp.PreForkVoteIDBound(proposalKappAcc, controller)
	require.NoError(t, err)
	require.Equal(t, uint64(4), bound, "and later proposals do not move it")
}

func TestVoteRequiresTheMinimumKFIAmountAfterTheFork(t *testing.T) {
	const minimum = int64(10_000_000)

	cases := []struct {
		name     string
		fixV7    bool
		amount   int64
		rejected bool
	}{
		{"below the minimum after the fork", true, minimum - 1, true},
		{"one atom after the fork", true, 1, true},
		{"exactly the minimum after the fork", true, minimum, false},
		{"above the minimum after the fork", true, minimum + 1, false},
		{"below the minimum before the fork", false, minimum - 1, false},
		{"one atom before the fork", false, 1, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proposalKApp, accCacher, forkController, kappController := setupVoteTest(t)
			activateVoterKeysAfterExistingProposals(forkController)
			forkController.FixAuditChangesV7Value = tc.fixV7
			setMinKFIVoteAmount(kappController, minimum)
			receipts := captureReceipts(kappController)

			voterAddr := makeAddress("voter")
			encodedVoter := hex.EncodeToString(voterAddr)
			createActiveProposal(t, proposalKApp, accCacher, 1, map[string]*kapps.ProposalData_VoteDetail{})
			createVoterAccount(t, accCacher, voterAddr, minimum*2)

			resultCode, err := proposalKApp.Vote(voterAddr, &transaction.VoteContract{
				ProposalID: 1,
				Amount:     tc.amount,
				Type:       transaction.VoteContract_Yes,
			})

			proposal := storedProposal(t, proposalKApp, accCacher, 1)

			if tc.rejected {
				require.Equal(t, transaction.Transaction_ParameterInvalid, resultCode)
				require.ErrorIs(t, err, common.ErrProposalVoteBelowMinimum)
				errs := receipts.GetErrors()
				require.Len(t, errs, 1)
				require.Equal(t, common.ErrFieldInvalidVoteAmount, string(errs[0].Data[1]))
				require.NotContains(t, proposal.Voters, encodedVoter)
				require.Zero(t, proposal.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
				return
			}

			require.Equal(t, transaction.Transaction_Ok, resultCode)
			require.NoError(t, err)
			require.Empty(t, receipts.GetErrors())
			require.Equal(t, tc.amount, proposal.Voters[encodedVoter].Amount)
			require.Equal(t, tc.amount, proposal.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
		})
	}
}

func TestVoteBelowTheMinimumLeavesAnExistingVoteUntouched(t *testing.T) {
	const (
		minimum = int64(100)
		cast    = int64(500)
	)

	for _, voteType := range []transaction.VoteContract_EnumVoteType{transaction.VoteContract_Yes, transaction.VoteContract_No} {
		t.Run(voteType.String(), func(t *testing.T) {
			proposalKApp, accCacher, forkController, kappController := setupVoteTest(t)
			activateVoterKeysAfterExistingProposals(forkController)
			setMinKFIVoteAmount(kappController, minimum)

			voterAddr := makeAddress("voter")
			encodedVoter := hex.EncodeToString(voterAddr)
			createActiveProposal(t, proposalKApp, accCacher, 1, map[string]*kapps.ProposalData_VoteDetail{})
			createVoterAccount(t, accCacher, voterAddr, 1000)

			resultCode, err := proposalKApp.Vote(voterAddr, &transaction.VoteContract{
				ProposalID: 1,
				Amount:     cast,
				Type:       transaction.VoteContract_Yes,
			})
			require.Equal(t, transaction.Transaction_Ok, resultCode)
			require.NoError(t, err)

			resultCode, err = proposalKApp.Vote(voterAddr, &transaction.VoteContract{
				ProposalID: 1,
				Amount:     minimum - 1,
				Type:       voteType,
			})
			require.Equal(t, transaction.Transaction_ParameterInvalid, resultCode)
			require.ErrorIs(t, err, common.ErrProposalVoteBelowMinimum)

			proposal := storedProposal(t, proposalKApp, accCacher, 1)
			require.Equal(t, cast, proposal.Voters[encodedVoter].Amount)
			require.Equal(t, kapps.ProposalData_VoteDetail_Yes, proposal.Voters[encodedVoter].Type)
			require.Equal(t, cast, proposal.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
			require.Zero(t, proposal.Votes[int32(kapps.ProposalData_VoteDetail_No)])

			proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
			require.NoError(t, err)
			entries, err := proposalKApp.GetAccountProposalVotes(proposalKappAcc, encodedVoter)
			require.NoError(t, err)
			require.Equal(t, []kapp.ProposalVoteIndexEntry{{ProposalID: 1, Amount: cast}}, entries)
		})
	}
}

func TestVoteOfAnotherVoterLeavesAVoteBelowARaisedMinimumCounted(t *testing.T) {
	proposalKApp, accCacher, _, kappController := setupVoteTest(t)
	setMinKFIVoteAmount(kappController, 100)

	smallVoter := makeAddress("small")
	otherVoter := makeAddress("other")
	encodedSmallVoter := hex.EncodeToString(smallVoter)
	createActiveProposal(t, proposalKApp, accCacher, 1, nil)
	createVoterAccount(t, accCacher, smallVoter, 1000)
	createVoterAccount(t, accCacher, otherVoter, 2000)

	castVote(t, proposalKApp, smallVoter, transaction.VoteContract_Yes, 500)

	setMinKFIVoteAmount(kappController, 1000)

	castVote(t, proposalKApp, otherVoter, transaction.VoteContract_No, 1000)

	record := storedVoterRecord(t, accCacher, 1, encodedSmallVoter)
	require.NotNil(t, record)
	require.Equal(t, kapps.ProposalData_VoteDetail_Yes, record.Type)
	require.Equal(t, int64(500), record.Amount)

	proposal := storedProposal(t, proposalKApp, accCacher, 1)
	require.Equal(t, int64(500), proposal.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
	require.Equal(t, int64(1000), proposal.Votes[int32(kapps.ProposalData_VoteDetail_No)])

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)
	entries, err := proposalKApp.GetAccountProposalVotes(proposalKappAcc, encodedSmallVoter)
	require.NoError(t, err)
	require.Equal(t, []kapp.ProposalVoteIndexEntry{{ProposalID: 1, Amount: 500}}, entries)
}

type voterRecordMarshalFailure struct {
	marshal.Marshalizer
	err error
}

func (m *voterRecordMarshalFailure) Marshal(obj any) ([]byte, error) {
	if _, isVoterRecord := obj.(*kapps.ProposalData_VoteDetail); isVoterRecord {
		return nil, m.err
	}

	return m.Marshalizer.Marshal(obj)
}

func TestVoteFailsWhenTheVoterRecordCannotBeStored(t *testing.T) {
	proposalKApp, accCacher, _, _ := setupVoteTest(t)

	voterAddr := makeAddress("voter")
	encodedVoter := hex.EncodeToString(voterAddr)
	createActiveProposal(t, proposalKApp, accCacher, 1, nil)
	createVoterAccount(t, accCacher, voterAddr, 1000)

	expectedErr := errors.New("voter record marshal failure")
	proposalKApp.marshalizer = &voterRecordMarshalFailure{Marshalizer: proposalKApp.marshalizer, err: expectedErr}

	resultCode, err := proposalKApp.Vote(voterAddr, &transaction.VoteContract{ProposalID: 1, Amount: 100, Type: transaction.VoteContract_Yes})
	require.Equal(t, transaction.Transaction_KAPPError, resultCode)
	require.ErrorIs(t, err, expectedErr)

	require.Nil(t, storedVoterRecord(t, accCacher, 1, encodedVoter))
	require.Zero(t, storedProposal(t, proposalKApp, accCacher, 1).Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
}

func createProposalSettingMinKFIVoteAmount(
	t *testing.T,
	fixV7 bool,
	value []byte,
) (transaction.Transaction_TXResultCode, *mock.ReceiptsContextStub, error) {
	proposalKApp, accCacher, forkController, kappController := setupVoteTest(t)
	forkController.FixAuditChangesV7Value = fixV7
	setMinKFIVoteAmount(kappController, 1)
	receipts := captureReceipts(kappController)

	proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)
	_, err = proposalKappAcc.StartProposalsKApp(forkController)
	require.NoError(t, err)
	require.NoError(t, accCacher.UpdateKapp(proposalKappAcc))
	createVoterAccount(t, accCacher, proposerAddr, 1000)

	code, err := proposalKApp.Create(proposerAddr, &transaction.ProposalContract{
		EpochsDuration: 10,
		Parameters: map[int32][]byte{
			int32(kapps.EnumParameter_MinKFIVoteAmount): value,
		},
	})

	return code, receipts, err
}

func TestCreateTreatsMinKFIVoteAmountAsAnUnknownParameterBeforeTheFork(t *testing.T) {
	cases := map[string][]byte{
		"valid value":         []byte("10000000"),
		"empty value":         {},
		"nil value":           nil,
		"not a number":        []byte("abc"),
		"not valid utf8":      {0xff, 0xfe},
		"longer than a limit": []byte(strings.Repeat("1", core.MaxProposalParamLength+1)),
	}

	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			code, receipts, err := createProposalSettingMinKFIVoteAmount(t, false, value)

			require.Equal(t, common.ErrInvalidParameter, err)
			require.Equal(t, transaction.Transaction_ParameterInvalid, code)
			errs := receipts.GetErrors()
			require.Len(t, errs, 1)
			require.Equal(t, common.ErrFieldInvalidProposalParams, string(errs[0].Data[1]))
		})
	}
}

func TestCreateValidatesMinKFIVoteAmountAfterTheFork(t *testing.T) {
	cases := []struct {
		name  string
		value []byte
		err   error
	}{
		{"the default", []byte("10000000"), nil},
		{"the floor", []byte("1"), nil},
		{"the ceiling", []byte("1000000000"), nil},
		{"below the floor", []byte("0"), common.ErrInvalidParameter},
		{"above the ceiling", []byte("1000000001"), common.ErrInvalidParameter},
		{"an empty value", []byte{}, common.ErrInvalidValue},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, receipts, err := createProposalSettingMinKFIVoteAmount(t, true, tc.value)

			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				require.Equal(t, transaction.Transaction_ParameterInvalid, code)
				return
			}

			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, code)
			require.Empty(t, receipts.GetErrors())
		})
	}
}

func TestCreateRejectsAZeroDurationProposalAfterTheFork(t *testing.T) {
	for _, fixV5 := range []bool{false, true} {
		t.Run(fmt.Sprintf("fixAuditChangesV5=%v", fixV5), func(t *testing.T) {
			proposalKApp, _, forkController, _ := setupVoteTest(t)
			forkController.FixAuditChangesV5Value = fixV5

			err := proposalKApp.validateCreateInput(&transaction.ProposalContract{
				EpochsDuration: 0,
				Parameters:     map[int32][]byte{int32(kapps.EnumParameter_ProposalMaxEpochsDuration): []byte("40")},
			})

			if fixV5 {
				require.ErrorIs(t, err, common.ErrInvalidValue)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// From FixAuditChangesV6, Create rejects a value outside its static bounds with the receipt a
// client sees, so it never reaches settlement. Before the fork it is accepted as it always was.
func TestCreateRejectsAnOutOfBoundsParameterAfterTheFork(t *testing.T) {
	cases := []struct {
		name     string
		fixV6    bool
		value    string
		rejected bool
	}{
		{"negative fee after the fork", true, "-1", true},
		{"fee at its floor after the fork", true, "0", false},
		{"negative fee before the fork", false, "-1", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proposalKApp, accCacher, forkController, kappController := setupVoteTest(t)
			forkController.FixAuditChangesV6Value = tc.fixV6

			receipts := mock.NewReceiptsContextStub()
			kappController.GetCurrentKAppContextCalled = func() kapp.KappContext {
				return &mock.KAppContextStub{
					BlockCalled: func() *block.Block {
						return &block.Block{Header: &block.BlockHeader{Epoch: 5, Timestamp: 1234567890}}
					},
					ContractIDCalled: func() int { return 1 },
					ReceiptsCalled:   func() kapp.ReceiptsContext { return receipts },
				}
			}
			kappController.GetProposalControllerCalled = func() kapps.ActiveProposalController {
				return &mock.ProposalControllerStub{
					GetParameterIntCalled: func(param kapps.EnumParameter) int64 {
						if param == kapps.EnumParameter_MinKFIStakedToEnableProposals {
							return 1000
						}
						return 0
					},
					GetParameterUintCalled: func(kapps.EnumParameter) uint64 { return 40 }, // ProposalMaxEpochsDuration
				}
			}

			proposalKappAcc, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
			require.NoError(t, err)
			_, err = proposalKappAcc.StartProposalsKApp(forkController)
			require.NoError(t, err)
			require.NoError(t, accCacher.UpdateKapp(proposalKappAcc))
			createVoterAccount(t, accCacher, proposerAddr, 1000)

			code, err := proposalKApp.Create(proposerAddr, &transaction.ProposalContract{
				EpochsDuration: 10,
				Parameters: map[int32][]byte{
					int32(kapps.EnumParameter_KAppFeeTransfer): []byte(tc.value),
				},
			})

			if !tc.rejected {
				require.NoError(t, err)
				require.Equal(t, transaction.Transaction_Ok, code)
				require.Empty(t, receipts.GetErrors())
				return
			}

			require.ErrorIs(t, err, common.ErrInvalidParameter)
			require.Equal(t, transaction.Transaction_ParameterInvalid, code)
			errs := receipts.GetErrors()
			require.Len(t, errs, 1)
			require.Equal(t, common.ErrFieldInvalidProposalParams, string(errs[0].Data[1]))
		})
	}
}
