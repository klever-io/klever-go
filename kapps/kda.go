package kapps

import (
	bytes "bytes"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
)

func (kda *KDAData) IsTransferAllowed(sender []byte, destination []byte) bool {
	if !kda.Properties.LimitTransfer {
		return true
	}

	senderRoles, err := kda.GetRoleByAddress(sender)
	if err == nil && senderRoles.HasRoleTransfer {
		return true
	}

	destinationRoles, err := kda.GetRoleByAddress(destination)
	if err == nil && destinationRoles.HasRoleDeposit {
		return true
	}

	return false
}

// GetRoleByAddress return the role of the given address into the kda or error if doesnt exist
func (kda *KDAData) GetRoleByAddress(address []byte) (*RolesData, error) {
	if bytes.Equal(kda.OwnerAddress, address) {
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

func (kda *KDAData) GetTransferRoyaltyByAmount(amount int64, isKdaFprFork bool) (int64, error) {
	if len(kda.Royalties.TransferPercentage) == 0 {
		return 0, common.ErrInvalidValue
	}

	//old flow
	if !isKdaFprFork {
		chosenRoyalty := kda.Royalties.TransferPercentage[len(kda.Royalties.TransferPercentage)-1]
		for _, royalty := range kda.Royalties.TransferPercentage {
			if amount > royalty.Amount {
				continue
			}

			chosenRoyalty = royalty
			break
		}

		return int64(float64(amount) * float64(chosenRoyalty.Percentage) / float64(core.HundredPercent)), nil
	}

	//new flow
	royaltySum := int64(0)
	for i, royalty := range kda.Royalties.TransferPercentage {
		if amount < royalty.Amount {
			royaltySum += int64(float64(amount) * float64(royalty.Percentage) / float64(core.HundredPercent))
			break
		}

		royaltyCalc := int64(float64(royalty.Amount) * float64(royalty.Percentage) / float64(core.HundredPercent))
		amount -= royalty.Amount
		royaltySum += royaltyCalc

		if i == len(kda.Royalties.TransferPercentage)-1 && amount > 0 {
			lastRoyalty := kda.Royalties.TransferPercentage[i]
			royaltySum += int64(float64(amount) * float64(lastRoyalty.Percentage) / float64(core.HundredPercent))
		}
	}

	return royaltySum, nil
}
