package block

import (
	"bytes"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/marshal"
)

func getMetricsFromHeader(
	blk *block.Block,
	numTxWithDst uint64,
	marshalizer marshal.Marshalizer,
	appStatusHandler core.AppStatusHandler,
) {
	headerSize := uint64(0)
	bodySize := uint64(0)

	marshalizedHeader, err := marshalizer.Marshal(blk.GetHeader())
	if err == nil {
		headerSize = uint64(len(marshalizedHeader))
	}

	marshalizedBody, err := marshalizer.Marshal(blk)
	if err == nil {
		bodySize = uint64(len(marshalizedBody)) - headerSize
	}

	appStatusHandler.SetUInt64Value(core.MetricHeaderSize, headerSize)
	appStatusHandler.SetUInt64Value(core.MetricBodyBlocksSize, bodySize)
	appStatusHandler.SetUInt64Value(core.MetricNumTxInBlock, uint64(blk.GetTxCount()))
	appStatusHandler.SetUInt64Value(core.MetricTxPoolLoad, numTxWithDst)
}

func getMetricsFromTXProcessed(
	processResults data.ProcessResults,
	appStatusHandler core.AppStatusHandler,
) {
	if processResults == nil {
		return
	}

	appStatusHandler.SetUInt64Value(core.MetricTXsBlocksSize, uint64(tools.SafeI64ToU64(processResults.Size())))
}

func saveMetricsForCommitBlock(
	appStatusHandler core.AppStatusHandler,
	header *block.Block,
	headerHash []byte,
	nodesCoordinator sharding.NodesCoordinator,
	highestFinalBlockNonce uint64,
) {
	appStatusHandler.SetStringValue(core.MetricCurrentBlockHash, logger.DisplayByteSlice(headerHash))
	appStatusHandler.SetUInt64Value(core.MetricEpochNumber, uint64(header.GetEpoch()))
	appStatusHandler.SetUInt64Value(core.MetricHighestFinalBlock, highestFinalBlockNonce)

	epoch := header.GetEpoch()
	if header.GetIsEpochStart() && epoch > 0 {
		epoch = epoch - 1
	}

	pubKeys, err := nodesCoordinator.GetConsensusValidatorsPublicKeys(
		header.GetPrevRandSeed(),
		header.GetSlot(),
		epoch,
	)
	if err != nil {
		log.Debug("cannot get validators public keys", "error", err.Error())
	}

	countAcceptedSignedBlocks(pubKeys, nodesCoordinator.GetOwnPublicKey(), appStatusHandler)
}

func countAcceptedSignedBlocks(
	publicKeys []string,
	ownPublicKey []byte,
	appStatusHandler core.AppStatusHandler,
) {
	isInConsensus := false

	for index, publicKey := range publicKeys {
		if bytes.Equal([]byte(publicKey), ownPublicKey) {
			if index == 0 {
				return
			}

			isInConsensus = true
			break
		}
	}

	if !isInConsensus {
		return
	}

	appStatusHandler.Increment(core.MetricCountConsensusAcceptedBlocks)
}
