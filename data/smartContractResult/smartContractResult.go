//go:generate protoc -I=. -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. smartContractResult.proto
package smartContractResult

import (
	"math/big"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/transaction"
)

var _ = data.TransactionHandler(&SmartContractResult{})

// IsInterfaceNil verifies if underlying object is nil
func (scr *SmartContractResult) IsInterfaceNil() bool {
	return scr == nil
}

// SetValue sets the value of the smart contract result
func (scr *SmartContractResult) SetValue(assetID string, value *big.Int) {
	scr.Value[assetID] = value.Int64()
}

// SetData sets the data of the smart contract result
func (scr *SmartContractResult) SetData(data []byte) {
	scr.SCData = data
}

// SetRcvAddr sets the receiver address of the smart contract result
func (scr *SmartContractResult) SetRcvAddr(addr []byte) {
	scr.RcvAddr = addr
}

// SetSndAddr sets the sender address of the smart contract result
func (scr *SmartContractResult) SetSndAddr(addr []byte) {
	scr.SndAddr = addr
}

// GetRcvUserName returns the receiver user name from the smart contract result
func (_ *SmartContractResult) GetRcvUserName() []byte {
	return nil
}

// TrimSlicePtr creates a copy of the provided slice without the excess capacity
func TrimSlicePtr(in []*SmartContractResult) []*SmartContractResult {
	if len(in) == 0 {
		return []*SmartContractResult{}
	}
	ret := make([]*SmartContractResult, len(in))
	copy(ret, in)
	return ret
}

// CheckIntegrity checks for not nil fields and negative value
func (scr *SmartContractResult) CheckIntegrity() error {
	if len(scr.RcvAddr) == 0 {
		return data.ErrNilRcvAddr
	}
	if len(scr.SndAddr) == 0 {
		return data.ErrNilSndAddr
	}
	for _, v := range scr.Value {
		if big.NewInt(v).Sign() < 0 {
			return data.ErrNegativeValue
		}
	}

	return nil
}

// GetData
func (sc *SmartContractResult) GetData() [][]byte {
	return [][]byte{sc.SCData}
}

func (sc *SmartContractResult) GetDataWithIdx(_ int) []byte {
	return sc.GetSCData()
}

// GetBandwidthFee implements data.TransactionHandler
func (*SmartContractResult) GetBandwidthFee() int64 {
	panic("unimplemented")
}

// GetRaw implements data.TransactionHandler
func (*SmartContractResult) GetRaw() *transaction.Transaction_Raw {
	panic("unimplemented")
}

// GetSender implements data.TransactionHandler
func (scr *SmartContractResult) GetSender() []byte {
	return scr.SndAddr
}

// GetTotalFees implements data.TransactionHandler
func (*SmartContractResult) GetTotalFees() int64 {
	panic("unimplemented")
}

func (*SmartContractResult) Size() int {
	panic("unimplemented")
}
