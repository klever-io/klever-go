package transaction

import (
	"bytes"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

type ReceiptType int8

const defaultTXContractID = int(-1)

const (
	Transfer ReceiptType = iota
	CreateKDA
	UpdateKDA
	Freeze
	Unfreeze
	Proposal
	ProposalVote
	Delegate
	ConfigITO
	SetITOPrices
	CreateMarketplace
	ConfigMarketplace
	UpdateValidator
	UpdateAccountPermission
	KAppTransfer //deprecated, create a new one in this spot if needed
	Sell
	Buy
	Claim
	Withdraw
	SignedBy
	UpdateMetadata
	Deposit
	UpdateKDAPool
	UpdateITO
	CancelOrder
	SCTrigger
)

var receiptTypeStrings = map[ReceiptType]string{
	Transfer:                "Transfer",
	CreateKDA:               "CreateKDA",
	UpdateKDA:               "UpdateKDA",
	Freeze:                  "Freeze",
	Unfreeze:                "Unfreeze",
	Proposal:                "Proposal",
	ProposalVote:            "ProposalVote",
	Delegate:                "Delegate",
	ConfigITO:               "ConfigITO",
	SetITOPrices:            "SetITOPrices",
	CreateMarketplace:       "CreateMarketplace",
	ConfigMarketplace:       "ConfigMarketplace",
	UpdateValidator:         "UpdateValidator",
	UpdateAccountPermission: "UpdateAccountPermission",
	KAppTransfer:            "KAppTransfer", //deprecated, create a new one in this spot if needed
	Sell:                    "Sell",
	Buy:                     "Buy",
	Claim:                   "Claim",
	Withdraw:                "Withdraw",
	SignedBy:                "SignedBy",
	UpdateMetadata:          "UpdateMetadata",
	Deposit:                 "Deposit",
	UpdateKDAPool:           "UpdateKDAPool",
	UpdateITO:               "UpdateITO",
	CancelOrder:             "CancelOrder",
	SCTrigger:               "SCTrigger",
}

func NewReceipt(receiptType ReceiptType, contractID int, params ...[]byte) *transaction.Transaction_Receipt {
	data := make([][]byte, len(params)+1)
	data[0] = []byte{byte(receiptType), byte(contractID)}

	for i, param := range params {
		data[i+1] = param
	}

	return &transaction.Transaction_Receipt{
		Data: data,
	}
}

func (rt ReceiptType) String() string {
	return receiptTypeStrings[rt]
}

func AssetGainReceipt(isForkClaimKFI bool, fromAsset []byte, assetID []byte) []byte {
	if isForkClaimKFI && bytes.Equal(assetID, kdautils.KFIIdentifier) && bytes.Equal(fromAsset, kdautils.KFIIdentifier) {
		return kdautils.KLVIdentifier
	}

	return assetID
}

func AssetTypeReceipt(isForkClaimKFI bool, fromAsset []byte, assetID []byte, asset *kapps.KDAData) []byte {
	if isForkClaimKFI && bytes.Equal(assetID, kdautils.KFIIdentifier) && bytes.Equal(fromAsset, kdautils.KFIIdentifier) {
		return []byte{byte(kapps.KDAData_Fungible)}
	}

	return []byte{byte(asset.AssetType)}
}
