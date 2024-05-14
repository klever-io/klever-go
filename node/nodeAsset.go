package node

import (
	"fmt"
	"strings"

	"github.com/klever-io/klever-go/common"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
)

// GetAsset will return asset details for a given assetID
func (n *Node) GetAsset(assetID string) (*kapps.KDAData, error) {
	if check.IfNil(n.kapps) {
		return &kapps.KDAData{}, common.ErrNilKAppAccountsAdapter
	}

	acnt, err := n.kapps.LoadAccount(kapps.KDAKAppAddress)
	if err != nil {
		return &kapps.KDAData{}, err
	}

	// remove nonce if needed
	tokenID, _, _, err := kdautils.ExtractAssetIDAndNonce([]byte(assetID))
	if err != nil {
		return &kapps.KDAData{}, err
	}

	kapp, ok := acnt.(state.KAppAccountHandler)
	if !ok {
		return &kapps.KDAData{}, common.ErrWrongTypeAssertion
	}

	key := kdautils.ToKDAKey([]byte(tokenID), nil)

	assetWrp, err := kapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return &kapps.KDAData{}, common.ErrAssetNotFound
	}

	asset := &kapps.KDAData{}
	err = n.internalMarshalizer.Unmarshal(asset, assetWrp)

	return asset, err
}

// GetNFT will return asset details for a given address
func (n *Node) GetNFT(owner string, id string) (*kapps.UserKDA, *kapps.KDAData, error) {
	ids := strings.Split(id, "/")
	if len(ids) == 3 {
		ids = ids[1:] // remove initial slash if any
	}

	if len(ids) != 2 {
		return &kapps.UserKDA{}, &kapps.KDAData{}, fmt.Errorf("%w: %s", common.ErrAssetIDInvalid, id)
	}

	kdaData, err := n.GetAsset(ids[0])
	if err != nil {
		return &kapps.UserKDA{}, &kapps.KDAData{}, err
	}
	if kdaData == nil ||
		string(kdaData.ID) != ids[0] ||
		!n.TokeTypeHasNonce(kdaData.AssetType) {
		return &kapps.UserKDA{}, &kapps.KDAData{}, fmt.Errorf("%w: %s", common.ErrAssetIDInvalid, ids[0])
	}

	acc, err := n.GetAccount(owner)
	if err != nil {
		return &kapps.UserKDA{}, &kapps.KDAData{}, err
	}

	userKDA, err := acc.GetUserKDA([]byte(ids[0]), []byte(ids[1]), n.forkController.EnableSmartContracts())
	if err != nil {
		return &kapps.UserKDA{}, &kapps.KDAData{}, err
	}

	if userKDA == nil || userKDA.Balance != 1 {
		return &kapps.UserKDA{}, &kapps.KDAData{}, fmt.Errorf("%w: %s", common.ErrAssetIDInvalid, id)
	}

	return userKDA, kdaData, err
}

// GetKDAFeePool will return pool details for a given asset
func (n *Node) GetKDAFeePool(id string) (*kdafeespool.KDAFeesPoolData, error) {
	kdaPool := &kdafeespool.KDAFeesPoolData{}

	kdaData, err := n.GetAsset(id)
	if err != nil {
		return kdaPool, err
	}

	acnt, err := n.kapps.LoadAccount(kapps.KDAFeesPoolKAppAddress)
	if err != nil {
		return kdaPool, err
	}

	kdaPoolKapp, ok := acnt.(state.KAppAccountHandler)
	if !ok {
		return kdaPool, common.ErrWrongTypeAssertion
	}

	poolBytes, err := kdaPoolKapp.DataTrieTracker().RetrieveValue([]byte(kdaData.ID))
	if err != nil {
		return kdaPool, err
	}

	err = n.internalMarshalizer.Unmarshal(kdaPool, poolBytes)

	return kdaPool, err
}

func (n *Node) TokeTypeHasNonce(tokenType kapps.KDAData_EnumAssetType) bool {
	if n.forkController.EnableSmartContracts() {
		return tokenType == kapps.KDAData_NonFungible ||
			tokenType == kapps.KDAData_SemiFungible
	}

	return tokenType == kapps.KDAData_NonFungible
}
