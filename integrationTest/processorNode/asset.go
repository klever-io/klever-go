package processorNode

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/kapps"
)

func (n *ProcessorNode) GetAsset(assetId []byte) (*kapps.KDAData, error) {

	assetKDAaccount, err := n.AccountsCacher.GetExistingKapp(kapps.KDAKAppAddress)
	if err != nil {
		return nil, err
	}

	key := kdautils.ToKDAKey(assetId, nil)

	kdaBytes, err := assetKDAaccount.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(kdaBytes) == 0 {
		return nil, common.ErrEmptyString
	}

	kda := &kapps.KDAData{}
	err = n.InternalMarshalizer.Unmarshal(kda, kdaBytes)
	if err != nil {
		return nil, err
	}

	return kda, nil
}

func (n *ProcessorNode) GetITO(assetId []byte) (*kapps.ITOData, error) {

	itoKappAccount, err := n.AccountsCacher.GetExistingKapp(kapps.ITOKAppAddress)
	if err != nil {
		return nil, err
	}

	key := kdautils.ToITOKey(assetId)

	itoBytes, err := itoKappAccount.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(itoBytes) == 0 {
		return nil, common.ErrEmptyString
	}

	ito := &kapps.ITOData{}
	err = n.InternalMarshalizer.Unmarshal(ito, itoBytes)
	if err != nil {
		return nil, err
	}

	return ito, nil
}

func (n *ProcessorNode) GetITOWhitelistByAddress(assetId []byte, address string) (*kapps.WhitelistData, error) {

	itoKappAccount, err := n.AccountsCacher.GetExistingKapp(kapps.ITOKAppAddress)
	if err != nil {
		return nil, err
	}

	key := kdautils.ToITOWhitelistKey(assetId, address)

	itoWhitelistBytes, err := itoKappAccount.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(itoWhitelistBytes) == 0 {
		newWhitelist := &kapps.WhitelistData{
			Limit: 0,
		}
		return newWhitelist, common.ErrNotFoundInKApp
	}

	itoWhitelist := &kapps.WhitelistData{}
	err = n.InternalMarshalizer.Unmarshal(itoWhitelist, itoWhitelistBytes)
	if err != nil {
		return nil, err
	}

	return itoWhitelist, nil
}
