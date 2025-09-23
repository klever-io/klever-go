package vm

import (
	"encoding/hex"
	"fmt"
	"math/big"
)

// VMOutputApi is a wrapper over the vmcommon's VMOutput
type VMOutputApi struct {
	ReturnData      [][]byte                     `json:"returnData"`
	ReturnCode      string                       `json:"returnCode"`
	ReturnMessage   string                       `json:"returnMessage"`
	GasRemaining    uint64                       `json:"gasRemaining"`
	OutputAccounts  map[string]*OutputAccountApi `json:"outputAccounts"`
	DeletedAccounts [][]byte                     `json:"deletedAccounts"`
	Logs            []*LogEntryApi               `json:"logs"`
}

// StorageUpdateApi is a wrapper over vmcommon's StorageUpdate
type StorageUpdateApi struct {
	Offset []byte `json:"offset"`
	Data   []byte `json:"data"`
}

// OutputAccountApi is a wrapper over vmcommon's OutputAccount
type OutputAccountApi struct {
	Address         string                       `json:"address"`
	StorageUpdates  map[string]*StorageUpdateApi `json:"storageUpdates"`
	Code            []byte                       `json:"code"`
	CodeMetadata    []byte                       `json:"codeMetaData"`
	OutputTransfers []OutputTransferApi          `json:"outputTransfers"`
	CallType        CallType                     `json:"callType"`
}

// OutputTransferApi is a wrapper over vmcommon's OutputTransfer
type OutputTransferApi struct {
	Data          []byte         `json:"data"`
	SenderAddress string         `json:"senderAddress"`
	Receiver      string         `json:"receiver"`
	KDATransfer   KDATransferApi `json:"tokenInfo"`
}

type KDATransferApi struct {
	KDAValue      *big.Int `json:"value"`
	KDATokenName  string   `json:"tokenName"`
	KDATokenType  uint32   `json:"tokenType"`
	KDATokenNonce uint64   `json:"tokenNonce"`
}

// LogEntryApi is a wrapper over vmcommon's LogEntry
type LogEntryApi struct {
	Identifier []byte   `json:"identifier"`
	Address    string   `json:"address"`
	Topics     [][]byte `json:"topics"`
	Data       [][]byte `json:"data"`
}

// GetFirstReturnData is a helper function that returns the first ReturnData of VMOutput, interpreted as specified.
func (vmOutput *VMOutputApi) GetFirstReturnData(asType ReturnDataKind) (interface{}, error) {
	if len(vmOutput.ReturnData) == 0 {
		return nil, fmt.Errorf("no return data")
	}

	returnData := vmOutput.ReturnData[0]

	switch asType {
	case AsBigInt:
		return big.NewInt(0).SetBytes(returnData), nil
	case AsBigIntString:
		return big.NewInt(0).SetBytes(returnData).String(), nil
	case AsString:
		return string(returnData), nil
	case AsHex:
		return hex.EncodeToString(returnData), nil
	}

	return nil, fmt.Errorf("can't interpret return data")
}
