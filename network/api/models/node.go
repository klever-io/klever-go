package models

// NodeOverview -
type NodeOverview struct {
	ChainID              string `json:"chainID"`
	BaseTxSize           int64  `json:"baseTxSize"`
	SlotAtEpochStart     int64  `json:"slotAtEpochStart"`
	SlotsPerEpoch        int64  `json:"slotsPerEpoch"`
	CurrentSlot          int64  `json:"currentSlot"`
	SlotDuration         int64  `json:"slotDuration"`
	SlotCurrentTimestamp int64  `json:"slotCurrentTimestamp"`
	StartTime            int64  `json:"startTime"`
	EpochNumber          int64  `json:"epochNumber"`
	NonceAtEpochStart    int64  `json:"nonceAtEpochStart"`
	Nonce                int64  `json:"nonce"`
}
