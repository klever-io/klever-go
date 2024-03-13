package node

import (
	"encoding/hex"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
)

// GetMarketplace will return marketplace details for a given marketplaceId
func (n *Node) GetMarketplace(marketplaceId string) (*api.Marketplace, error) {
	if check.IfNil(n.kapps) {
		return &api.Marketplace{}, common.ErrNilKAppAccountsAdapter
	}

	acnt, err := n.kapps.LoadAccount(kapps.MarketKAppAddress)
	if err != nil {
		return &api.Marketplace{}, err
	}

	kapp, ok := acnt.(state.KAppAccountHandler)
	if !ok {
		return &api.Marketplace{}, common.ErrWrongTypeAssertion
	}

	marketplaceIDBytes, err := hex.DecodeString(marketplaceId)
	if err != nil {
		return &api.Marketplace{}, err
	}

	key := kdautils.ToMarketplaceKey(marketplaceIDBytes)

	marketplaceWrp, err := kapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return &api.Marketplace{}, common.ErrMarketplaceNotFound
	}

	marketplace := &kapps.Marketplace{}
	err = n.internalMarshalizer.Unmarshal(marketplace, marketplaceWrp)
	if err != nil {
		return &api.Marketplace{}, err
	}

	apiMarketplace, err := n.convertMaketplaceToAPIStruct(marketplace)
	if err != nil {
		return &api.Marketplace{}, err
	}

	return apiMarketplace, err
}

func (n *Node) convertMaketplaceToAPIStruct(marketplace *kapps.Marketplace) (*api.Marketplace, error) {
	return &api.Marketplace{
		ID:                 hex.EncodeToString(marketplace.ID),
		Name:               string(marketplace.GetName()),
		OwnerAddress:       n.addressPubkeyConverter.Encode(marketplace.OwnerAddress),
		ReferralAddress:    n.addressPubkeyConverter.Encode(marketplace.ReferralAddress),
		ReferralPercentage: marketplace.ReferralPercentage,
	}, nil
}
