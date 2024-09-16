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
