package scenjsonwrite

import (
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

func checkKDADataToOJ(kdaItems []*scenjsonmodel.CheckKDAData, moreKDATokensAllowed bool) *orderedjson.OJsonMap {
	kdaItemsOJ := orderedjson.NewMap()
	for _, kdaItem := range kdaItems {
		kdaItemsOJ.Put(kdaItem.TokenIdentifier.Original, checkKDAItemToOJ(kdaItem))
	}
	if moreKDATokensAllowed {
		kdaItemsOJ.Put("+", stringToOJ(""))
	}
	return kdaItemsOJ
}

func checkKDAItemToOJ(kdaItem *scenjsonmodel.CheckKDAData) orderedjson.OJsonObject {
	if isCompactCheckKDA(kdaItem) {
		return checkBigIntToOJ(kdaItem.Instances[0].Balance)
	}

	kdaItemOJ := orderedjson.NewMap()

	// instances
	if len(kdaItem.Instances) > 0 {
		var convertedList []orderedjson.OJsonObject
		for _, kdaInstance := range kdaItem.Instances {
			kdaInstanceOJ := orderedjson.NewMap()
			appendCheckKDAInstanceToOJ(kdaInstance, kdaInstanceOJ)
			convertedList = append(convertedList, kdaInstanceOJ)
		}
		instancesOJList := orderedjson.OJsonList(convertedList)
		kdaItemOJ.Put("instances", &instancesOJList)
	}

	if len(kdaItem.LastNonce.Original) > 0 {
		kdaItemOJ.Put("lastNonce", checkUint64ToOJ(kdaItem.LastNonce))
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
		kdaItemOJ.Put("frozen", checkUint64ToOJ(kdaItem.Frozen))
	}

	return kdaItemOJ
}

func appendCheckKDAInstanceToOJ(kdaInstance *scenjsonmodel.CheckKDAInstance, targetOj *orderedjson.OJsonMap) {
	targetOj.Put("nonce", uint64ToOJ(kdaInstance.Nonce))

	if len(kdaInstance.Balance.Original) > 0 {
		targetOj.Put("balance", checkBigIntToOJ(kdaInstance.Balance))
	}
	if !kdaInstance.Creator.Unspecified && len(kdaInstance.Creator.Value) > 0 {
		targetOj.Put("creator", checkBytesToOJ(kdaInstance.Creator))
	}
	if !kdaInstance.Royalties.Unspecified && len(kdaInstance.Royalties.Original) > 0 {
		targetOj.Put("royalties", checkUint64ToOJ(kdaInstance.Royalties))
	}
	if !kdaInstance.Hash.Unspecified && len(kdaInstance.Hash.Value) > 0 {
		targetOj.Put("hash", checkBytesToOJ(kdaInstance.Hash))
	}
	if !kdaInstance.Uris.IsUnspecified() {
		targetOj.Put("uri", checkValueListToOJ(kdaInstance.Uris))
	}
	if !kdaInstance.Attributes.Unspecified && len(kdaInstance.Attributes.Value) > 0 {
		targetOj.Put("attributes", checkBytesToOJ(kdaInstance.Attributes))
	}
}

func isCompactCheckKDA(kdaItem *scenjsonmodel.CheckKDAData) bool {
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
