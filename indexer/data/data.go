package data

import "math/big"

// TPS is a structure containing all the fields that need to
// be saved for a statistic in the database
type TPS struct {
	LiveTPS               float64  `json:"liveTPS"`
	PeakTPS               float64  `json:"peakTPS"`
	AverageTPS            *big.Int `json:"averageTPS"`
	BlockNumber           uint64   `json:"blockNumber"`
	SlotNumber            uint64   `json:"slotNumber"`
	AverageBlockTxCount   *big.Int `json:"averageBlockTxCount"`
	TotalProcessedTxCount *big.Int `json:"totalProcessedTxCount"`
	CurrentBlockTxCount   uint32   `json:"currentBlockTxCount"`
}

// ValidatorRatingInfo is a structure containing validator rating information
type ValidatorRatingInfo struct {
	PublicKey string  `json:"publicKey"`
	Rating    float32 `json:"rating"`
}

// EpochInfo is a structure containing epoch information

type EpochInfo struct {
	Epoch                uint32           `json:"epoch"`
	Validators           []*ValidatorInfo `json:"validators"`
	KFICirculatingSupply int64            `json:"kfiCirculationSupply"`
	KLVCirculatingSupply int64            `json:"klvCirculationSupply"`
	KFITotalStaked       int64            `json:"kfiTotalStaked"`
	KLVTotalStaked       int64            `json:"klvTotalStaked"`
}
