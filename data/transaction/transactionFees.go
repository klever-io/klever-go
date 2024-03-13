package transaction

// CostResponse is structure used to return the transaction cost
type CostResponse struct {
	KAppFee       int64  `json:"kAppFee"`
	BandwidthFee  int64  `json:"bandwidthFee"`
	GasEstimated  uint64 `json:"gasEstimated"`
	GasMultiplier uint64 `json:"gasMultiplier"`
	RetMessage    string `json:"returnMessage"`
}

func (x *Transaction_KDAFee) IsInterfaceNil() bool {
	return x == nil
}
