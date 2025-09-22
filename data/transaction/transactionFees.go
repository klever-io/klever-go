package transaction

// CostResponse is structure used to return the transaction cost
type CostResponse struct {
	KAppFee       int64  `json:"kAppFee"`
	BandwidthFee  int64  `json:"bandwidthFee"`
	GasEstimated  uint64 `json:"gasEstimated"`
	SafetyMargin  uint64 `json:"safetyMargin"`
	GasMultiplier uint64 `json:"gasMultiplier"`
	RetMessage    string `json:"returnMessage"`
}

// FeesResponse is structure used to return the transaction fees
type FeesResponse struct {
	*CostResponse
	KDAFees *Transaction_KDAFee `json:"kdaFee"`
}

func (x *Transaction_KDAFee) IsInterfaceNil() bool {
	return x == nil
}
