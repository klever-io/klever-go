package bls

import (
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/statusHandler"
)

type subslotSignature struct {
	*slot.Subslot

	appStatusHandler      core.AppStatusHandler
	signatureCompleteChan chan struct{}
}

// NewSubslotSignature creates a subslotSignature object
func NewSubslotSignature(
	baseSubslot *slot.Subslot,
	extend func(subslotId int),
) (*subslotSignature, error) {
	err := checkNewSubslotSignatureParams(
		baseSubslot,
	)
	if err != nil {
		return nil, err
	}

	srSignature := subslotSignature{
		Subslot:               baseSubslot,
		appStatusHandler:      statusHandler.NewNilStatusHandler(),
		signatureCompleteChan: make(chan struct{}, 1),
	}
	srSignature.Job = srSignature.doSignatureJob
	srSignature.Check = srSignature.doSignatureConsensusCheck
	srSignature.Extend = extend

	return &srSignature, nil
}

// SetAppStatusHandler method set appStatusHandler
func (sr *subslotSignature) SetAppStatusHandler(ash core.AppStatusHandler) error {
	if ash == nil || ash.IsInterfaceNil() {
		return slot.ErrNilAppStatusHandler
	}

	sr.appStatusHandler = ash
	return nil
}

func checkNewSubslotSignatureParams(
	baseSubslot *slot.Subslot,
) error {
	if baseSubslot == nil {
		return slot.ErrNilSubslot
	}
	if baseSubslot.ConsensusState == nil {
		return slot.ErrNilConsensusState
	}

	err := slot.ValidateConsensusCore(baseSubslot.ConsensusCoreHandler)

	return err
}

// doSignatureJob method does the job of the subslot Signature
func (sr *subslotSignature) doSignatureJob() bool {
	if !sr.IsNodeInConsensusGroup(sr.SelfPubKey()) {
		return false
	}
	if !sr.CanDoSubslotJob(sr.Current()) {
		return false
	}

	signatureShare, err := sr.MultiSigner().CreateSignatureShare(sr.GetData(), nil)
	if err != nil {
		log.Debug("doSignatureJob.CreateSignatureShare", "error", err.Error())
		return false
	}

	isSelfLeader := sr.IsSelfLeaderInCurrentSlot()

	if !isSelfLeader {
		//TODO: Analyze it is possible to send message only to leader with O(1) instead of O(n)
		cnsMsg := consensus.NewConsensusMessage(
			sr.GetData(),
			signatureShare,
			nil,
			[]byte(sr.SelfPubKey()),
			nil,
			int(MtSignature),
			sr.SlotManager().Index(),
			sr.Header.GetNonce(),
			sr.ChainID(),
			nil,
			nil,
			nil,
			sr.CurrentPid(),
		)

		err = sr.BroadcastMessenger().BroadcastConsensusMessage(cnsMsg)
		if err != nil {
			log.Debug("doSignatureJob.BroadcastConsensusMessage", "error", err.Error())
			return false
		}

		log.Debug("step 2: signature has been sent")
	}

	err = sr.SetSelfJobDone(sr.Current(), true)
	if err != nil {
		log.Debug("doSignatureJob.SetSelfJobDone",
			"subslot", sr.Name(),
			"error", err.Error())
		return false
	}

	if isSelfLeader {
		sr.resetSignatureCompleteChan()
		go sr.waitAllSignatures()
	}

	return true
}

// receivedSignature method is called when a signature is received through the signature channel.
// If the signature is valid, than the jobDone map corresponding to the node which sent it,
// is set on true for the subslot Signature
func (sr *subslotSignature) receivedSignature(cnsDta *consensus.Message) bool {
	node := string(cnsDta.PubKey)

	if !sr.IsConsensusDataSet() {
		return false
	}

	if !sr.IsNodeInConsensusGroup(node) {
		sr.PeerHonestyHandler().ChangeScore(
			node,
			common.ConsensusTopic,
			slot.ValidatorPeerHonestyDecreaseFactor,
		)

		return false
	}

	if !sr.IsSelfLeaderInCurrentSlot() {
		return false
	}

	if !sr.IsConsensusDataEqual(cnsDta.BlockHeaderHash) {
		return false
	}

	if !sr.CanProcessReceivedMessage(cnsDta, sr.SlotManager().Index(), sr.Current()) {
		return false
	}

	index, err := sr.ConsensusGroupIndex(node)
	if err != nil {
		log.Debug("receivedSignature.ConsensusGroupIndex",
			"node", node,
			"error", err.Error())
		return false
	}

	currentMultiSigner := sr.MultiSigner()
	err = currentMultiSigner.VerifySignatureShare(uint16(index), cnsDta.SignatureShare, sr.GetData()) // #nosec G115
	if err != nil {
		log.Debug("receivedSignature.VerifySignatureShare",
			"index", index,
			"error", err.Error())
		return false
	}

	err = currentMultiSigner.StoreSignatureShare(uint16(index), cnsDta.SignatureShare) // #nosec G115
	if err != nil {
		log.Debug("receivedSignature.StoreSignatureShare",
			"index", index,
			"error", err.Error())
		return false
	}

	err = sr.SetJobDone(node, sr.Current(), true)
	if err != nil {
		log.Debug("receivedSignature.SetJobDone",
			"node", node,
			"subslot", sr.Name(),
			"error", err.Error())
		return false
	}

	sr.PeerHonestyHandler().ChangeScore(
		node,
		common.ConsensusTopic,
		slot.ValidatorPeerHonestyIncreaseFactor,
	)

	sr.appStatusHandler.SetStringValue(core.MetricConsensusSlotState, "signed")
	return true
}

// doSignatureConsensusCheck method checks if the consensus in the subslot Signature is achieved
func (sr *subslotSignature) doSignatureConsensusCheck() bool {
	if sr.SlotCanceled {
		return false
	}

	if sr.IsSubslotFinished(sr.Current()) {
		sr.appStatusHandler.SetStringValue(core.MetricConsensusSlotState, "signed")

		return true
	}

	isSelfLeader := sr.IsSelfLeaderInCurrentSlot()
	isSelfInConsensusGroup := sr.IsNodeInConsensusGroup(sr.SelfPubKey())

	threshold := sr.Threshold(sr.Current())
	if sr.FallbackHeaderValidator().ShouldApplyFallbackValidation(sr.Header) {
		threshold = sr.FallbackThreshold(sr.Current())
		log.Warn("subslotSignature.doSignatureConsensusCheck: fallback validation has been applied",
			"minimum number of signatures required", threshold,
			"actual number of signatures received", sr.getNumOfSignaturesCollected(),
		)
	}

	areSignaturesCollected, numSigs := sr.areSignaturesCollected(threshold)
	areAllSignaturesCollected := numSigs == sr.ConsensusGroupSize()

	isJobDoneByLeader := isSelfLeader && (areAllSignaturesCollected || (areSignaturesCollected && sr.WaitingAllSignaturesTimeOut))
	isJobDoneByConsensusNode := !isSelfLeader && isSelfInConsensusGroup && sr.IsSelfJobDone(sr.Current())

	isSubslotFinished := !isSelfInConsensusGroup || isJobDoneByConsensusNode || isJobDoneByLeader

	if isSubslotFinished {
		if isSelfLeader {
			log.Debug("step 2: signatures",
				"received", numSigs,
				"total", len(sr.ConsensusGroup()))

			// Trigger completion channel when all signatures are collected
			select {
			case sr.signatureCompleteChan <- struct{}{}:
			default:
				// Channel already notified or buffer full
			}
		}

		log.Debug("step 2: subslot has been finished",
			"subslot", sr.Name())
		sr.SetStatus(sr.Current(), slot.SsFinished)

		sr.appStatusHandler.SetStringValue(core.MetricConsensusSlotState, "signed")

		return true
	}

	return false
}

// areSignaturesCollected method checks if the signatures received from the nodes, belonging to the current
// jobDone group, are more than the necessary given threshold
func (sr *subslotSignature) areSignaturesCollected(threshold int) (bool, int) {
	n := sr.getNumOfSignaturesCollected()
	return n >= threshold, n
}

func (sr *subslotSignature) getNumOfSignaturesCollected() int {
	n := 0

	consensusGroup := sr.ConsensusGroup()
	for i := 0; i < len(consensusGroup); i++ {
		node := consensusGroup[i]

		isSignJobDone, err := sr.JobDone(node, sr.Current())
		if err != nil {
			log.Debug("getNumOfSignaturesCollected.JobDone",
				"node", node,
				"subslot", sr.Name(),
				"error", err.Error())
			continue
		}

		if isSignJobDone {
			n++
		}
	}

	return n
}

func (sr *subslotSignature) waitAllSignatures() {
	remainingTime := sr.remainingTime()
	timeout := time.NewTimer(remainingTime)
	defer timeout.Stop()

	select {
	case <-sr.signatureCompleteChan:
		// All signatures collected (100%), exit immediately
		return
	case <-timeout.C:
		// Timeout reached, check if subslot is already finished by threshold signatures
		if sr.IsSubslotFinished(sr.Current()) {
			return
		}

		sr.WaitingAllSignaturesTimeOut = true
		select {
		case sr.ConsensusChannel() <- true:
		default:
		}
		return
	}
}

func (sr *subslotSignature) remainingTime() time.Duration {
	startTime := sr.SlotManager().Timestamp()
	maxTime := time.Duration(float64(sr.StartTime()) + float64(sr.EndTime()-sr.StartTime())*waitingAllSigsMaxTimeThreshold)
	remainigTime := sr.SlotManager().RemainingTime(startTime, maxTime)

	return remainigTime
}

// resetSignatureCompleteChan drains and resets the signature completion channel
func (sr *subslotSignature) resetSignatureCompleteChan() {
	select {
	case <-sr.signatureCompleteChan:
		// Drain the channel if it has a value
	default:
		// Channel is already empty
	}
}
