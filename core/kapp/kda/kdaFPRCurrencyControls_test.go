package kda_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	rewardCurrencyID = []byte("CUR-1A2B")
	rewardIssuer     = makeAddress("issuer")
)

// setFixAuditChangesV6 flips the fork that gates the reward currency controls.
func setFixAuditChangesV6(t *testing.T, kc kapp.KAppController, enabled bool) {
	t.Helper()

	forkController, ok := kc.GetForkController().(*mock.ForkControllerStub)
	require.True(t, ok)
	forkController.SetFork("FixAuditChangesV6", enabled)
}

// registerRewardCurrency writes a fungible currency owned by a third party into the KDA trie and
// credits the default sender with some of it, so the sender is a plain holder of the currency.
func registerRewardCurrency(t *testing.T, kc kapp.KAppController, limitTransfer bool, senderRole *kapps.RolesData) {
	t.Helper()

	currency := &kapps.KDAData{
		ID:           rewardCurrencyID,
		Name:         rewardCurrencyID,
		Ticker:       rewardCurrencyID,
		AssetType:    kapps.KDAData_Fungible,
		OwnerAddress: rewardIssuer,
		Properties:   &kapps.PropertiesData{CanPause: true, LimitTransfer: limitTransfer},
		Attributes:   &kapps.AttributesData{},
	}
	if senderRole != nil {
		senderRole.Address = defaultSender
		currency.Roles = []*kapps.RolesData{senderRole}
	}

	kdaKapp, _, err := kc.GetKDAKApp().GetKDA(nil)
	require.NoError(t, err)
	require.NoError(t, kc.GetKDAKApp().SetKDA(kdaKapp, rewardCurrencyID, currency))
	require.NoError(t, kc.GetKDAKApp().GetAccountsCacher().UpdateKapp(kdaKapp))

	cacher := kc.GetKDAKApp().GetAccountsCacher()
	acc, err := cacher.GetExistingUser(defaultSender)
	require.NoError(t, err)
	require.NoError(t, acc.AddToBalance(1_000, rewardCurrencyID, true, nil))
	require.NoError(t, cacher.UpdateUser(acc))
}

func setRewardCurrencyPaused(t *testing.T, kc kapp.KAppController, paused bool) {
	t.Helper()

	setAssetPaused(t, kc, rewardCurrencyID, paused)
}

func setAssetPaused(t *testing.T, kc kapp.KAppController, assetID []byte, paused bool) {
	t.Helper()

	kdaKapp, asset, err := kc.GetKDAKApp().GetKDA(assetID)
	require.NoError(t, err)

	if asset.Attributes == nil {
		asset.Attributes = &kapps.AttributesData{}
	}
	asset.Attributes.IsPaused = paused
	require.NoError(t, kc.GetKDAKApp().SetKDA(kdaKapp, assetID, asset))
	require.NoError(t, kc.GetKDAKApp().GetAccountsCacher().UpdateKapp(kdaKapp))
}

func senderBalance(t *testing.T, kc kapp.KAppController, assetID []byte) int64 {
	t.Helper()

	acc, err := kc.GetKDAKApp().GetAccountsCacher().GetExistingUser(defaultSender)
	require.NoError(t, err)

	return acc.GetBalance(assetID, true)
}

func depositRewardCurrency(kc kapp.KAppController, amount int64) (transaction.Transaction_TXResultCode, error) {
	return kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
		ID:         defaultAssetID,
		CurrencyID: rewardCurrencyID,
		Amount:     amount,
	})
}

func TestKDAKapp_DepositRespectsTheRewardCurrencyControls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		v6            bool
		paused        bool
		limitTransfer bool
		senderRole    *kapps.RolesData
		expectedCode  transaction.Transaction_TXResultCode
		expectedErr   error
		expectedField string
	}{
		{
			name: "a paused currency is refused", v6: true, paused: true,
			expectedCode: transaction.Transaction_AssetPaused, expectedErr: process.ErrAssetIsPaused,
			expectedField: common.ErrFieldAssetPaused,
		},
		{
			name: "a limit-transfer currency is refused from a holder with no role", v6: true, limitTransfer: true,
			expectedCode: transaction.Transaction_KDATransferNotAllowed, expectedErr: process.ErrKDATransferNotAllowed,
			expectedField: common.ErrFieldTransferNotAllowed,
		},
		{
			name: "a limit-transfer currency is refused from a holder with only the deposit role", v6: true, limitTransfer: true,
			senderRole:   &kapps.RolesData{HasRoleDeposit: true},
			expectedCode: transaction.Transaction_KDATransferNotAllowed, expectedErr: process.ErrKDATransferNotAllowed,
			expectedField: common.ErrFieldTransferNotAllowed,
		},
		{
			name: "a limit-transfer currency is accepted from a holder with the transfer role", v6: true, limitTransfer: true,
			senderRole:   &kapps.RolesData{HasRoleTransfer: true},
			expectedCode: transaction.Transaction_Ok,
		},
		{
			name: "an unrestricted currency is accepted", v6: true,
			expectedCode: transaction.Transaction_Ok,
		},
		{
			name: "a paused currency is still accepted before the fork", paused: true,
			expectedCode: transaction.Transaction_Ok,
		},
		{
			name: "a limit-transfer currency is still accepted before the fork", limitTransfer: true,
			expectedCode: transaction.Transaction_Ok,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			kc, err := createMockControllers()
			require.NoError(t, err)
			setFixAuditChangesV6(t, kc, tt.v6)
			createStakingAsset(t, kc, transaction.StakingInfo_FPRI)
			registerRewardCurrency(t, kc, tt.limitTransfer, tt.senderRole)
			setRewardCurrencyPaused(t, kc, tt.paused)

			code, err := depositRewardCurrency(kc, 400)

			assert.Equal(t, tt.expectedCode, code)

			_, staking, errStaking := kc.GetKDAKApp().GetStaking(defaultAssetID)
			require.NoError(t, errStaking)

			if tt.expectedErr == nil {
				require.NoError(t, err)
				assert.Equal(t, int64(600), senderBalance(t, kc, rewardCurrencyID))
				assert.Equal(t, int64(400), nextEpochFPR(t, staking).KDAS[string(rewardCurrencyID)].GetTotalAmount())
				return
			}

			require.ErrorIs(t, err, tt.expectedErr)
			// nothing left the holder and nothing reached the pool
			assert.Equal(t, int64(1_000), senderBalance(t, kc, rewardCurrencyID))
			for _, fpr := range staking.FPR {
				assert.NotContains(t, fpr.KDAS, string(rewardCurrencyID))
			}

			receipts := kc.GetCurrentKAppContext().Receipts().GetByType(int8(txProcess.Error))
			require.Len(t, receipts, 1)
			assert.Equal(t, tt.expectedField, string(receipts[0].Data[1]))
		})
	}
}

// registerCollectionCurrency writes an NFT or SFT collection into the KDA trie. Its units live
// under nonce keys, so the bare collection key a currency debit reads holds no balance.
func registerCollectionCurrency(t *testing.T, kc kapp.KAppController, id []byte, assetType kapps.KDAData_EnumAssetType) {
	t.Helper()

	kdaKapp, _, err := kc.GetKDAKApp().GetKDA(nil)
	require.NoError(t, err)
	require.NoError(t, kc.GetKDAKApp().SetKDA(kdaKapp, id, &kapps.KDAData{
		ID:           id,
		Name:         id,
		Ticker:       id,
		AssetType:    assetType,
		OwnerAddress: rewardIssuer,
		Properties:   &kapps.PropertiesData{},
		Attributes:   &kapps.AttributesData{},
	}))
	require.NoError(t, kc.GetKDAKApp().GetAccountsCacher().UpdateKapp(kdaKapp))
}

// From FixAuditChangesV7 an FPRI deposit takes only KLV or a fungible currency (KLC-2876). Before
// it an NFT or SFT collection passes the existence check and fails on the empty bare-key balance.
func TestKDAKapp_DepositRefusesANonFungibleCurrency(t *testing.T) {
	t.Parallel()

	collectionID := []byte("COLL-1A2B")
	tests := []struct {
		name         string
		v7           bool
		currencyType kapps.KDAData_EnumAssetType
		expectedCode transaction.Transaction_TXResultCode
		expectedErr  error
	}{
		{
			name: "an NFT collection is refused from V7", v7: true, currencyType: kapps.KDAData_NonFungible,
			expectedCode: transaction.Transaction_AssetTypeInvalid, expectedErr: common.ErrAssetTypeInvalid,
		},
		{
			name: "an SFT collection is refused from V7", v7: true, currencyType: kapps.KDAData_SemiFungible,
			expectedCode: transaction.Transaction_AssetTypeInvalid, expectedErr: common.ErrAssetTypeInvalid,
		},
		{
			name: "an NFT collection fails on the balance before V7", currencyType: kapps.KDAData_NonFungible,
			expectedCode: transaction.Transaction_BalanceError,
		},
		{
			name: "an SFT collection fails on the balance before V7", currencyType: kapps.KDAData_SemiFungible,
			expectedCode: transaction.Transaction_BalanceError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			kc, err := createMockControllersWithFork(mock.NewForkControllerStub().
				SetFork("FixAuditChangesV6", true).
				SetFork("FixAuditChangesV7", tt.v7))
			require.NoError(t, err)
			createStakingAsset(t, kc, transaction.StakingInfo_FPRI)
			registerCollectionCurrency(t, kc, collectionID, tt.currencyType)

			code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
				ID:         defaultAssetID,
				CurrencyID: collectionID,
				Amount:     10,
			})

			require.Error(t, err)
			assert.Equal(t, tt.expectedCode, code)
			for _, fpr := range fprsOf(t, kc) {
				assert.NotContains(t, fpr.KDAS, string(collectionID))
			}
			if tt.expectedErr == nil {
				return
			}

			require.ErrorIs(t, err, tt.expectedErr)
			receipts := kc.GetCurrentKAppContext().Receipts().GetByType(int8(txProcess.Error))
			require.Len(t, receipts, 1)
			assert.Equal(t, common.ErrFieldInvalidAssetType, string(receipts[0].Data[1]))
		})
	}

	t.Run("a fungible currency is still accepted from V7", func(t *testing.T) {
		t.Parallel()

		kc, err := createMockControllersWithFork(mock.NewForkControllerStub().
			SetFork("FixAuditChangesV6", true).
			SetFork("FixAuditChangesV7", true))
		require.NoError(t, err)
		createStakingAsset(t, kc, transaction.StakingInfo_FPRI)
		registerRewardCurrency(t, kc, false, nil)

		code, err := depositRewardCurrency(kc, 400)

		require.NoError(t, err)
		assert.Equal(t, transaction.Transaction_Ok, code)
		assert.Equal(t, int64(600), senderBalance(t, kc, rewardCurrencyID))
	})
}

func fprsOf(t *testing.T, kc kapp.KAppController) []*kapps.FPRData {
	t.Helper()

	_, staking, err := kc.GetKDAKApp().GetStaking(defaultAssetID)
	require.NoError(t, err)

	return staking.FPR
}

// expiredFPRWithRewardCurrency is an FPR from epoch 0 holding unclaimed KLV, unclaimed staking
// asset and 300 of the reward currency, all deposited before any pause.
func expiredFPRWithRewardCurrency() []*kapps.FPRData {
	return []*kapps.FPRData{{
		Epoch:        0,
		TotalAmount:  100,
		TotalClaimed: 40,
		KDAS: map[string]*kapps.KDAFPRData{
			string(defaultAssetID):   {TotalAmount: 30, TotalClaimed: 10},
			string(rewardCurrencyID): {TotalAmount: 300, TotalClaimed: 50},
		},
	}}
}

func depositStakingAsset(t *testing.T, kc kapp.KAppController, amount int64) {
	t.Helper()

	code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
		ID:         defaultAssetID,
		CurrencyID: defaultAssetID,
		Amount:     amount,
	})
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, code)
}

// A paused asset takes no deposit into its own pool, whatever the currency deposited.
func TestKDAKapp_DepositIsRefusedWhileThePoolAssetIsPaused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		v6         bool
		currencyID []byte
		refused    bool
	}{
		{name: "the pool asset itself is refused", v6: true, currencyID: defaultAssetID, refused: true},
		{name: "another currency is refused", v6: true, currencyID: rewardCurrencyID, refused: true},
		{name: "the pool asset itself is still accepted before the fork", currencyID: defaultAssetID},
		{name: "another currency is still accepted before the fork", currencyID: rewardCurrencyID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			kc, err := createMockControllers()
			require.NoError(t, err)
			setFixAuditChangesV6(t, kc, tt.v6)
			createStakingAsset(t, kc, transaction.StakingInfo_FPRI)
			registerRewardCurrency(t, kc, false, nil)
			depositStakingAsset(t, kc, 10)
			setAssetPaused(t, kc, defaultAssetID, true)

			balanceBefore := senderBalance(t, kc, tt.currencyID)

			code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
				ID:         defaultAssetID,
				CurrencyID: tt.currencyID,
				Amount:     10,
			})

			if !tt.refused {
				require.NoError(t, err)
				assert.Equal(t, transaction.Transaction_Ok, code)
				assert.Equal(t, balanceBefore-10, senderBalance(t, kc, tt.currencyID))
				return
			}

			require.ErrorIs(t, err, process.ErrAssetIsPaused)
			assert.Equal(t, transaction.Transaction_AssetPaused, code)
			// nothing left the depositor and the pool kept only what it had before the pause
			assert.Equal(t, balanceBefore, senderBalance(t, kc, tt.currencyID))
			assert.Equal(t, int64(10), fprByEpoch(t, kc, 1).KDAS[string(defaultAssetID)].GetTotalAmount())
			assert.NotContains(t, fprByEpoch(t, kc, 1).KDAS, string(rewardCurrencyID))

			receipts := kc.GetCurrentKAppContext().Receipts().GetByType(int8(txProcess.Error))
			require.Len(t, receipts, 1)
			assert.Equal(t, common.ErrFieldAssetPaused, string(receipts[0].Data[1]))

			// the unpause opens the pool again
			setAssetPaused(t, kc, defaultAssetID, false)
			code, err = kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
				ID:         defaultAssetID,
				CurrencyID: tt.currencyID,
				Amount:     10,
			})
			require.NoError(t, err)
			assert.Equal(t, transaction.Transaction_Ok, code)
		})
	}
}

// forkCases runs a scenario once before and once after FixAuditChangesV6.
func forkCases(t *testing.T, run func(t *testing.T, kc kapp.KAppController, v6 bool)) {
	t.Helper()

	for _, tt := range []struct {
		name string
		v6   bool
	}{
		{name: "pre fork", v6: false},
		{name: "post fork", v6: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			kc, err := createMockControllers()
			require.NoError(t, err)
			setFixAuditChangesV6(t, kc, tt.v6)

			run(t, kc, tt.v6)
		})
	}
}

// Scenario 2, deposit side: a paused reward currency does not hold the pool of another asset, so its
// expired residual is still paid on both sides of the fork. The depositor here is the asset's owner.
func TestKDAKapp_DepositPaysTheExpiredResidualOfAPausedRewardCurrency(t *testing.T) {
	t.Parallel()

	forkCases(t, func(t *testing.T, kc kapp.KAppController, _ bool) {
		createStakingAsset(t, kc, transaction.StakingInfo_FPRI)
		registerRewardCurrency(t, kc, false, nil)
		overwriteFPR(t, kc, expiredFPRWithRewardCurrency())
		setRewardCurrencyPaused(t, kc, true)

		klvBefore := senderBalance(t, kc, nil)

		// MaxEpochsUnclaimed defaults to 100, so epoch 200 leaves the epoch 0 entry expired
		setBlockEpoch(kc, 200)
		depositStakingAsset(t, kc, 10)

		assert.Equal(t, klvBefore+60, senderBalance(t, kc, nil))
		assert.Equal(t, int64(1_250), senderBalance(t, kc, rewardCurrencyID))

		_, staking, err := kc.GetKDAKApp().GetStaking(defaultAssetID)
		require.NoError(t, err)
		require.Len(t, staking.FPR, 1)
		assert.Equal(t, uint32(201), staking.FPR[0].GetEpoch())
	})
}

// setStakingAssetOwner hands the staking asset to another owner and gives the deposit role to the
// depositor, who holds 10 of the asset and nothing else.
func setStakingAssetOwner(t *testing.T, kc kapp.KAppController, owner, depositor []byte) {
	t.Helper()

	cacher := kc.GetKDAKApp().GetAccountsCacher()

	kdaKapp, asset, err := kc.GetKDAKApp().GetKDA(defaultAssetID)
	require.NoError(t, err)
	asset.OwnerAddress = owner
	asset.Roles = append(asset.Roles, &kapps.RolesData{Address: depositor, HasRoleDeposit: true})
	require.NoError(t, kc.GetKDAKApp().SetKDA(kdaKapp, defaultAssetID, asset))
	require.NoError(t, cacher.UpdateKapp(kdaKapp))

	acc, err := cacher.LoadUser(depositor)
	require.NoError(t, err)
	require.NoError(t, acc.AddToBalance(10, defaultAssetID, true, nil))
	require.NoError(t, cacher.UpdateUser(acc))
}

// balanceOf is the balance of an account that may not exist yet.
func balanceOf(t *testing.T, kc kapp.KAppController, address, assetID []byte) int64 {
	t.Helper()

	acc, err := kc.GetKDAKApp().GetAccountsCacher().GetExistingUser(address)
	if err != nil {
		return 0
	}

	return acc.GetBalance(assetID, true)
}

// After the fork the expired residual of every currency is paid to the staking asset's owner,
// whatever account that is; before the fork it is paid to whoever deposits next.
func TestKDAKapp_DepositPaysTheExpiredResidualToTheStakingAssetOwner(t *testing.T) {
	t.Parallel()

	contractOwner := makeAddress("")
	copy(contractOwner[core.NumInitCharactersForScAddress-core.VMTypeLen:], common.WasmVirtualMachine)
	copy(contractOwner[core.NumInitCharactersForScAddress:], "contract-owner")
	require.True(t, core.IsSmartContractAddress(contractOwner))

	tests := []struct {
		name  string
		owner []byte
	}{
		{name: "an owner with an account", owner: defaultSender},
		{name: "an owner with no account yet", owner: makeAddress("ghost-owner")},
		{name: "an owner in the smart contract address format", owner: contractOwner},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			forkCases(t, func(t *testing.T, kc kapp.KAppController, v6 bool) {
				depositor := defaultOtherSender

				createStakingAsset(t, kc, transaction.StakingInfo_FPRI)
				registerRewardCurrency(t, kc, false, nil)
				setStakingAssetOwner(t, kc, tt.owner, depositor)
				overwriteFPR(t, kc, expiredFPRWithRewardCurrency())

				currencies := [][]byte{nil, defaultAssetID, rewardCurrencyID}
				residual := []int64{60, 20, 250}
				ownerBefore := make([]int64, len(currencies))
				depositorBefore := make([]int64, len(currencies))
				for i, currency := range currencies {
					ownerBefore[i] = balanceOf(t, kc, tt.owner, currency)
					depositorBefore[i] = balanceOf(t, kc, depositor, currency)
				}

				// MaxEpochsUnclaimed defaults to 100, so epoch 200 leaves the epoch 0 entry expired
				setBlockEpoch(kc, 200)
				code, err := kc.GetKDAKApp().Deposit(depositor, &transaction.DepositContract{
					ID:         defaultAssetID,
					CurrencyID: defaultAssetID,
					Amount:     10,
				})
				require.NoError(t, err)
				require.Equal(t, transaction.Transaction_Ok, code)

				paid, notPaid := depositor, tt.owner
				if v6 {
					paid, notPaid = tt.owner, depositor
				}

				// the depositor spent the 10 of the staking asset it deposited
				depositorBefore[1] -= 10
				for i, currency := range currencies {
					ownerAfter, depositorAfter := ownerBefore[i], depositorBefore[i]
					if v6 {
						ownerAfter += residual[i]
					} else {
						depositorAfter += residual[i]
					}
					assert.Equal(t, ownerAfter, balanceOf(t, kc, tt.owner, currency))
					assert.Equal(t, depositorAfter, balanceOf(t, kc, depositor, currency))
				}

				receipts := kc.GetCurrentKAppContext().Receipts().GetByType(int8(txProcess.Transfer))
				require.Len(t, receipts, 3)
				for _, receipt := range receipts {
					assert.Equal(t, paid, receipt.Data[2])
					assert.NotEqual(t, notPaid, receipt.Data[2])
				}
				// after the fork the KDA receipts follow the sorted currency IDs, not map order
				if v6 {
					wantOrder := [][]byte{kdautils.KLVIdentifier, rewardCurrencyID, defaultAssetID}
					for i, receipt := range receipts {
						assert.Equal(t, wantOrder[i], receipt.Data[4])
					}
				}

				_, staking, err := kc.GetKDAKApp().GetStaking(defaultAssetID)
				require.NoError(t, err)
				require.Len(t, staking.FPR, 1)
				assert.Equal(t, uint32(201), staking.FPR[0].GetEpoch())
			})
		})
	}
}

// An entry with nothing left to pay credits the owner nothing and records no Transfer receipt. The
// owner's account is resolved up front once FixAuditChangesV6 is active, so it is loaded there even
// though nothing is paid; before the fork it is never touched.
func TestKDAKapp_DepositWithNoResidualPaysTheOwnerNothing(t *testing.T) {
	t.Parallel()

	forkCases(t, func(t *testing.T, kc kapp.KAppController, v6 bool) {
		ghostOwner := makeAddress("ghost-owner")

		createStakingAsset(t, kc, transaction.StakingInfo_FPRI)
		setStakingAssetOwner(t, kc, ghostOwner, defaultOtherSender)
		overwriteFPR(t, kc, []*kapps.FPRData{{
			Epoch:        0,
			TotalAmount:  100,
			TotalClaimed: 100,
			KDAS:         map[string]*kapps.KDAFPRData{string(defaultAssetID): {TotalAmount: 30, TotalClaimed: 30}},
		}})

		setBlockEpoch(kc, 200)
		code, err := kc.GetKDAKApp().Deposit(defaultOtherSender, &transaction.DepositContract{
			ID:         defaultAssetID,
			CurrencyID: defaultAssetID,
			Amount:     10,
		})
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)

		ownerAcc, err := kc.GetKDAKApp().GetAccountsCacher().GetExistingUser(ghostOwner)
		if v6 {
			require.NoError(t, err)
			assert.Zero(t, ownerAcc.GetBalance(defaultAssetID, true))
		} else {
			require.Error(t, err)
		}
		assert.Empty(t, kc.GetCurrentKAppContext().Receipts().GetByType(int8(txProcess.Transfer)))
	})
}

// stakeAndFundFPRs has the default sender stake 100 of the staking asset at epoch 0, then funds
// three reward epochs for that stake: epoch 1 pays 10 of the staking asset, epoch 2 pays 20 of it
// plus 300 of the reward currency, and epoch 3 pays 40 of it.
func stakeAndFundFPRs(t *testing.T, kc kapp.KAppController) {
	t.Helper()

	freezeStakingAsset(t, kc, 100)

	overwriteFPR(t, kc, []*kapps.FPRData{
		{Epoch: 1, TotalStaked: 100, KDAS: map[string]*kapps.KDAFPRData{
			string(defaultAssetID): {TotalAmount: 10},
		}},
		{Epoch: 2, TotalStaked: 100, KDAS: map[string]*kapps.KDAFPRData{
			string(defaultAssetID):   {TotalAmount: 20},
			string(rewardCurrencyID): {TotalAmount: 300},
		}},
		{Epoch: 3, TotalStaked: 100, KDAS: map[string]*kapps.KDAFPRData{
			string(defaultAssetID): {TotalAmount: 40},
		}},
	})
}

func freezeStakingAsset(t *testing.T, kc kapp.KAppController, amount int64) {
	t.Helper()

	code, err := kc.GetAccountsKApp().Freeze(defaultSender, &transaction.FreezeContract{
		AssetID: defaultAssetID,
		Amount:  amount,
	})
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, code)
}

func claimStakingAsset(kc kapp.KAppController) (transaction.Transaction_TXResultCode, error) {
	return kc.GetAccountsKApp().ClaimStaking(defaultSender, &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim,
		ID:        defaultAssetID,
	})
}

func lastClaimEpoch(t *testing.T, kc kapp.KAppController) uint32 {
	t.Helper()

	acc, err := kc.GetKDAKApp().GetAccountsCacher().GetExistingUser(defaultSender)
	require.NoError(t, err)
	userKDA, err := acc.GetUserKDA(defaultAssetID, nil, true)
	require.NoError(t, err)

	return userKDA.GetLastClaim().GetEpoch()
}

func fprByEpoch(t *testing.T, kc kapp.KAppController, epoch uint32) *kapps.FPRData {
	t.Helper()

	_, staking, err := kc.GetKDAKApp().GetStaking(defaultAssetID)
	require.NoError(t, err)

	for _, fpr := range staking.FPR {
		if fpr.GetEpoch() == epoch {
			return fpr
		}
	}

	t.Fatalf("no FPR entry for epoch %d in %v", epoch, staking.FPR)
	return nil
}

// Scenario 1, claim side: after the fork a paused asset pays no claim from its own FPR pool and the
// rewards wait for the unpause; before the fork the claim is paid during the pause.
func TestAccountsKapp_ClaimOfAPausedPoolAsset(t *testing.T) {
	t.Parallel()

	forkCases(t, func(t *testing.T, kc kapp.KAppController, v6 bool) {
		createStakingAsset(t, kc, transaction.StakingInfo_FPRI)
		registerRewardCurrency(t, kc, false, nil)
		stakeAndFundFPRs(t, kc)
		setAssetPaused(t, kc, defaultAssetID, true)

		assetBefore := senderBalance(t, kc, defaultAssetID)
		cursorBefore := lastClaimEpoch(t, kc)

		setBlockEpoch(kc, 5)
		code, err := claimStakingAsset(kc)

		if !v6 {
			require.NoError(t, err)
			assert.Equal(t, transaction.Transaction_Ok, code)
			assert.Equal(t, assetBefore+70, senderBalance(t, kc, defaultAssetID))
			assert.Equal(t, int64(1_300), senderBalance(t, kc, rewardCurrencyID))
			assert.Equal(t, uint32(5), lastClaimEpoch(t, kc))
			return
		}

		require.ErrorIs(t, err, process.ErrAssetIsPaused)
		assert.Equal(t, transaction.Transaction_AssetPaused, code)

		// nothing was paid and the claim cursor did not move
		assert.Equal(t, assetBefore, senderBalance(t, kc, defaultAssetID))
		assert.Equal(t, int64(1_000), senderBalance(t, kc, rewardCurrencyID))
		assert.Equal(t, cursorBefore, lastClaimEpoch(t, kc))
		for _, epoch := range []uint32{1, 2, 3} {
			assert.Zero(t, fprByEpoch(t, kc, epoch).KDAS[string(defaultAssetID)].TotalClaimed)
		}
		assert.Zero(t, fprByEpoch(t, kc, 2).KDAS[string(rewardCurrencyID)].TotalClaimed)

		receipts := kc.GetCurrentKAppContext().Receipts().GetByType(int8(txProcess.Error))
		require.Len(t, receipts, 1)
		assert.Equal(t, common.ErrFieldAssetPaused, string(receipts[0].Data[1]))

		// after the unpause everything that waited is paid, once
		setAssetPaused(t, kc, defaultAssetID, false)

		code, err = claimStakingAsset(kc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
		assert.Equal(t, assetBefore+70, senderBalance(t, kc, defaultAssetID))
		assert.Equal(t, int64(1_300), senderBalance(t, kc, rewardCurrencyID))
		assert.Equal(t, uint32(5), lastClaimEpoch(t, kc))

		code, err = claimStakingAsset(kc)
		require.Error(t, err)
		assert.Equal(t, transaction.Transaction_ClaimError, code)
		assert.Equal(t, assetBefore+70, senderBalance(t, kc, defaultAssetID))
	})
}

// Scenario 2, claim side: a paused reward currency does not stop the claim of another asset's pool.
// The claim pays every currency, the paused one included, on both sides of the fork.
func TestAccountsKapp_ClaimPaysAPausedRewardCurrencyOfAnotherAssetsPool(t *testing.T) {
	t.Parallel()

	forkCases(t, func(t *testing.T, kc kapp.KAppController, _ bool) {
		createStakingAsset(t, kc, transaction.StakingInfo_FPRI)
		registerRewardCurrency(t, kc, false, nil)
		stakeAndFundFPRs(t, kc)
		setRewardCurrencyPaused(t, kc, true)

		assetBefore := senderBalance(t, kc, defaultAssetID)

		setBlockEpoch(kc, 5)
		code, err := claimStakingAsset(kc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)

		assert.Equal(t, assetBefore+70, senderBalance(t, kc, defaultAssetID))
		assert.Equal(t, int64(1_300), senderBalance(t, kc, rewardCurrencyID))
		assert.Equal(t, uint32(5), lastClaimEpoch(t, kc))
		assert.Equal(t, int64(300), fprByEpoch(t, kc, 2).KDAS[string(rewardCurrencyID)].TotalClaimed)

		// the paid epochs are not paid twice
		code, err = claimStakingAsset(kc)
		require.Error(t, err)
		assert.Equal(t, transaction.Transaction_ClaimError, code)
		assert.Equal(t, int64(1_300), senderBalance(t, kc, rewardCurrencyID))

		// the paid amount stays bound by the pause in the staker's account
		code, err = kc.GetAccountsKApp().Transfer(transaction.TXContract_TransferContractType, defaultSender, &transaction.TransferContract{
			ToAddress: rewardIssuer,
			AssetID:   rewardCurrencyID,
			Amount:    300,
		})
		require.ErrorIs(t, err, process.ErrAssetIsPaused)
		assert.Equal(t, transaction.Transaction_AssetPaused, code)
		assert.Equal(t, int64(1_300), senderBalance(t, kc, rewardCurrencyID))
	})
}

func unfreezeStakingAsset(t *testing.T, kc kapp.KAppController) (transaction.Transaction_TXResultCode, error) {
	t.Helper()

	acc, err := kc.GetKDAKApp().GetAccountsCacher().GetExistingUser(defaultSender)
	require.NoError(t, err)
	userKDA, err := acc.GetUserKDA(defaultAssetID, nil, true)
	require.NoError(t, err)
	require.Len(t, userKDA.GetBuckets(), 1)

	var bucketID string
	for id := range userKDA.GetBuckets() {
		bucketID = id
	}

	return kc.GetAccountsKApp().Unfreeze(defaultSender, &transaction.UnfreezeContract{
		AssetID:  defaultAssetID,
		BucketID: []byte(bucketID),
	})
}

func withdrawStakingAsset(kc kapp.KAppController) (transaction.Transaction_TXResultCode, error) {
	return kc.GetAccountsKApp().Withdraw(defaultSender, &transaction.WithdrawContract{AssetID: defaultAssetID})
}

func frozenBalance(t *testing.T, kc kapp.KAppController) int64 {
	t.Helper()

	acc, err := kc.GetKDAKApp().GetAccountsCacher().GetExistingUser(defaultSender)
	require.NoError(t, err)
	userKDA, err := acc.GetUserKDA(defaultAssetID, nil, true)
	require.NoError(t, err)

	return userKDA.GetFrozenBalance()
}

// Scenario 1, staking side: Freeze, Unfreeze and Withdraw all run the claim, so after the fork they
// are refused on a paused pool asset as well and pay nothing. Before the fork they run and pay.
func TestAccountsKapp_StakingOperationsOfAPausedPoolAsset(t *testing.T) {
	t.Parallel()

	operations := []struct {
		name string
		run  func(t *testing.T, kc kapp.KAppController) (transaction.Transaction_TXResultCode, error)
	}{
		{name: "freeze", run: func(_ *testing.T, kc kapp.KAppController) (transaction.Transaction_TXResultCode, error) {
			return kc.GetAccountsKApp().Freeze(defaultSender, &transaction.FreezeContract{AssetID: defaultAssetID, Amount: 1})
		}},
		{name: "unfreeze", run: unfreezeStakingAsset},
	}
	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) {
			t.Parallel()

			forkCases(t, func(t *testing.T, kc kapp.KAppController, v6 bool) {
				createStakingAsset(t, kc, transaction.StakingInfo_FPRI)
				registerRewardCurrency(t, kc, false, nil)
				stakeAndFundFPRs(t, kc)
				setAssetPaused(t, kc, defaultAssetID, true)

				assetBefore := senderBalance(t, kc, defaultAssetID)
				frozenBefore := frozenBalance(t, kc)
				cursorBefore := lastClaimEpoch(t, kc)

				setBlockEpoch(kc, 5)
				code, err := op.run(t, kc)

				if !v6 {
					require.NoError(t, err)
					assert.Equal(t, transaction.Transaction_Ok, code)
					// the claim the operation carries paid every reward epoch
					assert.Equal(t, int64(1_300), senderBalance(t, kc, rewardCurrencyID))
					assert.Equal(t, uint32(5), lastClaimEpoch(t, kc))
					return
				}

				require.ErrorIs(t, err, process.ErrAssetIsPaused)
				assert.Equal(t, transaction.Transaction_AssetPaused, code)
				assert.Equal(t, assetBefore, senderBalance(t, kc, defaultAssetID))
				assert.Equal(t, frozenBefore, frozenBalance(t, kc))
				assert.Equal(t, int64(1_000), senderBalance(t, kc, rewardCurrencyID))
				assert.Equal(t, cursorBefore, lastClaimEpoch(t, kc))

				// the unpause lets the operation through, and its claim pays what waited
				setAssetPaused(t, kc, defaultAssetID, false)
				code, err = op.run(t, kc)
				require.NoError(t, err)
				assert.Equal(t, transaction.Transaction_Ok, code)
				assert.Equal(t, int64(1_300), senderBalance(t, kc, rewardCurrencyID))
			})
		})
	}

	t.Run("withdraw", func(t *testing.T) {
		t.Parallel()

		forkCases(t, func(t *testing.T, kc kapp.KAppController, v6 bool) {
			createStakingAsset(t, kc, transaction.StakingInfo_FPRI)
			registerRewardCurrency(t, kc, false, nil)
			stakeAndFundFPRs(t, kc)

			// the stake is unfrozen before the pause, so only the withdraw is left to do
			setBlockEpoch(kc, 5)
			code, err := unfreezeStakingAsset(t, kc)
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, code)
			setAssetPaused(t, kc, defaultAssetID, true)

			assetBefore := senderBalance(t, kc, defaultAssetID)

			setBlockEpoch(kc, 10)
			code, err = withdrawStakingAsset(kc)

			if !v6 {
				require.NoError(t, err)
				assert.Equal(t, transaction.Transaction_Ok, code)
				assert.Equal(t, assetBefore+100, senderBalance(t, kc, defaultAssetID))
				return
			}

			require.ErrorIs(t, err, process.ErrAssetIsPaused)
			assert.Equal(t, transaction.Transaction_AssetPaused, code)
			assert.Equal(t, assetBefore, senderBalance(t, kc, defaultAssetID))

			setAssetPaused(t, kc, defaultAssetID, false)
			code, err = withdrawStakingAsset(kc)
			require.NoError(t, err)
			assert.Equal(t, transaction.Transaction_Ok, code)
			assert.Equal(t, assetBefore+100, senderBalance(t, kc, defaultAssetID))
		})
	})
}

// Scenario 2, staking side: a paused reward currency never blocks the principal. Freeze, Unfreeze and
// Withdraw of the staked asset keep working on both sides of the fork, and the claim they carry pays
// the paused currency.
func TestAccountsKapp_StakingOperationsWorkWhileARewardCurrencyIsPaused(t *testing.T) {
	t.Parallel()

	forkCases(t, func(t *testing.T, kc kapp.KAppController, _ bool) {
		createStakingAsset(t, kc, transaction.StakingInfo_FPRI)
		registerRewardCurrency(t, kc, false, nil)
		stakeAndFundFPRs(t, kc)
		setRewardCurrencyPaused(t, kc, true)

		assetBefore := senderBalance(t, kc, defaultAssetID)

		setBlockEpoch(kc, 5)
		code, err := kc.GetAccountsKApp().Freeze(defaultSender, &transaction.FreezeContract{AssetID: defaultAssetID, Amount: 1})
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
		assert.Equal(t, assetBefore+70-1, senderBalance(t, kc, defaultAssetID))
		assert.Equal(t, int64(1_300), senderBalance(t, kc, rewardCurrencyID))
		assert.Equal(t, uint32(5), lastClaimEpoch(t, kc))

		setBlockEpoch(kc, 10)
		code, err = unfreezeStakingAsset(t, kc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)

		setBlockEpoch(kc, 15)
		code, err = withdrawStakingAsset(kc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
		assert.Equal(t, assetBefore+70+100, senderBalance(t, kc, defaultAssetID))
		assert.Zero(t, frozenBalance(t, kc))
		assert.Equal(t, int64(1_300), senderBalance(t, kc, rewardCurrencyID))
	})
}

const (
	aprStakeTime = int64(1_000)
	secondsAYear = int64(365 * 24 * 60 * 60)
)

// setBlockEpochAndTime is setBlockEpoch with a block timestamp, which an APR claim accrues on.
func setBlockEpochAndTime(kc kapp.KAppController, epoch uint32, timestamp int64) {
	kc.SetCurrentKAppContext(kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: defaultSender,
		ContractID:     0,
		Block:          &block.Block{Header: &block.BlockHeader{Epoch: epoch, Timestamp: timestamp}},
	}))
}

// stakeInAPRPool creates the staking asset with an APR pool and has the default sender stake 100 of
// it, so that a year later the stake has accrued a gain to claim.
func stakeInAPRPool(t *testing.T, kc kapp.KAppController) {
	t.Helper()

	setBlockEpochAndTime(kc, 0, aprStakeTime)
	createStakingAsset(t, kc, transaction.StakingInfo_APRI)
	freezeStakingAsset(t, kc, 100)
}

// The pause of the staked asset closes an APR pool the same way it closes an FPR pool: after the
// fork the claim is refused and the accrued gain waits for the unpause; before the fork it is paid.
func TestAccountsKapp_ClaimOfAPausedAPRStakingAsset(t *testing.T) {
	t.Parallel()

	forkCases(t, func(t *testing.T, kc kapp.KAppController, v6 bool) {
		stakeInAPRPool(t, kc)
		setAssetPaused(t, kc, defaultAssetID, true)

		assetBefore := senderBalance(t, kc, defaultAssetID)

		setBlockEpochAndTime(kc, 5, aprStakeTime+secondsAYear)
		code, err := claimStakingAsset(kc)

		if !v6 {
			require.NoError(t, err)
			assert.Equal(t, transaction.Transaction_Ok, code)
			assert.Greater(t, senderBalance(t, kc, defaultAssetID), assetBefore)
			return
		}

		require.ErrorIs(t, err, process.ErrAssetIsPaused)
		assert.Equal(t, transaction.Transaction_AssetPaused, code)
		assert.Equal(t, assetBefore, senderBalance(t, kc, defaultAssetID))

		receipts := kc.GetCurrentKAppContext().Receipts().GetByType(int8(txProcess.Error))
		require.Len(t, receipts, 1)
		assert.Equal(t, common.ErrFieldAssetPaused, string(receipts[0].Data[1]))

		// APR gains do not expire, so the unpause pays everything that accrued during the pause
		setAssetPaused(t, kc, defaultAssetID, false)

		code, err = claimStakingAsset(kc)
		require.NoError(t, err)
		assert.Equal(t, transaction.Transaction_Ok, code)
		assert.Greater(t, senderBalance(t, kc, defaultAssetID), assetBefore)
	})
}

// Freeze, Unfreeze and Withdraw on a paused asset staked under APR: refused after the fork with no
// balance moved, unchanged before it.
func TestAccountsKapp_StakingOperationsOfAPausedAPRStakingAsset(t *testing.T) {
	t.Parallel()

	operations := []struct {
		name string
		run  func(t *testing.T, kc kapp.KAppController) (transaction.Transaction_TXResultCode, error)
	}{
		{name: "freeze", run: func(_ *testing.T, kc kapp.KAppController) (transaction.Transaction_TXResultCode, error) {
			return kc.GetAccountsKApp().Freeze(defaultSender, &transaction.FreezeContract{AssetID: defaultAssetID, Amount: 1})
		}},
		{name: "unfreeze", run: unfreezeStakingAsset},
	}
	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) {
			t.Parallel()

			forkCases(t, func(t *testing.T, kc kapp.KAppController, v6 bool) {
				stakeInAPRPool(t, kc)
				setAssetPaused(t, kc, defaultAssetID, true)

				assetBefore := senderBalance(t, kc, defaultAssetID)
				frozenBefore := frozenBalance(t, kc)

				setBlockEpochAndTime(kc, 5, aprStakeTime+secondsAYear)
				code, err := op.run(t, kc)

				if !v6 {
					require.NoError(t, err)
					assert.Equal(t, transaction.Transaction_Ok, code)
					assert.NotEqual(t, frozenBefore, frozenBalance(t, kc))
					return
				}

				require.ErrorIs(t, err, process.ErrAssetIsPaused)
				assert.Equal(t, transaction.Transaction_AssetPaused, code)
				assert.Equal(t, assetBefore, senderBalance(t, kc, defaultAssetID))
				assert.Equal(t, frozenBefore, frozenBalance(t, kc))

				// the unpause lets the operation through
				setAssetPaused(t, kc, defaultAssetID, false)
				code, err = op.run(t, kc)
				require.NoError(t, err)
				assert.Equal(t, transaction.Transaction_Ok, code)
				assert.NotEqual(t, frozenBefore, frozenBalance(t, kc))
			})
		})
	}

	t.Run("withdraw", func(t *testing.T) {
		t.Parallel()

		forkCases(t, func(t *testing.T, kc kapp.KAppController, v6 bool) {
			stakeInAPRPool(t, kc)

			// the stake is unfrozen before the pause, so only the withdraw is left to do
			setBlockEpochAndTime(kc, 5, aprStakeTime+secondsAYear)
			code, err := unfreezeStakingAsset(t, kc)
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, code)
			setAssetPaused(t, kc, defaultAssetID, true)

			assetBefore := senderBalance(t, kc, defaultAssetID)

			setBlockEpochAndTime(kc, 10, aprStakeTime+2*secondsAYear)
			code, err = withdrawStakingAsset(kc)

			if !v6 {
				require.NoError(t, err)
				assert.Equal(t, transaction.Transaction_Ok, code)
				assert.Equal(t, assetBefore+100, senderBalance(t, kc, defaultAssetID))
				return
			}

			require.ErrorIs(t, err, process.ErrAssetIsPaused)
			assert.Equal(t, transaction.Transaction_AssetPaused, code)
			assert.Equal(t, assetBefore, senderBalance(t, kc, defaultAssetID))

			setAssetPaused(t, kc, defaultAssetID, false)
			code, err = withdrawStakingAsset(kc)
			require.NoError(t, err)
			assert.Equal(t, transaction.Transaction_Ok, code)
			assert.Equal(t, assetBefore+100, senderBalance(t, kc, defaultAssetID))
		})
	})
}
