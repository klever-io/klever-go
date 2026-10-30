package kapps

import (
	bytes "bytes"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/tools"
)

// Define the pattern to check against (with ID set to 0x00 since it's variable)
var expectedPattern = []byte{
	// Prefix zeros
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 9 bytes
	// Identifier
	0x01,
	// Suffix zeros
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 19 bytes
	// ID byte (placeholder)
	0x00,
	// Suffix
	0xFF, 0xFF, // 2 bytes
}

func (kda *KDAData) IsTransferAllowed(sender []byte, destination []byte) bool {
	if kda.CanSenderTransfer(sender) {
		return true
	}

	destinationRoles, err := kda.GetRoleByAddress(destination)
	if err == nil && destinationRoles.HasRoleDeposit {
		return true
	}

	return false
}

// CanSenderTransfer reports whether sender may move the asset to any destination: always when the
// asset does not limit transfers, otherwise only with the transfer role (the owner and admin hold it).
func (kda *KDAData) CanSenderTransfer(sender []byte) bool {
	if !kda.GetProperties().GetLimitTransfer() {
		return true
	}

	senderRoles, err := kda.GetRoleByAddress(sender)

	return err == nil && senderRoles.HasRoleTransfer
}

// IsHeldNFT reports whether x, an NFT holder record, holds the NFT: every mint stores it with
// Balance 1. A record spent through the fungible balance path keeps Balance 0 and, when it also
// carries metadata, stays in the trie, where it must not pass as held (KLC-2835).
func (x *UserKDA) IsHeldNFT() bool {
	return x.GetBalance() == 1
}

func MatchesAddress(expected []byte, address []byte) bool {
	return len(address) > 0 && bytes.Equal(expected, address)
}

func (kda *KDAData) IsOwner(address []byte) bool {
	return MatchesAddress(kda.OwnerAddress, address)
}

func (kda *KDAData) IsOwnerOrAdmin(address []byte) bool {
	return kda.IsOwner(address) || MatchesAddress(kda.AdminAddress, address)
}

// GetRoleByAddress return the role of the given address into the kda or error if doesnt exist
func (kda *KDAData) GetRoleByAddress(address []byte) (*RolesData, error) {
	if kda.IsOwnerOrAdmin(address) {
		return &RolesData{
			Address:             address,
			HasRoleMint:         true,
			HasRoleSetITOPrices: true,
			HasRoleDeposit:      true,
			HasRoleTransfer:     true,
		}, nil
	}

	for _, role := range kda.Roles {
		if bytes.Equal(role.Address, address) {
			return role, nil
		}
	}

	return nil, common.ErrRoleNotFound
}

func (kda *KDAData) GetTransferRoyaltyByAmount(amount int64, isKdaFprFork bool, isEnableSmartContractsFork bool) (int64, error) {
	if len(kda.Royalties.TransferPercentage) == 0 {
		return 0, common.ErrInvalidValue
	}

	//old flow
	if !isKdaFprFork {
		return kda.computeOldFlowRoyalty(amount, isEnableSmartContractsFork)
	}

	//new flow
	return kda.computeNewFlowRoyalty(amount, isEnableSmartContractsFork)
}

func (kda *KDAData) computeNewFlowRoyalty(amount int64, isEnableSmartContractsFork bool) (int64, error) {
	royaltySum := int64(0)
	for i, royalty := range kda.Royalties.TransferPercentage {
		if amount < royalty.Amount {
			royaltyCalc, err := tools.ComputePercentageI64(amount, int64(royalty.Percentage), isEnableSmartContractsFork)
			if err != nil {
				return 0, err
			}
			royaltySum += royaltyCalc
			break
		}

		royaltyCalc, err := tools.ComputePercentageI64(royalty.Amount, int64(royalty.Percentage), isEnableSmartContractsFork)
		if err != nil {
			return 0, err
		}

		amount -= royalty.Amount
		royaltySum += royaltyCalc

		if i == len(kda.Royalties.TransferPercentage)-1 && amount > 0 {
			lastRoyalty := kda.Royalties.TransferPercentage[i]
			royaltyCalc, err := tools.ComputePercentageI64(amount, int64(lastRoyalty.Percentage), isEnableSmartContractsFork)
			if err != nil {
				return 0, err
			}

			royaltySum += royaltyCalc
		}
	}

	return royaltySum, nil
}

func (kda *KDAData) computeOldFlowRoyalty(amount int64, isEnableSmartContractsFork bool) (int64, error) {
	chosenRoyalty := kda.Royalties.TransferPercentage[len(kda.Royalties.TransferPercentage)-1]

	for _, royalty := range kda.Royalties.TransferPercentage {
		if amount > royalty.Amount {
			continue
		}

		chosenRoyalty = royalty
		break
	}

	splitToPay, err := tools.ComputePercentageI64(amount, int64(chosenRoyalty.Percentage), isEnableSmartContractsFork)
	if err != nil {
		return 0, err
	}

	return splitToPay, nil
}

func IsKAppAddress(address []byte) bool {
	if len(address) != 32 {
		return false
	}

	// Compare prefix
	if !bytes.Equal(address[:29], expectedPattern[:29]) {
		return false
	}

	// Compare suffix
	return bytes.Equal(address[30:], expectedPattern[30:])
}
