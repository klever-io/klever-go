package consensus

// BlsConsensusType specifies the signature scheme used in the consensus
const BlsConsensusType = "bls"

// GetPBFTThreshold returns the pBFT threshold for a given consensus size
func GetPBFTThreshold(consensusSize int) int {
	return consensusSize*2/3 + 1
}

// GetPBFTFallbackThreshold returns the pBFT fallback threshold for a given consensus size
func GetPBFTFallbackThreshold(consensusSize int) int {
	return consensusSize*1/2 + 1
}
