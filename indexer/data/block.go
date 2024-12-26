package data

import (
	"time"
)

// Block is a structure containing all the fields that need
//
//	to be saved for a block. It has all the default fields
//	plus some extra information for ease of search and filter
type Block struct {
	Hash               string        `json:"hash"`
	Nonce              uint64        `json:"nonce"`
	ParentHash         string        `json:"parentHash"`
	Timestamp          time.Duration `json:"timestamp"`
	Slot               uint64        `json:"slot"`
	Epoch              uint32        `json:"epoch"`
	IsEpochStart       bool          `json:"isEpochStart"`
	PrevEpochStartSlot uint64        `json:"prevEpochStartSlot"`
	Size               int64         `json:"size"`
	SizeTxs            int64         `json:"sizeTxs"`
	TxRootHash         string        `json:"txRootHash"`
	TrieRoot           string        `json:"trieRoot"`
	ValidatorsTrieRoot string        `json:"validatorsTrieRoot"`
	KAppsTrieRoot      string        `json:"kappsTrieRoot"`
	ProducerSignature  string        `json:"producerSignature"`
	Signature          string        `json:"signature"`
	PrevRandSeed       string        `json:"prevRandSeed,omitempty"`
	RandSeed           string        `json:"randSeed,omitempty"`
	TxCount            uint32        `json:"txCount"`
	TxFees             int64         `json:"txFees,omitempty"`
	TxBurnedFees       int64         `json:"txBurnedFees,omitempty"`
	KAppFees           int64         `json:"kAppFees,omitempty"`
	BlockRewards       int64         `json:"blockRewards,omitempty"`
	StakingRewards     int64         `json:"stakingRewards,omitempty"`
	BurnedUnclaimed    int64         `json:"burnedUnclaimed,omitempty"`
	TxHashes           []string      `json:"txHashes"`
	Validators         []string      `json:"validators"`
	SoftwareVersion    string        `json:"softwareVersion,omitempty"`
	ChainID            string        `json:"chainID,omitempty"`
	Reserved           string        `json:"reserved"`
	ProducerBLS        string        `json:"producerBLS,omitempty"`
}
