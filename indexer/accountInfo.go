package indexer

import (
	"encoding/hex"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	dataState "github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
)

// buildAccountInfo is the single source of truth for AccountInfo payloads consumed
// by both the elastic indexer and the websocket dispatcher. Keep this in sync — any
// new field must appear here, not in a caller-local helper.
func buildAccountInfo(
	addressPubkeyConverter core.PubkeyConverter,
	kappsController kapp.KAppController,
	userAccount dataState.UserAccountHandler,
	blockTimestamp int64,
) (*data.AccountInfo, error) {
	userKDA, err := userAccount.GetUserKDA(kdautils.KLVIdentifier, nil, true)
	if err != nil {
		return nil, err
	}

	return &data.AccountInfo{
		Address:         addressPubkeyConverter.Encode(userAccount.AddressBytes()),
		Nonce:           userAccount.GetNonce(),
		Name:            string(userAccount.GetName()),
		RootHash:        hex.EncodeToString(userAccount.GetRootHash()),
		Balance:         userAccount.GetBalance(kdautils.KLVIdentifier, true),
		FrozenBalance:   userKDA.FrozenBalance,
		UnfrozenBalance: calculateUnfrozenBalance(userKDA.Buckets),
		Allowance:       getAllowanceWithPendingRewards(kappsController, userAccount),
		Permissions:     convertPermissions(addressPubkeyConverter, userAccount.GetPermissions()),
		Timestamp:       toMilliseconds(blockTimestamp),
		UpdatedAt:       toMilliseconds(blockTimestamp),
		CodeHash:        hex.EncodeToString(userAccount.GetCodeHash()),
		CodeMetadata:    hex.EncodeToString(userAccount.GetCodeMetadata()),
		Foundation:      false,
	}, nil
}

// convertPermissions converts user account permissions to the indexer payload type.
func convertPermissions(
	addressPubkeyConverter core.PubkeyConverter,
	perms []*dataState.Permission,
) []data.Permissions {
	permissions := make([]data.Permissions, 0, len(perms))
	for _, p := range perms {
		keys := make([]data.PermissionKey, 0, len(p.Signers))
		for _, k := range p.Signers {
			keys = append(keys, data.PermissionKey{
				Address: addressPubkeyConverter.Encode(k.Address),
				Weight:  k.Weight,
			})
		}
		permissions = append(permissions, data.Permissions{
			ID:             p.ID,
			Type:           int32(p.Type),
			PermissionName: p.PermissionName,
			Threshold:      p.Threshold,
			Operations:     hex.EncodeToString(p.Operations),
			Signers:        keys,
		})
	}
	return permissions
}

// calculateUnfrozenBalance sums the value of all unstaked buckets.
func calculateUnfrozenBalance(buckets map[string]*kapps.UserBucket) int64 {
	unfrozenBalance := int64(0)
	for _, bucket := range buckets {
		if bucket.UnstakedEpoch != core.DefaultUnstakedEpoch {
			unfrozenBalance += bucket.Value
		}
	}
	return unfrozenBalance
}

// getAllowanceWithPendingRewards returns the user allowance including V2 pending
// validator rewards. A nil controller (or missing validators kapp) returns the
// raw allowance, matching prior behavior on both code paths.
func getAllowanceWithPendingRewards(
	kappsController kapp.KAppController,
	userAccount dataState.UserAccountHandler,
) int64 {
	allowance := userAccount.GetAllowance()
	if check.IfNil(kappsController) {
		return allowance
	}

	validatorsKApp := kappsController.GetValidatorsKApp()
	if check.IfNil(validatorsKApp) {
		return allowance
	}

	pendingRewards, err := validatorsKApp.GetPendingRewards(userAccount.AddressBytes())
	if err == nil && pendingRewards > 0 {
		allowance += pendingRewards
	}
	return allowance
}
