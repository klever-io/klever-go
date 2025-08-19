package kda

import (
	"fmt"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	vmStub "github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeAddress(prefix string) []byte {
	addr := make([]byte, 32)
	copy(addr, []byte(prefix))
	return addr
}

func setupKDAKapp(t *testing.T, cfg config.EnableEpochs) *kdaKapp {
	forkController := mock.NewForkControllerStub()
	forkController.SetByConfig(cfg)

	kdaArgs := ArgsNewKDAKApp{
		Hasher:         &mock.HasherMock{},
		Marshalizer:    &mock.ProtoMarshalizerMock{},
		PubkeyConv:     cryptoMock.NewPubkeyConverterMock(32),
		ForkController: forkController,
	}

	kdaKapp, err := NewKDAKApp(&kdaArgs)
	require.NoError(t, err)

	return kdaKapp
}

func genUris(size int, text string) map[string]string {
	uris := make(map[string]string)
	for i := 0; i <= size; i++ {
		uris[fmt.Sprintf("%d", i)] = text
	}
	return uris
}

func setupAccCacher(accCacher *mock.AccountsCacherStub) *mock.AccountsCacherStub {
	if accCacher != nil {
		return accCacher
	}

	return &mock.AccountsCacherStub{}
}

func setupFork(cfg *config.EnableEpochs) config.EnableEpochs {
	if cfg != nil {
		return *cfg
	}

	return config.EnableEpochs{}
}

func Test_CreateKDA(t *testing.T) {
	validAccCacher := &mock.AccountsCacherStub{
		GetExistingKappCalled: func(address []byte) (state.KAppAccountHandler, error) {
			// mock KDA KApps
			acc, _ := state.NewKAppAccount(address)
			trieStub := &mock.TrieStub{
				GetCalled: func(key []byte) ([]byte, error) {
					return nil, nil
				},
			}

			acc.SetDataTrie(trieStub)

			return acc, nil
		},
	}

	validKappController := &vmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				ContractID: 0,
				Block: &block.Block{
					Header: &block.BlockHeader{
						RandSeed: []byte{0},
					},
				},
			})
		},
	}

	var tests = []struct {
		Description         string
		Sender              []byte
		TransactionContract *transaction.CreateAssetContract
		KDAKappController   *vmStub.KAppControllerStub
		AccCacher           *mock.AccountsCacherStub
		Fork                *config.EnableEpochs
		Status              transaction.Transaction_TXResultCode
		Error               error
	}{
		{
			Description: "Invalid asset name length",
			TransactionContract: &transaction.CreateAssetContract{
				Name: []byte(strings.Repeat("a", core.MaxLengthForAssetName+1)),
			},
			Error:  common.ErrAssetNameInvalid,
			Status: transaction.Transaction_ContractInvalid,
		},
		{
			Description: "Invalid ticker name length",
			TransactionContract: &transaction.CreateAssetContract{
				Name:   []byte("KDA"),
				Ticker: []byte(strings.Repeat("a", core.MinLengthForAssetTicker-1)), // less than min len
			},
			Error:  common.ErrAssetTickerLengthInvalid,
			Status: transaction.Transaction_ContractInvalid,
		}, {
			Description: "Invalid Royalties for Fungible Tokens",
			TransactionContract: &transaction.CreateAssetContract{
				Name:   []byte("KDA"),
				Ticker: []byte("KDA"),
				Type:   transaction.CreateAssetContract_Fungible,
				Royalties: &transaction.RoyaltiesInfo{
					MarketFixed:      1,
					MarketPercentage: 1,
				},
			},
			Error:  fmt.Errorf("%w only NonFungible tokens have market royalties", process.ErrInvalidArgument),
			Status: transaction.Transaction_ContractInvalid,
		},
		{
			Description: "Asset name must be human readable",
			TransactionContract: &transaction.CreateAssetContract{
				Name:      []byte("!@#!@#@!"),
				Ticker:    []byte("KDA"),
				Type:      transaction.CreateAssetContract_Fungible,
				Royalties: &transaction.RoyaltiesInfo{},
			},
			Error:  process.ErrTokenNameNotHumanReadable,
			Status: transaction.Transaction_ContractInvalid,
		},
		{
			Description: "Invalid owner address",
			TransactionContract: &transaction.CreateAssetContract{
				OwnerAddress: []byte{0},
				Name:         []byte("KDA"),
				Ticker:       []byte("KDA"),
				Type:         transaction.CreateAssetContract_Fungible,
				Royalties:    &transaction.RoyaltiesInfo{},
			},
			Error:  process.ErrInvalidOwnerAddr,
			Status: transaction.Transaction_AccountError,
		},
		{
			Description: "Invalid Admin address",
			TransactionContract: &transaction.CreateAssetContract{
				OwnerAddress: makeAddress("valid"),
				AdminAddress: []byte{0},
				Name:         []byte("KDA"),
				Ticker:       []byte("KDA"),
				Type:         transaction.CreateAssetContract_Fungible,
				Royalties:    &transaction.RoyaltiesInfo{},
			},
			Error:  process.ErrInvalidAdminAddr,
			Status: transaction.Transaction_AccountError,
		},
		{
			Description: "Invalid max supply",
			TransactionContract: &transaction.CreateAssetContract{
				OwnerAddress: makeAddress("valid"),
				Name:         []byte("KDA"),
				Ticker:       []byte("KDA"),
				MaxSupply:    -100,
				Type:         transaction.CreateAssetContract_Fungible,
				Royalties:    &transaction.RoyaltiesInfo{},
			},
			Error:  process.ErrSupplyNotValid,
			Status: transaction.Transaction_ParameterInvalid,
		},
		{
			Description: "Parse roles error",
			TransactionContract: &transaction.CreateAssetContract{
				OwnerAddress: makeAddress("valid"),
				Name:         []byte("KDA"),
				Ticker:       []byte("KDA"),
				MaxSupply:    100,
				Type:         transaction.CreateAssetContract_Fungible,
				Royalties:    &transaction.RoyaltiesInfo{},
				Roles: []*transaction.RolesInfo{
					{
						Address: []byte{0},
					},
				},
			},
			AccCacher: validAccCacher,
			Error:     process.ErrInvalidRoleAddr,
			Status:    transaction.Transaction_AccountError,
		},
		{
			Description: "Roles already exist",
			TransactionContract: &transaction.CreateAssetContract{
				OwnerAddress: makeAddress("valid"),
				Name:         []byte("KDA"),
				Ticker:       []byte("KDA"),
				MaxSupply:    100,
				Type:         transaction.CreateAssetContract_Fungible,
				Royalties:    &transaction.RoyaltiesInfo{},
				Roles: []*transaction.RolesInfo{
					{
						Address:        makeAddress("valid-role"),
						HasRoleDeposit: true,
					},
					{
						Address:        makeAddress("valid-role"),
						HasRoleDeposit: true,
					},
				},
			},
			AccCacher: validAccCacher,
			Error:     process.ErrSupplyNotValid,
			Status:    transaction.Transaction_ParameterInvalid,
		},
		{
			Description: "Asset type invalid",
			TransactionContract: &transaction.CreateAssetContract{
				OwnerAddress: makeAddress("valid"),
				Name:         []byte("KDA"),
				Ticker:       []byte("KDA"),
				MaxSupply:    100,
				Type:         -1,
				Royalties:    &transaction.RoyaltiesInfo{},
			},
			AccCacher: validAccCacher,
			Error:     common.ErrAssetTypeInvalid,
			Status:    transaction.Transaction_AssetTypeInvalid,
		},
		{
			Description: "Invalid logo length error",
			TransactionContract: &transaction.CreateAssetContract{
				OwnerAddress: makeAddress("valid"),
				Name:         []byte("KDA"),
				Ticker:       []byte("KDA"),
				MaxSupply:    100,
				Type:         transaction.CreateAssetContract_Fungible,
				Royalties:    &transaction.RoyaltiesInfo{},
				Logo:         strings.Repeat("a", 300),
			},
			AccCacher: validAccCacher,
			Error:     common.ErrInvalidValue,
			Status:    transaction.Transaction_ParameterInvalid,
		},
		{
			Description: "Greater than max uri map size error",
			TransactionContract: &transaction.CreateAssetContract{
				OwnerAddress: makeAddress("valid"),
				Name:         []byte("KDA"),
				Ticker:       []byte("KDA"),
				MaxSupply:    100,
				Type:         transaction.CreateAssetContract_Fungible,
				Royalties:    &transaction.RoyaltiesInfo{},
				Logo:         "",
				URIs:         genUris(core.MaxURIMapSize+1, "mock"),
			},
			AccCacher: validAccCacher,
			Error:     common.ErrInvalidValue,
			Status:    transaction.Transaction_ParameterInvalid,
		},
		{
			Description: "Greater than uri key size error",
			TransactionContract: &transaction.CreateAssetContract{
				OwnerAddress: makeAddress("valid"),
				Name:         []byte("KDA"),
				Ticker:       []byte("KDA"),
				MaxSupply:    100,
				Type:         transaction.CreateAssetContract_Fungible,
				Royalties:    &transaction.RoyaltiesInfo{},
				Logo:         "",
				URIs:         genUris(1, strings.Repeat("a", core.MaxURIValueSize+1)),
			},
			AccCacher: validAccCacher,
			Error:     common.ErrInvalidValue,
			Status:    transaction.Transaction_ParameterInvalid,
		},
		{
			Description: "invalid admin address",
			TransactionContract: &transaction.CreateAssetContract{
				OwnerAddress:  makeAddress("valid"),
				AdminAddress:  []byte("invalid"),
				Name:          []byte("KDA"),
				Ticker:        []byte("KDA"),
				InitialSupply: 0,
				Properties: &transaction.PropertiesInfo{
					CanMint: true,
				},
				MaxSupply: 100000000,
				Precision: 6,
				Type:      transaction.CreateAssetContract_Fungible,
				Royalties: &transaction.RoyaltiesInfo{},
				Logo:      "",
				URIs:      genUris(1, strings.Repeat("a", core.MaxURIValueSize)),
			},
			AccCacher: validAccCacher,
			Error:     process.ErrInvalidAdminAddr,
			Status:    transaction.Transaction_AccountError,
		},
		{
			Description: "Exceed transfer percentage limit",
			TransactionContract: &transaction.CreateAssetContract{
				OwnerAddress: makeAddress("valid"),
				Name:         []byte("KDA"),
				Ticker:       []byte("KDA"),
				MaxSupply:    100,
				Type:         transaction.CreateAssetContract_Fungible,
				Royalties: &transaction.RoyaltiesInfo{
					TransferPercentage: make([]*transaction.RoyaltyInfo, core.MaxTransferRoyalties+1),
				},
			},
			AccCacher: validAccCacher,
			Error:     common.ErrInvalidValue,
			Status:    transaction.Transaction_ParameterInvalid,
		},
		{
			Description: "Invalid address length",
			TransactionContract: &transaction.CreateAssetContract{
				OwnerAddress: makeAddress("valid"),
				Name:         []byte("KDA"),
				Ticker:       []byte("KDA"),
				MaxSupply:    100,
				Type:         transaction.CreateAssetContract_Fungible,
				Royalties: &transaction.RoyaltiesInfo{
					Address: make([]byte, 31),
				},
			},
			AccCacher: validAccCacher,
			Error:     process.ErrInvalidOwnerAddr,
			Status:    transaction.Transaction_AccountError,
		},
		{
			Description: "Success",
			TransactionContract: &transaction.CreateAssetContract{
				OwnerAddress: makeAddress("valid"),
				Name:         []byte("KDA"),
				Ticker:       []byte("KDA"),
				MaxSupply:    0,
				Type:         transaction.CreateAssetContract_Fungible,
				Properties:   &transaction.PropertiesInfo{CanMint: true},
				Royalties: &transaction.RoyaltiesInfo{
					Address: makeAddress("valid"),
				},
				Logo: "",
				URIs: genUris(1, "mock"),
			},
			AccCacher: validAccCacher,
			Status:    transaction.Transaction_Ok,
		},
	}

	for _, tt := range tests {
		kdaKapp := setupKDAKapp(t, setupFork(tt.Fork))
		// These setters do not return an error at the moment, but we include require.NoError
		// to satisfy linting requirements and to make the tests resilient to future changes.
		require.NoError(t, kdaKapp.SetKAppController(validKappController))
		require.NoError(t, kdaKapp.SetAccountsCacher(setupAccCacher(tt.AccCacher)))

		t.Run(tt.Description, func(t *testing.T) {
			assert := assert.New(t)

			status, err := kdaKapp.Create(tt.Sender, tt.TransactionContract)
			assert.Equal(tt.Status, status)
			assert.Equal(tt.Error, err)
		})
	}
}
