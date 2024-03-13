package scenjsonwrite

import (
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

func kdaTxDataToOJ(kdaItems []*scenjsonmodel.KDATxData) orderedjson.OJsonObject {
	kdaItemList := orderedjson.OJsonList{}
	for _, kdaItemRaw := range kdaItems {
		kdaItemOJ := kdaTxRawEntryToOJ(kdaItemRaw)
		kdaItemList = append(kdaItemList, kdaItemOJ)
	}

	return &kdaItemList

}

func kdaTxRawEntryToOJ(kdaItemRaw *scenjsonmodel.KDATxData) *orderedjson.OJsonMap {
	kdaItemOJ := orderedjson.NewMap()

	if len(kdaItemRaw.TokenIdentifier.Original) > 0 {
		kdaItemOJ.Put("tokenIdentifier", bytesFromStringToOJ(kdaItemRaw.TokenIdentifier))
	}
	if len(kdaItemRaw.Nonce.Original) > 0 {
		kdaItemOJ.Put("nonce", uint64ToOJ(kdaItemRaw.Nonce))
	}
	if len(kdaItemRaw.Value.Original) > 0 {
		kdaItemOJ.Put("value", bigIntToOJ(kdaItemRaw.Value))
	}

	return kdaItemOJ
}

func kdaDataToOJ(kdaItems []*scenjsonmodel.KDAData) *orderedjson.OJsonMap {
	kdaItemsOJ := orderedjson.NewMap()
	for _, kdaItem := range kdaItems {
		kdaItemsOJ.Put(kdaItem.TokenIdentifier.Original, kdaItemToOJ(kdaItem))
	}
	return kdaItemsOJ
}

func kdaItemToOJ(kdaItem *scenjsonmodel.KDAData) orderedjson.OJsonObject {
	if isCompactKDA(kdaItem) {
		return bigIntToOJ(kdaItem.Instances[0].Balance)
	}

	kdaItemOJ := orderedjson.NewMap()

	// instances
	if len(kdaItem.Instances) > 0 {
		var convertedList []orderedjson.OJsonObject
		for _, kdaInstance := range kdaItem.Instances {
			kdaInstanceOJ := orderedjson.NewMap()
			appendKDAInstanceToOJ(kdaInstance, kdaInstanceOJ)
			convertedList = append(convertedList, kdaInstanceOJ)
		}
		instancesOJList := orderedjson.OJsonList(convertedList)
		kdaItemOJ.Put("instances", &instancesOJList)
	}

	if len(kdaItem.LastNonce.Original) > 0 {
		kdaItemOJ.Put("lastNonce", uint64ToOJ(kdaItem.LastNonce))
	}

	// roles
	if len(kdaItem.Roles) > 0 {
		var convertedList []orderedjson.OJsonObject
		for _, roleStr := range kdaItem.Roles {
			convertedList = append(convertedList, &orderedjson.OJsonString{Value: roleStr})
		}
		rolesOJList := orderedjson.OJsonList(convertedList)
		kdaItemOJ.Put("roles", &rolesOJList)
	}
	if len(kdaItem.Frozen.Original) > 0 {
		kdaItemOJ.Put("frozen", uint64ToOJ(kdaItem.Frozen))
	}

	return kdaItemOJ
}

func appendKDAInstanceToOJ(kdaInstance *scenjsonmodel.KDAInstance, targetOj *orderedjson.OJsonMap) {
	targetOj.Put("nonce", uint64ToOJ(kdaInstance.Nonce))

	if len(kdaInstance.Balance.Original) > 0 {
		targetOj.Put("balance", bigIntToOJ(kdaInstance.Balance))
	}
	if len(kdaInstance.Creator.Original) > 0 {
		targetOj.Put("creator", bytesFromStringToOJ(kdaInstance.Creator))
	}
	if len(kdaInstance.Royalties.Original) > 0 {
		targetOj.Put("royalties", uint64ToOJ(kdaInstance.Royalties))
	}
	if len(kdaInstance.Hash.Original) > 0 {
		targetOj.Put("hash", bytesFromStringToOJ(kdaInstance.Hash))
	}
	if !kdaInstance.Uris.IsUnspecified() {
		targetOj.Put("uri", valueListToOJ(kdaInstance.Uris))
	}
	if len(kdaInstance.Attributes.Value) > 0 {
		targetOj.Put("attributes", bytesFromTreeToOJ(kdaInstance.Attributes))
	}
}

func isCompactKDA(kdaItem *scenjsonmodel.KDAData) bool {
	if len(kdaItem.Instances) != 1 {
		return false
	}
	if len(kdaItem.Instances[0].Nonce.Original) > 0 {
		return false
	}
	if len(kdaItem.Roles) > 0 {
		return false
	}
	if len(kdaItem.Frozen.Original) > 0 {
		return false
	}
	return true
}
