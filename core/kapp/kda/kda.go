package kda

import (
	"bytes"
	"strconv"

	"github.com/klever-io/klever-go/common"
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
		if !bytes.Equal(kda.OwnerAddress, sender) && !bytes.Equal(kda.AdminAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if !kda.GetProperties().GetCanWipe() {
			return transaction.Transaction_AssetCantBeWiped, common.ErrAssetTriggerInvalid
		}
	}

	if !kda.Properties.CanBurn {
		return transaction.Transaction_AssetCantBeBurned, common.ErrAssetTriggerInvalid
	}

	if len(tc.GetToAddress()) != k.pubkeyConv.Len() {
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	acc, err := k.LoadUserAccount(tc.GetToAddress())
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	switch kda.AssetType {
	case kapps.KDAData_NonFungible:
		if len(internalID) == 0 {
			return transaction.Transaction_AssetTypeInvalid, process.ErrInvalidArgument
		}

		key := kdautils.ToKDAKey(assetID, internalID)

		data, err := acc.DataTrieTracker().RetrieveValue(key)
		if err != nil || len(data) == 0 {
			return transaction.Transaction_AssetError, process.ErrInvalidArgument
		}

		err = acc.DataTrieTracker().SaveKeyValue(key, nil)
		if err != nil {
			return transaction.Transaction_AssetError, err
		}

		if tc.GetAmount() != 1 {
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
	case kapps.KDAData_SemiFungible:
		if len(internalID) == 0 {
			return transaction.Transaction_AssetTypeInvalid, process.ErrInvalidArgument
		}

		if err = acc.SubFromBalanceWithNonce(tc.GetAmount(), assetID, internalID, k.forkController.EnableSmartContracts()); err != nil {
			return transaction.Transaction_BalanceError, err
		}

		negativeAmount := -tc.GetAmount()

		if err := k.KAppController.GetSystemAccountKApp().SFTAddCirculation(assetID, internalID, negativeAmount); err != nil {
			return transaction.Transaction_AssetError, err
		}

		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			acc.AddressBytes(),
			core.ZeroAddress,
			[]byte(strconv.FormatInt(tc.GetAmount(), 10)),
			assetID,
			internalID,
			[]byte{byte(kda.AssetType)},
		))

	case kapps.KDAData_Fungible:
		if len(internalID) != 0 {
			return transaction.Transaction_AssetTypeInvalid, process.ErrInvalidArgument
		}

		if tc.GetAmount() <= 0 {
			return transaction.Transaction_ParameterInvalid, process.ErrInvalidArgument
		}

		kda.BurnedValue += tc.GetAmount()
		kda.CirculatingSupply -= tc.GetAmount()

		if kda.BurnedValue <= 0 || kda.CirculatingSupply < 0 {
			// prevent overflow
			return transaction.Transaction_AssetError, process.ErrSupplyNotValid
		}

		err := acc.SubFromBalance(tc.GetAmount(), assetID, k.forkController.EnableSmartContracts())
		if err != nil {
			return transaction.Transaction_BalanceError, err
		}

		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			acc.AddressBytes(),
			core.ZeroAddress,
			[]byte(strconv.FormatInt(tc.GetAmount(), 10)),
			assetID,
			nil,
			[]byte{byte(kda.AssetType)},
		))
	default:
		return transaction.Transaction_ParameterInvalid, process.ErrInvalidUnitValue
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

func (k *kdaKapp) Deposit(sender []byte, tc *transaction.DepositContract) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	assetID := tc.GetID()
	if assetID == nil {
		return transaction.Transaction_AssetError, common.ErrInvalidAsset
	}

	if tc.GetAmount() <= 0 {
		return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
	}

	kdaKapp, kda, err := k.GetKDA(assetID)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	role, err := kda.GetRoleByAddress(sender)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if !role.HasRoleDeposit {
		return transaction.Transaction_AccountError, common.ErrInvalidValue
	}

	currencyID := tc.GetCurrencyID()
	if currencyID == nil {
		currencyID = kdautils.KLVIdentifier
	}

	// test if currency actually exists
	_, _, err = k.GetKDA(currencyID)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	stakingKapp, staking, err := k.GetStaking(assetID)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if staking.InterestType != kapps.StakingData_FPRI {
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	ownerAcc, err := k.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	if err := ownerAcc.SubFromBalance(tc.GetAmount(), currencyID, k.forkController.EnableSmartContracts()); err != nil {
		return transaction.Transaction_BalanceError, err
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
		if fpr.GetEpoch()+
			maxEpochsUnclaimed >=
			ctx.Block().GetEpoch() {
			validFPRs = append(validFPRs, fpr)
			continue
		}

		//pay expired KLV
		expiredKLV := fpr.TotalAmount - fpr.TotalClaimed
		if expiredKLV > 0 {
			err = ownerAcc.AddToBalance(expiredKLV, nil, k.forkController.EnableSmartContracts())
			if err != nil {
				return transaction.Transaction_AssetError, err
			}

			ctx.Receipts().Add(txProcess.NewReceipt(
				txProcess.Transfer,
				ctx.ContractID(),
				core.ZeroAddress,
				ownerAcc.AddressBytes(),
				[]byte(strconv.FormatInt(expiredKLV, 10)),
				kdautils.KLVIdentifier,
				nil,
				[]byte{byte(kapps.KDAData_Fungible)},
			))
		}

		//pay expired KDAS
		for key, value := range fpr.GetKDAS() {
			expiredKDA := value.TotalAmount - value.TotalClaimed
			if expiredKDA > 0 {
				err = ownerAcc.AddToBalance(expiredKDA, []byte(key), k.forkController.EnableSmartContracts())
				if err != nil {
					return transaction.Transaction_AssetError, err
				}

				ctx.Receipts().Add(txProcess.NewReceipt(
					txProcess.Transfer,
					ctx.ContractID(),
					core.ZeroAddress,
					ownerAcc.AddressBytes(),
					[]byte(strconv.FormatInt(expiredKDA, 10)),
					[]byte(key),
					nil,
					[]byte{byte(kapps.KDAData_Fungible)},
				))
			}
		}
	}

	//Max kdas per epoch is 20
	if len(newFPR.KDAS) >= core.MaxDepositKDAs && !bytes.Equal(currencyID, kdautils.KLVIdentifier) {
		return transaction.Transaction_MaxSupplyExceeded, common.ErrMaxSupplyExceeded
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Deposit,
		ctx.ContractID(),
		ownerAcc.AddressBytes(),
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
