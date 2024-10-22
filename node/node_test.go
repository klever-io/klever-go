package node_test

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/tools"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
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

var errSingleSignKeyGenMock = errors.New("errSingleSignKeyGenMock")

func getMarshalizer() marshal.Marshalizer {
	return &mock.MarshalizerFake{}
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
		node.WithChainID([]byte("chainID")),
		node.WithProposalController(proposalController),
		node.WithKAppController(kappController),
		node.WithForkController(forkController),
	)
	require.Nil(t, err)

	return n, nil
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

	byteSlice := make([]byte, tools.MegabyteSize+1)

	baseInfo := &transaction.TXBaseInfo{
		Sender:    createDummyHexAddress(64),
		Nonce:     uint64(0),
		DataField: [][]byte{byteSlice},
	}

	_, _, err = n.CreateTransaction(0, baseInfo, []json.RawMessage{transferContract}, false)
	require.Error(t, common.ErrDataFieldTooBig, err)
}
