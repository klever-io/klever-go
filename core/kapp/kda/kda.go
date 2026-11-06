package kda

import (
	"bytes"
	"errors"
	"strconv"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/types"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ kapp.KDAKapp = (*kdaKapp)(nil)

type kdaKapp struct {
	hasher         hashing.Hasher
	marshalizer    marshal.Marshalizer
	pubkeyConv     core.PubkeyConverter
	accountsCacher state.AccountsCacher
	forkController core.ForkController
	addressLen     int
	KAppController kapp.KAppController
}

// ArgsNewKdaKApp holds the arguments needed to create a KdaKapp
type ArgsNewKDAKApp struct {
	Hasher         hashing.Hasher
	Marshalizer    marshal.Marshalizer
	PubkeyConv     core.PubkeyConverter
	ForkController core.ForkController
}

// NewKDAKApp creates a kda KApp
func NewKDAKApp(
	args *ArgsNewKDAKApp,
) (*kdaKapp, error) {
	if check.IfNil(args.Hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.PubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}

	v := &kdaKapp{
		hasher:         args.Hasher,
		marshalizer:    args.Marshalizer,
		addressLen:     args.PubkeyConv.Len(),
		pubkeyConv:     args.PubkeyConv,
		forkController: args.ForkController,
	}

	return v, nil
}

// IsInterfaceNil verifies if the underlying object is nil or not
func (k *kdaKapp) IsInterfaceNil() bool {
	return k == nil
}

func (k *kdaKapp) SetKAppController(controller kapp.KAppController) error {
	k.KAppController = controller

	return nil
}

func (k *kdaKapp) SetAccountsCacher(cacher state.AccountsCacher) error {
	if check.IfNil(cacher) {
		return common.ErrNilAccountsAdapter
	}

	k.accountsCacher = cacher

	return nil
}

func (k *kdaKapp) GetAccountsCacher() state.AccountsCacher {
	return k.accountsCacher
}

func (k *kdaKapp) GetExistingUserAccount(pubkey []byte) (state.UserAccountHandler, error) {
	acc, err := k.accountsCacher.GetExistingUser(pubkey)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

func (k *kdaKapp) LoadUserAccount(pubkey []byte) (state.UserAccountHandler, error) {
	acc, err := k.accountsCacher.LoadUser(pubkey)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

func (k *kdaKapp) GetKDA(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
	kdaKapp, err := k.accountsCacher.GetExistingKapp(kapps.KDAKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	if assetID == nil {
		return kdaKapp, nil, nil
	}

	key := kdautils.ToKDAKey(assetID, nil)

	kdaBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, nil, err
	}
	if len(kdaBytes) == 0 {
		return nil, nil, common.ErrAssetNotFound
	}

	kda := &kapps.KDAData{}
	err = k.marshalizer.Unmarshal(kda, kdaBytes)
	if err != nil {
		return nil, nil, err
	}

	return kdaKapp, kda, nil
}

// GetKDAUncached is GetKDA for callers outside the processing goroutine (e.g. the transaction
// interceptors); see AccountsCacher.LoadKAppUncached. The cached KDA KApp is the hottest one on
// the chain - every asset mutation writes its TrackableDataTrie, a bare map with no lock - so a
// concurrent read of it from another goroutine is a fatal "concurrent map read and map write".
// Returns last committed state, which is what an interception-side check should see anyway.
func (k *kdaKapp) GetKDAUncached(assetID []byte) (*kapps.KDAData, error) {
	kdaKapp, err := k.accountsCacher.LoadKAppUncached(kapps.KDAKAppAddress)
	if err != nil {
		return nil, err
	}

	kdaBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(kdautils.ToKDAKey(assetID, nil))
	if err != nil {
		return nil, err
	}
	if len(kdaBytes) == 0 {
		return nil, common.ErrAssetNotFound
	}

	kda := &kapps.KDAData{}
	err = k.marshalizer.Unmarshal(kda, kdaBytes)
	if err != nil {
		return nil, err
	}

	return kda, nil
}

func (k *kdaKapp) SetKDA(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
	data, err := k.marshalizer.Marshal(kda)
	if err != nil {
		return err
	}

	key := kdautils.ToKDAKey(assetID, nil)

	err = kdaKapp.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return err
	}

	return nil
}

func (k *kdaKapp) GetStaking(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
	stakingKapp, err := k.accountsCacher.GetExistingKapp(kapps.StakingKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	if assetID == nil {
		return stakingKapp, nil, nil
	}

	key := kdautils.ToKDAKey(assetID, nil)

	stakedBytes, err := stakingKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, nil, err
	}
	if len(stakedBytes) == 0 {
		return nil, nil, common.ErrStakingNotFound
	}

	kdaStaking := &kapps.StakingData{}
	err = k.marshalizer.Unmarshal(kdaStaking, stakedBytes)
	if err != nil {
		return nil, nil, err
	}

	return stakingKapp, kdaStaking, nil
}

func (k *kdaKapp) SetStaking(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
	data, err := k.marshalizer.Marshal(staking)
	if err != nil {
		return err
	}

	key := kdautils.ToKDAKey(assetID, nil)

	err = stakingKapp.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return err
	}

	return nil
}

func (k *kdaKapp) parseAndLoadKDA(kdaID []byte) ([]byte, []byte, state.KAppAccountHandler, *kapps.KDAData, transaction.Transaction_TXResultCode, error) {
	parsedKDA := bytes.Split(kdaID, []byte(kapps.Sp))

	assetID := parsedKDA[0]
	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	kdaKapp, kda, err := k.GetKDA(assetID)
	if err != nil {
		return nil, nil, nil, nil, transaction.Transaction_KAPPError, err
	}

	var internalID []byte
	if len(parsedKDA) > 1 {
		if !k.TokeTypeHasNonce(kda.AssetType) || len(parsedKDA) != 2 {
			return nil, nil, nil, nil, transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		internalID = parsedKDA[1]
	}

	return assetID, internalID, kdaKapp, kda, transaction.Transaction_Ok, nil
}

func (k *kdaKapp) Burn(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	assetID, internalID, kdaKapp, kda, resultCode, err := k.parseAndLoadKDA(tc.GetAssetID())
	if err != nil {
		return resultCode, err
	}

	if tc.GetTriggerType() == transaction.AssetTriggerContract_Wipe {
		if !kda.IsOwnerOrAdmin(sender) {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if !kda.GetProperties().GetCanWipe() {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetCannotWipe, common.ErrAssetTriggerInvalid.Error())
			return transaction.Transaction_AssetCantBeWiped, common.ErrAssetTriggerInvalid
		}
	}

	if !kda.Properties.CanBurn {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetCannotBurn, common.ErrAssetTriggerInvalid.Error())
		return transaction.Transaction_AssetCantBeBurned, common.ErrAssetTriggerInvalid
	}

	if len(tc.GetToAddress()) != k.pubkeyConv.Len() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidToAddress, process.ErrInvalidRcvAddr.Error())
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	acc, err := k.LoadUserAccount(tc.GetToAddress())
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	switch kda.AssetType {
	case kapps.KDAData_NonFungible:
		resultCode, err = k.burnNFT(ctx, acc, kda, assetID, internalID, tc.GetAmount())
	case kapps.KDAData_SemiFungible:
		resultCode, err = k.burnSFT(ctx, acc, kda, assetID, internalID, tc.GetAmount())
	case kapps.KDAData_Fungible:
		resultCode, err = k.burnFungible(ctx, acc, kda, assetID, internalID, tc.GetAmount())
	default:
		return transaction.Transaction_ParameterInvalid, process.ErrInvalidUnitValue
	}
	if err != nil {
		return resultCode, err
	}

	if err := k.accountsCacher.UpdateUser(acc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = k.SetKDA(kdaKapp, assetID, kda)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := k.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}

// burnNFT burns the NFT internalID of collection assetID held by acc.
func (k *kdaKapp) burnNFT(ctx kapp.KappContext, acc state.UserAccountHandler, kda *kapps.KDAData, assetID []byte, internalID []byte, amount int64) (transaction.Transaction_TXResultCode, error) {
	if len(internalID) == 0 {
		return transaction.Transaction_AssetTypeInvalid, process.ErrInvalidArgument
	}

	// From FixAuditChangesV7 a spent record is refused: it holds no unit, so burning or wiping it
	// must not lower the supply (KLC-2835). A record that is missing, also from an account with no
	// trie, cleared earlier in the block or refused is an invalid argument; any other error is
	// returned as is.
	if _, err := acc.SubInternalKDA(assetID, internalID, k.forkController); err != nil {
		if errors.Is(err, common.ErrAssetNotFound) || errors.Is(err, common.ErrNilTrie) || errors.Is(err, common.ErrNegativeValue) {
			err = process.ErrInvalidArgument
		}
		return transaction.Transaction_AssetError, err
	}

	if amount != 1 {
		return transaction.Transaction_AssetError, process.ErrInvalidArgument
	}

	kda.BurnedValue++
	kda.CirculatingSupply--

	if kda.BurnedValue <= 0 || kda.CirculatingSupply < 0 {
		// prevent overflow
		return transaction.Transaction_AssetError, process.ErrSupplyNotValid
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		acc.AddressBytes(),
		core.ZeroAddress,
		[]byte("1"),
		assetID,
		internalID,
		[]byte{byte(kda.AssetType)},
	))

	return transaction.Transaction_Ok, nil
}

// burnSFT burns amount of the SFT internalID of collection assetID held by acc.
func (k *kdaKapp) burnSFT(ctx kapp.KappContext, acc state.UserAccountHandler, kda *kapps.KDAData, assetID []byte, internalID []byte, amount int64) (transaction.Transaction_TXResultCode, error) {
	if len(internalID) == 0 {
		return transaction.Transaction_AssetTypeInvalid, process.ErrInvalidArgument
	}

	if err := acc.SubFromBalanceWithNonce(amount, assetID, internalID, k.forkController.EnableSmartContracts()); err != nil {
		return transaction.Transaction_BalanceError, err
	}

	negativeAmount := -amount

	if err := k.KAppController.GetSystemAccountKApp().SFTAddCirculation(assetID, internalID, negativeAmount); err != nil {
		return transaction.Transaction_AssetError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		acc.AddressBytes(),
		core.ZeroAddress,
		[]byte(strconv.FormatInt(amount, 10)),
		assetID,
		internalID,
		[]byte{byte(kda.AssetType)},
	))

	return transaction.Transaction_Ok, nil
}

// burnFungible burns amount of the fungible assetID held by acc.
func (k *kdaKapp) burnFungible(ctx kapp.KappContext, acc state.UserAccountHandler, kda *kapps.KDAData, assetID []byte, internalID []byte, amount int64) (transaction.Transaction_TXResultCode, error) {
	if len(internalID) != 0 {
		return transaction.Transaction_AssetTypeInvalid, process.ErrInvalidArgument
	}

	if amount <= 0 {
		return transaction.Transaction_ParameterInvalid, process.ErrInvalidArgument
	}

	kda.BurnedValue += amount
	kda.CirculatingSupply -= amount

	if kda.BurnedValue <= 0 || !k.isValidCirculatingSupply(kda) {
		// prevent overflow
		return transaction.Transaction_AssetError, process.ErrSupplyNotValid
	}

	err := acc.SubFromBalance(amount, assetID, k.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		acc.AddressBytes(),
		core.ZeroAddress,
		[]byte(strconv.FormatInt(amount, 10)),
		assetID,
		nil,
		[]byte{byte(kda.AssetType)},
	))

	return transaction.Transaction_Ok, nil
}

func (k *kdaKapp) isValidCirculatingSupply(kda *kapps.KDAData) bool {
	if k.forkController.EnableSmartContracts() {
		return kda.CirculatingSupply >= 0
	}

	return kda.CirculatingSupply > 0
}

func (k *kdaKapp) validateStakingAPR(staking *transaction.StakingInfo) error {
	if !k.forkController.FixAuditChangesV6() || staking.GetType() != transaction.StakingInfo_APRI {
		return nil
	}

	if staking.GetAPR() > core.MaxStakingAPR {
		return common.ErrAPRAboveMaximum
	}

	return nil
}

// checkDepositPauseAndTransferControls refuses, once FixAuditChangesV6 is active, an FPR deposit
// into the pool of a paused asset, and one of a currency that is paused or that the depositor could
// not send to any account. The pool is not an account, so only the sender side of the transfer
// restriction applies. From FixAuditChangesV7 it also refuses a currency that is not KLV or a
// fungible asset: GetKDA also finds an NFT or SFT collection, whose bare key holds no balance
// (KLC-2876).
func (k *kdaKapp) checkDepositPauseAndTransferControls(sender []byte, asset, currency *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if k.forkController.FixAuditChangesV7() && currency.GetAssetType() != kapps.KDAData_Fungible {
		ctx := k.KAppController.GetCurrentKAppContext()
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetType, common.ErrAssetTypeInvalid.Error())
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	if !k.forkController.FixAuditChangesV6() {
		return transaction.Transaction_Ok, nil
	}

	ctx := k.KAppController.GetCurrentKAppContext()

	if asset.GetAttributes().GetIsPaused() || currency.GetAttributes().GetIsPaused() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetPaused, process.ErrAssetIsPaused.Error())
		return transaction.Transaction_AssetPaused, process.ErrAssetIsPaused
	}

	if !currency.CanSenderTransfer(sender) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldTransferNotAllowed, process.ErrKDATransferNotAllowed.Error())
		return transaction.Transaction_KDATransferNotAllowed, process.ErrKDATransferNotAllowed
	}

	return transaction.Transaction_Ok, nil
}

// expiredResidualAccount returns the account an expired FPR residual is paid to: the staking asset's
// owner once FixAuditChangesV6 is active, the depositor before it. The owner can be any address, so
// its account is loaded the way a transfer destination is.
func (k *kdaKapp) expiredResidualAccount(depositorAcc state.UserAccountHandler, asset *kapps.KDAData) (state.UserAccountHandler, error) {
	if !k.forkController.FixAuditChangesV6() || bytes.Equal(asset.GetOwnerAddress(), depositorAcc.AddressBytes()) {
		return depositorAcc, nil
	}

	return k.LoadUserAccount(asset.GetOwnerAddress())
}

// refundExpiredFPR pays the unclaimed residual of one currency of an expired FPR entry and records
// the Transfer receipt for it.
func (k *kdaKapp) refundExpiredFPR(recipient state.UserAccountHandler, amount int64, currencyID []byte) error {
	if err := recipient.AddToBalance(amount, currencyID, k.forkController.EnableSmartContracts()); err != nil {
		return err
	}

	ctx := k.KAppController.GetCurrentKAppContext()
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		core.ZeroAddress,
		recipient.AddressBytes(),
		[]byte(strconv.FormatInt(amount, 10)),
		currencyID,
		nil,
		[]byte{byte(kapps.KDAData_Fungible)},
	))

	return nil
}

func (k *kdaKapp) Deposit(sender []byte, tc *transaction.DepositContract) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	assetID := tc.GetID()
	if assetID == nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetID, common.ErrInvalidAsset.Error())
		return transaction.Transaction_AssetError, common.ErrInvalidAsset
	}

	if tc.GetAmount() <= 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, common.ErrInvalidValue.Error())
		return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
	}

	kdaKapp, kda, err := k.GetKDA(assetID)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	role, err := kda.GetRoleByAddress(sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoleAddress, err.Error())
		return transaction.Transaction_AssetError, err
	}

	if !role.HasRoleDeposit {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMissingDepositRole, common.ErrInvalidValue.Error())
		return transaction.Transaction_AccountError, common.ErrInvalidValue
	}

	currencyID := tc.GetCurrencyID()
	if currencyID == nil {
		currencyID = kdautils.KLVIdentifier
	}

	// test if currency actually exists
	_, currency, err := k.GetKDA(currencyID)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if resultCode, errControls := k.checkDepositPauseAndTransferControls(sender, kda, currency); errControls != nil {
		return resultCode, errControls
	}

	stakingKapp, staking, err := k.GetStaking(assetID)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidStakingType, err.Error())
		return transaction.Transaction_AssetError, err
	}

	if staking.InterestType != kapps.StakingData_FPRI {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidStakingType, common.ErrAssetTypeInvalid.Error())
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	depositorAcc, err := k.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	if err := depositorAcc.SubFromBalance(tc.GetAmount(), currencyID, k.forkController.EnableSmartContracts()); err != nil {
		return transaction.Transaction_BalanceError, err
	}

	residualAcc, err := k.expiredResidualAccount(depositorAcc, kda)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	validFPRs := make([]*kapps.FPRData, 0)
	newFPR := &kapps.FPRData{}
	for _, fpr := range staking.FPR {
		//if a fpr already exists for the next epoch, use it to add more deposits
		if fpr.GetEpoch() == ctx.Block().GetEpoch()+1 {
			newFPR = fpr
			continue
		}

		maxEpochsUnclaimed := tools.SafeI64ToU32(k.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MaxEpochsUnclaimed))
		//if a fpr is not expired, add it to the valid list
		notExpired := fpr.GetEpoch()+
			maxEpochsUnclaimed >=
			ctx.Block().GetEpoch()
		if k.forkController.FixAuditChangesV6() {
			// summed in uint64: a parameter saturated at MaxUint32 wrapped the uint32 sum above,
			// which made every FPR past epoch 0 look expired
			notExpired = uint64(fpr.GetEpoch())+uint64(maxEpochsUnclaimed) >= uint64(ctx.Block().GetEpoch())
		}
		if notExpired {
			validFPRs = append(validFPRs, fpr)
			continue
		}

		//pay expired KLV
		expiredKLV := fpr.TotalAmount - fpr.TotalClaimed
		if expiredKLV > 0 {
			if err := k.refundExpiredFPR(residualAcc, expiredKLV, kdautils.KLVIdentifier); err != nil {
				return transaction.Transaction_AssetError, err
			}
		}

		//pay expired KDAS
		kdas := fpr.GetKDAS()
		for _, key := range types.OrderedKeys(kdas, k.forkController.FixAuditChangesV6()) {
			value := kdas[key]
			expiredKDA := value.TotalAmount - value.TotalClaimed
			if expiredKDA > 0 {
				if err := k.refundExpiredFPR(residualAcc, expiredKDA, []byte(key)); err != nil {
					return transaction.Transaction_AssetError, err
				}
			}
		}
	}

	//Max kdas per epoch is 20
	if len(newFPR.KDAS) >= core.MaxDepositKDAs && !bytes.Equal(currencyID, kdautils.KLVIdentifier) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMaxDepositKDAsExceeded, common.ErrMaxSupplyExceeded.Error())
		return transaction.Transaction_MaxSupplyExceeded, common.ErrMaxSupplyExceeded
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Deposit,
		ctx.ContractID(),
		depositorAcc.AddressBytes(),
		[]byte(strconv.FormatInt(int64(transaction.DepositContract_FPRDeposit.Number()), 10)),
		[]byte(strconv.FormatInt(tc.GetAmount(), 10)),
		assetID,
		currencyID,
	))

	if bytes.Equal(currencyID, kdautils.KLVIdentifier) {
		validFPRs = append(validFPRs, &kapps.FPRData{
			TotalAmount:  newFPR.TotalAmount + tc.GetAmount(),
			TotalStaked:  staking.TotalStaked,
			Epoch:        ctx.Block().GetEpoch() + 1,
			TotalClaimed: newFPR.TotalClaimed,
			KDAS:         newFPR.KDAS,
		})
	} else {
		if newFPR.KDAS == nil {
			kdas := make(map[string]*kapps.KDAFPRData)
			kdas[string(currencyID)] = &kapps.KDAFPRData{
				TotalAmount:  tc.GetAmount(),
				TotalClaimed: 0,
			}
			newFPR.KDAS = kdas
		} else if newFPR.KDAS[string(currencyID)] != nil {
			newFPR.KDAS[string(currencyID)].TotalAmount = newFPR.KDAS[string(currencyID)].TotalAmount + tc.GetAmount()
		} else {
			newFPR.KDAS[string(currencyID)] = &kapps.KDAFPRData{
				TotalAmount:  tc.GetAmount(),
				TotalClaimed: 0,
			}
		}

		validFPRs = append(validFPRs, &kapps.FPRData{
			TotalAmount:  newFPR.TotalAmount,
			TotalStaked:  staking.TotalStaked,
			Epoch:        ctx.Block().GetEpoch() + 1,
			TotalClaimed: newFPR.TotalClaimed,
			KDAS:         newFPR.KDAS,
		})
	}

	staking.FPR = validFPRs

	err = k.SetStaking(stakingKapp, assetID, staking)
	if err != nil {
		return transaction.Transaction_SetStakingErr, err
	}

	if err := k.accountsCacher.UpdateKapp(stakingKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = k.SetKDA(kdaKapp, assetID, kda)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := k.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}

func (k *kdaKapp) TokeTypeHasNonce(tokenType kapps.KDAData_EnumAssetType) bool {
	if k.forkController.EnableSmartContracts() {
		return tokenType == kapps.KDAData_NonFungible ||
			tokenType == kapps.KDAData_SemiFungible
	}

	return tokenType == kapps.KDAData_NonFungible
}

func validateRoyaltiesTransferLimit(royaltiesInfo *transaction.RoyaltiesInfo) error {
	if royaltiesInfo == nil {
		return nil
	}

	if len(royaltiesInfo.TransferPercentage) > core.MaxTransferRoyalties ||
		len(royaltiesInfo.SplitRoyalties) > core.MaxTransferRoyalties {
		return common.ErrInvalidValue
	}

	return nil
}

func validateSplitRoyaltyPercentages(royaltiesInfo *transaction.RoyaltiesInfo, fixActive bool) error {
	if !fixActive || royaltiesInfo == nil {
		return nil
	}

	for _, split := range royaltiesInfo.GetSplitRoyalties() {
		if split.GetPercentTransferPercentage() > core.HundredPercent ||
			split.GetPercentTransferFixed() > core.HundredPercent ||
			split.GetPercentMarketPercentage() > core.HundredPercent ||
			split.GetPercentMarketFixed() > core.HundredPercent ||
			split.GetPercentITOPercentage() > core.HundredPercent ||
			split.GetPercentITOFixed() > core.HundredPercent {
			return common.ErrInvalidValue
		}
	}

	return nil
}
