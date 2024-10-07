//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf  --go_out=Mgoogle/protobuf/any.proto=google.golang.org/protobuf/types/known/anypb:. transaction.proto
//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf  --go_out=Mgoogle/protobuf/any.proto=google.golang.org/protobuf/types/known/anypb:. contracts.proto
//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf  --go_out=Mgoogle/protobuf/any.proto=google.golang.org/protobuf/types/known/anypb:. log.proto
package transaction

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/models"
	proto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

type TXBaseInfo struct {
	Sender         string
	Nonce          uint64
	SenderUsername []byte
	DataField      [][]byte
	PermID         int32
	KDAFee         string
}

// NewBaseTransaction create base transaction
func NewBaseTransaction(sender []byte, nonce uint64, data [][]byte, kAppFee int64, bdFee int64) *Transaction {
	tx := &Transaction{
		RawData: &Transaction_Raw{
			Sender:       make([]byte, len(sender)),
			Nonce:        nonce,
			Data:         make([][]byte, len(data)),
			KAppFee:      kAppFee,
			BandwidthFee: bdFee,
			Version:      1,
		},
	}

	copy(tx.RawData.Sender, sender)
	copy(tx.RawData.Data, data)
	return tx
}

// PrepareForProcessing clear TX previous processing results
func (t *Transaction) PrepareForProcessing() {
	t.Receipts = make([]*Transaction_Receipt, 0)
	t.Block = 0
	t.Result = Transaction_SUCCESS
	t.ResultCode = Transaction_Ok
}

func (t *Transaction) GetNonce() uint64 {
	if t.RawData != nil {
		return t.RawData.Nonce
	}
	return 0
}

// GetTransaction returns underlying transaction
func (t *Transaction) GetTransaction() *Transaction {
	return t
}

// GetRaw returns transaction raw data
func (t *Transaction) GetRaw() *Transaction_Raw {
	return t.GetRawData()
}

// GetDataSize returns the size of the transaction data field
func (t *Transaction) GetDataSize() (size int64) {
	for _, d := range t.GetRawData().GetData() {
		size += int64(len(d))
	}

	return
}

// GetData returns RawData.Data
func (t *Transaction) GetData() [][]byte {
	data := make([][]byte, len(t.GetRawData().GetData()))
	copy(data, t.GetRawData().GetData())

	return data
}

// GetDataWithIdx returns RawData.Data with contract ID reference
func (t *Transaction) GetDataWithIdx(idx int) []byte {
	// check index if exists
	if idx >= len(t.GetRawData().GetData()) {
		return nil
	}

	data := make([]byte, len(t.GetRawData().Data[idx]))
	copy(data, t.GetRawData().Data[idx])

	return data
}

// GetSize returns the size of the transaction
func (t *Transaction) GetSize() int {
	return proto.Size(t)
}

func (t *Transaction) GetKAppFee() int64 {
	if t.RawData != nil {
		return t.RawData.KAppFee
	}
	return 0
}

func (t *Transaction) GetBandwidthFee() int64 {
	if t.RawData != nil {
		return t.RawData.BandwidthFee
	}
	return 0
}

func (t *Transaction) GetTotalFees() int64 {
	if t.RawData != nil {
		return t.RawData.BandwidthFee + t.RawData.KAppFee
	}
	return 0
}

// GetSender returns transaction sender
func (t *Transaction) GetSender() []byte {
	if t.RawData != nil {
		return t.RawData.Sender
	}

	return []byte{}
}

// IsInterfaceNil returns true if there is no value under the interface
func (t *Transaction) IsInterfaceNil() bool {
	return t == nil
}

// Clone will return a clone of the object
func (t *Transaction) Clone() *Transaction {
	return proto.Clone(t).(*Transaction)
}

// SetChainID -
func (t *Transaction) SetChainID(chainID []byte) error {
	t.RawData.ChainID = make([]byte, len(chainID))
	copy(t.RawData.ChainID, chainID)
	return nil
}

// TXArgs -
type TXArgs struct {
	Type             uint32
	Sender           []byte
	Data             [][]byte
	Contract         json.RawMessage
	ActiveParameters map[int32]*kapps.Parameter
	NodeHelper       NodeHelper
}

// AddTransaction -
func (t *Transaction) AddTransaction(txArgs TXArgs) error {
	var err error
	// #nosec G115
	switch TXContract_ContractType(txArgs.Type) {
	case TXContract_TransferContractType:
		err = t.addTransfer(txArgs)
	case TXContract_CreateAssetContractType:
		err = t.addCreateAsset(txArgs)
	case TXContract_AssetTriggerContractType:
		err = t.addAssetTrigger(txArgs)
	case TXContract_CreateValidatorContractType:
		err = t.addCreateValidator(txArgs)
	case TXContract_ValidatorConfigContractType:
		err = t.addValidatorConfig(txArgs)
	case TXContract_FreezeContractType:
		err = t.addFreeze(txArgs)
	case TXContract_UnfreezeContractType:
		err = t.addUnfreeze(txArgs)
	case TXContract_DelegateContractType:
		err = t.addDelegate(txArgs)
	case TXContract_UndelegateContractType:
		err = t.addUndelegate(txArgs)
	case TXContract_WithdrawContractType:
		err = t.addWithdraw(txArgs)
	case TXContract_ClaimContractType:
		err = t.addClaim(txArgs)
	case TXContract_UnjailContractType:
		err = t.addUnjail(txArgs)
	case TXContract_SetAccountNameContractType:
		err = t.addSetAccountName(txArgs)
	case TXContract_ProposalContractType:
		err = t.addProposal(txArgs)
	case TXContract_VoteContractType:
		err = t.addVote(txArgs)
	case TXContract_ConfigITOContractType:
		err = t.addConfigITO(txArgs)
	case TXContract_SetITOPricesContractType:
		err = t.addSetITOPrices(txArgs)
	case TXContract_BuyContractType:
		err = t.addBuy(txArgs)
	case TXContract_SellContractType:
		err = t.addSell(txArgs)
	case TXContract_CancelMarketOrderContractType:
		err = t.addCancelMarketOrder(txArgs)
	case TXContract_CreateMarketplaceContractType:
		err = t.addCreateMarketplace(txArgs)
	case TXContract_ConfigMarketplaceContractType:
		err = t.addConfigMarketplace(txArgs)
	case TXContract_UpdateAccountPermissionContractType:
		err = t.updateAccountPermission(txArgs)
	case TXContract_DepositContractType:
		err = t.addDeposit(txArgs)
	case TXContract_ITOTriggerContractType:
		err = t.addITOTrigger(txArgs)
	case TXContract_SmartContractType:
		err = t.addSmartContract(txArgs)
	default:
		return common.ErrInvalidTransactionType
	}

	return err
}

func (t *Transaction) PushContract(cType TXContract_ContractType, contract protoreflect.ProtoMessage) error {
	param, err := anypb.New(contract)
	if err != nil {
		return err
	}

	txContract := &TXContract{
		Type:      cType,
		Parameter: param,
	}

	if t.RawData.Contract == nil {
		t.RawData.Contract = make([]*TXContract, 0)
	}
	t.RawData.Contract = append(t.RawData.Contract, txContract)

	return nil
}

// AddTransfer -
func (t *Transaction) addTransfer(txArgs TXArgs) error {
	contractRequest := models.TransferTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	if len(contractRequest.Receiver) > txArgs.NodeHelper.GetEncodedAddressLength() {
		return fmt.Errorf("%w for receiver", common.ErrInvalidAddressLength)
	}

	receiverAddress, err := txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.Receiver)
	if err != nil {
		return common.ErrInvalidReceiverAddress
	}

	if contractRequest.Amount < 0 {
		return common.ErrInvalidValue
	}

	royaltyAmount := contractRequest.KDARoyalties
	klvRoyalties := contractRequest.KLVRoyalties
	if (royaltyAmount == 0 || klvRoyalties == 0) &&
		contractRequest.KDA != "" &&
		contractRequest.KDA != string(kdautils.KLVIdentifier) &&
		contractRequest.KDA != string(kdautils.KFIIdentifier) {

		kda, err := txArgs.NodeHelper.GetAsset(contractRequest.KDA)
		if err != nil {
			return common.ErrInvalidReceiverAddress
		}

		if kda.Royalties != nil {
			if len(kda.Royalties.TransferPercentage) > 0 {
				royaltyAmount, err = kda.GetTransferRoyaltyByAmount(contractRequest.Amount, txArgs.NodeHelper.GetForkController().KdaFpr())
				if err != nil {
					return err
				}
			}

			klvRoyalties = kda.Royalties.TransferFixed
		}
	}

	contract := &TransferContract{
		AssetID:      []byte(contractRequest.KDA),
		ToAddress:    receiverAddress,
		Amount:       contractRequest.Amount,
		KDARoyalties: royaltyAmount,
		KLVRoyalties: klvRoyalties,
	}

	return t.PushContract(TXContract_TransferContractType, contract)
}

// AddCreateAsset -
func (t *Transaction) addCreateAsset(txArgs TXArgs) error {
	contractRequest := models.CreateAssetTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	if len(contractRequest.OwnerAddress) > txArgs.NodeHelper.GetEncodedAddressLength() {
		return fmt.Errorf("%w for ownerAddress", common.ErrInvalidAddressLength)
	}

	ownerAddress, err := txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.OwnerAddress)
	if err != nil {
		return common.ErrInvalidReceiverAddress
	}

	var isValid bool

	if txArgs.NodeHelper.GetForkController().EnableSmartContracts() {
		isValid = kdautils.IsAssetNameHumanReadable([]byte(contractRequest.Name))
	} else {
		isValid = kdautils.IsAssetNameHumanReadableOld([]byte(contractRequest.Name))
	}

	if !isValid {
		return common.ErrTokenNameNotHumanReadable
	}

	if !kdautils.IsTickerValid([]byte(contractRequest.Ticker)) {
		return common.ErrTickerNameNotValid
	}

	roles := make([]*RolesInfo, len(contractRequest.Roles))
	for i, role := range contractRequest.Roles {
		if len(role.Address) > txArgs.NodeHelper.GetEncodedAddressLength() {
			return fmt.Errorf("%w for role address", common.ErrInvalidAddressLength)
		}

		roleAddress, err := txArgs.NodeHelper.GetAddressPCK().Decode(role.Address)
		if err != nil {
			return errors.New("could not create role address from provided param")
		}

		roles[i] = &RolesInfo{
			Address:             roleAddress,
			HasRoleMint:         role.HasRoleMint,
			HasRoleSetITOPrices: role.HasRoleSetITOPrices,
			HasRoleDeposit:      role.HasRoleDeposit,
			HasRoleTransfer:     role.HasRoleTransfer,
		}
	}

	// prevents nil pointer when Royalties is intended to be empty
	if contractRequest.Royalties == nil {
		contractRequest.Royalties = &models.RoyaltiesInfo{}
	}

	royalties := &RoyaltiesInfo{
		MarketFixed:      contractRequest.Royalties.MarketFixed,
		MarketPercentage: contractRequest.Royalties.MarketPercentage,
		TransferFixed:    contractRequest.Royalties.TransferFixed,
		ITOFixed:         contractRequest.Royalties.ITOFixed,
		ITOPercentage:    contractRequest.Royalties.ITOPercentage,
	}

	if contractRequest.Royalties.Address != "" {
		royaltiesAddress, err := txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.Royalties.Address)
		if err != nil {
			return errors.New("could not create royalties address from provided param")
		}

		royalties.Address = royaltiesAddress
	}

	if contractRequest.Royalties.TransferPercentage != nil {
		percentages := make([]*RoyaltyInfo, len(contractRequest.Royalties.TransferPercentage))
		for i, p := range contractRequest.Royalties.TransferPercentage {
			percentages[i] = &RoyaltyInfo{
				Amount:     p.Amount,
				Percentage: p.Percentage,
			}
		}

		royalties.TransferPercentage = percentages
	}

	splitRoyalties := make(map[string]*RoyaltySplitInfo)
	for key, value := range contractRequest.Royalties.SplitRoyalties {
		decodedAddress, err := txArgs.NodeHelper.GetAddressPCK().Decode(key)
		if err != nil {
			return errors.New("could not create split royalties from provided param")
		}

		splitRoyalties[hex.EncodeToString(decodedAddress)] = &RoyaltySplitInfo{
			PercentTransferPercentage: value.PercentTransferPercentage,
			PercentTransferFixed:      value.PercentTransferFixed,
			PercentMarketPercentage:   value.PercentMarketPercentage,
			PercentMarketFixed:        value.PercentMarketFixed,
			PercentITOPercentage:      value.PercentITOPercentage,
			PercentITOFixed:           value.PercentITOFixed,
		}
	}

	royalties.SplitRoyalties = splitRoyalties

	// create asset
	contract := &CreateAssetContract{
		Type:          CreateAssetContract_EnumAssetType(contractRequest.Type), // #nosec G115
		Name:          []byte(contractRequest.Name),
		Ticker:        []byte(contractRequest.Ticker),
		OwnerAddress:  ownerAddress,
		Logo:          contractRequest.Logo,
		URIs:          contractRequest.URIs,
		Precision:     contractRequest.Precision,
		InitialSupply: contractRequest.InitialSupply,
		MaxSupply:     contractRequest.MaxSupply,
		Royalties:     royalties,
		Roles:         roles,
	}

	if contractRequest.AdminAddress != "" {
		if len(contractRequest.AdminAddress) > txArgs.NodeHelper.GetEncodedAddressLength() {
			return fmt.Errorf("%w for ownerAddress", common.ErrInvalidAddressLength)
		}

		adminAddress, err := txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.AdminAddress)
		if err != nil {
			return errors.New("could not create admin address from provided param")
		}

		contract.AdminAddress = adminAddress
	}

	if contractRequest.Attributes != nil {
		contract.Attributes = &AttributesInfo{
			IsPaused:                   contractRequest.Attributes.IsPaused,
			IsNFTMintStopped:           contractRequest.Attributes.IsNFTMintStopped,
			IsRoyaltiesChangeStopped:   contractRequest.Attributes.IsRoyaltiesChangeStopped,
			IsNFTMetadataChangeStopped: contractRequest.Attributes.IsNFTMetadataChangeStopped,
		}
	}

	if contractRequest.Properties != nil {
		contract.Properties = &PropertiesInfo{
			CanFreeze:      contractRequest.Properties.CanFreeze,
			CanWipe:        contractRequest.Properties.CanWipe,
			CanPause:       contractRequest.Properties.CanPause,
			CanMint:        contractRequest.Properties.CanMint,
			CanBurn:        contractRequest.Properties.CanBurn,
			CanChangeOwner: contractRequest.Properties.CanChangeOwner,
			CanAddRoles:    contractRequest.Properties.CanAddRoles,
			LimitTransfer:  contractRequest.Properties.LimitTransfer,
		}
	}

	if contractRequest.Staking != nil {
		contract.Staking = &StakingInfo{
			Type:                StakingInfo_InterestType(contractRequest.Staking.InterestType), // #nosec G115
			APR:                 contractRequest.Staking.APR,
			MinEpochsToClaim:    contractRequest.Staking.MinEpochsToClaim,
			MinEpochsToUnstake:  contractRequest.Staking.MinEpochsToUnstake,
			MinEpochsToWithdraw: contractRequest.Staking.MinEpochsToWithdraw,
		}
	}

	return t.PushContract(TXContract_CreateAssetContractType, contract)
}

// addAssetTrigger -
func (t *Transaction) addAssetTrigger(txArgs TXArgs) error {
	contractRequest := models.AssetTriggerTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	var receiverAddress []byte
	if len(contractRequest.Receiver) > 0 {
		receiverAddress, err = txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.Receiver)
		if err != nil {
			return common.ErrInvalidReceiverAddress
		}
	}

	if contractRequest.Amount < 0 {
		return common.ErrInvalidValue
	}

	roleInfo := &RolesInfo{}
	if contractRequest.Role != nil {
		if len(contractRequest.Role.Address) > 0 {
			roleInfo.Address, err = txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.Role.Address)
			if err != nil {
				return common.ErrInvalidReceiverAddress
			}
			roleInfo.HasRoleMint = contractRequest.Role.HasRoleMint
			roleInfo.HasRoleSetITOPrices = contractRequest.Role.HasRoleSetITOPrices
			roleInfo.HasRoleDeposit = contractRequest.Role.HasRoleDeposit
			roleInfo.HasRoleTransfer = contractRequest.Role.HasRoleTransfer
		} else if contractRequest.Role.HasRoleMint || contractRequest.Role.HasRoleSetITOPrices {
			return errors.New("invalid role address")
		}
	}

	contract := &AssetTriggerContract{
		TriggerType: AssetTriggerContract_EnumTriggerType(contractRequest.TriggerType), // #nosec G115
		AssetID:     []byte(contractRequest.AssetID),
		ToAddress:   receiverAddress,
		Amount:      contractRequest.Amount,
		MIME:        []byte(contractRequest.MIME),
		Logo:        contractRequest.Logo,
		Value:       contractRequest.Value,
		URIs:        contractRequest.URIs,
		Role:        roleInfo,
	}

	if contractRequest.Staking != nil {
		contract.Staking = &StakingInfo{
			Type:                StakingInfo_InterestType(contractRequest.Staking.InterestType), // #nosec G115
			APR:                 contractRequest.Staking.APR,
			MinEpochsToClaim:    contractRequest.Staking.MinEpochsToClaim,
			MinEpochsToUnstake:  contractRequest.Staking.MinEpochsToUnstake,
			MinEpochsToWithdraw: contractRequest.Staking.MinEpochsToWithdraw,
		}
	}

	if contractRequest.Royalties != nil {
		royalties := &RoyaltiesInfo{
			MarketFixed:      contractRequest.Royalties.MarketFixed,
			MarketPercentage: contractRequest.Royalties.MarketPercentage,
			TransferFixed:    contractRequest.Royalties.TransferFixed,
			ITOPercentage:    contractRequest.Royalties.ITOPercentage,
			ITOFixed:         contractRequest.Royalties.ITOFixed,
		}

		if contractRequest.Royalties.Address != "" {
			royaltiesAddress, err := txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.Royalties.Address)
			if err != nil {
				return errors.New("could not create royalties address from provided param")
			}

			royalties.Address = royaltiesAddress
		}

		if contractRequest.Royalties.TransferPercentage != nil {
			percentages := make([]*RoyaltyInfo, len(contractRequest.Royalties.TransferPercentage))
			for i, p := range contractRequest.Royalties.TransferPercentage {
				percentages[i] = &RoyaltyInfo{
					Amount:     p.Amount,
					Percentage: p.Percentage,
				}
			}

			royalties.TransferPercentage = percentages
		}

		splitRoyalties := make(map[string]*RoyaltySplitInfo)
		for key, value := range contractRequest.Royalties.SplitRoyalties {
			decodedAddress, err := txArgs.NodeHelper.GetAddressPCK().Decode(key)
			if err != nil {
				return errors.New("could not create split royalties from provided param")
			}

			splitRoyalties[hex.EncodeToString(decodedAddress)] = &RoyaltySplitInfo{
				PercentTransferPercentage: value.PercentTransferPercentage,
				PercentTransferFixed:      value.PercentTransferFixed,
				PercentMarketPercentage:   value.PercentMarketPercentage,
				PercentMarketFixed:        value.PercentMarketFixed,
				PercentITOPercentage:      value.PercentITOPercentage,
				PercentITOFixed:           value.PercentITOFixed,
			}
		}

		royalties.SplitRoyalties = splitRoyalties

		contract.Royalties = royalties
	}

	if contractRequest.KDAPool != nil {
		var adminAddress []byte
		if len(contractRequest.KDAPool.AdminAddress) > 0 {
			adminAddress, err = txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.KDAPool.AdminAddress)
			if err != nil {
				return errors.New("could not create admin address from provided param")
			}
		}

		contract.KDAPool = &KDAPoolInfo{
			Active:       contractRequest.KDAPool.Active,
			AdminAddress: adminAddress,
			FRatioKLV:    contractRequest.KDAPool.FRatioKLV,
			FRatioKDA:    contractRequest.KDAPool.FRatioKDA,
		}
	}

	return t.PushContract(TXContract_AssetTriggerContractType, contract)
}

// addCreateValidator -
func (t *Transaction) addCreateValidator(txArgs TXArgs) error {
	contractRequest := models.CreateValidatorTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	if contractRequest.Commission > core.HundredPercent {
		return errors.New("invalid commission")
	}

	if contractRequest.MaxDelegationAmount < 0 {
		return errors.New("invalid max delegation amount")
	}

	blsKey, err := txArgs.NodeHelper.GetValidatorPCK().Decode(contractRequest.BLSPublicKey)
	if err != nil {
		return errors.New("could not create bls key from provided param")
	}

	ownerAddress := txArgs.Sender
	if len(contractRequest.OwnerAddress) > 0 {
		ownerAddress, err = txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.OwnerAddress)
		if err != nil {
			return errors.New("could not create reward address from provided param")
		}
	}

	rewardAddress := txArgs.Sender
	if len(contractRequest.RewardAddress) > 0 {
		rewardAddress, err = txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.RewardAddress)
		if err != nil {
			return errors.New("could not create reward address from provided param")
		}
	}

	contract := &CreateValidatorContract{
		OwnerAddress: ownerAddress,
		Config: &ValidatorConfig{
			BLSPublicKey:        blsKey,
			RewardAddress:       rewardAddress,
			CanDelegate:         contractRequest.CanDelegate,
			Commission:          contractRequest.Commission,
			MaxDelegationAmount: contractRequest.MaxDelegationAmount,
			Name:                contractRequest.Name,
			Logo:                contractRequest.Logo,
			URIs:                contractRequest.URIs,
		},
	}

	return t.PushContract(TXContract_CreateValidatorContractType, contract)
}

// addValidatorConfig -
func (t *Transaction) addValidatorConfig(txArgs TXArgs) error {
	contractRequest := models.ValidatorConfigTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	if contractRequest.Commission > core.HundredPercent {
		return errors.New("invalid commission")
	}

	if contractRequest.MaxDelegationAmount < 0 {
		return errors.New("invalid max delegation amount")
	}

	var blsKey []byte
	if len(contractRequest.BLSPublicKey) > 0 {
		blsKey, err = txArgs.NodeHelper.GetValidatorPCK().Decode(contractRequest.BLSPublicKey)
		if err != nil {
			return errors.New("could not create bls key from provided param")
		}
	}

	rewardAddress := []byte(nil)
	if len(contractRequest.RewardAddress) > 0 {
		rewardAddress, err = txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.RewardAddress)
		if err != nil {
			return errors.New("could not create reward address from provided param")
		}
	}

	contract := &ValidatorConfigContract{
		Config: &ValidatorConfig{
			BLSPublicKey:        blsKey,
			RewardAddress:       rewardAddress,
			CanDelegate:         contractRequest.CanDelegate,
			Commission:          contractRequest.Commission,
			MaxDelegationAmount: contractRequest.MaxDelegationAmount,
			Logo:                contractRequest.Logo,
			URIs:                contractRequest.URIs,
			Name:                contractRequest.Name,
		},
	}

	return t.PushContract(TXContract_ValidatorConfigContractType, contract)
}

// addFreeze -
func (t *Transaction) addFreeze(txArgs TXArgs) error {
	contractRequest := models.FreezeTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	minBuckets, err := strconv.ParseInt(string(txArgs.ActiveParameters[int32(kapps.EnumParameter_MinKLVBucketAmount)].Value), 10, 64)
	if err != nil {
		return err
	}

	if (contractRequest.KDA == string(kdautils.KLVIdentifier) &&
		contractRequest.Amount < minBuckets) || contractRequest.Amount <= 0 {
		return common.ErrInvalidValue
	}

	contract := &FreezeContract{
		Amount:  contractRequest.Amount,
		AssetID: []byte(contractRequest.KDA),
	}

	return t.PushContract(TXContract_FreezeContractType, contract)
}

// addUnfreeze -
func (t *Transaction) addUnfreeze(txArgs TXArgs) error {
	contractRequest := models.UnfreezeTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	if len(contractRequest.BucketID) == 0 {
		return errors.New("invalid bucket id")
	}

	bucketID, err := hex.DecodeString(contractRequest.BucketID)
	if err != nil {
		bucketID = []byte(contractRequest.BucketID)
	}

	contract := &UnfreezeContract{
		BucketID: bucketID,
		AssetID:  []byte(contractRequest.KDA),
	}

	return t.PushContract(TXContract_UnfreezeContractType, contract)
}

// addDelegate -
func (t *Transaction) addDelegate(txArgs TXArgs) error {
	contractRequest := models.DelegateTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	blsKey, err := txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.Receiver)
	if err != nil {
		return errors.New("could not create reward address from provided param")
	}

	if len(contractRequest.BucketID) == 0 {
		return errors.New("invalid bucket id")
	}

	bucketID, err := hex.DecodeString(contractRequest.BucketID)
	if err != nil {
		return errors.New("could not decode bucket id")
	}

	contract := &DelegateContract{
		ToAddress: blsKey,
		BucketID:  bucketID,
	}

	return t.PushContract(TXContract_DelegateContractType, contract)
}

// addUndelegate -
func (t *Transaction) addUndelegate(txArgs TXArgs) error {
	contractRequest := models.UndelegateTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	if len(contractRequest.BucketID) == 0 {
		return errors.New("invalid bucket id")
	}

	bucketID, err := hex.DecodeString(contractRequest.BucketID)
	if err != nil {
		return errors.New("could not decode bucket id")
	}

	contract := &UndelegateContract{
		BucketID: bucketID,
	}

	return t.PushContract(TXContract_UndelegateContractType, contract)
}

// addWithdraw -
func (t *Transaction) addWithdraw(txArgs TXArgs) error {
	contractRequest := models.WithdrawTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	contract := &WithdrawContract{
		AssetID:      []byte(contractRequest.KDA),
		CurrencyID:   []byte(contractRequest.CurrencyID),
		Amount:       contractRequest.Amount,
		WithdrawType: WithdrawContract_EnumWithdrawType(contractRequest.WithdrawType),
	}

	return t.PushContract(TXContract_WithdrawContractType, contract)
}

// addClaim -
func (t *Transaction) addClaim(txArgs TXArgs) error {
	contractRequest := models.ClaimTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	id := []byte(contractRequest.ID)
	if ClaimContract_EnumClaimType(contractRequest.ClaimType) == ClaimContract_MarketClaim {
		id, err = hex.DecodeString(contractRequest.ID)
		if err != nil {
			return errors.New("could not decode order id")
		}
	}

	contract := &ClaimContract{
		ClaimType: ClaimContract_EnumClaimType(contractRequest.ClaimType),
		ID:        id,
	}

	return t.PushContract(TXContract_ClaimContractType, contract)
}

// addUnjail -
func (t *Transaction) addUnjail(txArgs TXArgs) error {
	contractRequest := models.UnjailTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	contract := &UnjailContract{}

	return t.PushContract(TXContract_UnjailContractType, contract)
}

func (t *Transaction) addSetAccountName(txArgs TXArgs) error {
	contractRequest := models.SetAccountNameTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	contract := &SetAccountNameContract{
		Name: []byte(contractRequest.Name),
	}

	return t.PushContract(TXContract_SetAccountNameContractType, contract)
}

func (t *Transaction) addProposal(txArgs TXArgs) error {
	contractRequest := models.ProposalTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	parameters := make(map[int32][]byte)
	for key, value := range contractRequest.Parameters {
		parameters[key] = []byte(value)
	}

	contract := &ProposalContract{
		Parameters:     parameters,
		Description:    []byte(contractRequest.Description),
		EpochsDuration: contractRequest.EpochsDuration,
	}

	return t.PushContract(TXContract_ProposalContractType, contract)
}

func (t *Transaction) addVote(txArgs TXArgs) error {
	contractRequest := models.VoteTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	contract := &VoteContract{
		Type:       VoteContract_EnumVoteType(contractRequest.Type), // #nosec G115
		ProposalID: contractRequest.ProposalID,
		Amount:     contractRequest.Amount,
	}

	return t.PushContract(TXContract_VoteContractType, contract)
}

func (t *Transaction) addConfigITO(txArgs TXArgs) error {
	contractRequest := models.ConfigITOTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	receiverAddress, err := txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.ReceiverAddress)
	if err != nil {
		return common.ErrInvalidReceiverAddress
	}

	packInfo := make(map[string]*PackInfo)
	for key, packData := range contractRequest.PackInfo {
		newPacks := make([]*PackItem, len(packData.Packs))
		for i, pack := range packData.Packs {
			if pack.Amount <= 0 || pack.Price <= 0 {
				return common.ErrInvalidValue
			}

			newPacks[i] = &PackItem{
				Amount: pack.Amount,
				Price:  pack.Price,
			}
		}

		packInfo[key] = &PackInfo{Packs: newPacks}
	}

	whitelistInfo := make(map[string]*WhitelistInfo)
	for key, value := range contractRequest.WhitelistInfo {
		if value.Limit < 0 {
			return common.ErrInvalidValue
		}

		decodedAddress, err := txArgs.NodeHelper.GetAddressPCK().Decode(key)
		if err != nil {
			return errors.New("could not create whitelist address from provided param")
		}

		whitelistInfo[hex.EncodeToString(decodedAddress)] = &WhitelistInfo{
			Limit: value.Limit,
		}
	}

	if contractRequest.MaxAmount < 0 || contractRequest.DefaultLimitPerAddress < 0 {
		return common.ErrInvalidValue
	}

	if ConfigITOContract_EnumITOStatus_name[contractRequest.Status] == "" {
		return common.ErrInvalidValue
	}

	if ConfigITOContract_EnumITOStatus(contractRequest.Status) != ConfigITOContract_ActiveITO &&
		ConfigITOContract_EnumITOStatus(contractRequest.Status) != ConfigITOContract_PausedITO {
		return common.ErrInvalidValue
	}

	if ConfigITOContract_EnumITOStatus_name[contractRequest.WhitelistStatus] == "" {
		return common.ErrInvalidValue
	}

	wlStart := time.Unix(contractRequest.WhitelistStartTime, 0)
	wlEnd := time.Unix(contractRequest.WhitelistEndTime, 0)

	if wlStart.After(wlEnd) {
		return common.ErrInvalidValue
	}

	start := time.Unix(contractRequest.StartTime, 0)
	end := time.Unix(contractRequest.EndTime, 0)

	if start.After(end) {
		return common.ErrInvalidValue
	}

	contract := &ConfigITOContract{
		AssetID:                []byte(contractRequest.KDA),
		ReceiverAddress:        receiverAddress,
		Status:                 ConfigITOContract_EnumITOStatus(contractRequest.Status),
		MaxAmount:              contractRequest.MaxAmount,
		PackInfo:               packInfo,
		DefaultLimitPerAddress: contractRequest.DefaultLimitPerAddress,
		WhitelistStatus:        ConfigITOContract_EnumITOStatus(contractRequest.WhitelistStatus),
		WhitelistInfo:          whitelistInfo,
		WhitelistStartTime:     contractRequest.WhitelistStartTime,
		WhitelistEndTime:       contractRequest.WhitelistEndTime,
		StartTime:              contractRequest.StartTime,
		EndTime:                contractRequest.EndTime,
	}

	return t.PushContract(TXContract_ConfigITOContractType, contract)
}

// addITOTrigger -
func (t *Transaction) addITOTrigger(txArgs TXArgs) error {
	contractRequest := models.ITOTriggerTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	var receiverAddress []byte
	if len(contractRequest.ReceiverAddress) > 0 {
		receiverAddress, err = txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.ReceiverAddress)
		if err != nil {
			return common.ErrInvalidReceiverAddress
		}
	}

	packInfo := make(map[string]*PackInfo)
	for key, packData := range contractRequest.PackInfo {
		if len(packData.Packs) == 0 {
			return common.ErrInvalidValue
		}

		newPacks := make([]*PackItem, len(packData.Packs))
		for i, pack := range packData.Packs {
			if pack.Amount <= 0 || pack.Price <= 0 {
				return common.ErrInvalidValue
			}

			newPacks[i] = &PackItem{
				Amount: pack.Amount,
				Price:  pack.Price,
			}
		}

		packInfo[key] = &PackInfo{Packs: newPacks}
	}

	parsedWhitelist := make(map[string]*WhitelistInfo)
	for key, value := range contractRequest.WhitelistInfo {
		if value.Limit < 0 {
			return common.ErrInvalidValue
		}

		decodedAddress, err := txArgs.NodeHelper.GetAddressPCK().Decode(key)
		if err != nil {
			return errors.New("could not create whitelist address from provided param")
		}

		parsedWhitelist[hex.EncodeToString(decodedAddress)] = &WhitelistInfo{Limit: value.Limit}
	}

	if contractRequest.MaxAmount < 0 || contractRequest.DefaultLimitPerAddress < 0 {
		return common.ErrInvalidValue
	}

	if ITOTriggerContract_EnumITOStatus_name[contractRequest.Status] == "" {
		return common.ErrInvalidValue
	}

	if contractRequest.TriggerType == uint32(ITOTriggerContract_UpdateStatus) && ITOTriggerContract_EnumITOStatus(contractRequest.Status) == ITOTriggerContract_DefaultITO {
		return common.ErrInvalidValue
	}

	if ITOTriggerContract_EnumITOStatus_name[contractRequest.WhitelistStatus] == "" {
		return common.ErrInvalidValue
	}

	if contractRequest.TriggerType == uint32(ITOTriggerContract_UpdateWhitelistStatus) && ITOTriggerContract_EnumITOStatus(contractRequest.WhitelistStatus) == ITOTriggerContract_DefaultITO {
		return common.ErrInvalidValue
	}

	if contractRequest.TriggerType == uint32(ITOTriggerContract_AddToWhitelist) || contractRequest.TriggerType == uint32(ITOTriggerContract_RemoveFromWhitelist) {
		if len(contractRequest.WhitelistInfo) == 0 || len(contractRequest.WhitelistInfo) > core.MaxWhitelistSize {
			return common.ErrInvalidValue
		}
	}

	wlStart := time.Unix(contractRequest.WhitelistStartTime, 0)
	wlEnd := time.Unix(contractRequest.WhitelistEndTime, 0)

	if wlStart.After(wlEnd) {
		return common.ErrInvalidValue
	}

	start := time.Unix(contractRequest.StartTime, 0)
	end := time.Unix(contractRequest.EndTime, 0)

	if start.After(end) {
		return common.ErrInvalidValue
	}

	contract := &ITOTriggerContract{
		TriggerType:            ITOTriggerContract_EnumITOTriggerType(contractRequest.TriggerType), // #nosec G115
		AssetID:                []byte(contractRequest.KDA),
		ReceiverAddress:        receiverAddress,
		Status:                 ITOTriggerContract_EnumITOStatus(contractRequest.Status),
		MaxAmount:              contractRequest.MaxAmount,
		PackInfo:               packInfo,
		DefaultLimitPerAddress: contractRequest.DefaultLimitPerAddress,
		WhitelistStatus:        ITOTriggerContract_EnumITOStatus(contractRequest.WhitelistStatus),
		WhitelistInfo:          parsedWhitelist,
		WhitelistStartTime:     contractRequest.WhitelistStartTime,
		WhitelistEndTime:       contractRequest.WhitelistEndTime,
		StartTime:              contractRequest.StartTime,
		EndTime:                contractRequest.EndTime,
	}

	return t.PushContract(TXContract_ITOTriggerContractType, contract)
}

func (t *Transaction) addSetITOPrices(txArgs TXArgs) error {
	contractRequest := models.SetITOPricesTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	packInfo := make(map[string]*PackInfo)
	for key, packData := range contractRequest.PackInfo {
		newPacks := make([]*PackItem, len(packData.Packs))
		for i, pack := range packData.Packs {
			if pack.Amount <= 0 || pack.Price <= 0 {
				return common.ErrInvalidValue
			}

			newPacks[i] = &PackItem{
				Amount: pack.Amount,
				Price:  pack.Price,
			}
		}

		packInfo[key] = &PackInfo{Packs: newPacks}
	}

	contract := &SetITOPricesContract{
		AssetID:  []byte(contractRequest.KDA),
		PackInfo: packInfo,
	}

	return t.PushContract(TXContract_SetITOPricesContractType, contract)
}

func (t *Transaction) addBuy(txArgs TXArgs) error {
	contractRequest := models.BuyTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	id := []byte(contractRequest.ID)
	if BuyContract_EnumBuyType(contractRequest.BuyType) == BuyContract_MarketBuy {
		id, err = hex.DecodeString(contractRequest.ID)
		if err != nil {
			return errors.New("could not decode order id")
		}
	}

	contract := &BuyContract{
		BuyType:        BuyContract_EnumBuyType(contractRequest.BuyType),
		ID:             id,
		CurrencyID:     []byte(contractRequest.CurrencyID),
		Amount:         contractRequest.Amount,
		CurrencyAmount: contractRequest.CurrencyAmount,
	}

	return t.PushContract(TXContract_BuyContractType, contract)
}

func (t *Transaction) addSell(txArgs TXArgs) error {
	contractRequest := models.SellTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	marketplaceID, err := hex.DecodeString(contractRequest.MarketplaceID)
	if err != nil {
		return errors.New("could not decode marketplace id")
	}

	contract := &SellContract{
		MarketType:    SellContract_EnumMarketType(contractRequest.MarketType),
		MarketplaceID: marketplaceID,
		AssetID:       []byte(contractRequest.AssetID),
		CurrencyID:    []byte(contractRequest.CurrencyID),
		Price:         contractRequest.Price,
		ReservePrice:  contractRequest.ReservePrice,
		EndTime:       contractRequest.EndTime,
	}

	return t.PushContract(TXContract_SellContractType, contract)
}

func (t *Transaction) addCancelMarketOrder(txArgs TXArgs) error {
	contractRequest := models.CancelMarketOrderTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	orderID, err := hex.DecodeString(contractRequest.OrderID)
	if err != nil {
		return errors.New("could not decode order id")
	}

	contract := &CancelMarketOrderContract{
		OrderID: orderID,
	}

	return t.PushContract(TXContract_CancelMarketOrderContractType, contract)
}

func (t *Transaction) addCreateMarketplace(txArgs TXArgs) error {
	contractRequest := models.CreateMarketplaceTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	referralAddress := []byte("")
	if contractRequest.ReferralAddress != "" {
		referralAddress, err = txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.ReferralAddress)
		if err != nil {
			return errors.New("could not create reward address from provided param")
		}
	}

	contract := &CreateMarketplaceContract{
		Name:               []byte(contractRequest.Name),
		ReferralAddress:    referralAddress,
		ReferralPercentage: contractRequest.ReferralPercentage,
	}

	return t.PushContract(TXContract_CreateMarketplaceContractType, contract)
}

func (t *Transaction) addConfigMarketplace(txArgs TXArgs) error {
	contractRequest := models.ConfigMarketplaceTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	referralAddress := []byte("")
	if contractRequest.ReferralAddress != "" {
		referralAddress, err = txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.ReferralAddress)
		if err != nil {
			return errors.New("could not create reward address from provided param")
		}
	}

	marketplaceID, err := hex.DecodeString(contractRequest.MarketplaceID)
	if err != nil {
		return errors.New("could not decode marketplace id")
	}

	contract := &ConfigMarketplaceContract{
		MarketplaceID:      marketplaceID,
		Name:               []byte(contractRequest.Name),
		ReferralAddress:    referralAddress,
		ReferralPercentage: contractRequest.ReferralPercentage,
	}

	return t.PushContract(TXContract_ConfigMarketplaceContractType, contract)
}

func (t *Transaction) updateAccountPermission(txArgs TXArgs) error {
	contractRequest := models.UpdateAccountPermissionTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	permissions := make([]*AccPermission, 0)
	for _, p := range contractRequest.Permissions {
		signers := make([]*AccKey, 0)
		for _, s := range p.Signers {
			addr, err := txArgs.NodeHelper.GetAddressPCK().Decode(s.Address)
			if err != nil {
				return errors.New("could not create address address from provided param")
			}

			signers = append(signers, &AccKey{
				Address: addr,
				Weight:  s.Weight,
			})
		}

		operations, err := hex.DecodeString(p.Operations)
		if err != nil {
			return errors.New("error decoding operations")
		}

		_, exist := AccPermission_AccPermissionType_name[int32(p.Type)]
		if !exist {
			return errors.New("invalid permission type")
		}

		permissions = append(permissions, &AccPermission{
			Type:           AccPermission_AccPermissionType(p.Type),
			PermissionName: p.PermissionName,
			Threshold:      p.Threshold,
			Operations:     operations,
			Signers:        signers,
		})
	}

	contract := &UpdateAccountPermissionContract{
		Permissions: permissions,
	}

	return t.PushContract(TXContract_UpdateAccountPermissionContractType, contract)
}

// addDeposit -
func (t *Transaction) addDeposit(txArgs TXArgs) error {
	contractRequest := models.DepositTXRequest{}
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return err
	}

	contract := &DepositContract{
		DepositType: DepositContract_EnumDepositType(contractRequest.DepositType),
		ID:          []byte(contractRequest.KDA),
		CurrencyID:  []byte(contractRequest.CurrencyID),
		Amount:      contractRequest.Amount,
	}

	return t.PushContract(TXContract_DepositContractType, contract)
}

// Helper function to parse the smart contract request
func (t *Transaction) parseContractRequest(txArgs TXArgs) (models.SmartContractRequest, error) {
	var contractRequest models.SmartContractRequest
	err := json.Unmarshal(txArgs.Contract, &contractRequest)
	if err != nil {
		return contractRequest, err
	}

	return contractRequest, nil
}

func getAssetRoyalties(assetName string, value int64, txArgs TXArgs) (int64, int64, error) {
	kdaRoyalties := int64(0)
	klvRoyalties := int64(0)
	kda, err := txArgs.NodeHelper.GetAsset(assetName)
	if err != nil {
		return 0, 0, errors.New("invalid asset name provided")
	}

	if kda == nil {
		return 0, 0, errors.New("could not find asset")
	}

	if kda.Royalties != nil {
		if len(kda.Royalties.TransferPercentage) > 0 {
			kdaRoyalties, err = kda.GetTransferRoyaltyByAmount(value, txArgs.NodeHelper.GetForkController().KdaFpr())
			if err != nil {
				return 0, 0, err
			}
		}

		klvRoyalties = kda.Royalties.TransferFixed
	}

	return kdaRoyalties, klvRoyalties, nil
}

// Helper function to calculate royalties for a given asset and value
func (t *Transaction) calculateRoyalties(txArgs TXArgs, key string, value int64) (int64, int64, error) {

	asset, _, _, err := kdautils.ExtractAssetIDAndNonce([]byte(key))
	if err != nil {
		return 0, 0, errors.New("could not extract asset from callValue")
	}

	assetName := string(asset)

	// check asset for royalties
	if assetName != "" &&
		assetName != string(kdautils.KLVIdentifier) &&
		assetName != string(kdautils.KFIIdentifier) {

		return getAssetRoyalties(assetName, value, txArgs)
	}

	return 0, 0, nil
}

// Helper function to construct the call value map
func (t *Transaction) constructCallValue(txArgs TXArgs, contractRequest models.SmartContractRequest) (map[string]*CallValue, error) {
	callValue := make(map[string]*CallValue)

	for key, value := range contractRequest.CallValue {
		kdaRoyalties, klvRoyalties, err := t.calculateRoyalties(txArgs, key, value)
		if err != nil {
			return nil, err
		}

		callValue[key] = &CallValue{
			Amount:       value,
			KDARoyalties: kdaRoyalties,
			KLVRoyalties: klvRoyalties,
		}
	}

	return callValue, nil
}

// Helper function to validate and parse the smart contract address
func (t *Transaction) validateAndParseAddress(txArgs TXArgs, contractRequest models.SmartContractRequest) ([]byte, error) {
	if SmartContract_SCType(txArgs.Type) != SmartContract_SCDeploy && // #nosec G115
		len(contractRequest.Address) > txArgs.NodeHelper.GetEncodedAddressLength() {
		return nil, fmt.Errorf("%w for address", common.ErrInvalidAddressLength)
	}

	if len(contractRequest.Address) == 0 {
		return make([]byte, 0), nil
	}

	parsedAddress, err := txArgs.NodeHelper.GetAddressPCK().Decode(contractRequest.Address)
	if err != nil {
		return nil, common.ErrInvalidReceiverAddress
	}

	return parsedAddress, nil
}

// addSmartContract -
func (t *Transaction) addSmartContract(txArgs TXArgs) error {
	contractRequest, err := t.parseContractRequest(txArgs)
	if err != nil {
		return err
	}

	parsedAddress, err := t.validateAndParseAddress(txArgs, contractRequest)
	if err != nil {
		return err
	}

	callValue, err := t.constructCallValue(txArgs, contractRequest)
	if err != nil {
		return err
	}

	contract := &SmartContract{
		Type:      SmartContract_SCType(contractRequest.SCType),
		Address:   parsedAddress,
		CallValue: callValue,
	}

	return t.PushContract(TXContract_SmartContractType, contract)
}

// GetContracts -
func (t *Transaction) GetContracts() []*TXContract {
	return t.RawData.Contract
}

// ValidatePermission - for each contract check if TXType match permission
func (t *Transaction) ValidatePermission(permission []byte) error {
	if len(permission) > core.MaxOperationsSize {
		return common.ErrInvalidPermission
	}
	if t == nil || t.RawData == nil || len(t.RawData.Contract) == 0 {
		return common.ErrInvalidContract
	}
	for _, c := range t.RawData.Contract {
		value := int32(c.Type)
		base := value / 8
		index := value % 8
		// #nosec G115
		if int32(len(permission)) <= base ||
			permission[base]&(1<<index) == 0 {
			return common.ErrNoPermission
		}
	}

	return nil
}

// GetTransferContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetTransferContract() (*TransferContract, error) {
	if t.Type != TXContract_TransferContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &TransferContract{}

	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}

	return tc, nil
}

// GetCreateAssetContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetCreateAssetContract() (*CreateAssetContract, error) {
	if t.Type != TXContract_CreateAssetContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &CreateAssetContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}

	return tc, nil
}

// GetAssetTriggerContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetAssetTriggerContract() (*AssetTriggerContract, error) {
	if t.Type != TXContract_AssetTriggerContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &AssetTriggerContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetCreateValidatorContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetCreateValidatorContract() (*CreateValidatorContract, error) {
	if t.Type != TXContract_CreateValidatorContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &CreateValidatorContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})

	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetValidatorConfigContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetValidatorConfigContract() (*ValidatorConfigContract, error) {
	if t.Type != TXContract_ValidatorConfigContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &ValidatorConfigContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetFreezeContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetFreezeContract() (*FreezeContract, error) {
	if t.Type != TXContract_FreezeContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &FreezeContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetUnfreezeContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetUnfreezeContract() (*UnfreezeContract, error) {
	if t.Type != TXContract_UnfreezeContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &UnfreezeContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetDelegateContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetDelegateContract() (*DelegateContract, error) {
	if t.Type != TXContract_DelegateContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &DelegateContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetUndelegateContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetUndelegateContract() (*UndelegateContract, error) {
	if t.Type != TXContract_UndelegateContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &UndelegateContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetWithdrawContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetWithdrawContract() (*WithdrawContract, error) {
	if t.Type != TXContract_WithdrawContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &WithdrawContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetClaimContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetClaimContract() (*ClaimContract, error) {
	if t.Type != TXContract_ClaimContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &ClaimContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetUnjailContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetUnjailContract() (*UnjailContract, error) {
	if t.Type != TXContract_UnjailContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &UnjailContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetSetAccountNameContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetSetAccountNameContract() (*SetAccountNameContract, error) {
	if t.Type != TXContract_SetAccountNameContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &SetAccountNameContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetProposalContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetProposalContract() (*ProposalContract, error) {
	if t.Type != TXContract_ProposalContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &ProposalContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetVoteContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetVoteContract() (*VoteContract, error) {
	if t.Type != TXContract_VoteContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &VoteContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetConfigITOContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetConfigITOContract() (*ConfigITOContract, error) {
	if t.Type != TXContract_ConfigITOContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &ConfigITOContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetITOTriggerContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetITOTriggerContract() (*ITOTriggerContract, error) {
	if t.Type != TXContract_ITOTriggerContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &ITOTriggerContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetSetITOPricesContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetSetITOPricesContract() (*SetITOPricesContract, error) {
	if t.Type != TXContract_SetITOPricesContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &SetITOPricesContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetBuyContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetBuyContract() (*BuyContract, error) {
	if t.Type != TXContract_BuyContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &BuyContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetSellContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetSellContract() (*SellContract, error) {
	if t.Type != TXContract_SellContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &SellContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetCancelMarketOrderContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetCancelMarketOrderContract() (*CancelMarketOrderContract, error) {
	if t.Type != TXContract_CancelMarketOrderContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &CancelMarketOrderContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetCreateMarketplaceContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetCreateMarketplaceContract() (*CreateMarketplaceContract, error) {
	if t.Type != TXContract_CreateMarketplaceContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &CreateMarketplaceContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetConfigMarketplaceContract unmarshal contract parameters and return a transfer contract
func (t *TXContract) GetConfigMarketplaceContract() (*ConfigMarketplaceContract, error) {
	if t.Type != TXContract_ConfigMarketplaceContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &ConfigMarketplaceContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetUpdateAccountPermissionContract unmarshal contract parameters and return a UpdateAccountPermission contract
func (t *TXContract) GetUpdateAccountPermissionContract() (*UpdateAccountPermissionContract, error) {
	if t.Type != TXContract_UpdateAccountPermissionContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &UpdateAccountPermissionContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetDepositContract unmarshal contract parameters and return a deposit contract
func (t *TXContract) GetDepositContract() (*DepositContract, error) {
	if t.Type != TXContract_DepositContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &DepositContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetSmartContract unmarshal contract parameters and return a smart contract
func (t *TXContract) GetSmartContract() (*SmartContract, error) {
	if t.Type != TXContract_SmartContractType {
		return nil, common.ErrInvalidContract
	}
	tc := &SmartContract{}
	// Get the message name which is compatible with the golang proto.protoType registry
	err := anypb.UnmarshalTo(t.Parameter, tc, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	return tc, nil
}

func (t *Transaction) Size() int {
	return t.GetSize()
}

func (t *Transaction) CheckIntegrity() error {
	panic("NOT IMPLEMENTED")
}

func (t *Transaction) AddSignature(signature []byte) {
	// check if signature already exists
	for _, sig := range t.Signature {
		if bytes.Equal(sig, signature) {
			return
		}
	}
	t.Signature = append(t.Signature, signature)
}
