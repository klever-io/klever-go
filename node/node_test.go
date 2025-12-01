package node_test

import (
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
