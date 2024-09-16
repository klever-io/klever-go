package kda_test

import (
	"bytes"
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
) {
	tc := &transaction.CreateAssetContract{
		Type:          assetType,
		Name:          defaultTicker,
		Ticker:        defaultTicker,
		OwnerAddress:  defaultSender,
		Precision:     precision,
		InitialSupply: initialSupply,
		MaxSupply:     maxSupply,
		Properties: &transaction.PropertiesInfo{
			CanFreeze:      true,
			CanWipe:        true,
			CanPause:       true,
			CanMint:        true,
			CanBurn:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
			LimitTransfer:  false,
		},
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

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0)

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

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0)

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

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0)

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

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0)

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

	createDefaultAsset(t, kc, transaction.CreateAssetContract_NonFungible, 0, 0, 0)

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

	createDefaultAsset(t, kc, transaction.CreateAssetContract_NonFungible, 0, 0, 0)

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

	createDefaultAsset(t, kc, transaction.CreateAssetContract_SemiFungible, 0, 0, 0)

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

	createDefaultAsset(t, kc, transaction.CreateAssetContract_SemiFungible, 0, 0, 0)

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

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0)

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

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0)

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
