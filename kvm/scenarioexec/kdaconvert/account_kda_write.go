package kdaconvert

import (
	"bytes"
	"math/big"
	"strconv"

	worldmock "github.com/klever-io/klever-go/kvm/mock/world"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
)

func RoleListToKDARole(roles []string) *kapps.RolesData {
	HasRoleMint := false
	HasRoleSetITOPrices := false
	HasRoleDeposit := false
	HasRoleTransfer := false
	for _, scenRoles := range roles {
		if scenRoles == "KDARoleMint" {
			HasRoleMint = true
		} else if scenRoles == "KDARoleSetITOPrices" {
			HasRoleSetITOPrices = true
		} else if scenRoles == "KDARoleDeposit" {
			HasRoleDeposit = true
		} else if scenRoles == "KDARoleTransfer" {
			HasRoleTransfer = true
		}
	}
	return &kapps.RolesData{
		HasRoleMint:         HasRoleMint,
		HasRoleSetITOPrices: HasRoleSetITOPrices,
		HasRoleDeposit:      HasRoleDeposit,
		HasRoleTransfer:     HasRoleTransfer,
	}
}

func KDARoleToRoleList(roles *kapps.RolesData) [][]byte {
	roleList := make([][]byte, 0)
	if roles.HasRoleMint {
		roleList = append(roleList, []byte("KDARoleMint"))
	}
	if roles.HasRoleSetITOPrices {
		roleList = append(roleList, []byte("KDARoleSetITOPrices"))
	}
	if roles.HasRoleDeposit {
		roleList = append(roleList, []byte("KDARoleDeposit"))
	}
	if roles.HasRoleTransfer {
		roleList = append(roleList, []byte("KDARoleTransfer"))
	}
	return roleList
}

// WriteScenariosKDAToStorage writes the Scenarios KDA data to the provided storage map
func WriteScenariosKDAToStorage(kdaData []*scenjsonmodel.KDAData, destination state.UserAccountHandler) error {
	for _, scenKDAData := range kdaData {

		for _, instance := range scenKDAData.Instances {
			assetID := scenKDAData.TokenIdentifier.Value
			if assetID == nil {
				assetID = kdautils.KLVIdentifier
			}

			if instance.Balance.Value == nil {
				instance.Balance.Value = big.NewInt(0)
			}

			userKDA := kapps.UserKDA{
				Balance:  instance.Balance.Value.Int64(),
				Metadata: instance.Attributes.Value,
			}

			nonce := []byte(strconv.FormatUint(instance.Nonce.Value, 10))
			err := destination.SetUserKDA(assetID, nonce, &userKDA)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func SetMultiKDARoles(destination state.UserAccountHandler, scenData []*scenjsonmodel.KDAData, world *worldmock.MockWorld) error {
	for _, kdaData := range scenData {
		err := SetKDARoles(destination, kdaData, world)
		if err != nil {
			return err
		}
	}
	return nil
}

func SetKDARoles(destination state.UserAccountHandler, scenData *scenjsonmodel.KDAData, world *worldmock.MockWorld) error {
	kda, err := world.GetKDAData(scenData.TokenIdentifier.Value, nil)
	if err != nil {
		return err
	}
	roles := make([]*kapps.RolesData, 0)
	for _, role := range kda.Roles {
		// remove any existent role for the address, while keeping roles from other addresses
		if !bytes.Equal(role.Address, destination.AddressBytes()) {
			roles = append(roles, role)
		}
	}
	userRole := RoleListToKDARole(scenData.Roles)
	// skip if no roles
	if userRole.HasRoleMint || userRole.HasRoleSetITOPrices || userRole.HasRoleDeposit || userRole.HasRoleTransfer {
		userRole.Address = destination.AddressBytes()
		roles = append(kda.Roles, userRole)
	}
	kda.Roles = roles
	return world.SetKDAData(scenData.TokenIdentifier.Value, nil, kda)
}
