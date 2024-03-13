package workItems

import (
	"encoding/hex"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
)

type itemAsset struct {
	indexer  saveAssetIndexer
	KDAsData []*kapps.KDAData

	addressPubkeyConverter core.PubkeyConverter
}

// NewItemAsset will create a new instance of ItemAsset
func NewItemAsset(
	indexer saveAssetIndexer,
	KDAsData []*kapps.KDAData,
	addressPubkeyConverter core.PubkeyConverter,
) WorkItemHandler {
	return &itemAsset{
		indexer:                indexer,
		KDAsData:               KDAsData,
		addressPubkeyConverter: addressPubkeyConverter,
	}
}

// Save will save information about an account
func (wia *itemAsset) Save() error {

	var assets []*data.Asset
	for _, kda := range wia.KDAsData {

		propertiesInfo := &data.PropertiesInfo{
			CanFreeze:      kda.Properties.CanFreeze,
			CanWipe:        kda.Properties.CanWipe,
			CanPause:       kda.Properties.CanPause,
			CanMint:        kda.Properties.CanMint,
			CanBurn:        kda.Properties.CanBurn,
			CanChangeOwner: kda.Properties.CanChangeOwner,
			CanAddRoles:    kda.Properties.CanAddRoles,
			LimitTransfer:  kda.Properties.LimitTransfer,
		}

		attributesInfo := &data.AttributesInfo{
			IsPaused:                 kda.Attributes.IsPaused,
			IsNFTMintStopped:         kda.Attributes.IsNFTMintStopped,
			IsRoyaltiesChangeStopped: kda.Attributes.IsRoyaltiesChangeStopped,
		}

		royaltiesAddress := ""
		if len(kda.Royalties.Address) > 0 {
			royaltiesAddress = wia.addressPubkeyConverter.Encode(kda.Royalties.Address)
		}
		royaltiesInfo := &data.RoyaltiesInfo{
			Address:            royaltiesAddress,
			TransferPercentage: make([]*data.RoyaltyDataInfo, 0),
			TransferFixed:      kda.Royalties.TransferFixed,
			MarketPercentage:   kda.Royalties.MarketPercentage,
			MarketFixed:        kda.Royalties.MarketFixed,
			ITOPercentage:      kda.Royalties.ITOPercentage,
			ITOFixed:           kda.Royalties.ITOFixed,
		}

		for _, royaltyData := range kda.Royalties.TransferPercentage {
			royaltiesInfo.TransferPercentage = append(royaltiesInfo.TransferPercentage, &data.RoyaltyDataInfo{
				Amount:     royaltyData.Amount,
				Percentage: royaltyData.Percentage,
			})
		}

		for address, splitRoyaltiesInfo := range kda.Royalties.SplitRoyalties {
			var convertedAddress string
			decodedAddress, err := hex.DecodeString(address)
			if err != nil {
				log.Error("can not decode address")
				convertedAddress = address
			} else {
				convertedAddress = wia.addressPubkeyConverter.Encode(decodedAddress)
			}

			royaltiesInfo.SplitRoyalties = append(royaltiesInfo.SplitRoyalties, &data.RoyaltySplitInfo{
				Address:                   convertedAddress,
				PercentTransferPercentage: splitRoyaltiesInfo.PercentTransferPercentage,
				PercentTransferFixed:      splitRoyaltiesInfo.PercentTransferFixed,
				PercentMarketPercentage:   splitRoyaltiesInfo.PercentMarketPercentage,
				PercentMarketFixed:        splitRoyaltiesInfo.PercentMarketFixed,
				PercentITOPercentage:      splitRoyaltiesInfo.PercentITOPercentage,
				PercentITOFixed:           splitRoyaltiesInfo.PercentITOFixed,
			})
		}

		var rolesInfo []*data.RolesInfo
		for _, role := range kda.Roles {
			roleInfo := &data.RolesInfo{
				Address:             wia.addressPubkeyConverter.Encode(role.Address),
				HasRoleMint:         role.HasRoleMint,
				HasRoleSetITOPrices: role.HasRoleSetITOPrices,
			}
			rolesInfo = append(rolesInfo, roleInfo)
		}

		owner := ""
		if len(kda.OwnerAddress) > 0 {
			owner = wia.addressPubkeyConverter.Encode(kda.OwnerAddress)
		}

		uris := kda.GetURIs()
		var convertedURIs []*data.URI
		for key, value := range uris {
			convertedURIs = append(convertedURIs, &data.URI{
				Key:   key,
				Value: value,
			})
		}

		initialAssetPermissionsState := false
		asset := &data.Asset{
			AssetID:           string(kda.ID),
			AssetType:         kda.AssetType.String(),
			Name:              string(kda.Name),
			Ticker:            string(kda.Ticker),
			OwnerAddress:      owner,
			Logo:              kda.GetLogo(),
			URIs:              convertedURIs,
			Precision:         kda.Precision,
			InitialSupply:     kda.InitialSupply,
			CirculatingSupply: kda.CirculatingSupply,
			MaxSupply:         kda.MaxSupply,
			MintedValue:       kda.MintedValue,
			BurnedValue:       kda.BurnedValue,
			IssueDate:         kda.IssueDate,
			Royalties:         royaltiesInfo,
			Properties:        propertiesInfo,
			Attributes:        attributesInfo,
			Roles:             rolesInfo,
			Hidden:            &initialAssetPermissionsState,
			Verified:          &initialAssetPermissionsState,
		}

		assets = append(assets, asset)
	}

	err := wia.indexer.SaveAssets(assets)
	if err != nil {
		log.Warn("itemAsset.Save",
			"could not index asset",
			"error", err.Error())
		return err
	}

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (wia *itemAsset) IsInterfaceNil() bool {
	return wia == nil
}
