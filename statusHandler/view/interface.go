package view

// Presenter defines the methods that return information about node
type Presenter interface {
	GetAppVersion() string
	GetNodeName() string
	GetPublicKeyBlockSign() string
	GetRedundancyLevel() int64
	GetRedundancyIsMainActive() string
	GetChainID() string
	GetNodeType() string
	GetPeerType() string
	GetCountConsensus() uint64
	GetCountConsensusAcceptedBlocks() uint64
	GetCountLeader() uint64
	GetCountAcceptedBlocks() uint64
	GetIsSyncing() uint64
	GetTxPoolLoad() uint64
	GetTPSCurrent() uint64
	GetTPSPeak() uint64
	GetNonce() uint64
	GetProbableHighestNonce() uint64
	GetSynchronizedSlot() uint64
	GetSlotTime() uint64
	GetLiveValidatorNodes() uint64
	GetConnectedNodes() uint64
	GetNumConnectedPeers() uint64
	GetCurrentSlot() uint64
	GetNumTxInBlock() uint64
	GetConsensusState() string
	GetConsensusSlotState() string
	GetCPULoadPercent() uint64
	GetMemLoadPercent() uint64
	GetTotalMem() uint64
	GetMemUsedByNode() uint64
	GetNetworkRecvPercent() uint64
	GetNetworkRecvBps() uint64
	GetNetworkRecvBpsPeak() uint64
	GetNetworkSentPercent() uint64
	GetNetworkSentBps() uint64
	GetNetworkSentBpsPeak() uint64
	GetLogLines() []string
	GetNumTxProcessed() uint64
	GetCurrentBlockHash() string
	GetEpochNumber() uint64
	GetEpochInfo() (uint64, uint64, int, string)
	CalculateTimeToSynchronize(numMillisecondsRefreshTime int) string
	CalculateSynchronizationSpeed(numMillisecondsRefreshTime int) uint64
	GetCurrentSlotTimestamp() uint64
	GetBlockSize() uint64
	GetHighestFinalBlock() uint64
	CheckSoftwareVersion() (bool, string)

	GetNetworkSentBytesInEpoch() uint64
	GetNetworkReceivedBytesInEpoch() uint64

	GetTotalRewardsValue() (string, string)
	CalculateRewardsPerHour() string
	GetZeros() string

	InvalidateCache()

	// IsInterfaceNil returns true if there is no value under the interface
	IsInterfaceNil() bool
}
