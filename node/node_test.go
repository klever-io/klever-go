package node_test

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	disabledSig "github.com/klever-io/klever-go/crypto/signing/disabled/singlesig"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/network/api/models"
	heartbeatProcess "github.com/klever-io/klever-go/node/heartbeat/process"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage/timecache"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/core/watchdog"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/node"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// nonSubscriberForkController wraps a ForkController so the wrapper type does
// NOT satisfy core.EpochSubscriberHandler; StartConsensus then fails on its
// fork-controller type assertion right after the coordinator readiness check,
// which lets the tests below stop deterministically at that point
type nonSubscriberForkController struct {
	core.ForkController
}

// createStartConsensusNode builds a node with just enough dependencies for
// StartConsensus to reach the nodes-coordinator readiness check
// crtHeader is variadic so the existing two-argument call sites keep compiling;
// pass a header to exercise the restart path where the epoch comes from the
// current block instead of the genesis fallback
func createStartConsensusNode(t *testing.T, coordinator sharding.NodesCoordinator, crtHeader ...data.HeaderHandler) *node.Node {
	storerMock := mock.NewStorerMock("", 0)
	genesisHeader := &block.Block{Header: &block.BlockHeader{Epoch: 0}}

	dataPool := &mock.PoolsHolderStub{
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return &mock.ShardedDataStub{}
		},
		HeadersCalled: func() retriever.HeadersPool {
			return &mock.HeadersCacherStub{}
		},
	}

	kappsAdapter := &mock.AccountsStub{
		LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) {
			return &mock.KAppAccountHandlerStub{}, nil
		},
	}

	n, err := node.NewNode(
		node.WithDataPool(dataPool),
		node.WithInternalMarshalizer(getMarshalizer()),
		node.WithTxSignMarshalizer(getMarshalizer()),
		node.WithDataStore(&mock.ChainStorerMock{
			GetCalled: func(_ retriever.UnitType, key []byte) ([]byte, error) {
				return storerMock.Get(key)
			},
			GetStorerCalled: func(_ retriever.UnitType) storage.Storer {
				return storerMock
			},
		}),
		node.WithUint64ByteSliceConverter(mock.NewNonceHashConverterMock()),
		node.WithAddressPubkeyConverter(createMockPubkeyConverter()),
		node.WithValidatorPubkeyConverter(createMockPubkeyConverter()),
		node.WithAccountsAdapter(getAccAdapter(100)),
		node.WithKAppsAdapter(kappsAdapter),
		node.WithHasher(getHasher()),
		node.WithTxSignHasher(getHasher()),
		node.WithKeyGen(createKeyGenMock()),
		node.WithTxFeeHandler(&mock.FeeHandlerStub{}),
		node.WithSingleSigner(&cryptoMock.SignerMock{}),
		node.WithTxSingleSigner(&disabledSig.DisabledSingleSig{}),
		node.WithChainID(chainID),
		node.WithBlockChain(&mock.BlockChainMock{
			GetGenesisHeaderCalled: func() data.HeaderHandler {
				return genesisHeader
			},
			GetGenesisHeaderHashCalled: func() []byte {
				return []byte("genesis hash")
			},
			GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
				if len(crtHeader) == 0 {
					return nil
				}
				return crtHeader[0]
			},
		}),
		node.WithMessenger(&mock.MessengerStub{}),
		node.WithPrivKey(&cryptoMock.PrivateKeyStub{}),
		node.WithPeerSignatureHandler(&mock.PeerSignatureHandler{}),
		node.WithInterceptorsContainer(&mock.InterceptorsContainerStub{}),
		node.WithForkDetector(&mock.ForkDetectorMock{}),
		node.WithBlockProcessor(&consensusMock.BlockProcessorMock{}),
		node.WithBootStorer(&mock.BoostrapStorerMock{
			GetHighestSlotCalled: func() int64 { return 0 },
			GetCalled: func(_ int64) (*bootstrapStorage.BootstrapData, error) {
				return nil, errors.New("no boot data")
			},
		}),
		node.WithEpochStartTrigger(&mock.EpochStartTriggerStub{}),
		node.WithRequestHandler(&mock.RequestHandlerStub{}),
		node.WithSlotManager(&consensusMock.SlotManagerMock{}),
		node.WithSyncer(&consensusMock.SyncTimerMock{}),
		node.WithGenesisTime(time.Now()),
		node.WithIndexer(indexer.NewNilIndexer()),
		node.WithWatchdogTimer(&watchdog.DisabledWatchdog{}),
		node.WithBlockBlackListHandler(timecache.NewTimeCache(time.Second)),
		node.WithNodesCoordinator(coordinator),
		node.WithForkController(&nonSubscriberForkController{ForkController: &mock.ForkControllerStub{}}),
	)
	require.Nil(t, err)

	return n
}

// heartbeatHandlerStub implements node.HeartbeatHandler for StartConsensus tests
type heartbeatHandlerStub struct {
	refreshedEpoch *uint32
}

func (h *heartbeatHandlerStub) Monitor() *heartbeatProcess.Monitor { return nil }

func (h *heartbeatHandlerStub) Sender() *heartbeatProcess.Sender { return nil }
func (h *heartbeatHandlerStub) RefreshPeerTypeCache(epoch uint32) {
	*h.refreshedEpoch = epoch
}
func (h *heartbeatHandlerStub) Close() error         { return nil }
func (h *heartbeatHandlerStub) IsInterfaceNil() bool { return h == nil }

func TestStartConsensus_RefreshesPeerTypeCacheWithCurrentEpoch(t *testing.T) {
	t.Parallel()

	n := createStartConsensusNode(t, &mock.NodesCoordinatorMock{})

	// sentinel start value: the refresh must overwrite it with the genesis
	// epoch (0), proving RefreshPeerTypeCache was actually called
	refreshedEpoch := uint32(999)
	n.SetHeartbeatHandler(&heartbeatHandlerStub{refreshedEpoch: &refreshedEpoch})

	// the wrapped fork controller does not implement EpochSubscriberHandler, so
	// StartConsensus stops right after the peer type cache refresh
	err := n.StartConsensus()
	require.Equal(t, common.ErrWrongTypeAssertion, err)
	require.Equal(t, uint32(0), refreshedEpoch)
}

func TestStartConsensus_RefreshesPeerTypeCacheWithCurrentBlockEpoch(t *testing.T) {
	t.Parallel()

	// the mid-epoch restart case of issue #88: the restored chain head carries
	// the real epoch, which is what the peer type cache must be rebuilt for
	crtHeader := &block.Block{Header: &block.BlockHeader{Epoch: 7}}
	n := createStartConsensusNode(t, &mock.NodesCoordinatorMock{}, crtHeader)

	refreshedEpoch := uint32(999)
	n.SetHeartbeatHandler(&heartbeatHandlerStub{refreshedEpoch: &refreshedEpoch})

	err := n.StartConsensus()
	require.Equal(t, common.ErrWrongTypeAssertion, err)
	require.Equal(t, uint32(7), refreshedEpoch)
}

func TestStartConsensus_SkipsPeerTypeCacheRefreshWithoutHeartbeat(t *testing.T) {
	t.Parallel()

	n := createStartConsensusNode(t, &mock.NodesCoordinatorMock{})

	// no heartbeat handler set: the refresh is skipped and StartConsensus
	// proceeds to the fork-controller assertion
	err := n.StartConsensus()
	require.Equal(t, common.ErrWrongTypeAssertion, err)
}
