package scenjsonwrite

import (
	"encoding/hex"

	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

func resultToOJ(res *scenjsonmodel.TransactionResult) orderedjson.OJsonObject {
	resultOJ := orderedjson.NewMap()

	resultOJ.Put("out", checkValueListToOJ(res.Out))

	if !res.Status.IsUnspecified() {
		resultOJ.Put("status", checkBigIntToOJ(res.Status))
	}
	if !res.Message.IsUnspecified() {
		resultOJ.Put("message", checkBytesToOJ(res.Message))
	}
	if !res.Logs.IsUnspecified {
		if res.Logs.IsStar {
			resultOJ.Put("logs", stringToOJ("*"))
		} else {
			resultOJ.Put("logs", logsToOJ(res.Logs))

		}
	}
	if !res.Gas.IsUnspecified() {
		resultOJ.Put("gas", checkUint64ToOJ(res.Gas))
	}
	if !res.Refund.IsUnspecified() {
		resultOJ.Put("refund", checkBigIntToOJ(res.Refund))
	}

	return resultOJ
}

// LogToString returns a json representation of a log entry, we use it for debugging
func LogToString(logEntry *scenjsonmodel.LogEntry) string {
	logOJ := logToOJ(logEntry)
	return orderedjson.JSONString(logOJ)
}

func logToOJ(logEntry *scenjsonmodel.LogEntry) orderedjson.OJsonObject {
	logOJ := orderedjson.NewMap()
	logOJ.Put("address", checkBytesToOJ(logEntry.Address))
	logOJ.Put("endpoint", checkBytesToOJ(logEntry.Endpoint))
	logOJ.Put("topics", checkValueListToOJ(logEntry.Topics))
	logOJ.Put("data", checkValueListToOJ(logEntry.Data))

	return logOJ
}

func logsToOJ(logEntries scenjsonmodel.LogList) orderedjson.OJsonObject {
	var logList []orderedjson.OJsonObject
	for _, logEntry := range logEntries.List {
		logOJ := logToOJ(logEntry)
		logList = append(logList, logOJ)
	}
	if logEntries.MoreAllowedAtEnd {
		logList = append(logList, stringToOJ("+"))
	}
	logOJList := orderedjson.OJsonList(logList)
	return &logOJList
}

func bigIntToOJ(i scenjsonmodel.JSONBigInt) orderedjson.OJsonObject {
	return &orderedjson.OJsonString{Value: i.Original}
}

func checkBigIntToOJ(i scenjsonmodel.JSONCheckBigInt) orderedjson.OJsonObject {
	return &orderedjson.OJsonString{Value: i.Original}
}

func bytesFromStringToString(bytes scenjsonmodel.JSONBytesFromString) string {
	if len(bytes.Original) == 0 && len(bytes.Value) > 0 {
		bytes.Original = hex.EncodeToString(bytes.Value)
	}
	return bytes.Original
}

func bytesFromStringToOJ(bytes scenjsonmodel.JSONBytesFromString) orderedjson.OJsonObject {
	return &orderedjson.OJsonString{Value: bytesFromStringToString(bytes)}
}

func bytesFromTreeToOJ(bytes scenjsonmodel.JSONBytesFromTree) orderedjson.OJsonObject {
	if bytes.OriginalEmpty() {
		bytes.Original = &orderedjson.OJsonString{Value: hex.EncodeToString(bytes.Value)}
	}
	return bytes.Original
}

func checkBytesToOJ(checkBytes scenjsonmodel.JSONCheckBytes) orderedjson.OJsonObject {
	if checkBytes.OriginalEmpty() && len(checkBytes.Value) > 0 {
		checkBytes.Original = &orderedjson.OJsonString{Value: hex.EncodeToString(checkBytes.Value)}
	}
	return checkBytes.Original
}

func valueListToOJ(jsonBytesList scenjsonmodel.JSONValueList) orderedjson.OJsonObject {
	var valuesList []orderedjson.OJsonObject
	for _, blh := range jsonBytesList.Values {
		valuesList = append(valuesList, bytesFromStringToOJ(blh))
	}
	ojList := orderedjson.OJsonList(valuesList)
	return &ojList
}

func checkValueListToOJ(jcbl scenjsonmodel.JSONCheckValueList) orderedjson.OJsonObject {
	if jcbl.IsStar {
		return &orderedjson.OJsonString{Value: "*"}
	}

	var valuesList []orderedjson.OJsonObject
	for _, jcb := range jcbl.Values {
		valuesList = append(valuesList, checkBytesToOJ(jcb))
	}
	ojList := orderedjson.OJsonList(valuesList)
	return &ojList
}

func uint64ToOJ(i scenjsonmodel.JSONUint64) orderedjson.OJsonObject {
	return &orderedjson.OJsonString{Value: i.Original}
}

func checkUint64ToOJ(i scenjsonmodel.JSONCheckUint64) orderedjson.OJsonObject {
	return &orderedjson.OJsonString{Value: i.Original}
}

func stringToOJ(str string) orderedjson.OJsonObject {
	return &orderedjson.OJsonString{Value: str}
}

func boolToOJ(val bool) orderedjson.OJsonObject {
	obj := orderedjson.OJsonBool(val)
	return &obj
}
