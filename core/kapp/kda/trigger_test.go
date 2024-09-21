package kda_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/kda"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/require"
)

var defaultTicker []byte = []byte("KDA")
var defaultAssetID []byte = []byte("KDA-3W0I")

func createDefaultAsset(
	t *testing.T,
	kc kapp.KAppController,
	assetType transaction.CreateAssetContract_EnumAssetType,
	precision uint32,
	initialSupply int64,
	maxSupply int64,
	properties *transaction.PropertiesInfo,
) {
	if properties == nil {
		properties = &transaction.PropertiesInfo{
			CanFreeze:      true,
			CanWipe:        true,
			CanPause:       true,
			CanMint:        true,
			CanBurn:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
			LimitTransfer:  false,
		}
	}

	tc := &transaction.CreateAssetContract{
		Type:          assetType,
		Name:          defaultTicker,
		Ticker:        defaultTicker,
		OwnerAddress:  defaultSender,
		Precision:     precision,
		InitialSupply: initialSupply,
		MaxSupply:     maxSupply,
		Properties:    properties,
	}

	_, err := kc.GetKDAKApp().Create(defaultSender, tc)
	require.NoError(t, err)
}

func createDefaultKDAPool(
	t *testing.T,
	kc kapp.KAppController,
	assetID []byte,
	adminAddress []byte,
	active bool,
	fRatioKLV int64,
	fRatioKDA int64,
) {
	tc := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateKDAFeePool,
		AssetID:     assetID,
		KDAPool: &transaction.KDAPoolInfo{
			Active:       active,
			AdminAddress: adminAddress,
			FRatioKLV:    fRatioKLV,
			FRatioKDA:    fRatioKDA,
		},
	}

	_, err := kc.GetKDAKApp().Trigger(adminAddress, &tc, nil)
	require.NoError(t, err)
}

func TestKDATrigger_AssetMaxRole(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0, nil)

	for i := 0; i <= core.MaxAssetRoles; i++ {
		sender := fmt.Sprintf("sender_%d", i)
		validAddress := makeAddress(sender)
		tc := transaction.AssetTriggerContract{
			TriggerType: transaction.AssetTriggerContract_AddRole,
			AssetID:     defaultAssetID,
			Role: &transaction.RolesInfo{
				Address:     validAddress,
				HasRoleMint: true,
			},
		}
		_, err = kc.GetKDAKApp().Trigger(defaultSender, &tc, nil)
		require.NoError(t, err)
	}
	// Add one more role should fail
	validAddress := makeAddress("sender_21")
	tc := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_AddRole,
		AssetID:     defaultAssetID,
		Role: &transaction.RolesInfo{
			Address:     validAddress,
			HasRoleMint: true,
		},
	}
	_, err = kc.GetKDAKApp().Trigger(defaultSender, &tc, nil)
	require.Equal(t, common.ErrRoleLimitReached, err)
}

func TestKDATrigger_MintInvalidAssetShouldFail(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0, nil)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultTicker,
		ToAddress:   defaultSender,
		Amount:      1000,
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, common.ErrAssetNotFound, err)

	tc = &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     append(defaultAssetID, []byte("/1")...),
		ToAddress:   defaultSender,
		Amount:      1000,
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, common.ErrInvalidValue, err)
}

func TestKDATrigger_MintFTShouldWork(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0, nil)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      100,
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.NoError(t, err)

	// check balance
	acc, err := kc.GetAccountsKApp().GetAccountsCacher().GetExistingUser(defaultSender)
	require.NoError(t, err)
	balance := acc.GetBalanceWithNonce(defaultAssetID, nil, true)
	require.Equal(t, int64(100), balance) // NFT balance always 1

}

func TestKDATrigger_MintFTInvalidAmountShouldFail(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0, nil)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      0, // Not allowed to mint 0 amount
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, process.ErrInvalidArgument, err)

	tc = &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      -10, // Not allowed negative amount
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, process.ErrInvalidArgument, err)
}

func TestKDATrigger_MintNFTShouldWork(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_NonFungible, 0, 0, 0, nil)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      50,
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.NoError(t, err)

	acc, err := kc.GetAccountsKApp().GetAccountsCacher().GetExistingUser(defaultSender)
	require.NoError(t, err)
	// check balance of internal asset
	for i := 1; i <= 50; i++ {
		balance := acc.GetBalanceWithNonce(defaultAssetID, []byte(fmt.Sprintf("%d", i)), true)
		require.Equal(t, int64(1), balance) // NFT balance always 1
	}
}

func TestKDATrigger_MintNFTInvalidAmountShouldFail(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_NonFungible, 0, 0, 0, nil)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      0, // Not allowed to mint 0 amount
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, process.ErrInvalidArgument, err)

	tc = &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      100, // Not allowed to mint > 50 NFT per contract
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, process.ErrInvalidArgument, err)

	tc = &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      -10, // Not allowed negative amount
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, process.ErrInvalidArgument, err)
}

func TestKDATrigger_MintSNFTShouldWork(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_SemiFungible, 0, 0, 0, nil)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      0,
	}
	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.NoError(t, err)

	// check balance of internal asset, should be zero
	acc, err := kc.GetAccountsKApp().GetAccountsCacher().GetExistingUser(defaultSender)
	require.NoError(t, err)
	balance := acc.GetBalanceWithNonce(defaultAssetID, []byte("1"), true)
	require.Equal(t, int64(0), balance)

	// create another internal SFT with balance 100
	tc = &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      100,
	}
	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.NoError(t, err)

	// check balance of internal asset, should be zero
	acc, err = kc.GetAccountsKApp().GetAccountsCacher().GetExistingUser(defaultSender)
	require.NoError(t, err)
	balance = acc.GetBalanceWithNonce(defaultAssetID, []byte("2"), true)
	require.Equal(t, int64(100), balance)
}

func TestKDATrigger_MintSNFTInvalidAmountShouldFail(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_SemiFungible, 0, 0, 0, nil)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      -1,
	}
	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, process.ErrInvalidArgument, err)
}

func TestKDATrigger_ChangeOwnerWithoutPool(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0, nil)

	// Change Owner without having KDA Fee Pool
	tc := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_ChangeOwner,
		AssetID:     defaultAssetID,
		ToAddress:   defaultOtherSender,
	}

	_, err = kc.GetKDAKApp().Trigger(defaultSender, &tc, nil)

	require.NoError(t, err)
}

func TestKDATrigger_ChangeOwner(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0, nil)

	createDefaultKDAPool(t, kc, defaultAssetID, defaultSender, true, 1000, 100000)

	tests := []struct {
		name          string
		sender        []byte
		toAddress     []byte
		expectError   error
		expectedOwner []byte
	}{
		{
			name:          "Sender is not the owner",
			sender:        makeAddress("notOwner"),
			toAddress:     defaultOtherSender,
			expectError:   common.ErrAccNotOwner,
			expectedOwner: defaultSender,
		},
		{
			name:          "Invalid ToAddress",
			sender:        defaultSender,
			toAddress:     []byte("invalid_address"),
			expectError:   process.ErrInvalidRcvAddr,
			expectedOwner: defaultSender,
		},
		{
			name:          "Successful Change Owner",
			sender:        defaultSender,
			toAddress:     defaultOtherSender,
			expectError:   nil,
			expectedOwner: defaultOtherSender,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_ChangeOwner,
				AssetID:     defaultAssetID,
				ToAddress:   tt.toAddress,
			}

			_, err := kc.GetKDAKApp().Trigger(tt.sender, &tc, nil)

			assert.Equal(t, tt.expectError, err)

			assetOwner, _ := kc.GetKDAFeesPoolKApp().GetPoolOwner(defaultAssetID)
			assert.Equal(t, tt.expectedOwner, assetOwner)
		})
	}
}

func TestKDATrigger_ProcessRoyaltiesTransferPercentage(t *testing.T) {
	tests := []struct {
		name        string
		tp          []*transaction.RoyaltyInfo
		result      []*kapps.RoyaltyData
		expectError error
	}{
		{
			name: "Valid Royalty Transfer Ordered",
			tp: []*transaction.RoyaltyInfo{
				{Amount: 1000, Percentage: 10},
				{Amount: 2000, Percentage: 10},
			},
			result: []*kapps.RoyaltyData{
				{Amount: 1000, Percentage: 10},
				{Amount: 2000, Percentage: 10}},
			expectError: nil,
		},
		{
			name: "Valid Royalty Transfer Reverse",
			tp: []*transaction.RoyaltyInfo{
				{Amount: 2000, Percentage: 10},
				{Amount: 1000, Percentage: 10},
			},
			result: []*kapps.RoyaltyData{
				{Amount: 1000, Percentage: 10},
				{Amount: 2000, Percentage: 10}},
			expectError: nil,
		},
		{
			name: "Invalid Amount",
			tp: []*transaction.RoyaltyInfo{
				{Amount: -10, Percentage: 10},
				{Amount: 2000, Percentage: 10},
			},
			result:      nil,
			expectError: common.ErrInvalidValue,
		},
		{
			name: "Invalid Percentage",
			tp: []*transaction.RoyaltyInfo{
				{Amount: 1000, Percentage: 10001},
			},
			result:      nil,
			expectError: common.ErrInvalidValue,
		},
		{
			name:        "Empty Royalty Transfer",
			tp:          []*transaction.RoyaltyInfo{},
			result:      nil,
			expectError: nil,
		},
		{
			name: "Single Valid Royalty Transfer",
			tp: []*transaction.RoyaltyInfo{
				{Amount: 1000, Percentage: 10},
			},
			result: []*kapps.RoyaltyData{
				{Amount: 1000, Percentage: 10}},
			expectError: nil,
		},
		{
			name:        "Nil Royalty Transfer",
			tp:          nil,
			result:      nil,
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := kda.ProcessRoyaltiesTransferPercentage(tt.tp)
			assert.Equal(t, tt.expectError, err)
			assert.Equal(t, tt.result, result)
		})
	}
}

func TestKDATrigger_ValidateRoyalties(t *testing.T) {
	tests := []struct {
		name        string
		royalties   *transaction.RoyaltiesInfo
		checkMP     bool
		expectError error
	}{
		{
			name: "Valid Royalties - no market percentage check",
			royalties: &transaction.RoyaltiesInfo{
				ITOPercentage: 100,
				ITOFixed:      1000,
				TransferFixed: 1000,
			},

			checkMP:     false,
			expectError: nil,
		},
		{
			name: "Valid Royalties - market percentage check",
			royalties: &transaction.RoyaltiesInfo{
				ITOPercentage:    50,
				ITOFixed:         500,
				TransferFixed:    500,
				MarketPercentage: 100,
			},
			checkMP:     true,
			expectError: nil,
		},
		{
			name: "Invalid ITOPercentage",
			royalties: &transaction.RoyaltiesInfo{
				ITOPercentage: 20000, // Invalid, more than 100%
				ITOFixed:      1000,
				TransferFixed: 1000,
			},
			checkMP:     false,
			expectError: common.ErrInvalidValue,
		},
		{
			name: "Invalid TransferFixed",
			royalties: &transaction.RoyaltiesInfo{
				ITOPercentage: 100,
				ITOFixed:      1000,
				TransferFixed: -50, // Invalid, negative value
			},
			checkMP:     false,
			expectError: common.ErrInvalidValue,
		},
		{
			name: "Invalid MarketPercentage",
			royalties: &transaction.RoyaltiesInfo{
				ITOPercentage:    100,
				ITOFixed:         1000,
				TransferFixed:    1000,
				MarketPercentage: 20000, // Invalid, more than 100%
			},
			checkMP:     true,
			expectError: common.ErrInvalidValue,
		},
		{
			name: "Valid with MarketPercentage check off",
			royalties: &transaction.RoyaltiesInfo{
				ITOPercentage:    50,
				ITOFixed:         500,
				TransferFixed:    500,
				MarketPercentage: 20000, // Should not trigger validation when checkMP is false
			},
			checkMP:     false,
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := kda.ValidateRoyalties(
				&transaction.AssetTriggerContract{Royalties: tt.royalties},
				tt.checkMP,
			)
			assert.Equal(t, tt.expectError, err)
		})
	}
}

func TestKDATrigger_TriggerInvalidAssetIDShouldErr(t *testing.T) {
	tests := []struct {
		name          string
		assetID       []byte
		expectErrCode transaction.Transaction_TXResultCode
		expectErr     error
	}{
		{
			name:          "Nill AssetID",
			assetID:       nil,
			expectErrCode: transaction.Transaction_AssetError,
			expectErr:     common.ErrInvalidValue,
		},
		{
			name:          "Invalid AssetID Double Separator",
			assetID:       []byte("KDA-3W0I/1/2"),
			expectErrCode: transaction.Transaction_AssetIDInvalid,
			expectErr:     common.ErrInvalidValue,
		},
	}

	kdaKApp := kda.NewKDAKappForTests()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := transaction.AssetTriggerContract{
				AssetID: tt.assetID,
			}
			errCode, err := kdaKApp.Trigger(nil, &tc, nil)
			assert.Equal(t, tt.expectErrCode, errCode)
			assert.Equal(t, tt.expectErr, err)
		})
	}
}

func TestKDATrigger_TriggerProcessCheckValidTypes(t *testing.T) {
	kdaKApp := kda.NewKDAKappForTests()
	mockSender := []byte("mockSender")
	mockAssetID := bytes.Split([]byte("kda-0123"), []byte(kapps.Sp))
	mockAsset := &kapps.KDAData{
		// KDA Fee Pool is requires Asset to be FT with no nonce
		AssetType: kapps.KDAData_Fungible,
		// change owner requires asset properties to pass the check
		Properties: &kapps.PropertiesData{CanChangeOwner: true},
	}

	// this is a sanity check to ensure that all trigger types are implemented
	// no further checks are taken here
	for typeName, typeCode := range transaction.AssetTriggerContract_EnumTriggerType_value {
		tc := transaction.AssetTriggerContract{
			TriggerType: transaction.AssetTriggerContract_EnumTriggerType(typeCode),
			// KDAFeePool requires PooID not nil
			KDAPool: &transaction.KDAPoolInfo{},
		}

		// a valid trigger type will mostly be blocked when checking for ownership of the asset
		errCode, err := kdaKApp.ProcessTriggerType(mockSender, &tc, nil, mockAssetID, mockAsset, nil)
		assert.Equal(t, common.ErrAccNotOwner, err, fmt.Sprintf("TriggerType: %s", typeName))
		assert.Equal(t, transaction.Transaction_AccountNotOwner, errCode, fmt.Sprintf("TriggerType: %s", typeName))
	}

	// check invalid trigger type
	tc := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_EnumTriggerType(200),
	}
	_, err := kdaKApp.ProcessTriggerType(mockSender, &tc, nil, mockAssetID, mockAsset, nil)
	assert.Equal(t, common.ErrAssetTriggerInvalid, err)
}

func TestKDATrigger_TriggerRemoveRole(t *testing.T) {
	sender := makeAddress("owner_address")
	admin := makeAddress("admin_address")
	nonOwner := makeAddress("non_owner_address")
	toAddress := makeAddress("role_to_remove")
	assetID := [][]byte{[]byte("asset_id")}

	// Role to remove
	roleToRemove := &kapps.RolesData{Address: toAddress}
	remainingRole := &kapps.RolesData{Address: []byte("other_role")}

	// Test cases
	tests := []struct {
		name           string
		asset          *kapps.KDAData
		sender         []byte
		expectedResult transaction.Transaction_TXResultCode
		expectedError  error
		expectedRoles  []*kapps.RolesData
	}{
		{
			name: "Successful role removal",
			asset: &kapps.KDAData{
				Roles:        []*kapps.RolesData{roleToRemove, remainingRole},
				OwnerAddress: sender,
				AdminAddress: admin,
			},
			sender:         sender,
			expectedResult: transaction.Transaction_Ok,
			expectedError:  nil,
			expectedRoles:  []*kapps.RolesData{remainingRole}, // Only remaining role should be left
		},
		{
			name: "Sender is admin - successful role removal",
			asset: &kapps.KDAData{
				Roles:        []*kapps.RolesData{roleToRemove, remainingRole},
				OwnerAddress: sender,
				AdminAddress: admin,
			},
			sender:         admin,
			expectedResult: transaction.Transaction_Ok,
			expectedError:  nil,
			expectedRoles:  []*kapps.RolesData{remainingRole}, // Only remaining role should be left
		},
		{
			name: "Sender is not owner or admin - fail",
			asset: &kapps.KDAData{
				Roles:        []*kapps.RolesData{roleToRemove, remainingRole},
				OwnerAddress: sender,
				AdminAddress: admin,
			},
			sender:         nonOwner,
			expectedResult: transaction.Transaction_AccountNotOwner,
			expectedError:  common.ErrAccNotOwner,
			expectedRoles:  []*kapps.RolesData{roleToRemove, remainingRole}, // Roles unchanged
		},
		{
			name: "Role not found",
			asset: &kapps.KDAData{
				Roles:        []*kapps.RolesData{remainingRole},
				OwnerAddress: sender,
				AdminAddress: admin,
			},
			sender:         sender,
			expectedResult: transaction.Transaction_Ok,
			expectedError:  nil,
			expectedRoles:  []*kapps.RolesData{remainingRole}, // Unchanged as role doesn't exist
		},
	}

	kdaKAppData := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					return nil
				},
			}
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock KAppAccountHandler
			kdaKApp := kda.NewKDAKappForTests()

			// Call the function under test
			result, err := kdaKApp.HandleRemoveRole(tt.sender, &transaction.AssetTriggerContract{ToAddress: toAddress}, kdaKAppData, assetID, tt.asset)

			// Assert results
			assert.Equal(t, tt.expectedResult, result)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedRoles, tt.asset.Roles)
		})
	}
}

func TestKDATrigger_TriggerAddRole(t *testing.T) {
	sender := makeAddress("owner_address")
	admin := makeAddress("admin_address")
	nonOwner := makeAddress("non_owner_address")
	newRoleAddress := makeAddress("new_role_address")
	existingRoleAddress := makeAddress("existing_role_address")
	assetID := [][]byte{[]byte("asset_id")}

	// New role to add
	newRole := &kapps.RolesData{Address: newRoleAddress, HasRoleMint: true, HasRoleSetITOPrices: true}
	existingRole := &kapps.RolesData{Address: existingRoleAddress}

	// Test cases
	tests := []struct {
		name           string
		asset          *kapps.KDAData
		sender         []byte
		role           *transaction.RolesInfo
		expectedResult transaction.Transaction_TXResultCode
		expectedError  error
		expectedRoles  []*kapps.RolesData
	}{
		{
			name: "Successful role addition",
			asset: &kapps.KDAData{
				Roles:        []*kapps.RolesData{existingRole},
				OwnerAddress: sender,
				Properties: &kapps.PropertiesData{
					CanAddRoles: true,
				},
			},
			sender: sender,
			role: &transaction.RolesInfo{
				Address:             newRoleAddress,
				HasRoleMint:         true,
				HasRoleSetITOPrices: true,
			},
			expectedResult: transaction.Transaction_Ok,
			expectedError:  nil,
			expectedRoles:  []*kapps.RolesData{existingRole, newRole},
		},
		{
			name: "Sender is not owner or admin - fail",
			asset: &kapps.KDAData{
				Roles:        []*kapps.RolesData{existingRole},
				OwnerAddress: sender,
				AdminAddress: admin,
				Properties: &kapps.PropertiesData{
					CanAddRoles: true,
				},
			},
			sender: nonOwner,
			role: &transaction.RolesInfo{
				Address: newRoleAddress,
			},
			expectedResult: transaction.Transaction_AccountNotOwner,
			expectedError:  common.ErrAccNotOwner,
			expectedRoles:  []*kapps.RolesData{existingRole}, // No roles added
		},
		{
			name: "Asset can't add roles - fail",
			asset: &kapps.KDAData{
				Roles:        []*kapps.RolesData{existingRole},
				OwnerAddress: sender,
				Properties: &kapps.PropertiesData{
					CanAddRoles: false,
				},
			},
			sender: sender,
			role: &transaction.RolesInfo{
				Address: newRoleAddress,
			},
			expectedResult: transaction.Transaction_AssetCantBeBurned,
			expectedError:  common.ErrAssetTriggerInvalid,
			expectedRoles:  []*kapps.RolesData{existingRole}, // No roles added
		},
		{
			name: "Address length mismatch - fail",
			asset: &kapps.KDAData{
				Roles:        []*kapps.RolesData{existingRole},
				OwnerAddress: sender,
				Properties: &kapps.PropertiesData{
					CanAddRoles: true,
				},
			},
			sender: sender,
			role: &transaction.RolesInfo{
				Address: []byte("short_address"), // Invalid length
			},
			expectedResult: transaction.Transaction_AccountError,
			expectedError:  process.ErrInvalidRcvAddr,
			expectedRoles:  []*kapps.RolesData{existingRole}, // No roles added
		},
		{
			name: "Role already exists - update role",
			asset: &kapps.KDAData{
				Roles:        []*kapps.RolesData{existingRole},
				OwnerAddress: sender,
				Properties: &kapps.PropertiesData{
					CanAddRoles: true,
				},
			},
			sender: sender,
			role: &transaction.RolesInfo{
				Address:             existingRoleAddress, // Existing role, should be updated
				HasRoleMint:         true,
				HasRoleSetITOPrices: true,
			},
			expectedResult: transaction.Transaction_Ok,
			expectedError:  nil,
			expectedRoles: []*kapps.RolesData{{
				Address:             existingRoleAddress, // Updated role
				HasRoleMint:         true,
				HasRoleSetITOPrices: true,
			}},
		},
	}

	kdaKAppData := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					return nil
				},
			}
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			kdaKApp := kda.NewKDAKappForTests()

			// Call the function under test
			result, err := kdaKApp.HandleAddRole(tt.sender, &transaction.AssetTriggerContract{Role: tt.role}, kdaKAppData, assetID, tt.asset)

			// Assert results
			assert.Equal(t, tt.expectedResult, result)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedRoles, tt.asset.Roles)
		})
	}
}

func TestHandleStopNFTMetadataChange(t *testing.T) {
	sender := makeAddress("owner_address")
	admin := makeAddress("admin_address")
	nonOwner := makeAddress("non_owner_address")
	assetID := [][]byte{[]byte("asset_id")}
	kdaKApp := kda.NewKDAKappForTests()

	tests := []struct {
		name         string
		sender       []byte
		asset        *kapps.KDAData
		expectedErr  error
		expectedCode transaction.Transaction_TXResultCode
	}{
		{
			name:   "Asset type does not have nonce",
			sender: sender,
			asset: &kapps.KDAData{
				AssetType:    kapps.KDAData_Fungible,
				OwnerAddress: sender,
				AdminAddress: admin,
			},
			expectedErr:  common.ErrAssetTypeInvalid,
			expectedCode: transaction.Transaction_AssetTypeInvalid,
		},
		{
			name:   "Sender is not owner or admin",
			sender: nonOwner,
			asset: &kapps.KDAData{
				OwnerAddress: sender,
				AdminAddress: admin,
				AssetType:    kapps.KDAData_NonFungible,
			},
			expectedErr:  common.ErrAccNotOwner,
			expectedCode: transaction.Transaction_AccountNotOwner,
		},
		{
			name:   "NFT metadata change already stopped",
			sender: sender,
			asset: &kapps.KDAData{
				OwnerAddress: sender,
				AssetType:    kapps.KDAData_NonFungible,
				Attributes: &kapps.AttributesData{
					IsNFTMetadataChangeStopped: true,
				},
			},
			expectedErr:  common.ErrAssetTriggerInvalid,
			expectedCode: transaction.Transaction_NFTMetadataChangeStopped,
		},
		{
			name:   "Successful stop of NFT metadata change",
			sender: sender,
			asset: &kapps.KDAData{
				OwnerAddress: sender,
				AssetType:    kapps.KDAData_NonFungible,
				Attributes:   &kapps.AttributesData{},
			},
			expectedErr:  nil,
			expectedCode: transaction.Transaction_Ok,
		},
	}

	kdaKAppData := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					return nil
				},
			}
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tc := &transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_StopNFTMetadataChange,
			}

			code, err := kdaKApp.ProcessTriggerType(tt.sender, tc, kdaKAppData, assetID, tt.asset, nil)
			require.Equal(t, tt.expectedCode, code)
			require.Equal(t, tt.expectedErr, err)

			if tt.expectedErr == nil {
				assert.True(t, tt.asset.Attributes.IsNFTMetadataChangeStopped)
			}
		})
	}
}

func TestKDATrigger_HandlePauseOrResume(t *testing.T) {
	kdaKApp := kda.NewKDAKappForTests()

	assetID := [][]byte{defaultAssetID}
	owner := []byte("owner")
	admin := []byte("admin")
	nonOwner := []byte("nonOwner")

	tests := []struct {
		name           string
		sender         []byte
		triggerType    transaction.AssetTriggerContract_EnumTriggerType
		asset          *kapps.KDAData
		expectedCode   transaction.Transaction_TXResultCode
		expectedError  error
		expectedPaused bool
	}{
		{
			name:           "Pause by owner",
			sender:         owner,
			triggerType:    transaction.AssetTriggerContract_Pause,
			asset:          &kapps.KDAData{OwnerAddress: owner, Properties: &kapps.PropertiesData{CanPause: true}, Attributes: &kapps.AttributesData{IsPaused: false}},
			expectedCode:   transaction.Transaction_Ok,
			expectedError:  nil,
			expectedPaused: true,
		},
		{
			name:           "Resume by admin",
			sender:         admin,
			triggerType:    transaction.AssetTriggerContract_Resume,
			asset:          &kapps.KDAData{OwnerAddress: owner, AdminAddress: admin, Attributes: &kapps.AttributesData{IsPaused: true}},
			expectedCode:   transaction.Transaction_Ok,
			expectedError:  nil,
			expectedPaused: false,
		},
		{
			name:           "Pause by non-owner",
			sender:         nonOwner,
			triggerType:    transaction.AssetTriggerContract_Pause,
			asset:          &kapps.KDAData{OwnerAddress: owner, Attributes: &kapps.AttributesData{IsPaused: false}},
			expectedCode:   transaction.Transaction_AccountNotOwner,
			expectedError:  common.ErrAccNotOwner,
			expectedPaused: false,
		},
		{
			name:           "Pause when CanPause is false",
			sender:         owner,
			triggerType:    transaction.AssetTriggerContract_Pause,
			asset:          &kapps.KDAData{OwnerAddress: owner, Properties: &kapps.PropertiesData{CanPause: false}, Attributes: &kapps.AttributesData{IsPaused: false}},
			expectedCode:   transaction.Transaction_AssetCantBePaused,
			expectedError:  common.ErrAssetTriggerInvalid,
			expectedPaused: false,
		},
	}

	kdaAccHandler := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					return nil
				},
			}
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &transaction.AssetTriggerContract{
				TriggerType: tt.triggerType,
			}

			code, err := kdaKApp.HandlePauseOrResume(tt.sender, tc, kdaAccHandler, assetID, tt.asset)
			assert.Equal(t, tt.expectedCode, code)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedPaused, tt.asset.Attributes.IsPaused)
		})
	}
}

func TestKDATrigger_UpdateMetadata(t *testing.T) {
	kdaKApp := kda.NewKDAKappForTests()

	assetID := [][]byte{defaultAssetID, []byte("1")}
	owner := makeAddress("owner")
	nonOwner := makeAddress("nonOwner")

	tests := []struct {
		name          string
		sender        []byte
		asset         *kapps.KDAData
		tc            *transaction.AssetTriggerContract
		txData        [][]byte
		expectedCode  transaction.Transaction_TXResultCode
		expectedError error
	}{
		{
			name:   "Update metadata for NFT by owner",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_NonFungible,
				Attributes:   &kapps.AttributesData{},
			},
			tc: &transaction.AssetTriggerContract{
				ToAddress: owner,
				MIME:      []byte("image/png"),
			},
			txData:        [][]byte{[]byte("metadata")},
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
		},
		{
			name:   "Update metadata for NFT by non-owner",
			sender: nonOwner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_NonFungible,
				Attributes:   &kapps.AttributesData{},
			},
			tc: &transaction.AssetTriggerContract{
				ToAddress: owner,
				MIME:      []byte("image/png"),
			},
			txData:        [][]byte{[]byte("metadata")},
			expectedCode:  transaction.Transaction_AccountNotOwner,
			expectedError: common.ErrAccNotOwner,
		},
		{
			name:   "Update metadata for non-nonce asset",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_Fungible,
				Attributes:   &kapps.AttributesData{},
			},
			tc: &transaction.AssetTriggerContract{
				ToAddress: owner,
				MIME:      []byte("image/png"),
			},
			txData:        [][]byte{[]byte("metadata")},
			expectedCode:  transaction.Transaction_AssetTypeInvalid,
			expectedError: common.ErrAssetTypeInvalid,
		},
		{
			name:   "Update metadata for SFT",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_SemiFungible,
				Attributes:   &kapps.AttributesData{},
			},
			tc: &transaction.AssetTriggerContract{
				ToAddress: owner,
			},
			txData:        [][]byte{[]byte("6e657756616c7565@6e65774b6579")},
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
		},
	}

	kdaAccHandler := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					return nil
				},
			}
		},
	}

	kdaKApp.GetAccountsCacher().GetExistingUserCalled = func(address []byte) (state.UserAccountHandler, error) {
		return &mock.UserAccountHandlerStub{
			GetUserKDACalled: func(assetID []byte, nonce []byte, _ bool) (*kapps.UserKDA, error) {
				return &kapps.UserKDA{Balance: 1}, nil
			},
			DataTrieTrackerCalled: func() state.DataTrieTracker {
				return &mock.DataTrieTrackerStub{
					SaveKeyValueCalled: func(key []byte, value []byte) error {
						return nil
					},
				}
			},
			SetRootHashCalled: func(rootHash []byte) {
				_ = rootHash
				return
			},
		}, nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := kdaKApp.UpdateMetadata(tt.sender, tt.tc, kdaAccHandler, assetID, tt.asset, tt.txData)
			assert.Equal(t, tt.expectedCode, code)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestKDATrigger_HandleStopNFTMint(t *testing.T) {
	kdaKApp := kda.NewKDAKappForTests()

	assetID := [][]byte{defaultAssetID}
	owner := []byte("owner")
	admin := []byte("admin")
	nonOwner := []byte("nonOwner")

	tests := []struct {
		name            string
		sender          []byte
		asset           *kapps.KDAData
		expectedCode    transaction.Transaction_TXResultCode
		expectedError   error
		expectedStopped bool
	}{
		{
			name:   "Asset type does not have nonce",
			sender: owner,
			asset: &kapps.KDAData{
				AssetType:    kapps.KDAData_Fungible,
				OwnerAddress: owner,
				Attributes:   &kapps.AttributesData{},
			},
			expectedCode:    transaction.Transaction_AssetTypeInvalid,
			expectedError:   common.ErrAssetTypeInvalid,
			expectedStopped: false,
		},
		{
			name:   "Stop NFT mint by owner",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_NonFungible,
				Attributes:   &kapps.AttributesData{},
			},
			expectedCode:    transaction.Transaction_Ok,
			expectedError:   nil,
			expectedStopped: true,
		},
		{
			name:   "Stop NFT mint by admin",
			sender: admin,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AdminAddress: admin,
				AssetType:    kapps.KDAData_NonFungible,
				Attributes:   &kapps.AttributesData{},
			},
			expectedCode:    transaction.Transaction_Ok,
			expectedError:   nil,
			expectedStopped: true,
		},
		{
			name:   "Stop NFT mint by non-owner",
			sender: nonOwner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_NonFungible,
				Attributes:   &kapps.AttributesData{},
			},
			expectedCode:    transaction.Transaction_AccountNotOwner,
			expectedError:   common.ErrAccNotOwner,
			expectedStopped: false,
		},
		{
			name:   "Stop NFT mint for already stopped asset",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_NonFungible,
				Attributes:   &kapps.AttributesData{IsNFTMintStopped: true},
			},
			expectedCode:    transaction.Transaction_NFTMintStopped,
			expectedError:   common.ErrAssetTriggerInvalid,
			expectedStopped: true,
		},
	}

	kdaAccHandler := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					return nil
				},
			}
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := kdaKApp.HandleStopNFTMint(tt.sender, kdaAccHandler, assetID, tt.asset)
			assert.Equal(t, tt.expectedCode, code)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedStopped, tt.asset.Attributes.IsNFTMintStopped)
		})
	}
}

func TestKDATrigger_HandleUpdateLogo(t *testing.T) {
	kdaKApp := kda.NewKDAKappForTests()

	assetID := [][]byte{defaultAssetID}
	owner := []byte("owner")
	admin := []byte("admin")
	nonOwner := []byte("nonOwner")

	tests := []struct {
		name          string
		sender        []byte
		asset         *kapps.KDAData
		logo          string
		expectedCode  transaction.Transaction_TXResultCode
		expectedError error
		expectedLogo  string
	}{
		{
			name:   "Update logo by owner",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
			},
			logo:          "https://example.com/logo.png",
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
			expectedLogo:  "https://example.com/logo.png",
		},
		{
			name:   "Update logo by admin",
			sender: admin,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AdminAddress: admin,
			},
			logo:          "https://example.com/new_logo.png",
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
			expectedLogo:  "https://example.com/new_logo.png",
		},
		{
			name:   "Update logo by non-owner",
			sender: nonOwner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
			},
			logo:          "https://example.com/invalid_logo.png",
			expectedCode:  transaction.Transaction_AccountNotOwner,
			expectedError: common.ErrAccNotOwner,
			expectedLogo:  "",
		},
		{
			name:   "Update logo with invalid URI",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
			},
			logo:          string(make([]byte, core.MaxLogoURISize+1)),
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidValue,
			expectedLogo:  "",
		},
	}

	kdaAccHandler := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					return nil
				},
			}
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &transaction.AssetTriggerContract{
				Logo: tt.logo,
			}

			code, err := kdaKApp.HandleUpdateLogo(tt.sender, tc, kdaAccHandler, assetID, tt.asset)
			assert.Equal(t, tt.expectedCode, code)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedLogo, tt.asset.Logo)
		})
	}
}

func TestKDATrigger_HandleStopRoyaltiesChange(t *testing.T) {
	kdaKApp := kda.NewKDAKappForTests()

	assetID := [][]byte{defaultAssetID}
	owner := []byte("owner")
	admin := []byte("admin")
	nonOwner := []byte("nonOwner")

	tests := []struct {
		name                     string
		sender                   []byte
		asset                    *kapps.KDAData
		expectedCode             transaction.Transaction_TXResultCode
		expectedError            error
		expectedRoyaltiesStopped bool
	}{
		{
			name:   "Stop royalties change by owner",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				Attributes:   &kapps.AttributesData{},
			},
			expectedCode:             transaction.Transaction_Ok,
			expectedError:            nil,
			expectedRoyaltiesStopped: true,
		},
		{
			name:   "Stop royalties change by admin",
			sender: admin,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AdminAddress: admin,
				Attributes:   &kapps.AttributesData{},
			},
			expectedCode:             transaction.Transaction_Ok,
			expectedError:            nil,
			expectedRoyaltiesStopped: true,
		},
		{
			name:   "Stop royalties change by non-owner",
			sender: nonOwner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				Attributes:   &kapps.AttributesData{},
			},
			expectedCode:             transaction.Transaction_AccountNotOwner,
			expectedError:            common.ErrAccNotOwner,
			expectedRoyaltiesStopped: false,
		},
		{
			name:   "Royalties change already stopped",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				Attributes:   &kapps.AttributesData{IsRoyaltiesChangeStopped: true},
			},
			expectedCode:             transaction.Transaction_RoyaltiesChangeStopped,
			expectedError:            common.ErrAssetTriggerInvalid,
			expectedRoyaltiesStopped: true,
		},
	}

	kdaAccHandler := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					return nil
				},
			}
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := kdaKApp.HandleStopRoyaltiesChange(tt.sender, kdaAccHandler, assetID, tt.asset)
			assert.Equal(t, tt.expectedCode, code)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedRoyaltiesStopped, tt.asset.Attributes.IsRoyaltiesChangeStopped)
		})
	}
}

func TestKDATrigger_UpdateRoyaltiesReceiver(t *testing.T) {
	kdaKApp := kda.NewKDAKappForTests()

	assetID := [][]byte{defaultAssetID}
	owner := makeAddress("owner")
	admin := makeAddress("admin")
	nonOwner := makeAddress("nonOwner")
	newReceiver := makeAddress("newReceiver")

	tests := []struct {
		name          string
		sender        []byte
		asset         *kapps.KDAData
		toAddress     []byte
		expectedCode  transaction.Transaction_TXResultCode
		expectedError error
	}{
		{
			name:   "Update royalties receiver by owner",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				Royalties:    &kapps.RoyaltiesData{},
			},
			toAddress:     newReceiver,
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
		},
		{
			name:   "Update royalties receiver by admin",
			sender: admin,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AdminAddress: admin,
				Royalties:    &kapps.RoyaltiesData{},
			},
			toAddress:     newReceiver,
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
		},
		{
			name:   "Update royalties receiver by non-owner",
			sender: nonOwner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				Royalties:    &kapps.RoyaltiesData{},
			},
			toAddress:     newReceiver,
			expectedCode:  transaction.Transaction_AccountNotOwner,
			expectedError: common.ErrAccNotOwner,
		},
		{
			name:   "Invalid receiver address",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				Royalties:    &kapps.RoyaltiesData{},
			},
			toAddress:     []byte("invalidAddress"),
			expectedCode:  transaction.Transaction_AccountError,
			expectedError: process.ErrInvalidRcvAddr,
		},
	}

	kdaAccHandler := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					return nil
				},
			}
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &transaction.AssetTriggerContract{
				ToAddress: tt.toAddress,
			}
			code, err := kdaKApp.UpdateRoyaltiesReceiver(tt.sender, tc, kdaAccHandler, assetID, tt.asset)
			assert.Equal(t, tt.expectedCode, code)
			assert.Equal(t, tt.expectedError, err)
			if err == nil {
				assert.Equal(t, tt.toAddress, tt.asset.Royalties.Address)
			}
		})
	}
}

func TestKDATrigger_UpdateStaking(t *testing.T) {
	kdaKApp := kda.NewKDAKappForTests()

	assetID := [][]byte{defaultAssetID}
	owner := []byte("owner")
	admin := []byte("admin")
	nonOwner := []byte("nonOwner")

	tests := []struct {
		name          string
		sender        []byte
		asset         *kapps.KDAData
		staking       *transaction.StakingInfo
		stakingData   *kapps.StakingData
		expectedCode  transaction.Transaction_TXResultCode
		expectedError error
	}{
		{
			name:   "Update staking by owner",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_Fungible,
				ID:           assetID[0],
			},
			staking: &transaction.StakingInfo{
				Type:                transaction.StakingInfo_APRI,
				MinEpochsToClaim:    10,
				MinEpochsToUnstake:  20,
				MinEpochsToWithdraw: 30,
				APR:                 5000,
			},
			stakingData:   &kapps.StakingData{},
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
		},
		{
			name:   "Update staking by admin",
			sender: admin,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AdminAddress: admin,
				AssetType:    kapps.KDAData_Fungible,
				ID:           assetID[0],
			},
			staking: &transaction.StakingInfo{
				Type:                transaction.StakingInfo_APRI,
				MinEpochsToClaim:    15,
				MinEpochsToUnstake:  25,
				MinEpochsToWithdraw: 35,
			},
			stakingData:   &kapps.StakingData{},
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
		},
		{
			name:   "Update staking FPRI diff type Should Fail",
			sender: admin,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AdminAddress: admin,
				AssetType:    kapps.KDAData_Fungible,
				ID:           assetID[0],
			},
			staking: &transaction.StakingInfo{
				Type:                transaction.StakingInfo_FPRI,
				MinEpochsToClaim:    15,
				MinEpochsToUnstake:  25,
				MinEpochsToWithdraw: 35,
			},
			stakingData:   &kapps.StakingData{},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidValue,
		},
		{
			name:   "Update staking FPRI same type Should Work",
			sender: admin,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AdminAddress: admin,
				AssetType:    kapps.KDAData_Fungible,
				ID:           assetID[0],
			},
			staking: &transaction.StakingInfo{
				Type:                transaction.StakingInfo_FPRI,
				MinEpochsToClaim:    15,
				MinEpochsToUnstake:  25,
				MinEpochsToWithdraw: 35,
			},
			stakingData:   &kapps.StakingData{InterestType: kapps.StakingData_FPRI},
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
		},
		{
			name:   "Update staking invalid type",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_Fungible,
				ID:           assetID[0],
			},
			staking: &transaction.StakingInfo{
				Type: 999, // Invalid type
			},
			stakingData:   &kapps.StakingData{InterestType: 999}, // Invalid type
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrAssetTypeInvalid,
		},
		{
			name:   "Update staking by non-owner",
			sender: nonOwner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_Fungible,
				ID:           assetID[0],
			},
			staking: &transaction.StakingInfo{
				Type: transaction.StakingInfo_APRI,
			},
			stakingData:   &kapps.StakingData{},
			expectedCode:  transaction.Transaction_AccountNotOwner,
			expectedError: common.ErrAccNotOwner,
		},
		{
			name:   "Update staking for non-fungible asset",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_NonFungible,
				ID:           assetID[0],
			},
			staking: &transaction.StakingInfo{
				Type: transaction.StakingInfo_APRI,
			},
			stakingData:   &kapps.StakingData{},
			expectedCode:  transaction.Transaction_AssetTypeInvalid,
			expectedError: common.ErrAssetTypeInvalid,
		},
		{
			name:   "Update staking nil staking contract",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_Fungible,
				ID:           assetID[0],
			},
			staking:       nil,
			stakingData:   &kapps.StakingData{},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidValue,
		},
		{
			name:   "Update staking NFT asset",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_NonFungible,
				ID:           assetID[0],
			},
			staking:       &transaction.StakingInfo{},
			stakingData:   &kapps.StakingData{},
			expectedCode:  transaction.Transaction_AssetTypeInvalid,
			expectedError: common.ErrAssetTypeInvalid,
		},
	}

	kdaAccHandler := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					return nil
				},
			}
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// update staking data
			kdaKApp.GetAccountsCacher().GetExistingKappCalled = func(assetID []byte) (state.KAppAccountHandler, error) {
				return &mock.KAppAccountHandlerStub{
					DataTrieTrackerCalled: func() state.DataTrieTracker {
						return &mock.DataTrieTrackerStub{
							RetrieveValueCalled: func(key []byte) ([]byte, error) {
								return json.Marshal(tt.stakingData)
							},
							SaveKeyValueCalled: func(key []byte, value []byte) error {
								return nil
							},
						}
					},
				}, nil
			}

			tc := &transaction.AssetTriggerContract{
				Staking: tt.staking,
			}
			code, err := kdaKApp.UpdateStaking(tt.sender, tc, kdaAccHandler, assetID, tt.asset)
			assert.Equal(t, tt.expectedCode, code)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestKDATrigger_UpdateStakingNilStakingForAsset(t *testing.T) {
	kdaKApp := kda.NewKDAKappForTests()

	assetID := [][]byte{defaultAssetID}
	owner := []byte("owner")
	asset := &kapps.KDAData{
		OwnerAddress: owner,
		AssetType:    kapps.KDAData_Fungible,
	}

	kdaAccHandler := &mock.KAppAccountHandlerStub{}

	tc := &transaction.AssetTriggerContract{
		Staking: &transaction.StakingInfo{},
	}

	// nil asset ID (DB inconsistency)
	code, err := kdaKApp.UpdateStaking(owner, tc, kdaAccHandler, assetID, asset)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, code)
	assert.Equal(t, common.ErrInvalidValue, err)

	asset.ID = assetID[0]
	// no staking data for asset
	kdaKApp.GetAccountsCacher().GetExistingKappCalled = func(assetID []byte) (state.KAppAccountHandler, error) {
		return &mock.KAppAccountHandlerStub{
			DataTrieTrackerCalled: func() state.DataTrieTracker {
				return &mock.DataTrieTrackerStub{
					RetrieveValueCalled: func(key []byte) ([]byte, error) {
						return nil, common.ErrNilTrie
					},
				}
			},
		}, nil
	}

	code, err = kdaKApp.UpdateStaking(owner, tc, kdaAccHandler, assetID, asset)
	assert.Equal(t, transaction.Transaction_AssetError, code)
	assert.Equal(t, common.ErrNilTrie, err)

	// staking data empty for asset (not found)
	kdaKApp.GetAccountsCacher().GetExistingKappCalled = func(assetID []byte) (state.KAppAccountHandler, error) {
		return &mock.KAppAccountHandlerStub{
			DataTrieTrackerCalled: func() state.DataTrieTracker {
				return &mock.DataTrieTrackerStub{
					RetrieveValueCalled: func(key []byte) ([]byte, error) {
						return []byte{}, nil
					},
				}
			},
		}, nil
	}

	code, err = kdaKApp.UpdateStaking(owner, tc, kdaAccHandler, assetID, asset)
	assert.Equal(t, transaction.Transaction_AssetError, code)
	assert.Equal(t, common.ErrStakingNotFound, err)

	// failed marshalling of staking data (db inconsistency)
	kdaKApp.GetAccountsCacher().GetExistingKappCalled = func(assetID []byte) (state.KAppAccountHandler, error) {
		return &mock.KAppAccountHandlerStub{
			DataTrieTrackerCalled: func() state.DataTrieTracker {
				return &mock.DataTrieTrackerStub{
					RetrieveValueCalled: func(key []byte) ([]byte, error) {
						return []byte("01010101"), nil
					},
				}
			},
		}, nil
	}

	code, err = kdaKApp.UpdateStaking(owner, tc, kdaAccHandler, assetID, asset)
	assert.Equal(t, transaction.Transaction_AssetError, code)
	assert.Contains(t, err.Error(), "invalid character")

}

func TestKDATrigger_UpdateStakingMaxUpdatesOverwrite(t *testing.T) {
	kdaKApp := kda.NewKDAKappForTests()

	assetID := [][]byte{defaultAssetID}
	owner := []byte("owner")
	asset := &kapps.KDAData{
		OwnerAddress: owner,
		AssetType:    kapps.KDAData_Fungible,
		ID:           assetID[0],
	}

	saveCounter := 0

	kdaAccHandler := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					saveCounter++
					return nil
				},
			}
		},
	}

	stakingInfo, _ := json.Marshal(&kapps.StakingData{})

	kdaKApp.GetAccountsCacher().GetExistingKappCalled = func(assetID []byte) (state.KAppAccountHandler, error) {
		return &mock.KAppAccountHandlerStub{
			DataTrieTrackerCalled: func() state.DataTrieTracker {
				return &mock.DataTrieTrackerStub{
					RetrieveValueCalled: func(key []byte) ([]byte, error) {
						return stakingInfo, nil
					},
					SaveKeyValueCalled: func(key []byte, value []byte) error {
						stakingInfo = value
						return nil
					},
				}
			},
		}, nil
	}

	tc := &transaction.AssetTriggerContract{
		Staking: &transaction.StakingInfo{},
	}

	for i := 0; i < core.MaxAPRPercentageUpdates; i++ {
		tc.Staking.APR = uint32(i)
		code, err := kdaKApp.UpdateStaking(owner, tc, kdaAccHandler, assetID, asset)
		assert.Equal(t, transaction.Transaction_Ok, code)
		assert.Nil(t, err)
		assert.Equal(t, i+1, saveCounter)
	}

	// check asset APR len
	stakingData := &kapps.StakingData{}
	_ = json.Unmarshal(stakingInfo, stakingData)
	assert.Len(t, stakingData.APR, core.MaxAPRPercentageUpdates)

	// overwrite first APR
	code, err := kdaKApp.UpdateStaking(owner, tc, kdaAccHandler, assetID, asset)
	assert.Equal(t, transaction.Transaction_Ok, code)
	assert.Nil(t, err)

	// check asset APR len
	stakingData = &kapps.StakingData{}
	_ = json.Unmarshal(stakingInfo, stakingData)
	assert.Len(t, stakingData.APR, core.MaxAPRPercentageUpdates)

	// check first APR value
	assert.Equal(t, uint32(1), stakingData.APR[0].Value)
}

func TestKDATrigger_UpdateRoyalties(t *testing.T) {
	kdaKApp := kda.NewKDAKappForTests()

	assetID := [][]byte{defaultAssetID}
	owner := []byte("owner")
	admin := []byte("admin")
	nonOwner := []byte("nonOwner")

	tests := []struct {
		name          string
		sender        []byte
		asset         *kapps.KDAData
		royalties     *transaction.RoyaltiesInfo
		expectedCode  transaction.Transaction_TXResultCode
		expectedError error
		validate      func(t *testing.T, asset *kapps.KDAData)
	}{
		{
			name:   "Update royalties for fungible asset by owner",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_Fungible,
				Royalties:    &kapps.RoyaltiesData{},
				Attributes:   &kapps.AttributesData{},
			},
			royalties: &transaction.RoyaltiesInfo{
				TransferPercentage: []*transaction.RoyaltyInfo{{Amount: 1000, Percentage: 10}},
				TransferFixed:      100,
				ITOPercentage:      5000,
				ITOFixed:           200,
			},
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
			validate: func(t *testing.T, asset *kapps.KDAData) {
				assert.Equal(t, int64(100), asset.Royalties.TransferFixed)
				assert.Equal(t, uint32(5000), asset.Royalties.ITOPercentage)
				assert.Equal(t, int64(200), asset.Royalties.ITOFixed)
				assert.Len(t, asset.Royalties.TransferPercentage, 1)
			},
		},
		{
			name:   "Update royalties for non-fungible asset by admin",
			sender: admin,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AdminAddress: admin,
				AssetType:    kapps.KDAData_NonFungible,
				Royalties:    &kapps.RoyaltiesData{},
				Attributes:   &kapps.AttributesData{},
			},
			royalties: &transaction.RoyaltiesInfo{
				MarketPercentage: 2000,
				MarketFixed:      300,
				TransferFixed:    150,
				ITOPercentage:    3000,
				ITOFixed:         250,
			},
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
			validate: func(t *testing.T, asset *kapps.KDAData) {
				assert.Equal(t, uint32(2000), asset.Royalties.MarketPercentage)
				assert.Equal(t, int64(300), asset.Royalties.MarketFixed)
				assert.Equal(t, int64(150), asset.Royalties.TransferFixed)
				assert.Equal(t, uint32(3000), asset.Royalties.ITOPercentage)
				assert.Equal(t, int64(250), asset.Royalties.ITOFixed)
			},
		},
		{
			name:   "Update royalties by non-owner",
			sender: nonOwner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_Fungible,
				Royalties:    &kapps.RoyaltiesData{},
				Attributes:   &kapps.AttributesData{},
			},
			royalties: &transaction.RoyaltiesInfo{
				TransferFixed: 100,
			},
			expectedCode:  transaction.Transaction_AccountNotOwner,
			expectedError: common.ErrAccNotOwner,
		},
		{
			name:   "Update royalties when change is stopped",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_Fungible,
				Royalties:    &kapps.RoyaltiesData{},
				Attributes:   &kapps.AttributesData{IsRoyaltiesChangeStopped: true},
			},
			royalties: &transaction.RoyaltiesInfo{
				TransferFixed: 100,
			},
			expectedCode:  transaction.Transaction_RoyaltiesChangeStopped,
			expectedError: common.ErrAssetTriggerInvalid,
		},
		{
			name:   "Update royalties with invalid percentage",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_Fungible,
				Royalties:    &kapps.RoyaltiesData{},
				Attributes:   &kapps.AttributesData{},
			},
			royalties: &transaction.RoyaltiesInfo{
				ITOPercentage: 20000, // Invalid: > 100%
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidValue,
		},
		{
			name:   "Update royalties with split royalties",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_NonFungible,
				Royalties:    &kapps.RoyaltiesData{},
				Attributes:   &kapps.AttributesData{},
			},
			royalties: &transaction.RoyaltiesInfo{
				MarketPercentage: 1000,
				SplitRoyalties: map[string]*transaction.RoyaltySplitInfo{
					hex.EncodeToString(makeAddress("address1")): {PercentMarketPercentage: 500},
					hex.EncodeToString(makeAddress("address2")): {PercentMarketPercentage: 500},
				},
			},
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
			validate: func(t *testing.T, asset *kapps.KDAData) {
				assert.Equal(t, uint32(1000), asset.Royalties.MarketPercentage)
				assert.Len(t, asset.Royalties.SplitRoyalties, 2)
			},
		},
		{
			name:   "Update royalties with split royalties > 100p",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_NonFungible,
				Royalties:    &kapps.RoyaltiesData{},
				Attributes:   &kapps.AttributesData{},
			},
			royalties: &transaction.RoyaltiesInfo{
				MarketPercentage: 1000,
				SplitRoyalties: map[string]*transaction.RoyaltySplitInfo{
					hex.EncodeToString(makeAddress("address1")): {PercentMarketPercentage: 90000},
					hex.EncodeToString(makeAddress("address2")): {PercentMarketPercentage: 10001},
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidValue,
			validate: func(t *testing.T, asset *kapps.KDAData) {
				assert.Equal(t, uint32(1000), asset.Royalties.MarketPercentage)
				assert.Len(t, asset.Royalties.SplitRoyalties, 2)
			},
		},
		{
			name:   "Update royalties with split royalties Fungible asset",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_Fungible,
				Royalties:    &kapps.RoyaltiesData{},
				Attributes:   &kapps.AttributesData{},
			},
			royalties: &transaction.RoyaltiesInfo{
				MarketPercentage: 1000,
				SplitRoyalties: map[string]*transaction.RoyaltySplitInfo{
					hex.EncodeToString(makeAddress("address1")): {PercentTransferPercentage: 500},
					hex.EncodeToString(makeAddress("address2")): {PercentTransferPercentage: 500},
				},
			},
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
			validate: func(t *testing.T, asset *kapps.KDAData) {
				assert.Equal(t, uint32(1000), asset.Royalties.MarketPercentage)
				assert.Len(t, asset.Royalties.SplitRoyalties, 2)
			},
		},
		{
			name:   "Update royalties with split royalties > 100p Fungible asset",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AssetType:    kapps.KDAData_Fungible,
				Royalties:    &kapps.RoyaltiesData{},
				Attributes:   &kapps.AttributesData{},
			},
			royalties: &transaction.RoyaltiesInfo{
				MarketPercentage: 1000,
				SplitRoyalties: map[string]*transaction.RoyaltySplitInfo{
					hex.EncodeToString(makeAddress("address1")): {PercentTransferPercentage: 90000},
					hex.EncodeToString(makeAddress("address2")): {PercentTransferPercentage: 10001},
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidValue,
			validate: func(t *testing.T, asset *kapps.KDAData) {
				assert.Equal(t, uint32(1000), asset.Royalties.MarketPercentage)
				assert.Len(t, asset.Royalties.SplitRoyalties, 2)
			},
		},
	}

	kdaAccHandler := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					return nil
				},
			}
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &transaction.AssetTriggerContract{
				Royalties: tt.royalties,
			}
			code, err := kdaKApp.UpdateRoyalties(tt.sender, tc, kdaAccHandler, assetID, tt.asset)
			assert.Equal(t, tt.expectedCode, code)
			assert.Equal(t, tt.expectedError, err)

			if tt.validate != nil && err == nil {
				tt.validate(t, tt.asset)
			}
		})
	}
}

func TestKDATrigger_HandleUpdateURIs(t *testing.T) {
	kdaKApp := kda.NewKDAKappForTests()

	assetID := [][]byte{defaultAssetID}
	owner := []byte("owner")
	admin := []byte("admin")
	nonOwner := []byte("nonOwner")

	tests := []struct {
		name          string
		sender        []byte
		asset         *kapps.KDAData
		uris          map[string]string
		expectedCode  transaction.Transaction_TXResultCode
		expectedError error
		validate      func(t *testing.T, asset *kapps.KDAData)
	}{
		{
			name:   "Update URIs by owner",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				URIs:         make(map[string]string),
			},
			uris: map[string]string{
				"website": "https://example.com",
				"twitter": "https://twitter.com/example",
			},
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
			validate: func(t *testing.T, asset *kapps.KDAData) {
				assert.Equal(t, "https://example.com", asset.URIs["website"])
				assert.Equal(t, "https://twitter.com/example", asset.URIs["twitter"])
			},
		},
		{
			name:   "Update URIs by admin",
			sender: admin,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				AdminAddress: admin,
				URIs:         make(map[string]string),
			},
			uris: map[string]string{
				"telegram": "https://t.me/example",
			},
			expectedCode:  transaction.Transaction_Ok,
			expectedError: nil,
			validate: func(t *testing.T, asset *kapps.KDAData) {
				assert.Equal(t, "https://t.me/example", asset.URIs["telegram"])
			},
		},
		{
			name:   "Update URIs by non-owner",
			sender: nonOwner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				URIs:         make(map[string]string),
			},
			uris: map[string]string{
				"website": "https://example.com",
			},
			expectedCode:  transaction.Transaction_AccountNotOwner,
			expectedError: common.ErrAccNotOwner,
		},
		{
			name:   "Update URIs with too many entries",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				URIs:         make(map[string]string),
			},
			uris:          generateLargeURIMap(core.MaxURIMapSize + 1),
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidValue,
		},
		{
			name:   "Update URIs with invalid key length",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				URIs:         make(map[string]string),
			},
			uris: map[string]string{
				string(make([]byte, core.MaxURIKeySize+1)): "https://example.com",
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidValue,
		},
		{
			name:   "Update URIs with invalid value length",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				URIs:         make(map[string]string),
			},
			uris: map[string]string{
				"website": string(make([]byte, core.MaxURIValueSize+1)),
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidValue,
		},
		{
			name:   "Update URIs with non-UTF8 key",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				URIs:         make(map[string]string),
			},
			uris: map[string]string{
				string([]byte{0xFF, 0xFE, 0xFD}): "https://example.com",
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidValue,
		},
		{
			name:   "Update URIs with non-UTF8 value",
			sender: owner,
			asset: &kapps.KDAData{
				OwnerAddress: owner,
				URIs:         make(map[string]string),
			},
			uris: map[string]string{
				"website": string([]byte{0xFF, 0xFE, 0xFD}),
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidValue,
		},
	}

	kdaAccHandler := &mock.KAppAccountHandlerStub{
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				SaveKeyValueCalled: func(key []byte, value []byte) error {
					return nil
				},
			}
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &transaction.AssetTriggerContract{
				URIs: tt.uris,
			}
			code, err := kdaKApp.HandleUpdateURIs(tt.sender, tc, kdaAccHandler, assetID, tt.asset)
			assert.Equal(t, tt.expectedCode, code)
			assert.Equal(t, tt.expectedError, err)

			if tt.validate != nil && err == nil {
				tt.validate(t, tt.asset)
			}
		})
	}
}

// Helper function to generate a large URI map for testing
func generateLargeURIMap(size int) map[string]string {
	uris := make(map[string]string)
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("https://example%d.com", i)
		uris[key] = value
	}
	return uris
}
