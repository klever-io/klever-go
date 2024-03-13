package ito

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math"
	"math/big"
	"sort"
	"strconv"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"google.golang.org/protobuf/proto"
)

var _ kapp.ITOKapp = (*itoKapp)(nil)

type itoKapp struct {
	marshalizer    marshal.Marshalizer
	pubkeyConv     core.PubkeyConverter
	accountsCacher state.AccountsCacher
	forkController core.ForkController
	addressLen     int
	KAppController kapp.KAppController
}

// ArgsNewITOKApp holds the arguments needed to create a ITOKapp
type ArgsNewITOKApp struct {
	Marshalizer    marshal.Marshalizer
	PubkeyConv     core.PubkeyConverter
	ForkController core.ForkController
}

// NewITOKApp creates an ito KApp
func NewITOKApp(
	args *ArgsNewITOKApp,
) (*itoKapp, error) {
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.PubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}

	v := &itoKapp{
		marshalizer:    args.Marshalizer,
		addressLen:     args.PubkeyConv.Len(),
		pubkeyConv:     args.PubkeyConv,
		forkController: args.ForkController,
	}

	return v, nil
}

// IsInterfaceNil verifies if the underlying object is nil or not
func (i *itoKapp) IsInterfaceNil() bool {
	return i == nil
}

func (i *itoKapp) SetKAppController(controller kapp.KAppController) error {
	i.KAppController = controller

	return nil
}

func (i *itoKapp) SetAccountsCacher(cacher state.AccountsCacher) error {
	if check.IfNil(cacher) {
		return common.ErrNilAccountsAdapter
	}

	i.accountsCacher = cacher

	return nil
}

func (i *itoKapp) GetAccountsCacher() state.AccountsCacher {
	return i.accountsCacher
}

func (i *itoKapp) GetExistingUserAccount(pubkey []byte) (state.UserAccountHandler, error) {
	acc, err := i.accountsCacher.GetExistingUser(pubkey)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

func (i *itoKapp) LoadUserAccount(pubkey []byte) (state.UserAccountHandler, error) {
	acc, err := i.accountsCacher.LoadUser(pubkey)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

func (i *itoKapp) GetITO(assetID []byte) (state.KAppAccountHandler, *kapps.ITOData, error) {
	itoKApp, err := i.accountsCacher.LoadKApp(kapps.ITOKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	if assetID == nil {
		return itoKApp, nil, nil
	}

	key := kdautils.ToITOKey(assetID)

	itoBytes, err := itoKApp.DataTrieTracker().RetrieveValue(key)
	if err != nil && !errors.Is(err, common.ErrNilTrie) {
		return nil, nil, err
	}
	if len(itoBytes) == 0 {
		newITO := &kapps.ITOData{
			PackData: make(map[string]*kapps.PackData),
		}
		return itoKApp, newITO, common.ErrNotFoundInKApp
	}

	ito := &kapps.ITOData{}
	err = i.marshalizer.Unmarshal(ito, itoBytes)
	if err != nil {
		return nil, nil, err
	}

	return itoKApp, ito, nil
}

func (i *itoKapp) SetITO(itoKApp state.KAppAccountHandler, assetID []byte, ito *kapps.ITOData) error {
	data, err := i.marshalizer.Marshal(ito)
	if err != nil {
		return err
	}

	key := kdautils.ToITOKey(assetID)

	err = itoKApp.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return err
	}

	return nil
}

func (i *itoKapp) GetITOWhitelistByAddress(assetID []byte, hexAddress string) (state.KAppAccountHandler, *kapps.WhitelistData, error) {
	itoKApp, err := i.accountsCacher.GetExistingKapp(kapps.ITOKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	key := kdautils.ToITOWhitelistKey(assetID, hexAddress)

	itoWhitelistBytes, err := itoKApp.DataTrieTracker().RetrieveValue(key)
	if err != nil && !errors.Is(err, common.ErrNilTrie) {
		return nil, nil, err
	}
	if len(itoWhitelistBytes) == 0 {
		newWhitelist := &kapps.WhitelistData{
			Limit: 0,
		}
		return itoKApp, newWhitelist, common.ErrNotFoundInKApp
	}

	itoWhitelist := &kapps.WhitelistData{}
	err = i.marshalizer.Unmarshal(itoWhitelist, itoWhitelistBytes)
	if err != nil {
		return nil, nil, err
	}

	return itoKApp, itoWhitelist, nil
}

func (i *itoKapp) SetITOWhitelists(itoKApp state.KAppAccountHandler, assetID []byte, whitelist map[string]*kapps.WhitelistData) error {
	for key, value := range whitelist {
		data, err := i.marshalizer.Marshal(value)
		if err != nil {
			return err
		}

		whitelistKey := kdautils.ToITOWhitelistKey(assetID, key)

		err = itoKApp.DataTrieTracker().SaveKeyValue(whitelistKey, data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (i *itoKapp) Buy(sender []byte, tc *transaction.BuyContract) (transaction.Transaction_TXResultCode, error) {
	ctx := i.KAppController.GetCurrentKAppContext()

	if tc.GetAmount() <= 0 {
		return transaction.Transaction_AssetError, common.ErrInvalidValue
	}

	assetID := tc.GetID()

	currencyID := tc.GetCurrencyID()
	if currencyID == nil {
		currencyID = kdautils.KLVIdentifier
	}

	_, asset, err := i.KAppController.GetKDAKApp().GetKDA(assetID)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if !i.ValidAssetType(asset.AssetType) {
		return transaction.Transaction_AssetTypeInvalid, common.ErrInvalidValue
	}

	itoKapp, ito, err := i.GetITO(assetID)
	if err != nil {
		return transaction.Transaction_ITOKAPPError, err
	}

	if !ito.IsActive {
		return transaction.Transaction_ITONotActive, common.ErrITONotActive
	}

	if (ito.StartTime != 0 && ito.StartTime > ctx.Block().GetTimestamp()) ||
		(ito.EndTime != 0 && ito.EndTime < ctx.Block().GetTimestamp()) {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if ito.IsWhitelistActive &&
		(ito.WhitelistStartTime == 0 || ito.WhitelistStartTime <= ctx.Block().GetTimestamp()) &&
		(ito.WhitelistEndTime == 0 || ito.WhitelistEndTime >= ctx.Block().GetTimestamp()) {

		_, whitelist, err := i.GetITOWhitelistByAddress(assetID, hex.EncodeToString(sender))
		if err != nil {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		if whitelist.Limit < tc.GetAmount() {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		whitelist.Limit -= tc.GetAmount()

		err = i.SetITOWhitelists(itoKapp, assetID, map[string]*kapps.WhitelistData{hex.EncodeToString(sender): whitelist})
		if err != nil {
			return transaction.Transaction_ITOWhiteListError, err
		}
	}

	pack, err := ito.GetPackByAmount(currencyID, tc.GetAmount())
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	// TODO: Check SFT Package for ITO
	if asset.AssetType == kapps.KDAData_NonFungible && tc.GetAmount() != pack.Amount {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	ito.MintedAmount += tc.GetAmount()

	if ito.MaxAmount > 0 && ito.MintedAmount > ito.MaxAmount {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	value := new(big.Int).Mul(big.NewInt(tc.GetAmount()), big.NewInt(pack.Price))
	if i.forkController.ProcessorFlowITOPrice() {
		// Check fork cost by token unit
		value = value.Div(value, big.NewInt(int64(math.Pow10(int(asset.Precision)))))
	}
	valueInt := value.Int64()

	if valueInt <= 0 {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if asset.Royalties == nil {
		asset.Royalties = &kapps.RoyaltiesData{}
	}

	ownerAcc, err := i.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	royaltiesPercentAmount := int64(float64(valueInt) * float64(asset.Royalties.ITOPercentage) / float64(core.HundredPercent))
	itoOwnerAmount := valueInt - royaltiesPercentAmount

	if asset.Royalties.ITOFixed > 0 {
		balance := ownerAcc.GetBalance(kdautils.KLVIdentifier)
		if balance < asset.Royalties.ITOFixed {
			return transaction.Transaction_OutOfFunds, process.ErrInsufficientFunds
		}

		err = ownerAcc.SubFromBalance(asset.Royalties.ITOFixed, kdautils.KLVIdentifier)
		if err != nil {
			return transaction.Transaction_BalanceError, err
		}

		if err := i.accountsCacher.UpdateUser(ownerAcc); err != nil {
			return transaction.Transaction_SaveAccountError, err
		}

		royaltiesITOFixedToPay := asset.Royalties.ITOFixed
		for key, value := range asset.Royalties.SplitRoyalties {
			decodedAddress, err := hex.DecodeString(key)
			if err != nil {
				return transaction.Transaction_LoadAccountError, err
			}

			splitRoyalty, err := i.LoadUserAccount(decodedAddress)
			if err != nil {
				return transaction.Transaction_LoadAccountError, err
			}

			splitToPay := int64(float64(asset.Royalties.ITOFixed) * float64(value.PercentITOFixed) / float64(core.HundredPercent))
			royaltiesITOFixedToPay -= splitToPay

			err = splitRoyalty.AddToBalance(splitToPay, kdautils.KLVIdentifier)
			if err != nil {
				return transaction.Transaction_BalanceError, err
			}

			if err := i.accountsCacher.UpdateUser(splitRoyalty); err != nil {
				return transaction.Transaction_SaveAccountError, err
			}

			ctx.Receipts().Add(txProcess.NewReceipt(
				txProcess.Transfer,
				ctx.ContractID(),
				ownerAcc.AddressBytes(),
				splitRoyalty.AddressBytes(),
				[]byte(strconv.FormatInt(splitToPay, 10)),
				kdautils.KLVIdentifier,
				nil,
				[]byte{byte(kapps.KDAData_Fungible)},
				nil,
				nil,
			))
		}

		if royaltiesITOFixedToPay > 0 {
			kdaOwner, err := i.LoadUserAccount(asset.Royalties.Address)
			if err != nil {
				return transaction.Transaction_LoadAccountError, err
			}

			err = kdaOwner.AddToBalance(royaltiesITOFixedToPay, kdautils.KLVIdentifier)
			if err != nil {
				return transaction.Transaction_BalanceError, err
			}

			if err := i.accountsCacher.UpdateUser(kdaOwner); err != nil {
				return transaction.Transaction_SaveAccountError, err
			}

			ctx.Receipts().Add(txProcess.NewReceipt(
				txProcess.Transfer,
				ctx.ContractID(),
				ownerAcc.AddressBytes(),
				kdaOwner.AddressBytes(),
				[]byte(strconv.FormatInt(royaltiesITOFixedToPay, 10)),
				kdautils.KLVIdentifier,
				nil,
				[]byte{byte(kapps.KDAData_Fungible)},
				nil,
				nil,
			))
		}
	}

	if royaltiesPercentAmount > 0 {
		royaltiesITOPercentageToPay := royaltiesPercentAmount
		for key, value := range asset.Royalties.SplitRoyalties {
			decodedAddress, err := hex.DecodeString(key)
			if err != nil {
				return transaction.Transaction_LoadAccountError, err
			}

			splitRoyalty, err := i.GetExistingUserAccount(decodedAddress)
			if err != nil {
				return transaction.Transaction_LoadAccountError, err
			}

			splitToPay := int64(float64(royaltiesPercentAmount) * float64(value.PercentITOPercentage) / float64(core.HundredPercent))
			royaltiesITOPercentageToPay -= splitToPay

			err = splitRoyalty.AddToBalance(splitToPay, currencyID)
			if err != nil {
				return transaction.Transaction_BalanceError, err
			}

			if err := i.accountsCacher.UpdateUser(splitRoyalty); err != nil {
				return transaction.Transaction_SaveAccountError, err
			}

			ctx.Receipts().Add(txProcess.NewReceipt(
				txProcess.Transfer,
				ctx.ContractID(),
				ownerAcc.AddressBytes(),
				splitRoyalty.AddressBytes(),
				[]byte(strconv.FormatInt(splitToPay, 10)),
				currencyID,
				nil,
				[]byte{byte(kapps.KDAData_Fungible)},
				nil,
				nil,
			))
		}

		if royaltiesITOPercentageToPay > 0 {
			kdaOwner, err := i.GetExistingUserAccount(asset.Royalties.Address)
			if !i.forkController.KdaFpr() {
				kdaOwner, err = i.GetExistingUserAccount(asset.OwnerAddress)
			}
			if err != nil {
				return transaction.Transaction_LoadAccountError, err
			}

			err = kdaOwner.AddToBalance(royaltiesITOPercentageToPay, currencyID)
			if err != nil {
				return transaction.Transaction_BalanceError, err
			}

			if err := i.accountsCacher.UpdateUser(kdaOwner); err != nil {
				return transaction.Transaction_SaveAccountError, err
			}

			ctx.Receipts().Add(txProcess.NewReceipt(
				txProcess.Transfer,
				ctx.ContractID(),
				ownerAcc.AddressBytes(),
				kdaOwner.AddressBytes(),
				[]byte(strconv.FormatInt(royaltiesITOPercentageToPay, 10)),
				currencyID,
				nil,
				[]byte{byte(kapps.KDAData_Fungible)},
				nil,
				nil,
			))
		}

	}

	err = ownerAcc.SubFromBalance(valueInt, currencyID)
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	if err := i.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	receiverAcc, err := i.LoadUserAccount(ito.GetReceiverAddress())
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	err = receiverAcc.AddToBalance(itoOwnerAmount, currencyID)
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	if err := i.accountsCacher.UpdateUser(receiverAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		ownerAcc.AddressBytes(),
		receiverAcc.AddressBytes(),
		[]byte(strconv.FormatInt(itoOwnerAmount, 10)),
		currencyID,
		nil,
		[]byte{byte(kapps.KDAData_Fungible)},
	))

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateKDA,
		ctx.ContractID(),
		assetID,
	))

	initialMintValue := asset.MintedValue

	//Need to change the Sender to ITOKAppAddress to mint in the behalf of the ito contract
	resultCode, err := i.KAppController.GetKDAKApp().Mint(kapps.ITOKAppAddress, &transaction.AssetTriggerContract{AssetID: assetID, Amount: tc.GetAmount(), ToAddress: sender})
	if err != nil {
		return resultCode, err
	}

	err = i.SetITO(itoKapp, assetID, ito)
	if err != nil {
		return transaction.Transaction_ITOKAPPError, err
	}

	if err := i.accountsCacher.UpdateKapp(itoKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateITO,
		ctx.ContractID(),
		ownerAcc.AddressBytes(),
		assetID,
		[]byte(strconv.FormatInt(asset.MintedValue-initialMintValue, 10)),
	))

	return transaction.Transaction_Ok, nil
}

func (i *itoKapp) Config(sender []byte, tc *transaction.ConfigITOContract) (transaction.Transaction_TXResultCode, error) {
	ctx := i.KAppController.GetCurrentKAppContext()

	if tc.GetAssetID() == nil {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	kdaKapp, asset, err := i.KAppController.GetKDAKApp().GetKDA(tc.GetAssetID())
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if !i.ValidAssetType(asset.AssetType) {
		return transaction.Transaction_AssetTypeInvalid, common.ErrInvalidValue
	}

	if !bytes.Equal(asset.OwnerAddress, sender) {
		return transaction.Transaction_AccountNotOwner, common.ErrInvalidValue
	}

	if !asset.Properties.CanMint || asset.Attributes.IsNFTMintStopped {
		return transaction.Transaction_AssetCantBeMinted, common.ErrAssetTypeInvalid
	}

	itoKapp, ito, err := i.GetITO(tc.GetAssetID())
	if err != nil && !errors.Is(err, common.ErrNotFoundInKApp) {
		return transaction.Transaction_ITOKAPPError, err
	}
	if i.forkController.KdaFpr() && err == nil && !proto.Equal(ito, &kapps.ITOData{}) {
		return transaction.Transaction_AlreadyExists, common.ErrITOAlreadyExists
	}

	if tc.GetMaxAmount() < 0 {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetMaxAmount() > 0 {
		if ito.MintedAmount > tc.GetMaxAmount() {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		ito.MaxAmount = tc.GetMaxAmount()
	}

	if tc.GetStartTime() != 0 && tc.GetStartTime() <= ctx.Block().GetTimestamp() {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetEndTime() != 0 &&
		(tc.GetEndTime() <= tc.GetStartTime() ||
			tc.GetEndTime() <= ctx.Block().GetTimestamp()) {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetWhitelistStartTime() != 0 && tc.GetWhitelistStartTime() <= ctx.Block().GetTimestamp() {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetWhitelistEndTime() != 0 &&
		(tc.GetWhitelistEndTime() <= tc.GetWhitelistStartTime() ||
			tc.GetWhitelistEndTime() <= ctx.Block().GetTimestamp()) {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetDefaultLimitPerAddress() < 0 || (tc.GetMaxAmount() > 0 && tc.GetDefaultLimitPerAddress() > tc.GetMaxAmount()) {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	ito.WhitelistStartTime = tc.GetWhitelistStartTime()
	ito.WhitelistEndTime = tc.GetWhitelistEndTime()
	ito.StartTime = tc.GetStartTime()
	ito.EndTime = tc.GetEndTime()
	ito.DefaultLimitPerAddress = tc.GetDefaultLimitPerAddress()

	validateAndSetReceiverAddress := func() error {
		if len(tc.GetReceiverAddress()) != i.pubkeyConv.Len() {
			return process.ErrInvalidRcvAddr
		}

		ito.ReceiverAddress = tc.GetReceiverAddress()
		return nil
	}

	if i.forkController.KdaFpr() {
		err := validateAndSetReceiverAddress()
		if err != nil {
			return transaction.Transaction_ParameterInvalid, err
		}

	} else {
		if tc.GetReceiverAddress() != nil {
			err := validateAndSetReceiverAddress()
			if err != nil {
				return transaction.Transaction_ParameterInvalid, err
			}
		}

		if ito.ReceiverAddress == nil {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}
	}

	switch tc.GetStatus() {
	case transaction.ConfigITOContract_DefaultITO:
		if i.forkController.KdaFpr() {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

	case transaction.ConfigITOContract_ActiveITO:
		ito.IsActive = true

		updated := false
		for i, role := range asset.Roles {
			if bytes.Equal(role.Address, kapps.ITOKAppAddress) {
				asset.Roles[i] = &kapps.RolesData{
					Address:             kapps.ITOKAppAddress,
					HasRoleMint:         true,
					HasRoleSetITOPrices: true,
				}
				updated = true
				break
			}
		}
		if !updated {
			asset.Roles = append(asset.Roles, &kapps.RolesData{
				Address:             kapps.ITOKAppAddress,
				HasRoleMint:         true,
				HasRoleSetITOPrices: true,
			})
		}

		err = i.KAppController.GetKDAKApp().SetKDA(kdaKapp, tc.GetAssetID(), asset)
		if err != nil {
			return transaction.Transaction_KAPPError, err
		}
	case transaction.ConfigITOContract_PausedITO:
		ito.IsActive = false

		newRoles := make([]*kapps.RolesData, 0)

		for _, role := range asset.Roles {
			if !bytes.Equal(role.Address, kapps.ITOKAppAddress) {
				newRoles = append(newRoles, role)
			}
		}

		asset.Roles = newRoles

		err = i.KAppController.GetKDAKApp().SetKDA(kdaKapp, tc.GetAssetID(), asset)
		if err != nil {
			return transaction.Transaction_KAPPError, err
		}
	default:
		return transaction.Transaction_ParameterInvalid, common.ErrITOTypeInvalid
	}

	switch tc.GetWhitelistStatus() {
	case transaction.ConfigITOContract_DefaultITO,
		transaction.ConfigITOContract_PausedITO:
		ito.IsWhitelistActive = false
	case transaction.ConfigITOContract_ActiveITO:
		ito.IsWhitelistActive = true
	default:
		return transaction.Transaction_ParameterInvalid, common.ErrWhitelistStatusInvalid
	}

	if len(tc.GetPackInfo()) > 0 {
		newPackData := make(map[string]*kapps.PackData, len(tc.GetPackInfo()))

		if len(tc.GetPackInfo()) > core.MaxPacks {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		for key, packInfo := range tc.GetPackInfo() {
			if len(packInfo.GetPacks()) > core.MaxPackItems {
				return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
			}

			// validate if at least one pack for the given asset exists
			if len(packInfo.GetPacks()) == 0 {
				return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
			}

			//validate if provided pack asset exists
			_, _, err := i.KAppController.GetKDAKApp().GetKDA([]byte(key))
			if err != nil {
				return transaction.Transaction_KAPPError, err
			}

			newPacks := make([]*kapps.Pack, len(packInfo.Packs))
			for index, pack := range packInfo.Packs {
				invalidPrice := pack.Price < 0
				if !i.forkController.KdaFpr() {
					invalidPrice = pack.Price <= 0
				}

				if pack.Amount <= 0 || invalidPrice {
					return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
				}

				newPacks[index] = &kapps.Pack{
					Amount: pack.Amount,
					Price:  pack.Price,
				}
			}

			sort.SliceStable(newPacks, func(i, j int) bool {
				return newPacks[i].Amount < newPacks[j].Amount
			})

			newPackData[key] = &kapps.PackData{Packs: newPacks}
		}

		ito.PackData = newPackData
	}

	if len(tc.GetWhitelistInfo()) > 0 {
		if len(tc.GetWhitelistInfo()) > core.MaxWhitelistSize {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		whitelistData := make(map[string]*kapps.WhitelistData)
		for hexKey, value := range tc.GetWhitelistInfo() {
			decodedAddress, err := hex.DecodeString(hexKey)
			if err != nil {
				return transaction.Transaction_AccountError, process.ErrInvalidWhitelistAddr
			}

			if len(decodedAddress) != i.pubkeyConv.Len() {
				return transaction.Transaction_AccountError, process.ErrInvalidWhitelistAddr
			}

			if value.Limit < 0 {
				return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
			}

			whitelistData[hexKey] = &kapps.WhitelistData{Limit: ito.GetDefaultLimitPerAddress()}
			if value.Limit > 0 {
				whitelistData[hexKey] = &kapps.WhitelistData{Limit: value.Limit}
			}
		}

		ito.WhitelistLen = int32(len(whitelistData))

		err = i.SetITOWhitelists(itoKapp, tc.GetAssetID(), whitelistData)
		if err != nil {
			return transaction.Transaction_ITOWhiteListError, err
		}
	}

	err = i.SetITO(itoKapp, tc.GetAssetID(), ito)
	if err != nil {
		return transaction.Transaction_ITOKAPPError, err
	}

	if err := i.accountsCacher.UpdateKapp(itoKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	if err := i.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.ConfigITO,
		ctx.ContractID(),
		tc.GetAssetID(),
	))

	return transaction.Transaction_Ok, nil
}

func (i *itoKapp) Trigger(sender []byte, tc *transaction.ITOTriggerContract) (transaction.Transaction_TXResultCode, error) {
	ctx := i.KAppController.GetCurrentKAppContext()

	if tc.GetAssetID() == nil {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	kdaKapp, asset, err := i.KAppController.GetKDAKApp().GetKDA(tc.GetAssetID())
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	itoKapp, ito, err := i.GetITO(tc.GetAssetID())
	if err != nil {
		return transaction.Transaction_ITOKAPPError, err
	}

	if !i.ValidAssetType(asset.AssetType) {
		return transaction.Transaction_AssetTypeInvalid, common.ErrInvalidValue
	}

	switch tc.GetTriggerType() {
	case transaction.ITOTriggerContract_SetITOPrices:
		if tc.GetPackInfo() == nil {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		if len(tc.GetPackInfo()) > core.MaxPacks {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		role, err := asset.GetRoleByAddress(sender)
		if err != nil {
			return transaction.Transaction_AssetError, err
		}

		if !role.HasRoleSetITOPrices {
			return transaction.Transaction_AccountError, common.ErrInvalidValue
		}

		newPackData := make(map[string]*kapps.PackData, len(tc.GetPackInfo()))

		for key, packData := range tc.GetPackInfo() {
			if len(packData.GetPacks()) > core.MaxPackItems {
				return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
			}

			// validate if at least one pack for the given asset exists
			if len(packData.GetPacks()) == 0 {
				return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
			}

			//validate if provided pack asset exists
			_, _, err := i.KAppController.GetKDAKApp().GetKDA([]byte(key))
			if err != nil {
				return transaction.Transaction_KAPPError, err
			}

			newPacks := make([]*kapps.Pack, len(packData.Packs))
			for index, pack := range packData.Packs {
				invalidPrice := pack.Price < 0
				if !i.forkController.KdaFpr() {
					invalidPrice = pack.Price <= 0
				}

				if pack.Amount <= 0 || invalidPrice || (ito.MaxAmount > 0 && pack.Amount > ito.MaxAmount) {
					return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
				}

				newPacks[index] = &kapps.Pack{
					Amount: pack.Amount,
					Price:  pack.Price,
				}
			}

			sort.SliceStable(newPacks, func(i, j int) bool {
				return newPacks[i].Amount < newPacks[j].Amount
			})

			newPackData[key] = &kapps.PackData{Packs: newPacks}
		}

		ito.PackData = newPackData

		err = i.SetITO(itoKapp, tc.GetAssetID(), ito)
		if err != nil {
			return transaction.Transaction_ITOKAPPError, err
		}

		if err := i.accountsCacher.UpdateKapp(itoKapp); err != nil {
			return transaction.Transaction_SaveAccountError, err
		}
	case transaction.ITOTriggerContract_UpdateStatus:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		switch tc.GetStatus() {
		case transaction.ITOTriggerContract_DefaultITO:
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		case transaction.ITOTriggerContract_ActiveITO:
			ito.IsActive = true

			updated := false
			for i, role := range asset.Roles {
				if bytes.Equal(role.Address, kapps.ITOKAppAddress) {
					asset.Roles[i] = &kapps.RolesData{
						Address:             kapps.ITOKAppAddress,
						HasRoleMint:         true,
						HasRoleSetITOPrices: true,
					}
					updated = true
					break
				}
			}
			if !updated {
				asset.Roles = append(asset.Roles, &kapps.RolesData{
					Address:             kapps.ITOKAppAddress,
					HasRoleMint:         true,
					HasRoleSetITOPrices: true,
				})
			}

			err = i.KAppController.GetKDAKApp().SetKDA(kdaKapp, tc.GetAssetID(), asset)
			if err != nil {
				return transaction.Transaction_KAPPError, err
			}
		case transaction.ITOTriggerContract_PausedITO:
			ito.IsActive = false

			newRoles := make([]*kapps.RolesData, 0)

			for _, role := range asset.Roles {
				if !bytes.Equal(role.Address, kapps.ITOKAppAddress) {
					newRoles = append(newRoles, role)
				}
			}

			asset.Roles = newRoles

			err = i.KAppController.GetKDAKApp().SetKDA(kdaKapp, tc.GetAssetID(), asset)
			if err != nil {
				return transaction.Transaction_KAPPError, err
			}
		default:
			return transaction.Transaction_ParameterInvalid, common.ErrITOStatusInvalid
		}

	case transaction.ITOTriggerContract_UpdateReceiverAddress:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if len(tc.GetReceiverAddress()) != i.pubkeyConv.Len() {
			return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
		}

		ito.ReceiverAddress = tc.GetReceiverAddress()
	case transaction.ITOTriggerContract_UpdateMaxAmount:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if tc.GetMaxAmount() < 0 {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		if tc.GetMaxAmount() > 0 && ito.MintedAmount > tc.GetMaxAmount() {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		ito.MaxAmount = tc.GetMaxAmount()
	case transaction.ITOTriggerContract_UpdateDefaultLimitPerAddress:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if tc.GetDefaultLimitPerAddress() < 0 || (ito.MaxAmount > 0 && tc.GetDefaultLimitPerAddress() > ito.MaxAmount) {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		ito.DefaultLimitPerAddress = tc.GetDefaultLimitPerAddress()
	case transaction.ITOTriggerContract_UpdateTimes:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if tc.GetEndTime() <= tc.GetStartTime() ||
			tc.GetStartTime() <= ctx.Block().GetTimestamp() ||
			tc.GetEndTime() <= ctx.Block().GetTimestamp() {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		ito.StartTime = tc.GetStartTime()
		ito.EndTime = tc.GetEndTime()
	case transaction.ITOTriggerContract_AddToWhitelist:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if len(tc.GetWhitelistInfo()) == 0 || len(tc.GetWhitelistInfo()) > core.MaxWhitelistSize {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		newWhitelistAdditions := int32(0)
		whitelistData := make(map[string]*kapps.WhitelistData)
		for hexKey, value := range tc.GetWhitelistInfo() {
			decodedAddress, err := hex.DecodeString(hexKey)
			if err != nil {
				return transaction.Transaction_AccountError, process.ErrInvalidWhitelistAddr
			}

			if len(decodedAddress) != i.pubkeyConv.Len() {
				return transaction.Transaction_AccountError, process.ErrInvalidWhitelistAddr
			}

			_, _, err = i.GetITOWhitelistByAddress(tc.GetAssetID(), hexKey)
			if err != nil && errors.Is(err, common.ErrNotFoundInKApp) {
				newWhitelistAdditions += 1
			}

			if value.Limit < 0 {
				return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
			}

			whitelistData[hexKey] = &kapps.WhitelistData{Limit: ito.GetDefaultLimitPerAddress()}
			if value.Limit > 0 {
				whitelistData[hexKey] = &kapps.WhitelistData{Limit: value.Limit}
			}
		}

		ito.WhitelistLen += newWhitelistAdditions

		err = i.SetITOWhitelists(itoKapp, tc.GetAssetID(), whitelistData)
		if err != nil {
			return transaction.Transaction_ITOWhiteListError, err
		}
	case transaction.ITOTriggerContract_RemoveFromWhitelist:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if len(tc.GetWhitelistInfo()) == 0 || len(tc.GetWhitelistInfo()) > core.MaxWhitelistSize {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		whitelistToRemove := int32(0)
		whitelistData := make(map[string]*kapps.WhitelistData)
		for hexKey := range tc.GetWhitelistInfo() {
			decodedAddress, err := hex.DecodeString(hexKey)
			if err != nil {
				return transaction.Transaction_AccountError, process.ErrInvalidWhitelistAddr
			}

			if len(decodedAddress) != i.pubkeyConv.Len() {
				return transaction.Transaction_AccountError, process.ErrInvalidWhitelistAddr
			}

			_, _, err = i.GetITOWhitelistByAddress(tc.GetAssetID(), hexKey)
			if err != nil && errors.Is(err, common.ErrNotFoundInKApp) {
				whitelistToRemove -= 1
			}

			whitelistData[hexKey] = nil
		}

		ito.WhitelistLen -= whitelistToRemove

		err = i.SetITOWhitelists(itoKapp, tc.GetAssetID(), whitelistData)
		if err != nil {
			return transaction.Transaction_ITOWhiteListError, err
		}
	case transaction.ITOTriggerContract_UpdateWhitelistTimes:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if tc.GetWhitelistEndTime() <= tc.GetWhitelistStartTime() ||
			tc.GetWhitelistStartTime() <= ctx.Block().GetTimestamp() ||
			tc.GetWhitelistEndTime() <= ctx.Block().GetTimestamp() {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		ito.WhitelistStartTime = tc.GetWhitelistStartTime()
		ito.WhitelistEndTime = tc.GetWhitelistEndTime()
	case transaction.ITOTriggerContract_UpdateWhitelistStatus:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		switch tc.GetWhitelistStatus() {
		case transaction.ITOTriggerContract_DefaultITO:
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		case transaction.ITOTriggerContract_ActiveITO:
			ito.IsWhitelistActive = true
		case transaction.ITOTriggerContract_PausedITO:
			ito.IsWhitelistActive = false
		default:
			return transaction.Transaction_ParameterInvalid, common.ErrITOWhitelistStatusInvalid
		}
	default:
		return transaction.Transaction_ParameterInvalid, common.ErrITOTriggerInvalid
	}

	err = i.SetITO(itoKapp, tc.GetAssetID(), ito)
	if err != nil {
		return transaction.Transaction_ITOKAPPError, err
	}

	if err := i.accountsCacher.UpdateKapp(itoKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	if err := i.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateITO,
		ctx.ContractID(),
		tc.GetAssetID(),
	))

	return transaction.Transaction_Ok, nil
}

func (i *itoKapp) SetPrices(sender []byte, tc *transaction.SetITOPricesContract) (transaction.Transaction_TXResultCode, error) {
	ctx := i.KAppController.GetCurrentKAppContext()

	if tc.GetPackInfo() == nil {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetAssetID() == nil {
		return transaction.Transaction_AssetError, common.ErrInvalidValue
	}

	if len(tc.GetPackInfo()) > core.MaxPacks {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	_, asset, err := i.KAppController.GetKDAKApp().GetKDA(tc.GetAssetID())
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	role, err := asset.GetRoleByAddress(sender)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if !role.HasRoleSetITOPrices {
		return transaction.Transaction_AccountError, common.ErrInvalidValue
	}

	itoKapp, ito, err := i.GetITO(tc.GetAssetID())
	if err != nil {
		return transaction.Transaction_ITOKAPPError, err
	}

	newPackData := make(map[string]*kapps.PackData, len(tc.GetPackInfo()))

	for key, packData := range tc.GetPackInfo() {
		if len(packData.GetPacks()) > core.MaxPackItems {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		// validate if at least one pack for the given asset exists
		if len(packData.GetPacks()) == 0 {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		//validate if provided pack asset exists
		_, _, err := i.KAppController.GetKDAKApp().GetKDA([]byte(key))
		if err != nil {
			return transaction.Transaction_KAPPError, err
		}

		newPacks := make([]*kapps.Pack, len(packData.Packs))
		for i, pack := range packData.Packs {
			if pack.Amount <= 0 || pack.Price <= 0 {
				return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
			}

			newPacks[i] = &kapps.Pack{
				Amount: pack.Amount,
				Price:  pack.Price,
			}
		}

		sort.SliceStable(newPacks, func(i, j int) bool {
			return newPacks[i].Amount < newPacks[j].Amount
		})

		newPackData[key] = &kapps.PackData{Packs: newPacks}
	}

	ito.PackData = newPackData

	err = i.SetITO(itoKapp, tc.GetAssetID(), ito)
	if err != nil {
		return transaction.Transaction_ITOKAPPError, err
	}

	if err := i.accountsCacher.UpdateKapp(itoKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.SetITOPrices,
		ctx.ContractID(),
		tc.GetAssetID(),
	))

	return transaction.Transaction_Ok, nil
}

func (i *itoKapp) ValidAssetType(assetType kapps.KDAData_EnumAssetType) bool {
	return assetType == kapps.KDAData_Fungible || assetType == kapps.KDAData_NonFungible
}
