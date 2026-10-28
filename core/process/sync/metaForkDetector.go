package sync

import (
	"math"

	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/tools/check"
)

var _ process.ForkDetector = (*metaForkDetector)(nil)

// metaForkDetector implements the meta fork detector mechanism
type metaForkDetector struct {
	*baseForkDetector
}

// NewMetaForkDetector method creates a new metaForkDetector object
func NewMetaForkDetector(
	slotManager consensus.SlotManager,
	blackListHandler process.TimeCacher,
	genesisTime int64,
) (*metaForkDetector, error) {

	if check.IfNil(slotManager) {
		return nil, process.ErrNilSlotManager
	}
	if check.IfNil(blackListHandler) {
		return nil, process.ErrNilBlackListCacher
	}

	bfd := &baseForkDetector{
		slotManager:      slotManager,
		blackListHandler: blackListHandler,
		genesisTime:      genesisTime,
	}

	bfd.headers = make(map[uint64][]*headerInfo)
	bfd.fork.checkpoint = make([]*checkpointInfo, 0)
	checkpoint := &checkpointInfo{
		nonce: bfd.genesisNonce,
		slot:  bfd.genesisSlot,
	}
	bfd.setFinalCheckpoint(checkpoint)
	bfd.addCheckpoint(checkpoint)
	bfd.fork.rollBackNonce = math.MaxUint64
	bfd.fork.probableHighestNonce = bfd.genesisNonce
	bfd.fork.highestNonceReceived = bfd.genesisNonce

	mfd := metaForkDetector{
		baseForkDetector: bfd,
	}

	bfd.forkDetector = &mfd

	return &mfd, nil
}

// AddHeader method adds a new header to headers map
func (mfd *metaForkDetector) AddHeader(
	header data.HeaderHandler,
	headerHash []byte,
	state process.BlockHeaderState,
	selfNotarizedHeaders []data.HeaderHandler,
	selfNotarizedHeadersHashes [][]byte,
) error {
	return mfd.addHeader(
		header,
		headerHash,
		state,
		selfNotarizedHeaders,
		selfNotarizedHeadersHashes,
		mfd.doJobOnBHProcessed,
	)
}

func (mfd *metaForkDetector) doJobOnBHProcessed(
	header data.HeaderHandler,
	headerHash []byte,
	_ []data.HeaderHandler,
	_ [][]byte,
) {
	mfd.setFinalCheckpoint(mfd.lastCheckpoint())
	mfd.addCheckpoint(&checkpointInfo{nonce: header.GetNonce(), slot: header.GetSlot(), hash: headerHash})
	mfd.removePastOrInvalidRecords()
}

func (mfd *metaForkDetector) computeFinalCheckpoint() {
}
