package data

import "math/big"

// TPS is a structure containing all the fields that need to
//
//	be saved for a shard statistic in the database
type TPS struct {
	LiveTPS               float64         `json:"liveTPS"`
	PeakTPS               float64         `json:"peakTPS"`
	BlockNumber           uint64          `json:"blockNumber"`
	SlotNumber            uint64          `json:"slotNumber"`
	SlotTime              uint64          `json:"slotTime"`
	AverageBlockTxCount   *big.Int        `json:"averageBlockTxCount"`
	TotalProcessedTxCount *big.Int        `json:"totalProcessedTxCount"`
	ChainStatistics       *ChainStatistic `json:"chainStatistics"`
	LastBlockTxCount      uint32          `json:"lastBlockTxCount"`
}

type ChainStatistic struct {
	LiveTPS               float64  `json:"liveTPS"`
	AverageTPS            *big.Int `json:"averageTPS"`
	PeakTPS               float64  `json:"peakTPS"`
	CurrentBlockNonce     uint64   `json:"currentBlockNonce"`
	TotalProcessedTxCount *big.Int `json:"totalProcessedTxCount"`
	AverageBlockTxCount   uint32   `json:"averageBlockTxCount"`
	LastBlockTxCount      uint32   `json:"lastBlockTxCount"`
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
