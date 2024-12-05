package ito

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	vmStub "github.com/klever-io/klever-go/kvm/mock/stub"

	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var mockAsset = "ITO-1234"
var mockSender = "mock-address"
var mockError = fmt.Errorf("mock-error")

func makeAddress(prefix string) []byte {
	addr := make([]byte, 32)
	copy(addr, []byte(prefix))
	return addr
}

func setupITOKapp(t *testing.T, cfg config.EnableEpochs) *itoKapp {
	epochNotifier := &mock.EpochNotifierStub{}
	forkController, err := fork.NewForkController(
		cfg,
		epochNotifier,
	)
	require.NoError(t, err)

	itoArgs := ArgsNewITOKApp{
		Marshalizer:    &mock.ProtoMarshalizerMock{},
		PubkeyConv:     cryptoMock.NewPubkeyConverterMock(32),
		ForkController: forkController,
	}

	itoKapp, err := NewITOKApp(&itoArgs)
	require.NoError(t, err)

	return itoKapp
}

func Test_NewItoKapp_NilMarshalizer(t *testing.T) {
	epochNotifier := &mock.EpochNotifierStub{}
	forkController, err := fork.NewForkController(
		config.EnableEpochs{},
		epochNotifier,
	)
	require.NoError(t, err)

	itoArgs := ArgsNewITOKApp{
		Marshalizer:    nil,
		PubkeyConv:     cryptoMock.NewPubkeyConverterMock(32),
		ForkController: forkController,
	}

	_, err = NewITOKApp(&itoArgs)
	require.Error(t, err)
	assert.Equal(t, common.ErrNilMarshalizer, err)
}

func Test_NewItoKapp_NilPubkeyConverter(t *testing.T) {
	epochNotifier := &mock.EpochNotifierStub{}
	forkController, err := fork.NewForkController(
		config.EnableEpochs{},
		epochNotifier,
	)
	require.NoError(t, err)

	itoArgs := ArgsNewITOKApp{
		Marshalizer:    &mock.ProtoMarshalizerMock{},
		PubkeyConv:     nil,
		ForkController: forkController,
	}

	_, err = NewITOKApp(&itoArgs)
	require.Error(t, err)
	assert.Equal(t, common.ErrNilPubkeyConverter, err)
}

func Test_SetAccountsCacher_NilAccountsAdapter(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	err := itoKapp.SetAccountsCacher(nil)
	require.Error(t, err)
	assert.Equal(t, common.ErrNilAccountsAdapter, err)
}

func Test_IsInterfaceNil(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})
	isInterfaceNil := itoKapp.IsInterfaceNil()
	require.False(t, isInterfaceNil)
}

func Test_GetExistingUserAccount_ShouldError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	accCacher := &mock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return nil, mockError
		},
	}
	_ = itoKapp.SetAccountsCacher(accCacher)

	_, err := itoKapp.GetExistingUserAccount([]byte("mock"))
	require.Error(t, err)
}

func Test_GetITO_LoadKAppError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	accCacher := &mock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return nil, mockError
		},
	}
	_ = itoKapp.SetAccountsCacher(accCacher)

	_, _, err := itoKapp.GetITO([]byte("mock"))
	require.Error(t, err)
}

func Test_GetITO_EmptyAssetIDError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	accCacher := &mock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{}, nil
		},
	}
	_ = itoKapp.SetAccountsCacher(accCacher)

	_, ito, err := itoKapp.GetITO(nil)
	require.NoError(t, err)
	assert.Nil(t, ito)
}

func Test_GetITO_ItoNotExistsError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	accCacher := &mock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return nil, mockError
						},
					}
				},
			}, nil
		},
	}
	_ = itoKapp.SetAccountsCacher(accCacher)

	_, ito, err := itoKapp.GetITO([]byte("ito-asset"))
	require.Error(t, err)
	assert.Nil(t, ito)
}

func Test_GetITO_ItoNotFoundError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	accCacher := &mock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return []byte{}, nil
						},
					}
				},
			}, nil
		},
	}
	_ = itoKapp.SetAccountsCacher(accCacher)

	_, _, err := itoKapp.GetITO([]byte("ito-asset"))
	require.Error(t, err)
	assert.Equal(t, common.ErrNotFoundInKApp, err)
}

func Test_GetITO_UnmarshalError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	accCacher := &mock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return []byte{0, 0}, nil
						},
					}
				},
			}, nil
		},
	}
	_ = itoKapp.SetAccountsCacher(accCacher)

	_, _, err := itoKapp.GetITO([]byte("ito-asset"))
	require.Error(t, err)
}

func Test_SetITO_MarshalizerError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	itoKapp.marshalizer = &mock.MarshalizerStub{
		MarshalCalled: func(obj interface{}) ([]byte, error) {
			return nil, mockError
		},
	}

	err := itoKapp.SetITO(nil, nil, nil)
	require.Error(t, err)
	assert.Equal(t, mockError, err)
}

func Test_SetITO_SaveKeyValueError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	itoAccountHandler := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key, value []byte) error {
					return mockError
				},
			}
		},
	}

	err := itoKapp.SetITO(itoAccountHandler, nil, nil)
	require.Error(t, err)
	assert.Equal(t, mockError, err)
}

func Test_LoadUserAccount_ShouldError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	accCacher := &mock.AccountsCacherStub{
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return nil, mockError
		},
	}
	_ = itoKapp.SetAccountsCacher(accCacher)

	_, err := itoKapp.LoadUserAccount([]byte("mock"))
	require.Error(t, err)
}

func Test_Trigger_SetITOPrices_InvalidPackShoulErr(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	triggerContract := &transaction.ITOTriggerContract{}

	asset := &kapps.KDAData{
		OwnerAddress: []byte(mockSender),
		AdminAddress: []byte(mockSender),
	}

	ito := &kapps.ITOData{}

	status, err := itoKapp.SetITOPrices(triggerContract, ito, asset, []byte(mockSender))
	require.Error(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}

func Test_Trigger_SetITOPrices_ExceedPackAmountShoulErr(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	packInfo := make(map[string]*transaction.PackInfo)

	for i := 0; i <= core.MaxPacks; i++ {
		packInfo[fmt.Sprintf("%d", i)] = &transaction.PackInfo{}
	}

	triggerContract := &transaction.ITOTriggerContract{
		PackInfo: packInfo,
	}

	asset := &kapps.KDAData{
		OwnerAddress: []byte(mockSender),
		AdminAddress: []byte(mockSender),
	}

	ito := &kapps.ITOData{}

	status, err := itoKapp.SetITOPrices(triggerContract, ito, asset, []byte(mockSender))
	require.Error(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}

func Test_Trigger_SetITOPrices_WrongRoleShouldErr(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	packInfo := make(map[string]*transaction.PackInfo)
	packInfo[mockAsset] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{
			{
				Amount: 10,
				Price:  10,
			},
		},
	}

	triggerContract := &transaction.ITOTriggerContract{
		PackInfo: packInfo,
	}

	asset := &kapps.KDAData{
		OwnerAddress: []byte(mockSender),
		AdminAddress: []byte(mockSender),
	}

	ito := &kapps.ITOData{}

	status, err := itoKapp.SetITOPrices(triggerContract, ito, asset, []byte("wrong-sender"))
	require.Error(t, common.ErrRoleNotFound, err)
	assert.Equal(t, transaction.Transaction_AssetError, status)
}

func Test_Trigger_SetITOPrices_InvalidPackItemsAmountShoulErr(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	packInfo := make(map[string]*transaction.PackInfo)
	packs := make([]*transaction.PackItem, 0)

	packInfo[mockAsset] = &transaction.PackInfo{
		Packs: packs,
	}

	triggerContract := &transaction.ITOTriggerContract{
		PackInfo: packInfo,
	}

	asset := &kapps.KDAData{
		OwnerAddress: []byte(mockSender),
		AdminAddress: []byte(mockSender),
	}

	ito := &kapps.ITOData{}

	status, err := itoKapp.SetITOPrices(triggerContract, ito, asset, []byte(mockSender))
	require.Error(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}

func Test_Trigger_SetITOPrices_ExceedPackItemsAmountShoulErr(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	packInfo := make(map[string]*transaction.PackInfo)
	packs := make([]*transaction.PackItem, 0)
	for i := 0; i <= core.MaxPackItems; i++ {
		packs = append(packs, &transaction.PackItem{
			Amount: 1,
			Price:  1,
		})
	}

	packInfo[mockAsset] = &transaction.PackInfo{
		Packs: packs,
	}

	triggerContract := &transaction.ITOTriggerContract{
		PackInfo: packInfo,
	}

	asset := &kapps.KDAData{
		OwnerAddress: []byte(mockSender),
		AdminAddress: []byte(mockSender),
	}

	ito := &kapps.ITOData{}

	status, err := itoKapp.SetITOPrices(triggerContract, ito, asset, []byte(mockSender))
	require.Error(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}

func Test_Trigger_SetITOPrices_DontHaveSetItoPricesRoleShouldErr(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	packInfo := make(map[string]*transaction.PackInfo)
	packInfo[mockAsset] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{
			{
				Amount: 10,
				Price:  10,
			},
		},
	}

	triggerContract := &transaction.ITOTriggerContract{
		PackInfo: packInfo,
	}

	asset := &kapps.KDAData{
		OwnerAddress: []byte(mockSender),
		AdminAddress: []byte(mockSender),
		Roles: []*kapps.RolesData{
			{
				Address:             []byte("dont-have-role"),
				HasRoleSetITOPrices: false,
			},
		},
	}

	ito := &kapps.ITOData{}

	status, err := itoKapp.SetITOPrices(triggerContract, ito, asset, []byte("dont-have-role"))
	require.Error(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_AccountError, status)
}

func Test_Trigger_SetITOPrices_InvalidPriceShouldErr(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	packInfo := make(map[string]*transaction.PackInfo)
	packInfo[mockAsset] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{
			{
				Amount: 10,
				Price:  -1,
			},
		},
	}

	triggerContract := &transaction.ITOTriggerContract{
		PackInfo: packInfo,
	}

	asset := &kapps.KDAData{
		OwnerAddress: []byte(mockSender),
		AdminAddress: []byte(mockSender),
	}

	ito := &kapps.ITOData{}

	_ = itoKapp.SetKAppController(&vmStub.KAppControllerStub{
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &vmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, nil, nil
				},
			}
		},
	})

	status, err := itoKapp.SetITOPrices(triggerContract, ito, asset, []byte(mockSender))
	require.Error(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}

func Test_Trigger_SetITOPrices_ShouldWork(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{})

	packInfo := make(map[string]*transaction.PackInfo)
	packInfo[mockAsset] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{
			{
				Amount: 10,
				Price:  10,
			},
		},
	}

	triggerContract := &transaction.ITOTriggerContract{
		PackInfo: packInfo,
	}

	asset := &kapps.KDAData{
		OwnerAddress: []byte(mockSender),
		AdminAddress: []byte(mockSender),
	}

	ito := &kapps.ITOData{}

	_ = itoKapp.SetKAppController(&vmStub.KAppControllerStub{
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &vmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, nil, nil
				},
			}
		},
	})

	status, err := itoKapp.SetITOPrices(triggerContract, ito, asset, []byte(mockSender))
	require.NoError(t, err)
	assert.Equal(t, transaction.Transaction_Ok, status)
}

func Test_Trigger_RemoveFromWhitelist_BeforeSmartContractFork_ShouldWork(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 100_000,
	})

	whitelistUserA := hex.EncodeToString(makeAddress("A"))
	whitelistUserB := hex.EncodeToString(makeAddress("B"))

	whitelistData := make(map[string]*kapps.WhitelistData)
	whitelistData[whitelistUserA] = &kapps.WhitelistData{
		Limit: 1000,
	}
	whitelistData[whitelistUserB] = &kapps.WhitelistData{
		Limit: 10,
	}

	packInfo := make(map[string]*transaction.PackInfo)
	packInfo[mockAsset] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{
			{
				Amount: 10,
				Price:  10,
			},
		},
	}

	whitelistToRemove := make(map[string]*transaction.WhitelistInfo)
	whitelistToRemove[whitelistUserA] = &transaction.WhitelistInfo{
		Limit: 1000,
	}

	triggerContract := &transaction.ITOTriggerContract{
		PackInfo:      packInfo,
		WhitelistInfo: whitelistToRemove,
	}

	asset := &kapps.KDAData{
		OwnerAddress: []byte(mockSender),
		AdminAddress: []byte(mockSender),
	}

	ito := &kapps.ITOData{
		WhitelistLen: 2,
	}

	whitelistReturnData, err := itoKapp.marshalizer.Marshal(&kapps.WhitelistData{
		Limit: 10,
	})
	require.NoError(t, err)

	accCacher := &mock.AccountsCacherStub{
		GetExistingKappCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return whitelistReturnData, nil
						},
					}
				},
			}, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	whitelist := make(map[string]*kapps.WhitelistData)
	status, err := itoKapp.RemoveFromWhitelist(triggerContract, ito, asset, []byte(mockSender), whitelist)
	require.NoError(t, err)
	assert.Equal(t, transaction.Transaction_Ok, status)
}

func Test_Trigger_RemoveFromWhitelist_BeforeSmartContractFork_CantRemoveWhitelistShoudErr(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 100_000,
	})

	whitelistUserA := hex.EncodeToString(makeAddress("A"))
	whitelistUserB := hex.EncodeToString(makeAddress("B"))

	whitelistData := make(map[string]*kapps.WhitelistData)
	whitelistData[whitelistUserA] = &kapps.WhitelistData{
		Limit: 1000,
	}

	packInfo := make(map[string]*transaction.PackInfo)
	packInfo[mockAsset] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{
			{
				Amount: 10,
				Price:  10,
			},
		},
	}

	whitelistToRemove := make(map[string]*transaction.WhitelistInfo)
	whitelistToRemove[whitelistUserB] = &transaction.WhitelistInfo{
		Limit: 10,
	}

	triggerContract := &transaction.ITOTriggerContract{
		PackInfo:      packInfo,
		WhitelistInfo: whitelistToRemove,
	}

	asset := &kapps.KDAData{
		OwnerAddress: []byte(mockSender),
		AdminAddress: []byte(mockSender),
	}

	ito := &kapps.ITOData{
		WhitelistLen: 2,
	}

	accCacher := &mock.AccountsCacherStub{
		GetExistingKappCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return nil, common.ErrNilTrie // forcing whitelist not found
						},
					}
				},
			}, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	whitelist := make(map[string]*kapps.WhitelistData)
	status, err := itoKapp.RemoveFromWhitelist(triggerContract, ito, asset, []byte(mockSender), whitelist)
	require.NoError(t, err)
	assert.Equal(t, transaction.Transaction_Ok, status)
}

func Test_Trigger_RemoveFromWhitelist_AfterSmartContractFork_ShouldWork(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	whitelistUserA := hex.EncodeToString(makeAddress("A"))
	whitelistUserB := hex.EncodeToString(makeAddress("B"))

	whitelistData := make(map[string]*kapps.WhitelistData)
	whitelistData[whitelistUserA] = &kapps.WhitelistData{
		Limit: 1000,
	}
	whitelistData[whitelistUserB] = &kapps.WhitelistData{
		Limit: 10,
	}

	packInfo := make(map[string]*transaction.PackInfo)
	packInfo[mockAsset] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{
			{
				Amount: 10,
				Price:  10,
			},
		},
	}

	whitelistToRemove := make(map[string]*transaction.WhitelistInfo)
	whitelistToRemove[whitelistUserA] = &transaction.WhitelistInfo{
		Limit: 1000,
	}

	triggerContract := &transaction.ITOTriggerContract{
		PackInfo:      packInfo,
		WhitelistInfo: whitelistToRemove,
	}

	asset := &kapps.KDAData{
		OwnerAddress: []byte(mockSender),
		AdminAddress: []byte(mockSender),
	}

	ito := &kapps.ITOData{
		WhitelistLen: 2,
	}

	whitelistReturnData, err := itoKapp.marshalizer.Marshal(&kapps.WhitelistData{
		Limit: 10,
	})
	require.NoError(t, err)

	accCacher := &mock.AccountsCacherStub{
		GetExistingKappCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return whitelistReturnData, nil
						},
					}
				},
			}, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	whitelist := make(map[string]*kapps.WhitelistData)
	status, err := itoKapp.RemoveFromWhitelist(triggerContract, ito, asset, []byte(mockSender), whitelist)
	require.NoError(t, err)
	assert.Equal(t, transaction.Transaction_Ok, status)
}

func Test_Trigger_RemoveFromWhitelist_AfterSmartContractFork_CantRemoveWhitelistShoudErr(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	whitelistUserA := hex.EncodeToString(makeAddress("A"))
	whitelistUserB := hex.EncodeToString(makeAddress("B"))

	whitelistData := make(map[string]*kapps.WhitelistData)
	whitelistData[whitelistUserA] = &kapps.WhitelistData{
		Limit: 1000,
	}

	packInfo := make(map[string]*transaction.PackInfo)
	packInfo[mockAsset] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{
			{
				Amount: 10,
				Price:  10,
			},
		},
	}

	whitelistToRemove := make(map[string]*transaction.WhitelistInfo)
	whitelistToRemove[whitelistUserB] = &transaction.WhitelistInfo{
		Limit: 10,
	}

	triggerContract := &transaction.ITOTriggerContract{
		PackInfo:      packInfo,
		WhitelistInfo: whitelistToRemove,
	}

	asset := &kapps.KDAData{
		OwnerAddress: []byte(mockSender),
		AdminAddress: []byte(mockSender),
	}

	ito := &kapps.ITOData{
		WhitelistLen: 2,
	}

	accCacher := &mock.AccountsCacherStub{
		GetExistingKappCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return nil, common.ErrNilTrie // forcing whitelist not found
						},
					}
				},
			}, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	whitelist := make(map[string]*kapps.WhitelistData)
	status, err := itoKapp.RemoveFromWhitelist(triggerContract, ito, asset, []byte(mockSender), whitelist)
	require.Error(t, process.ErrInvalidWhitelistAddr, err)
	assert.Equal(t, transaction.Transaction_AccountError, status)
}

func Test_BuyIto_BeforeSmartContractFork_ShouldWork(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 100_00,
	})

	// Acc Cacher Mock
	packData := make(map[string]*kapps.PackData, 0)
	packs := make([]*kapps.Pack, 0)

	packs = append(packs, &kapps.Pack{
		Amount: 100,
		Price:  100,
	})

	packData["klv"] = &kapps.PackData{
		Packs: packs,
	}

	ito := &kapps.ITOData{
		IsActive:  true,
		StartTime: 1,
		EndTime:   100_00,
		PackData:  packData,
	}

	itoReturnData, err := itoKapp.marshalizer.Marshal(ito)
	require.NoError(t, err)

	accCacher := &mock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return &mock.AccountWrapMock{}, nil
		},
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return &mock.UserAccountHandlerStub{
				AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
			}, nil
		},
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return itoReturnData, nil
						},
						SaveKeyValueCalled: func(key, value []byte) error {
							return nil
						},
					}
				},
			}, nil
		},
		UpdateUserCalled: func(account state.AccountHandler) error { return nil },
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	// Kapp Controller Mock

	blockTimestamp := int64(100)

	kappControllerStub := &vmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				Block: &block.Block{
					Header: &block.BlockHeader{ // we need a block timestamp to validate the start and end of ito
						Timestamp: blockTimestamp,
					},
				},
			})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &vmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						Precision: 2,
						AssetType: kapps.KDAData_Fungible,
					}, nil
				},
				MintCalled: func(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error) {
					return transaction.Transaction_Ok, nil
				},
			}
		},
	}

	itoKapp.SetKAppController(kappControllerStub)

	buyItoContract := &transaction.BuyContract{
		Amount:     100,
		ID:         []byte("ito-kda"),
		CurrencyID: []byte("klv"),
	}

	status, err := itoKapp.Buy([]byte(mockSender), buyItoContract)
	require.NoError(t, err)
	assert.Equal(t, transaction.Transaction_Ok, status)
}

func Test_BuyIto_AfterSmartContractFork_ShouldWork(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	// Acc Cacher Mock
	packData := make(map[string]*kapps.PackData, 0)
	packs := make([]*kapps.Pack, 0)

	packs = append(packs, &kapps.Pack{
		Amount: 1000000,
		Price:  1000000,
	}) // 1 ITOKDA/KDA

	packData["klv"] = &kapps.PackData{
		Packs: packs,
	}

	ito := &kapps.ITOData{
		IsActive:  true,
		StartTime: 1,
		EndTime:   100_00,
		PackData:  packData,
	}

	itoReturnData, err := itoKapp.marshalizer.Marshal(ito)
	require.NoError(t, err)

	accCacher := &mock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return &mock.AccountWrapMock{}, nil
		},
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return &mock.UserAccountHandlerStub{
				AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
			}, nil
		},
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return itoReturnData, nil
						},
						SaveKeyValueCalled: func(key, value []byte) error {
							return nil
						},
					}
				},
			}, nil
		},
		UpdateUserCalled: func(account state.AccountHandler) error { return nil },
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	// Kapp Controller Mock
	blockTimestamp := int64(100)

	kappControllerStub := &vmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				Block: &block.Block{
					Header: &block.BlockHeader{ // we need a block timestamp to validate the start and end of ito
						Timestamp: blockTimestamp,
					},
				},
			})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &vmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						Precision: 6,
						AssetType: kapps.KDAData_Fungible,
					}, nil
				},
				MintCalled: func(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error) {
					return transaction.Transaction_Ok, nil
				},
			}
		},
	}

	itoKapp.SetKAppController(kappControllerStub)

	buyItoContract := &transaction.BuyContract{
		Amount:         1000000,
		CurrencyAmount: 1000000, // to buy 1 ITOKDA we need to send 1 KDA
		ID:             []byte("ito-kda"),
		CurrencyID:     []byte("klv"),
	}

	status, err := itoKapp.Buy([]byte(mockSender), buyItoContract)
	require.NoError(t, err)
	assert.Equal(t, transaction.Transaction_Ok, status)
}

func Test_BuyIto_AfterSmartContractFork_InvalidAmountError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	kappControllerStub := &vmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{})
		},
	}
	itoKapp.SetKAppController(kappControllerStub)

	buyItoContract := &transaction.BuyContract{
		Amount: -100,
	}

	status, err := itoKapp.Buy([]byte(mockSender), buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_AssetError, status)
}

func Test_BuyIto_AfterSmartContractFork_InvalidAssetError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	kappControllerStub := &vmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &vmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, nil, common.ErrAssetNotFound
				},
			}
		},
	}

	itoKapp.SetKAppController(kappControllerStub)

	buyItoContract := &transaction.BuyContract{
		Amount: 100,
	}

	status, err := itoKapp.Buy([]byte(mockSender), buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrAssetNotFound, err)
	assert.Equal(t, transaction.Transaction_KAPPError, status)
}

func Test_BuyIto_AfterSmartContractFork_InvalidAssetTypeError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	kappControllerStub := &vmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &vmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						AssetType: kapps.KDAData_SemiFungible, // Invalid KDA Type
					}, nil
				},
			}
		},
	}

	itoKapp.SetKAppController(kappControllerStub)

	buyItoContract := &transaction.BuyContract{
		Amount: 100,
	}

	status, err := itoKapp.Buy([]byte(mockSender), buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_AssetTypeInvalid, status)
}

func Test_BuyIto_AfterSmartContractFork_InvalidITOError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	accCacher := &mock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return []byte{}, nil
						},
					}
				},
			}, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	kappControllerStub := &vmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &vmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						AssetType: kapps.KDAData_Fungible,
					}, nil
				},
			}
		},
	}

	itoKapp.SetKAppController(kappControllerStub)

	buyItoContract := &transaction.BuyContract{
		Amount: 100,
		ID:     []byte("mock-ito"),
	}

	status, err := itoKapp.Buy([]byte(mockSender), buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrNotFoundInKApp, err)
	assert.Equal(t, transaction.Transaction_ITOKAPPError, status)
}

func Test_BuyIto_AfterSmartContractFork_ITONotActiveError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ito := &kapps.ITOData{
		IsActive:               false,
		DefaultLimitPerAddress: 10,
	}

	itoReturnData, err := itoKapp.marshalizer.Marshal(ito)
	require.NoError(t, err)

	accCacher := &mock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return itoReturnData, nil
						},
					}
				},
			}, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	kappControllerStub := &vmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &vmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						AssetType: kapps.KDAData_Fungible,
					}, nil
				},
			}
		},
	}

	itoKapp.SetKAppController(kappControllerStub)

	buyItoContract := &transaction.BuyContract{
		Amount: 100,
		ID:     []byte("mock-ito"),
	}

	status, err := itoKapp.Buy([]byte(mockSender), buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrITONotActive, err)
	assert.Equal(t, transaction.Transaction_ITONotActive, status)
}

func Test_BuyIto_AfterSmartContractFork_ITONotStartedError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ito := &kapps.ITOData{
		IsActive:  true,
		StartTime: 1_000,
		EndTime:   10_000,
	}

	itoReturnData, err := itoKapp.marshalizer.Marshal(ito)
	require.NoError(t, err)

	accCacher := &mock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return itoReturnData, nil
						},
					}
				},
			}, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	kappControllerStub := &vmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				Block: &block.Block{
					Header: &block.BlockHeader{ // we need a block timestamp to validate the start and end of ito
						Timestamp: 100,
					},
				},
			})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &vmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						AssetType: kapps.KDAData_Fungible,
					}, nil
				},
			}
		},
	}

	itoKapp.SetKAppController(kappControllerStub)

	buyItoContract := &transaction.BuyContract{
		Amount: 100,
		ID:     []byte("mock-ito"),
	}

	status, err := itoKapp.Buy([]byte(mockSender), buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}

// more ito tests

func Test_BuyIto_AfterSmartContractFork_WhitelistDontExistError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ito := &kapps.ITOData{
		IsActive:           true,
		StartTime:          0,
		EndTime:            10_000,
		IsWhitelistActive:  true,
		WhitelistStartTime: 0,
		WhitelistEndTime:   10_000,
	}

	itoReturnData, err := itoKapp.marshalizer.Marshal(ito)
	require.NoError(t, err)

	accCacher := &mock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return itoReturnData, nil
						},
					}
				},
			}, nil
		},
		GetExistingKappCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return []byte{}, nil
						},
					}
				},
			}, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	kappControllerStub := &vmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				Block: &block.Block{
					Header: &block.BlockHeader{ // we need a block timestamp to validate the start and end of ito
						Timestamp: 100,
					},
				},
			})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &vmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						AssetType: kapps.KDAData_Fungible,
					}, nil
				},
			}
		},
	}

	itoKapp.SetKAppController(kappControllerStub)

	buyItoContract := &transaction.BuyContract{
		Amount: 100,
		ID:     []byte("mock-ito"),
	}

	status, err := itoKapp.Buy([]byte(mockSender), buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}

func Test_BuyIto_AfterSmartContractFork_WhitelistLimitNotEnoughError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ito := &kapps.ITOData{
		IsActive:           true,
		StartTime:          0,
		EndTime:            10_000,
		IsWhitelistActive:  true,
		WhitelistStartTime: 0,
		WhitelistEndTime:   10_000,
	}

	itoReturnData, err := itoKapp.marshalizer.Marshal(ito)
	require.NoError(t, err)

	itoWhitelist := &kapps.WhitelistData{
		Limit: 10,
	}

	itoWhitelistReturnData, err := itoKapp.marshalizer.Marshal(itoWhitelist)
	require.NoError(t, err)

	accCacher := &mock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return itoReturnData, nil
						},
					}
				},
			}, nil
		},
		GetExistingKappCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return itoWhitelistReturnData, nil
						},
					}
				},
			}, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	kappControllerStub := &vmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				Block: &block.Block{
					Header: &block.BlockHeader{ // we need a block timestamp to validate the start and end of ito
						Timestamp: 100,
					},
				},
			})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &vmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						AssetType: kapps.KDAData_Fungible,
					}, nil
				},
			}
		},
	}

	itoKapp.SetKAppController(kappControllerStub)

	buyItoContract := &transaction.BuyContract{
		Amount: 100,
		ID:     []byte("mock-ito"),
	}

	status, err := itoKapp.Buy([]byte(mockSender), buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}

func Test_BuyIto_AfterSmartContractFork_WhitelistSetError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ito := &kapps.ITOData{
		IsActive:           true,
		StartTime:          0,
		EndTime:            10_000,
		IsWhitelistActive:  true,
		WhitelistStartTime: 0,
		WhitelistEndTime:   10_000,
	}

	itoReturnData, err := itoKapp.marshalizer.Marshal(ito)
	require.NoError(t, err)

	itoWhitelist := &kapps.WhitelistData{
		Limit: 1000,
	}

	itoWhitelistReturnData, err := itoKapp.marshalizer.Marshal(itoWhitelist)
	require.NoError(t, err)

	accCacher := &mock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return itoReturnData, nil
						},
						SaveKeyValueCalled: func(key, value []byte) error {
							return common.ErrLeafSizeTooBig
						},
					}
				},
			}, nil
		},
		GetExistingKappCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return itoWhitelistReturnData, nil
						},
					}
				},
			}, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	kappControllerStub := &vmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				Block: &block.Block{
					Header: &block.BlockHeader{ // we need a block timestamp to validate the start and end of ito
						Timestamp: 100,
					},
				},
			})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &vmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						AssetType: kapps.KDAData_Fungible,
					}, nil
				},
			}
		},
	}

	itoKapp.SetKAppController(kappControllerStub)

	buyItoContract := &transaction.BuyContract{
		Amount: 100,
		ID:     []byte("mock-ito"),
	}

	status, err := itoKapp.Buy([]byte(mockSender), buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrLeafSizeTooBig, err)
	assert.Equal(t, transaction.Transaction_ITOWhiteListError, status)
}

func Test_BuyIto_ComputeBuyCost_InvalidPackError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ito := &kapps.ITOData{}
	asset := &kapps.KDAData{}
	buyItoContract := &transaction.BuyContract{
		CurrencyID: []byte{}, // converts do KLV
	}

	_, _, status, err := itoKapp.ComputeBuyCost(ito, asset, buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}

func Test_BuyIto_ComputeBuyCost_InvalidPackAmountIfNFTError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	packData := make(map[string]*kapps.PackData, 0)
	packs := make([]*kapps.Pack, 0)

	packs = append(packs, &kapps.Pack{
		Amount: 1,
		Price:  1,
	})

	packData["klv"] = &kapps.PackData{
		Packs: packs,
	}

	ito := &kapps.ITOData{
		PackData: packData,
	}
	asset := &kapps.KDAData{
		AssetType: kapps.KDAData_NonFungible,
	}
	buyItoContract := &transaction.BuyContract{
		CurrencyID: []byte("klv"),
		Amount:     10,
	}

	_, _, status, err := itoKapp.ComputeBuyCost(ito, asset, buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}

func Test_BuyIto_ComputeBuyCost_OverMaxAmountError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	packData := make(map[string]*kapps.PackData, 0)
	packs := make([]*kapps.Pack, 0)
	packs = append(packs, &kapps.Pack{
		Amount: 100,
		Price:  1,
	})
	packData["klv"] = &kapps.PackData{
		Packs: packs,
	}

	ito := &kapps.ITOData{
		PackData:     packData,
		MaxAmount:    10,
		MintedAmount: 0,
	}
	asset := &kapps.KDAData{
		AssetType: kapps.KDAData_NonFungible,
	}
	buyItoContract := &transaction.BuyContract{
		CurrencyID: []byte("klv"),
		Amount:     100,
	}

	_, _, status, err := itoKapp.ComputeBuyCost(ito, asset, buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}

func Test_BuyIto_ComputeBuyCost_WrongCurrencyCostError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	packData := make(map[string]*kapps.PackData, 0)
	packs := make([]*kapps.Pack, 0)
	packs = append(packs, &kapps.Pack{
		Amount: 1_000_000,
		Price:  1_000_000,
	})
	packData["klv"] = &kapps.PackData{
		Packs: packs,
	}

	ito := &kapps.ITOData{
		PackData: packData,
	}
	asset := &kapps.KDAData{
		AssetType: kapps.KDAData_Fungible,
		Precision: 6,
	}
	buyItoContract := &transaction.BuyContract{
		ID:             []byte("mock-ito"),
		CurrencyID:     []byte("klv"),
		Amount:         1,
		CurrencyAmount: 200_000_000, // Wrong Cost
	}

	_, _, status, err := itoKapp.ComputeBuyCost(ito, asset, buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}

func Test_BuyIto_ComputeBuyCost_NegativeValueError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 100_000,
	})

	packData := make(map[string]*kapps.PackData, 0)
	packs := make([]*kapps.Pack, 0)
	packs = append(packs, &kapps.Pack{
		Amount: 1_000_000,
		Price:  1_000_000,
	})
	packData["klv"] = &kapps.PackData{
		Packs: packs,
	}

	ito := &kapps.ITOData{
		PackData: packData,
	}
	asset := &kapps.KDAData{
		AssetType: kapps.KDAData_Fungible,
		Precision: 6,
	}
	buyItoContract := &transaction.BuyContract{
		ID:         []byte("mock-ito"),
		CurrencyID: []byte("klv"),
		Amount:     -1,
	}

	_, _, status, err := itoKapp.ComputeBuyCost(ito, asset, buyItoContract)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, status)
}

func Test_BuyIto_ComputeITOFixedBuyRoyalties_HaveNoRoyalties(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)
	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			ITOFixed: 0,
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{}

	status, err := itoKapp.ComputeITOFixedBuyRoyalties(ctx, asset, ownerAcc, value)
	require.NoError(t, err)
	assert.Equal(t, transaction.Transaction_Ok, status)
}

func Test_BuyIto_ComputeITOFixedBuyRoyalties_OutOfFundsError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)
	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			ITOFixed: 10, // 10% ?
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 0
		},
	}

	status, err := itoKapp.ComputeITOFixedBuyRoyalties(ctx, asset, ownerAcc, value)
	require.Error(t, err)
	assert.Equal(t, process.ErrInsufficientFunds, err)
	assert.Equal(t, transaction.Transaction_OutOfFunds, status)
}

func Test_BuyIto_ComputeITOFixedBuyRoyalties_OwnerBalanceError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)
	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			ITOFixed: 1_000_000, // 1 KLV
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return process.ErrInsufficientFunds
		},
	}

	status, err := itoKapp.ComputeITOFixedBuyRoyalties(ctx, asset, ownerAcc, value)
	require.Error(t, err)
	assert.Equal(t, process.ErrInsufficientFunds, err)
	assert.Equal(t, transaction.Transaction_BalanceError, status)
}

func Test_BuyIto_ComputeITOFixedBuyRoyalties_OwnerUpdateUserError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)
	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			ITOFixed: 1_000_000, // 1 KLV
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	accCacher := &mock.AccountsCacherStub{
		UpdateUserCalled: func(account state.AccountHandler) error {
			return mockError
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	status, err := itoKapp.ComputeITOFixedBuyRoyalties(ctx, asset, ownerAcc, value)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_SaveAccountError, status)
}

func Test_BuyIto_ComputeITOFixedBuyRoyalties_KDAOwnerLoadUserError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	kdaOwnerAddress := []byte("kda-owner")

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)
	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			Address:  kdaOwnerAddress,
			ITOFixed: 1_000_000, // 1 KLV
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	accCacher := &mock.AccountsCacherStub{
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return nil, mockError
		},
		UpdateUserCalled: func(account state.AccountHandler) error {
			return nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	status, err := itoKapp.ComputeITOFixedBuyRoyalties(ctx, asset, ownerAcc, value)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_LoadAccountError, status)
}

func Test_BuyIto_ComputeITOFixedBuyRoyalties_KDAOwnerAddToBalanceError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	kdaOwnerAddress := []byte("kda-owner")

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)
	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			Address:  kdaOwnerAddress,
			ITOFixed: 1_000_000, // 1 KLV
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	accCacher := &mock.AccountsCacherStub{
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return &mock.UserAccountHandlerStub{
				AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return mockError
				},
			}, nil
		},
		UpdateUserCalled: func(account state.AccountHandler) error {
			return nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	status, err := itoKapp.ComputeITOFixedBuyRoyalties(ctx, asset, ownerAcc, value)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_BalanceError, status)
}

func Test_BuyIto_ComputeITOFixedBuyRoyalties_KDAOwnerUpdateUserError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	kdaOwnerAddress := []byte("kda-owner")

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)
	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			Address:  kdaOwnerAddress,
			ITOFixed: 1_000_000, // 1 KLV
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	kdaOwner := &mock.UserAccountHandlerStub{
		AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	accCacher := &mock.AccountsCacherStub{
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return kdaOwner, nil
		},
		UpdateUserCalled: func(account state.AccountHandler) error {
			if account == kdaOwner {
				return mockError
			}
			return nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	status, err := itoKapp.ComputeITOFixedBuyRoyalties(ctx, asset, ownerAcc, value)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_SaveAccountError, status)
}

func Test_BuyIto_ComputeITOFixedBuyRoyalties_InvalidSplitRoyaltiesError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)

	royalties := &kapps.RoyaltySplitData{
		PercentITOFixed: 50_00, // 50%
	}

	splitRoyalties := make(map[string]*kapps.RoyaltySplitData)
	splitRoyalties["user-1"] = royalties

	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			ITOFixed:       1_000_000, // 1 KLV
			SplitRoyalties: splitRoyalties,
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	accCacher := &mock.AccountsCacherStub{
		UpdateUserCalled: func(account state.AccountHandler) error {
			return nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	status, err := itoKapp.ComputeITOFixedBuyRoyalties(ctx, asset, ownerAcc, value)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_LoadAccountError, status)
}

func Test_BuyIto_ComputeITOFixedBuyRoyalties_SplitRoyaltiesBalanceError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)

	royalties := &kapps.RoyaltySplitData{
		PercentITOFixed: 50_00, // 50%
	}

	userAddress := hex.EncodeToString(makeAddress("user-1"))

	splitRoyalties := make(map[string]*kapps.RoyaltySplitData)
	splitRoyalties[userAddress] = royalties

	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			ITOFixed:       1_000_000, // 1 KLV
			SplitRoyalties: splitRoyalties,
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	user1 := &mock.UserAccountHandlerStub{
		AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return mockError
		},
	}

	accCacher := &mock.AccountsCacherStub{
		UpdateUserCalled: func(account state.AccountHandler) error {
			return nil
		},
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return user1, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	status, err := itoKapp.ComputeITOFixedBuyRoyalties(ctx, asset, ownerAcc, value)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_BalanceError, status)
}

func Test_BuyIto_ComputeITOFixedBuyRoyalties_SplitRoyaltiesUpdateUserError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)

	royalties := &kapps.RoyaltySplitData{
		PercentITOFixed: 50_00, // 50%
	}

	userAddress := hex.EncodeToString(makeAddress("user-1"))

	splitRoyalties := make(map[string]*kapps.RoyaltySplitData)
	splitRoyalties[userAddress] = royalties

	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			ITOFixed:       1_000_000, // 1 KLV
			SplitRoyalties: splitRoyalties,
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	user1 := &mock.UserAccountHandlerStub{
		AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	accCacher := &mock.AccountsCacherStub{
		UpdateUserCalled: func(account state.AccountHandler) error {
			if account == user1 {
				return mockError
			}

			return nil
		},
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return user1, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	status, err := itoKapp.ComputeITOFixedBuyRoyalties(ctx, asset, ownerAcc, value)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_SaveAccountError, status)
}

func Test_BuyIto_ComputeITOFixedBuyRoyalties_ShouldWork(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)

	royalties := &kapps.RoyaltySplitData{
		PercentITOFixed: 50_00, // 50%
	}

	userAddress := hex.EncodeToString(makeAddress("user-1"))

	splitRoyalties := make(map[string]*kapps.RoyaltySplitData)
	splitRoyalties[userAddress] = royalties

	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			ITOFixed:       1_000_000, // 1 KLV
			SplitRoyalties: splitRoyalties,
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	user1 := &mock.UserAccountHandlerStub{
		AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	accCacher := &mock.AccountsCacherStub{
		UpdateUserCalled: func(account state.AccountHandler) error {
			if account == user1 {
				return nil
			}

			return nil
		},
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return user1, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	status, err := itoKapp.ComputeITOFixedBuyRoyalties(ctx, asset, ownerAcc, value)
	require.NoError(t, err)
	assert.Equal(t, transaction.Transaction_Ok, status)
}

func Test_BuyIto_ComputeITOPercentBuyRoyalties_HaveNoRoyalties(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	currencyID := kdautils.KLVIdentifier
	value := int64(100_000_000)
	royaltiesPercentAmount := int64(0)
	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{},
	}
	ownerAcc := &mock.UserAccountHandlerStub{}

	status, err := itoKapp.ComputeITOPercentBuyRoyalties(ctx, asset, ownerAcc, currencyID, value, royaltiesPercentAmount)
	require.NoError(t, err)
	assert.Equal(t, transaction.Transaction_Ok, status)
}

func Test_BuyIto_ComputeITOPercentBuyRoyalties_InvalidSplitRoyaltiesError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)

	royalties := &kapps.RoyaltySplitData{
		PercentITOPercentage: 50_00, // 50%
	}

	splitRoyalties := make(map[string]*kapps.RoyaltySplitData)
	splitRoyalties["user-1"] = royalties

	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			SplitRoyalties: splitRoyalties,
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	accCacher := &mock.AccountsCacherStub{
		UpdateUserCalled: func(account state.AccountHandler) error {
			return nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)
	currencyID := kdautils.KLVIdentifier
	royaltiesPercentAmount := int64(20)

	status, err := itoKapp.ComputeITOPercentBuyRoyalties(ctx, asset, ownerAcc, currencyID, value, royaltiesPercentAmount)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_LoadAccountError, status)
}

func Test_BuyIto_ComputeITOPercentBuyRoyalties_SplitRoyaltiesBalanceError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)

	royalties := &kapps.RoyaltySplitData{
		PercentITOPercentage: 50_00, // 50%
	}

	userAddress := hex.EncodeToString(makeAddress("user-1"))

	splitRoyalties := make(map[string]*kapps.RoyaltySplitData)
	splitRoyalties[userAddress] = royalties

	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			SplitRoyalties: splitRoyalties,
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	user1 := &mock.UserAccountHandlerStub{
		AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return mockError
		},
	}

	accCacher := &mock.AccountsCacherStub{
		UpdateUserCalled: func(account state.AccountHandler) error {
			return nil
		},
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return user1, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)
	currencyID := kdautils.KLVIdentifier
	royaltiesPercentAmount := int64(20)

	status, err := itoKapp.ComputeITOPercentBuyRoyalties(ctx, asset, ownerAcc, currencyID, value, royaltiesPercentAmount)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_BalanceError, status)
}

func Test_BuyIto_ComputeITOPercentBuyRoyalties_SplitRoyaltiesUpdateUserError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)

	royalties := &kapps.RoyaltySplitData{
		PercentITOPercentage: 50_00, // 50%
	}

	userAddress := hex.EncodeToString(makeAddress("user-1"))

	splitRoyalties := make(map[string]*kapps.RoyaltySplitData)
	splitRoyalties[userAddress] = royalties

	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			ITOFixed:       1_000_000, // 1 KLV
			SplitRoyalties: splitRoyalties,
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	user1 := &mock.UserAccountHandlerStub{
		AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	accCacher := &mock.AccountsCacherStub{
		UpdateUserCalled: func(account state.AccountHandler) error {
			if account == user1 {
				return mockError
			}

			return nil
		},
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return user1, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)
	currencyID := kdautils.KLVIdentifier
	royaltiesPercentAmount := int64(20)

	status, err := itoKapp.ComputeITOPercentBuyRoyalties(ctx, asset, ownerAcc, currencyID, value, royaltiesPercentAmount)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_SaveAccountError, status)
}

func Test_BuyIto_ComputeITOPercentBuyRoyalties_KDAOwnerLoadUserError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	kdaOwnerAddress := []byte("kda-owner")

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)
	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			Address: kdaOwnerAddress,
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	accCacher := &mock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return nil, mockError
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)
	currencyID := kdautils.KLVIdentifier
	royaltiesPercentAmount := int64(20)

	status, err := itoKapp.ComputeITOPercentBuyRoyalties(ctx, asset, ownerAcc, currencyID, value, royaltiesPercentAmount)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_LoadAccountError, status)
}

func Test_BuyIto_ComputeITOPercentBuyRoyalties_KDAOwnerAddToBalanceError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	kdaOwnerAddress := []byte("kda-owner")

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)
	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			Address: kdaOwnerAddress,
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	accCacher := &mock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return &mock.UserAccountHandlerStub{
				AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return mockError
				},
			}, nil
		},
		UpdateUserCalled: func(account state.AccountHandler) error {
			return nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)
	currencyID := kdautils.KLVIdentifier
	royaltiesPercentAmount := int64(20)

	status, err := itoKapp.ComputeITOPercentBuyRoyalties(ctx, asset, ownerAcc, currencyID, value, royaltiesPercentAmount)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_BalanceError, status)
}

func Test_BuyIto_ComputeITOPercentBuyRoyalties_KDAOwnerUpdateUserError(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	kdaOwnerAddress := []byte("kda-owner")

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)
	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			Address: kdaOwnerAddress,
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	kdaOwner := &mock.UserAccountHandlerStub{
		AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	accCacher := &mock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return kdaOwner, nil
		},
		UpdateUserCalled: func(account state.AccountHandler) error {
			if account == kdaOwner {
				return mockError
			}
			return nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)
	currencyID := kdautils.KLVIdentifier
	royaltiesPercentAmount := int64(20)

	status, err := itoKapp.ComputeITOPercentBuyRoyalties(ctx, asset, ownerAcc, currencyID, value, royaltiesPercentAmount)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_SaveAccountError, status)
}

func Test_BuyIto_ComputeITOPercentBuyRoyalties_ShouldWork(t *testing.T) {
	itoKapp := setupITOKapp(t, config.EnableEpochs{
		SmartContracts: 0,
	})

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{})
	value := int64(100_000_000)

	royalties := &kapps.RoyaltySplitData{
		PercentITOPercentage: 50_00, // 50%
	}

	userAddress := hex.EncodeToString(makeAddress("user-1"))

	splitRoyalties := make(map[string]*kapps.RoyaltySplitData)
	splitRoyalties[userAddress] = royalties

	asset := &kapps.KDAData{
		Royalties: &kapps.RoyaltiesData{
			SplitRoyalties: splitRoyalties,
		},
	}
	ownerAcc := &mock.UserAccountHandlerStub{
		GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
			return 1_000_000_000_000 // 1M KLV
		},
		SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	user1 := &mock.UserAccountHandlerStub{
		AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	accCacher := &mock.AccountsCacherStub{
		UpdateUserCalled: func(account state.AccountHandler) error {
			if account == user1 {
				return nil
			}

			return nil
		},
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return user1, nil
		},
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return user1, nil
		},
	}

	_ = itoKapp.SetAccountsCacher(accCacher)

	currencyID := kdautils.KLVIdentifier
	royaltiesPercentAmount := int64(20)

	status, err := itoKapp.ComputeITOPercentBuyRoyalties(ctx, asset, ownerAcc, currencyID, value, royaltiesPercentAmount)
	require.NoError(t, err)
	assert.Equal(t, transaction.Transaction_Ok, status)
}

func Test_BuyITO_Retrieve_Account_To_Send_Percent_And_Fixed_Royalties(t *testing.T) {
	errLoadingUser := fmt.Errorf("Error on loading user account")

	cases := []struct {
		title                string
		accCacherStrub       state.AccountsCacher
		enableSCFork         bool
		err                  error
		expectedTxResultCode transaction.Transaction_TXResultCode
		expectedErr          error
	}{
		{
			title: "After fork error on LoadUserAccount",
			accCacherStrub: &mock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, errLoadingUser
				},
			},
			enableSCFork:         true,
			expectedTxResultCode: transaction.Transaction_LoadAccountError,
			expectedErr:          errLoadingUser,
		},
		{
			title: "After Fork success on LoadUserAccount",
			accCacherStrub: &mock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &mock.UserAccountHandlerStub{
						AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			enableSCFork:         true,
			expectedTxResultCode: transaction.Transaction_Ok,
			expectedErr:          nil,
		},
		{
			title: "Before fork error on GetUserAccount",
			accCacherStrub: &mock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, errLoadingUser
				},
			},
			enableSCFork:         false,
			expectedTxResultCode: transaction.Transaction_LoadAccountError,
			expectedErr:          errLoadingUser,
		},
		{
			title: "Before fork success on GetUserAccount",
			accCacherStrub: &mock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &mock.UserAccountHandlerStub{
						AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			enableSCFork:         false,
			expectedTxResultCode: transaction.Transaction_Ok,
			expectedErr:          nil,
		},
	}

	for _, tt := range cases {
		t.Run(tt.title, func(t *testing.T) {
			mockITOArgs := &ArgsNewITOKApp{
				Marshalizer:    &mock.MarshalizerMock{},
				PubkeyConv:     &mock.PubkeyConverterMock{},
				ForkController: mock.NewForkControllerStub(),
			}
			mockITOArgs.ForkController.(*mock.ForkControllerStub).SetFork(
				"EnableSmartContracts",
				tt.enableSCFork,
			)

			itoKapp, _ := NewITOKApp(mockITOArgs)
			itoKapp.SetAccountsCacher(tt.accCacherStrub)

			royaltiesToPay := int64(200)
			resCode, err := itoKapp.computeSplitRoyalties(
				&mock.KAppContextStub{
					ReceiptsCalled: func() kapp.ReceiptsContext {
						return &kapp.ReceiptSlice{}
					},
				},
				fmt.Sprintf("%x", "TestAddress"),
				[]byte("AST-HJK7"),
				&mock.AccountWrapMock{},
				10000,
				5,
				&royaltiesToPay,
			)

			assert.Equal(t, tt.expectedTxResultCode, resCode)
			require.Equal(t, tt.expectedErr, err)
		})
	}
}
