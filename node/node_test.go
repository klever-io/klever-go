package node_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	disabledSig "github.com/klever-io/klever-go/crypto/signing/disabled/singlesig"
	"github.com/klever-io/klever-go/network/api/models"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	indexerData "github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/node"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

var sigOk = []byte{191, 150, 24, 156, 89, 18, 71, 123, 244, 251, 51, 26, 55, 130, 91, 227, 104, 159, 51, 243, 201, 219, 75, 212, 173, 18, 167, 48, 22, 49, 94, 136, 109, 173, 4, 140, 86, 193, 35, 146, 217, 154, 232, 45, 10, 117, 14, 144, 24, 177, 224, 125, 161, 190, 78, 156, 145, 162, 252, 143, 180, 218, 92, 9}
var chainID = []byte("chainID")
var errSingleSignKeyGenMock = errors.New("errSingleSignKeyGenMock")

func getMarshalizer() marshal.Marshalizer {
	return &mock.ProtoMarshalizerMock{}
}

func createMockPubkeyConverter() *mock.PubkeyConverterMock {
	return mock.NewPubkeyConverterMock(32)
}

func getHasher() hashing.Hasher {
	return &mock.HasherMock{}
}

func getAccAdapter(balance int64) *mock.AccountsStub {
	accDB := &mock.AccountsStub{}
	accDB.GetExistingAccountCalled = func(address []byte) (handler state.AccountHandler, e error) {
		acc, _ := state.NewUserAccount(address)
		_ = acc.AddToBalance(balance, nil, true)

		return acc, nil
	}
	return accDB
}

func createDummyHexAddress(hexChars int) string {
	if hexChars < 1 {
		return ""
	}

	buff := make([]byte, hexChars/2)
	_, _ = rand.Reader.Read(buff)

	return hex.EncodeToString(buff)
}

func createKeyGenMock() crypto.KeyGenerator {
	return &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, e error) {
			if string(b) == "" {
				return nil, errSingleSignKeyGenMock
			}

			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
}

func getKAppController(t *testing.T, accAdapter state.AccountsAdapter) kapp.KAppController {
	marshalizerMock := &mock.ProtoMarshalizerMock{}
	pubkeyConvMock := createMockPubkeyConverter()
	ratingsDataMock := &mock.RatingsInfoMock{}

	accCacher, err := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: accAdapter,
			Kapps:    accAdapter,
			Peers:    accAdapter,
		},
	)

	require.Nil(t, err)

	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	argsKapp := kappcontroller.ArgsNewKApp{
		Hasher:         getHasher(),
		Marshalizer:    marshalizerMock,
		PubkeyConv:     pubkeyConvMock,
		ForkController: forkController,
		AccountsCacher: accCacher,
		RatingsData:    ratingsDataMock,
	}

	kAppController, err := kappcontroller.NewKappController(argsKapp)
	require.Nil(t, err)

	return kAppController
}

func createNode(t *testing.T) (*node.Node, error) {
	accAdapter := getAccAdapter(100)
	return createNodeWithAccountsAdapter(t, accAdapter)
}

func createNodeWithAccountsAdapter(t *testing.T, accAdapter *mock.AccountsStub) (*node.Node, error) {
	uint64Converter := mock.NewNonceHashConverterMock()
	storerMock := mock.NewStorerMock("", 0)

	dataPool := &mock.PoolsHolderStub{
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return &mock.ShardedDataStub{}
		},
	}
	keyGen := &cryptoMock.KeyGenMock{
		PublicKeyFromByteArrayMock: func(b []byte) (crypto.PublicKey, error) {
			return nil, nil
		},
	}
	feeHandler := &mock.FeeHandlerStub{}

	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	proposalController, _ := kapps.NewProposalController(forkController)

	kappController := getKAppController(t, accAdapter)

	n, err := node.NewNode(
		node.WithDataPool(dataPool),
		node.WithInternalMarshalizer(getMarshalizer()),
		node.WithTxSignMarshalizer(getMarshalizer()),
		node.WithDataStore(&mock.ChainStorerMock{
			GetCalled: func(unitType retriever.UnitType, key []byte) ([]byte, error) {
				return storerMock.Get(key)
			},
			GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
				return storerMock
			},
		}),
		node.WithUint64ByteSliceConverter(uint64Converter),
		node.WithAddressPubkeyConverter(createMockPubkeyConverter()),
		node.WithValidatorPubkeyConverter(createMockPubkeyConverter()),
		node.WithAccountsAdapter(accAdapter),
		node.WithWhiteListHandler(&mock.WhiteListHandlerStub{}),
		node.WithWhiteListHandlerVerified(&mock.WhiteListHandlerStub{}),
		node.WithUint64ByteSliceConverter(uint64Converter),
		node.WithHasher(getHasher()),
		node.WithTxSignHasher(getHasher()),
		node.WithKeyGen(createKeyGenMock()),
		node.WithKeyGenForAccounts(keyGen),
		node.WithTxFeeHandler(feeHandler),
		node.WithSingleSigner(&cryptoMock.SignerMock{}),
		node.WithTxSingleSigner(&disabledSig.DisabledSingleSig{}),
		node.WithChainID(chainID),
		node.WithProposalController(proposalController),
		node.WithKAppController(kappController),
		node.WithForkController(forkController),
	)
	require.Nil(t, err)

	return n, nil
}

func createNodeWithFeeHandler(t *testing.T, feeHandler *mock.FeeHandlerStub) (*node.Node, error) {
	uint64Converter := mock.NewNonceHashConverterMock()
	storerMock := mock.NewStorerMock("", 0)

	dataPool := &mock.PoolsHolderStub{
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return &mock.ShardedDataStub{}
		},
	}
	keyGen := &cryptoMock.KeyGenMock{
		PublicKeyFromByteArrayMock: func(b []byte) (crypto.PublicKey, error) {
			return nil, nil
		},
	}

	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	proposalController, _ := kapps.NewProposalController(forkController)

	accAdapter := getAccAdapter(1e9)
	kappController := getKAppController(t, accAdapter)

	n, err := node.NewNode(
		node.WithDataPool(dataPool),
		node.WithInternalMarshalizer(getMarshalizer()),
		node.WithTxSignMarshalizer(getMarshalizer()),
		node.WithDataStore(&mock.ChainStorerMock{
			GetCalled: func(unitType retriever.UnitType, key []byte) ([]byte, error) {
				return storerMock.Get(key)
			},
			GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
				return storerMock
			},
		}),
		node.WithUint64ByteSliceConverter(uint64Converter),
		node.WithAddressPubkeyConverter(createMockPubkeyConverter()),
		node.WithValidatorPubkeyConverter(createMockPubkeyConverter()),
		node.WithAccountsAdapter(accAdapter),
		node.WithWhiteListHandler(&mock.WhiteListHandlerStub{}),
		node.WithWhiteListHandlerVerified(&mock.WhiteListHandlerStub{}),
		node.WithUint64ByteSliceConverter(uint64Converter),
		node.WithHasher(getHasher()),
		node.WithTxSignHasher(getHasher()),
		node.WithKeyGen(createKeyGenMock()),
		node.WithKeyGenForAccounts(keyGen),
		node.WithTxFeeHandler(feeHandler),
		node.WithSingleSigner(&cryptoMock.SignerMock{}),
		node.WithTxSingleSigner(&disabledSig.DisabledSingleSig{}),
		node.WithChainID(chainID),
		node.WithProposalController(proposalController),
		node.WithKAppController(kappController),
		node.WithForkController(forkController),
	)
	require.Nil(t, err)

	return n, nil
}

// nodeTestOptions holds optional parameters for creating test nodes.
type nodeTestOptions struct {
	accAdapter     *mock.AccountsStub
	kappsAdapter   *mock.AccountsStub
	kappController kapp.KAppController
	blockchain     *mock.BlockChainMock
}

// createNodeWithOptions creates a node with customizable options for testing.
// All fields in nodeTestOptions are optional - nil values are skipped.
func createNodeWithOptions(t *testing.T, opts nodeTestOptions) (*node.Node, error) {
	uint64Converter := mock.NewNonceHashConverterMock()
	storerMock := mock.NewStorerMock("", 0)

	dataPool := &mock.PoolsHolderStub{
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return &mock.ShardedDataStub{}
		},
	}
	keyGen := &cryptoMock.KeyGenMock{
		PublicKeyFromByteArrayMock: func(b []byte) (crypto.PublicKey, error) {
			return nil, nil
		},
	}
	feeHandler := &mock.FeeHandlerStub{}

	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	proposalController, _ := kapps.NewProposalController(forkController)

	// Build options list with required options
	options := []node.Option{
		node.WithDataPool(dataPool),
		node.WithInternalMarshalizer(getMarshalizer()),
		node.WithTxSignMarshalizer(getMarshalizer()),
		node.WithDataStore(&mock.ChainStorerMock{
			GetCalled: func(unitType retriever.UnitType, key []byte) ([]byte, error) {
				return storerMock.Get(key)
			},
			GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
				return storerMock
			},
		}),
		node.WithUint64ByteSliceConverter(uint64Converter),
		node.WithAddressPubkeyConverter(createMockPubkeyConverter()),
		node.WithValidatorPubkeyConverter(createMockPubkeyConverter()),
		node.WithWhiteListHandler(&mock.WhiteListHandlerStub{}),
		node.WithWhiteListHandlerVerified(&mock.WhiteListHandlerStub{}),
		node.WithHasher(getHasher()),
		node.WithTxSignHasher(getHasher()),
		node.WithKeyGen(createKeyGenMock()),
		node.WithKeyGenForAccounts(keyGen),
		node.WithTxFeeHandler(feeHandler),
		node.WithSingleSigner(&cryptoMock.SignerMock{}),
		node.WithTxSingleSigner(&disabledSig.DisabledSingleSig{}),
		node.WithChainID(chainID),
		node.WithProposalController(proposalController),
		node.WithForkController(forkController),
	}

	// Add optional options only if provided
	if opts.accAdapter != nil {
		options = append(options, node.WithAccountsAdapter(opts.accAdapter))
	}
	if opts.kappsAdapter != nil {
		options = append(options, node.WithKAppsAdapter(opts.kappsAdapter))
	}
	if opts.kappController != nil {
		options = append(options, node.WithKAppController(opts.kappController))
	}
	if opts.blockchain != nil {
		options = append(options, node.WithBlockChain(opts.blockchain))
	}

	n, err := node.NewNode(options...)
	require.Nil(t, err)

	return n, nil
}

// createNodeWithKAppController is a convenience wrapper for tests that only need
// accounts adapter and kapp controller.
func createNodeWithKAppController(t *testing.T, accAdapter *mock.AccountsStub, kappController kapp.KAppController) (*node.Node, error) {
	return createNodeWithOptions(t, nodeTestOptions{
		accAdapter:     accAdapter,
		kappController: kappController,
	})
}

func TestWithConsensusMonitoring(t *testing.T) {
	t.Parallel()

	t.Run("nil config disables monitoring", func(t *testing.T) {
		t.Parallel()

		n, err := createNode(t)
		require.NoError(t, err)

		err = n.ApplyOptions(node.WithConsensusMonitoring(nil))
		require.NoError(t, err)
		assert.Equal(t, uint32(0), n.GetNetworkDegradedThreshold())
		assert.Equal(t, uint32(0), n.GetNetworkDegradedCooldownSlots())
	})

	t.Run("zero threshold disables monitoring", func(t *testing.T) {
		t.Parallel()

		n, err := createNode(t)
		require.NoError(t, err)

		cfg := &config.ConsensusMonitoringConfig{
			NetworkDegradedThreshold:     0,
			NetworkDegradedCooldownSlots: 10,
		}

		err = n.ApplyOptions(node.WithConsensusMonitoring(cfg))
		require.NoError(t, err)
		assert.Equal(t, uint32(0), n.GetNetworkDegradedThreshold())
		assert.Equal(t, uint32(0), n.GetNetworkDegradedCooldownSlots())
	})

	t.Run("valid config sets threshold and cooldown", func(t *testing.T) {
		t.Parallel()

		n, err := createNode(t)
		require.NoError(t, err)

		cfg := &config.ConsensusMonitoringConfig{
			NetworkDegradedThreshold:     3,
			NetworkDegradedCooldownSlots: 15,
		}

		err = n.ApplyOptions(node.WithConsensusMonitoring(cfg))
		require.NoError(t, err)
		assert.Equal(t, uint32(3), n.GetNetworkDegradedThreshold())
		assert.Equal(t, uint32(15), n.GetNetworkDegradedCooldownSlots())
	})

	t.Run("valid config via NewNode sets threshold and cooldown", func(t *testing.T) {
		t.Parallel()

		cfg := &config.ConsensusMonitoringConfig{
			NetworkDegradedThreshold:     5,
			NetworkDegradedCooldownSlots: 20,
		}

		accAdapter := getAccAdapter(100)
		uint64Converter := mock.NewNonceHashConverterMock()
		storerMock := mock.NewStorerMock("", 0)
		dataPool := &mock.PoolsHolderStub{
			TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
				return &mock.ShardedDataStub{}
			},
		}
		keyGen := &cryptoMock.KeyGenMock{
			PublicKeyFromByteArrayMock: func(b []byte) (crypto.PublicKey, error) {
				return nil, nil
			},
		}
		epochNotifier := &mock.EpochNotifierStub{}
		forkController, _ := fork.NewForkController(config.EnableEpochs{}, epochNotifier)
		proposalController, _ := kapps.NewProposalController(forkController)
		kappController := getKAppController(t, accAdapter)

		n, err := node.NewNode(
			node.WithDataPool(dataPool),
			node.WithInternalMarshalizer(getMarshalizer()),
			node.WithTxSignMarshalizer(getMarshalizer()),
			node.WithDataStore(&mock.ChainStorerMock{
				GetCalled: func(unitType retriever.UnitType, key []byte) ([]byte, error) {
					return storerMock.Get(key)
				},
				GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
					return storerMock
				},
			}),
			node.WithUint64ByteSliceConverter(uint64Converter),
			node.WithAddressPubkeyConverter(createMockPubkeyConverter()),
			node.WithValidatorPubkeyConverter(createMockPubkeyConverter()),
			node.WithAccountsAdapter(accAdapter),
			node.WithWhiteListHandler(&mock.WhiteListHandlerStub{}),
			node.WithWhiteListHandlerVerified(&mock.WhiteListHandlerStub{}),
			node.WithHasher(getHasher()),
			node.WithTxSignHasher(getHasher()),
			node.WithKeyGen(createKeyGenMock()),
			node.WithKeyGenForAccounts(keyGen),
			node.WithTxFeeHandler(&mock.FeeHandlerStub{}),
			node.WithSingleSigner(&cryptoMock.SignerMock{}),
			node.WithTxSingleSigner(&disabledSig.DisabledSingleSig{}),
			node.WithChainID(chainID),
			node.WithProposalController(proposalController),
			node.WithKAppController(kappController),
			node.WithForkController(forkController),
			node.WithConsensusMonitoring(cfg),
		)

		require.NoError(t, err)
		assert.Equal(t, uint32(5), n.GetNetworkDegradedThreshold())
		assert.Equal(t, uint32(20), n.GetNetworkDegradedCooldownSlots())
	})
}

func TestCreateTransaction_ShouldWork(t *testing.T) {
	t.Parallel()

	n, err := createNode(t)
	require.NoError(t, err)

	transferContract, _ := json.Marshal(struct {
		Receiver string
		Amount   int64
		Asset    string
	}{
		Receiver: createDummyHexAddress(64),
		Amount:   10,
		Asset:    "token",
	})

	baseInfo := &transaction.TXBaseInfo{
		Sender:    createDummyHexAddress(64),
		Nonce:     uint64(0),
		DataField: [][]byte{[]byte("data")},
	}

	tx, txHash, err := n.CreateTransaction(0, baseInfo, []json.RawMessage{transferContract}, false)
	assert.NotNil(t, tx)
	assert.NotNil(t, txHash)
	assert.NoError(t, err)

	freezeContract, _ := json.Marshal(struct {
		ContractType uint64 `json:"contractType"`
		models.FreezeTXRequest
	}{
		ContractType: 4,
		FreezeTXRequest: models.FreezeTXRequest{
			Amount: 1000_000_000,
			KDA:    "KLV",
		},
	})

	baseInfo.Nonce++

	tx, txHash, err = n.CreateTransaction(0, baseInfo, []json.RawMessage{transferContract, freezeContract}, false)
	assert.NotNil(t, tx)
	assert.NotNil(t, txHash)
	assert.NoError(t, err)
}

func TestCreateTransaction_ShouldFail(t *testing.T) {
	t.Parallel()

	n, err := createNode(t)
	require.NoError(t, err)

	transferContract, _ := json.Marshal(struct {
		Receiver string
		Amount   int64
		Asset    string
	}{
		Receiver: createDummyHexAddress(64),
		Amount:   10,
		Asset:    "token",
	})

	byteSlice := make([]byte, core.MaxDataSize+1)

	baseInfo := &transaction.TXBaseInfo{
		Sender:    createDummyHexAddress(64),
		Nonce:     uint64(0),
		DataField: [][]byte{byteSlice},
	}

	_, _, err = n.CreateTransaction(0, baseInfo, []json.RawMessage{transferContract}, false)
	require.Error(t, common.ErrDataFieldTooBig, err)
}

func TestSendTransaction_ShouldWork(t *testing.T) {
	t.Parallel()

	n, err := createNode(t)
	require.NoError(t, err)

	transferContract, _ := json.Marshal(struct {
		Receiver string
		Amount   int64
		Asset    string
	}{
		Receiver: createDummyHexAddress(64),
		Amount:   10,
		Asset:    "token",
	})

	baseInfo := &transaction.TXBaseInfo{
		Sender:    createDummyHexAddress(64),
		Nonce:     uint64(0),
		DataField: [][]byte{[]byte("data")},
	}

	tx, _, err := n.CreateTransaction(0, baseInfo, []json.RawMessage{transferContract}, false)
	require.NoError(t, err)

	tx.AddSignature(sigOk)

	_, err = n.SendTransaction(tx)
	require.NoError(t, err)
}

func TestSendBulkTransactions_ShouldWork(t *testing.T) {
	t.Parallel()

	n, err := createNode(t)
	require.NoError(t, err)

	transferContract, _ := json.Marshal(struct {
		Receiver string
		Amount   int64
		Asset    string
	}{
		Receiver: createDummyHexAddress(64),
		Amount:   10,
		Asset:    "token",
	})

	baseInfo := &transaction.TXBaseInfo{
		Sender:    createDummyHexAddress(64),
		Nonce:     uint64(0),
		DataField: [][]byte{[]byte("data")},
	}

	tx, _, err := n.CreateTransaction(0, baseInfo, []json.RawMessage{transferContract}, false)
	require.NoError(t, err)

	tx.AddSignature(sigOk)

	_, err = n.SendBulkTransactions([]*transaction.Transaction{tx})
	require.NoError(t, err)
}

func TestSendBulkTransactions_NilTransactionShouldError(t *testing.T) {
	t.Parallel()

	n, err := createNode(t)
	require.NoError(t, err)

	transferContract, _ := json.Marshal(struct {
		Receiver string
		Amount   int64
		Asset    string
	}{
		Receiver: createDummyHexAddress(64),
		Amount:   10,
		Asset:    "token",
	})

	baseInfo := &transaction.TXBaseInfo{
		Sender:    createDummyHexAddress(64),
		Nonce:     uint64(0),
		DataField: [][]byte{[]byte("data")},
	}

	tx, _, err := n.CreateTransaction(0, baseInfo, []json.RawMessage{transferContract}, false)
	require.NoError(t, err)

	tx.AddSignature(sigOk)

	t.Run("only a nil entry", func(t *testing.T) {
		t.Parallel()

		_, err := n.SendBulkTransactions([]*transaction.Transaction{nil})
		require.ErrorIs(t, err, common.ErrNilTransaction)
	})

	t.Run("nil entry after a valid one", func(t *testing.T) {
		t.Parallel()

		_, err := n.SendBulkTransactions([]*transaction.Transaction{tx, nil})
		require.ErrorIs(t, err, common.ErrNilTransaction)
		require.ErrorContains(t, err, "invalid transaction 1")
	})
}

func TestEstimateTransactionsFees(t *testing.T) {
	kAppFee := int64(1e6)
	bandwidthFee := int64(1e6)
	gasMultiplier := uint64(10)

	validAddress, err := hex.DecodeString(createDummyHexAddress(64))
	require.NoError(t, err)

	feeHandler := &mock.FeeHandlerStub{
		ComputeTransactionCostCalled: func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
			return &transaction.CostResponse{
				KAppFee:       kAppFee,
				BandwidthFee:  bandwidthFee,
				GasMultiplier: gasMultiplier,
			}, nil
		},
	}

	n, err := createNodeWithFeeHandler(t, feeHandler)
	require.Nil(t, err)

	t.Run("should fail with nil transaction", func(t *testing.T) {
		t.Parallel()

		var tx *transaction.Transaction

		cost, err := n.EstimateTransactionFees(tx)
		assert.Nil(t, cost)
		assert.Equal(t, common.ErrNilTransaction, err)
	})

	t.Run("should fail with empty raw transaction", func(t *testing.T) {
		t.Parallel()

		tx := &transaction.Transaction{}

		cost, err := n.EstimateTransactionFees(tx)
		assert.Nil(t, cost)
		assert.Equal(t, common.ErrNilRawTransaction, err)
	})

	t.Run("should fail missing contract", func(t *testing.T) {
		t.Parallel()

		tx := &transaction.Transaction{
			RawData: &transaction.Transaction_Raw{
				Sender: validAddress,
			},
		}

		cost, err := n.EstimateTransactionFees(tx)
		assert.Nil(t, cost)
		assert.NotNil(t, err)
	})

	t.Run("should fail with nil contract", func(t *testing.T) {
		t.Parallel()

		feeHandlerCalled := false
		feeHandler := &mock.FeeHandlerStub{
			ComputeTransactionCostCalled: func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
				feeHandlerCalled = true
				return &transaction.CostResponse{}, nil
			},
		}
		nodeWithFeeHandler, err := createNodeWithFeeHandler(t, feeHandler)
		require.Nil(t, err)

		tx := &transaction.Transaction{
			RawData: &transaction.Transaction_Raw{
				Sender:   validAddress,
				Contract: []*transaction.TXContract{nil},
			},
		}

		cost, err := nodeWithFeeHandler.EstimateTransactionFees(tx)
		assert.Nil(t, cost)
		assert.Equal(t, common.ErrInvalidContract, err)
		assert.False(t, feeHandlerCalled)
	})

	t.Run("should work transaction with transfer contract", func(t *testing.T) {
		t.Parallel()

		tx := transaction.NewBaseTransaction(validAddress, 0, nil, 0, 0)
		err = tx.SetChainID(chainID)
		require.Nil(t, err)

		txArgs := transaction.TXArgs{
			Type:   uint32(transaction.TXContract_TransferContractType),
			Sender: validAddress,
			Contract: json.RawMessage(`{
				"receiver": "ff5f4bf41899fcabd6751809c037f7f18838eacad8c59d27f221dc9be9301854",
				"amount": 1000,
				"KDA": "KLV"
			}`),
			NodeHelper: n,
		}

		err = tx.AddTransaction(txArgs)
		require.Nil(t, err)

		cost, err := n.EstimateTransactionFees(tx)
		require.NotNil(t, cost)
		require.Nil(t, err)

		assert.Equal(t, kAppFee, cost.KAppFee)
		assert.Equal(t, bandwidthFee, cost.BandwidthFee)
	})

	t.Run("should fail transaction with smart contract estimated gas too big", func(t *testing.T) {
		t.Parallel()

		gasMultiplier := uint64(1)

		feeHandler := &mock.FeeHandlerStub{
			ComputeTransactionCostCalled: func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
				return &transaction.CostResponse{
					KAppFee:       kAppFee,
					BandwidthFee:  bandwidthFee,
					GasMultiplier: gasMultiplier,
					GasEstimated:  math.MaxInt64 + 1,
				}, nil
			},
		}

		n, err := createNodeWithFeeHandler(t, feeHandler)
		require.Nil(t, err)

		tx := transaction.NewBaseTransaction(validAddress, 0, nil, 0, 0)
		err = tx.SetChainID(chainID)
		require.Nil(t, err)

		txArgs := transaction.TXArgs{
			Type:   uint32(transaction.TXContract_SmartContractType),
			Sender: validAddress,
			Contract: json.RawMessage(`{
				"SCType": 1,
				"callValue": {
					"KLV": 1000
				}
			}`),
			NodeHelper: n,
		}

		err = tx.AddTransaction(txArgs)
		require.Nil(t, err)

		cost, err := n.EstimateTransactionFees(tx)
		require.Nil(t, cost)
		require.Equal(t, common.ErrEstimateGasTooBig, err)
	})

	t.Run("should fail with nil vm output", func(t *testing.T) {
		t.Parallel()

		feeHandler := &mock.FeeHandlerStub{
			ComputeTransactionCostCalled: func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
				return nil, process.ErrNilVMOutput
			},
		}

		n, err := createNodeWithFeeHandler(t, feeHandler)
		require.Nil(t, err)

		tx := transaction.NewBaseTransaction(validAddress, 0, nil, 0, 0)
		err = tx.SetChainID(chainID)
		require.Nil(t, err)

		txArgs := transaction.TXArgs{
			Type:   uint32(transaction.TXContract_SmartContractType),
			Sender: validAddress,
			Contract: json.RawMessage(`{
				"SCType": 1,
				"callValue": {
					"KLV": 1000
				}
			}`),
			NodeHelper: n,
		}

		err = tx.AddTransaction(txArgs)
		require.Nil(t, err)

		cost, err := n.EstimateTransactionFees(tx)
		require.Nil(t, cost)
		require.Equal(t, process.ErrNilVMOutput, err)
	})

	t.Run("should work transaction with smart contract transaction", func(t *testing.T) {
		t.Parallel()

		gasEstimated := uint64(1e3)
		expectedTotalBandwidthFee := bandwidthFee + int64(gasEstimated/gasMultiplier)

		feeHandler := &mock.FeeHandlerStub{
			ComputeTransactionCostCalled: func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
				return &transaction.CostResponse{
					KAppFee:       kAppFee,
					BandwidthFee:  bandwidthFee,
					GasMultiplier: gasMultiplier,
					GasEstimated:  gasEstimated,
				}, nil
			},
		}

		n, err := createNodeWithFeeHandler(t, feeHandler)
		require.Nil(t, err)

		tx := transaction.NewBaseTransaction(validAddress, 0, nil, 0, 0)
		err = tx.SetChainID(chainID)
		require.Nil(t, err)

		txArgs := transaction.TXArgs{
			Type:   uint32(transaction.TXContract_SmartContractType),
			Sender: validAddress,
			Contract: json.RawMessage(`{
				"SCType": 1,
				"callValue": {
					"KLV": 1000
				}
			}`),
			NodeHelper: n,
		}

		err = tx.AddTransaction(txArgs)
		require.Nil(t, err)

		cost, err := n.EstimateTransactionFees(tx)
		require.NotNil(t, cost)
		require.Nil(t, err)

		assert.Equal(t, kAppFee, cost.KAppFee)
		assert.Equal(t, expectedTotalBandwidthFee, cost.BandwidthFee)
	})
}

func TestDecodeTransaction(t *testing.T) {
	t.Parallel()

	validAddress, err := hex.DecodeString(createDummyHexAddress(64))
	require.NoError(t, err)

	n, err := createNode(t)
	require.NoError(t, err)

	t.Run("should fail with nil transaction", func(t *testing.T) {
		t.Parallel()

		var tx *transaction.Transaction

		decoded, err := n.DecodeTransaction(tx)
		assert.Nil(t, decoded)
		assert.Equal(t, common.ErrNilTransaction, err)
	})

	t.Run("should fail with empty raw transaction", func(t *testing.T) {
		t.Parallel()

		tx := &transaction.Transaction{}

		decoded, err := n.DecodeTransaction(tx)
		assert.Nil(t, decoded)
		assert.Equal(t, common.ErrNilRawTransaction, err)
	})

	t.Run("should fail with nil contract", func(t *testing.T) {
		t.Parallel()

		tx := &transaction.Transaction{
			RawData: &transaction.Transaction_Raw{
				Sender:   validAddress,
				Contract: []*transaction.TXContract{nil},
			},
		}

		decoded, err := n.DecodeTransaction(tx)
		assert.Nil(t, decoded)
		assert.Equal(t, common.ErrInvalidContract, err)
	})

	t.Run("should fail with too many contracts", func(t *testing.T) {
		t.Parallel()

		contracts := make([]*transaction.TXContract, core.MaxLengthOfContracts+1)
		for i := range contracts {
			contracts[i] = &transaction.TXContract{}
		}
		tx := &transaction.Transaction{
			RawData: &transaction.Transaction_Raw{
				Sender:   validAddress,
				Contract: contracts,
			},
		}

		decoded, err := n.DecodeTransaction(tx)
		assert.Nil(t, decoded)
		assert.Equal(t, common.ErrInvalidContract, err)
	})

	t.Run("should not reject the maximum number of contracts", func(t *testing.T) {
		t.Parallel()

		parameter, err := anypb.New(&transaction.CreateValidatorContract{})
		require.NoError(t, err)
		contracts := make([]*transaction.TXContract, core.MaxLengthOfContracts)
		for i := range contracts {
			contracts[i] = &transaction.TXContract{
				Type:      transaction.TXContract_CreateValidatorContractType,
				Parameter: parameter,
			}
		}
		tx := &transaction.Transaction{
			RawData: &transaction.Transaction_Raw{
				Sender:   validAddress,
				Contract: contracts,
			},
		}

		decoded, err := n.DecodeTransaction(tx)
		require.NoError(t, err)
		require.NotNil(t, decoded)
		assert.Len(t, decoded.Contracts, core.MaxLengthOfContracts)
	})

	t.Run("should work with empty validator contracts", func(t *testing.T) {
		t.Parallel()

		createValidatorParameter, err := anypb.New(&transaction.CreateValidatorContract{})
		require.NoError(t, err)
		validatorConfigParameter, err := anypb.New(&transaction.ValidatorConfigContract{})
		require.NoError(t, err)

		tx := &transaction.Transaction{
			RawData: &transaction.Transaction_Raw{
				Sender: validAddress,
				Contract: []*transaction.TXContract{
					{Type: transaction.TXContract_CreateValidatorContractType, Parameter: createValidatorParameter},
					{Type: transaction.TXContract_ValidatorConfigContractType, Parameter: validatorConfigParameter},
				},
			},
		}

		decoded, err := n.DecodeTransaction(tx)
		require.NoError(t, err)
		require.NotNil(t, decoded)
		assert.Len(t, decoded.Contracts, 2)
	})

	t.Run("should work transaction with transfer contract", func(t *testing.T) {
		t.Parallel()

		tx := transaction.NewBaseTransaction(validAddress, 0, nil, 0, 0)
		err := tx.SetChainID(chainID)
		require.Nil(t, err)

		txArgs := transaction.TXArgs{
			Type:   uint32(transaction.TXContract_TransferContractType),
			Sender: validAddress,
			Contract: json.RawMessage(`{
				"receiver": "ff5f4bf41899fcabd6751809c037f7f18838eacad8c59d27f221dc9be9301854",
				"amount": 1000,
				"KDA": "KLV"
			}`),
			NodeHelper: n,
		}

		err = tx.AddTransaction(txArgs)
		require.Nil(t, err)

		decoded, err := n.DecodeTransaction(tx)
		require.Nil(t, err)
		require.NotNil(t, decoded)
		assert.Len(t, decoded.Contracts, 1)
		assert.NotEmpty(t, decoded.Hash)
	})
}

func TestDecodeTransaction_WrongSizeAddressesSkipConverter(t *testing.T) {
	t.Parallel()

	const addressLen = 32
	encodedLens := make([]int, 0)
	converter := &mock.PubkeyConverterStub{
		LenCalled: func() int { return addressLen },
		EncodeCalled: func(pkBytes []byte) string {
			encodedLens = append(encodedLens, len(pkBytes))
			return hex.EncodeToString(pkBytes)
		},
	}
	n, err := node.NewNode(
		node.WithAddressPubkeyConverter(converter),
		node.WithValidatorPubkeyConverter(createMockPubkeyConverter()),
		node.WithTxSignMarshalizer(getMarshalizer()),
		node.WithTxSignHasher(getHasher()),
	)
	require.NoError(t, err)

	validAddress := bytes.Repeat([]byte{1}, addressLen)
	roles := []*transaction.RolesInfo{{Address: validAddress}}
	for i := 0; i < 1000; i++ {
		roles = append(roles, &transaction.RolesInfo{Address: []byte{byte(i)}})
	}
	parameter, err := anypb.New(&transaction.CreateAssetContract{
		OwnerAddress: validAddress,
		Roles:        roles,
	})
	require.NoError(t, err)
	tx := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{
			Sender: []byte{1},
			Contract: []*transaction.TXContract{
				{Type: transaction.TXContract_CreateAssetContractType, Parameter: parameter},
			},
		},
	}

	decoded, err := n.DecodeTransaction(tx)
	require.NoError(t, err)
	require.NotNil(t, decoded)
	assert.Empty(t, decoded.Sender)
	for _, l := range encodedLens {
		assert.Equal(t, addressLen, l)
	}

	parsed, ok := decoded.Contracts[0].Parameter.(indexerData.CreateAssetContract)
	require.True(t, ok)
	require.Len(t, parsed.Roles, len(roles))
	assert.Equal(t, hex.EncodeToString(validAddress), parsed.Roles[0].Address)
	assert.Empty(t, parsed.Roles[1].Address)
}

// the concurrent case lives in TestNode_PublishProposalController, which races
// readers against the wiring path StartConsensus actually uses
func TestNode_GetProposalParameters(t *testing.T) {
	t.Parallel()

	n, err := node.NewNode()
	require.Nil(t, err)

	// API requests are served from the moment the REST server starts, which is
	// before StartConsensus wires the proposal controller in
	params, err := n.GetProposalParameters()
	require.Nil(t, params)
	require.Equal(t, process.ErrProposalNotInitialized, err)

	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{}, epochNotifier)
	proposalController, _ := kapps.NewProposalController(forkController)

	require.Nil(t, node.WithProposalController(proposalController)(n))

	params, err = n.GetProposalParameters()
	require.Nil(t, err)
	require.Equal(t, proposalController.GetActiveParameters(), params)
}

func TestNode_PublishProposalController(t *testing.T) {
	t.Parallel()

	newController := func() kapps.ActiveProposalController {
		forkController, _ := fork.NewForkController(config.EnableEpochs{}, &mock.EpochNotifierStub{})
		controller, err := kapps.NewProposalController(forkController)
		require.Nil(t, err)

		return controller
	}

	// createNodeWithConsumers wires a node with the four consumers
	// publishProposalController fans out to, each recording what it received
	createNodeWithConsumers := func(t *testing.T, kappErr error) (*node.Node, map[string]kapps.ActiveProposalController) {
		got := make(map[string]kapps.ActiveProposalController)
		record := func(name string) func(kapps.ActiveProposalController) error {
			return func(controller kapps.ActiveProposalController) error {
				got[name] = controller
				return nil
			}
		}

		n, err := node.NewNode(
			node.WithTxFeeHandler(&mock.FeeHandlerStub{SetProposalControllerCalled: record("feeHandler")}),
			node.WithBlockProcessor(&consensusMock.BlockProcessorMock{SetProposalControllerCalled: record("blockProcessor")}),
			node.WithKAppController(&stub.KAppControllerStub{SetProposalControllerCalled: func(controller kapps.ActiveProposalController) error {
				if kappErr != nil {
					return kappErr
				}

				return record("kappController")(controller)
			}}),
			node.WithSimulatorKAppController(&stub.KAppControllerStub{SetProposalControllerCalled: record("kappControllerSimulator")}),
		)
		require.Nil(t, err)

		return n, got
	}

	t.Run("every consumer receives the same controller", func(t *testing.T) {
		t.Parallel()

		n, got := createNodeWithConsumers(t, nil)
		controller := newController()

		require.Nil(t, n.PublishProposalController(controller))

		require.Len(t, got, 4)
		for name, received := range got {
			assert.Same(t, controller, received, "consumer %s got a different controller", name)
		}

		params, err := n.GetProposalParameters()
		require.Nil(t, err)
		require.Equal(t, controller.GetActiveParameters(), params)
	})

	t.Run("a failing consumer aborts the wiring", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("kapp controller refused")
		n, got := createNodeWithConsumers(t, expectedErr)

		require.Equal(t, expectedErr, n.PublishProposalController(newController()))

		// the setters after the failing one never ran
		require.NotContains(t, got, "kappControllerSimulator")
	})

	t.Run("readers race against the wiring", func(t *testing.T) {
		t.Parallel()

		n, _ := createNodeWithConsumers(t, nil)
		controller := newController()

		// the REST API is already serving when StartConsensus publishes
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := 0; i < 1000; i++ {
				params, err := n.GetProposalParameters()
				if err == nil {
					require.NotEmpty(t, params)
					continue
				}
				require.Equal(t, process.ErrProposalNotInitialized, err)
			}
		}()

		require.Nil(t, n.PublishProposalController(controller))
		<-done
	})
}
