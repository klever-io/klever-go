package bls

import (
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/ntp"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/marshal"
)

const ProcessingThresholdPercent = processingThresholdPercent

// factory

// Factory defines a type for the factory structure
type Factory *factory

// BlockChain gets the chain handler object
func (fct *factory) BlockChain() data.ChainHandler {
	return fct.consensusCore.Blockchain()
}

// BlockProcessor gets the block processor object
func (fct *factory) BlockProcessor() process.BlockProcessor {
	return fct.consensusCore.BlockProcessor()
}

// Bootstrapper gets the bootstrapper object
func (fct *factory) Bootstrapper() process.Bootstrapper {
	return fct.consensusCore.BootStrapper()
}

// ChronologyHandler gets the chronology handler object
func (fct *factory) ChronologyHandler() consensus.ChronologyHandler {
	return fct.consensusCore.Chronology()
}

// ConsensusState gets the consensus state struct pointer
func (fct *factory) ConsensusState() *slot.ConsensusState {
	return fct.consensusState
}

// Hasher gets the hasher object
func (fct *factory) Hasher() hashing.Hasher {
	return fct.consensusCore.Hasher()
}

// Marshalizer gets the marshalizer object
func (fct *factory) Marshalizer() marshal.Marshalizer {
	return fct.consensusCore.Marshalizer()
}

// MultiSigner gets the multi signer object
func (fct *factory) MultiSigner() crypto.MultiSigner {
	return fct.consensusCore.MultiSigner()
}

// SlotManager gets the slot manager object
func (fct *factory) SlotManager() consensus.SlotManager {
	return fct.consensusCore.SlotManager()
}

// SyncTimer gets the sync timer object
func (fct *factory) SyncTimer() ntp.SyncTimer {
	return fct.consensusCore.SyncTimer()
}

// NodesCoordinator gets the nodes coordinator object
func (fct *factory) NodesCoordinator() sharding.NodesCoordinator {
	return fct.consensusCore.NodesCoordinator()
}

// Worker gets the worker object
func (fct *factory) Worker() slot.WorkerHandler {
	return fct.worker
}

// SetWorker sets the worker object
func (fct *factory) SetWorker(worker slot.WorkerHandler) {
	fct.worker = worker
}

// GenerateStartSlotSubslot generates the instance of subslot StartSlot and added it to the chronology subslots list
func (fct *factory) GenerateStartSlotSubslot() error {
	return fct.generateStartSlotSubslot()
}

// GenerateBlockSubslot generates the instance of subslot Block and added it to the chronology subslots list
func (fct *factory) GenerateBlockSubslot() error {
	return fct.generateBlockSubslot()
}

// GenerateSignatureSubslot generates the instance of subslot Signature and added it to the chronology subslots list
func (fct *factory) GenerateSignatureSubslot() error {
	return fct.generateSignatureSubslot()
}

// GenerateEndSlotSubslot generates the instance of subslot EndSlot and added it to the chronology subslots list
func (fct *factory) GenerateEndSlotSubslot() error {
	return fct.generateEndSlotSubslot()
}

// AppStatusHandler gets the app status handler object
func (fct *factory) AppStatusHandler() core.AppStatusHandler {
	return fct.appStatusHandler
}

// Indexer gets the indexer object
func (fct *factory) Indexer() slot.ConsensusDataIndexer {
	return fct.indexer
}

// subslotStartSlot

// SubslotStartSlot defines a type for the subslotStartSlot structure
type SubslotStartSlot *subslotStartSlot

// DoStartSlotJob method does the job of the subslot StartSlot
func (sr *subslotStartSlot) DoStartSlotJob() bool {
	return sr.doStartSlotJob()
}

// DoStartSlotConsensusCheck method checks if the consensus is achieved in the subslot StartSlot
func (sr *subslotStartSlot) DoStartSlotConsensusCheck() bool {
	return sr.doStartSlotConsensusCheck()
}

// GenerateNextConsensusGroup generates the next consensu group based on current (random seed, shard id and slot)
func (sr *subslotStartSlot) GenerateNextConsensusGroup(slotIndex int64) error {
	return sr.generateNextConsensusGroup(slotIndex)
}

// InitCurrentSlot inits all the stuff needed in the current slot
func (sr *subslotStartSlot) InitCurrentSlot() bool {
	return sr.initCurrentSlot()
}

// subslotBlock

// SubslotBlock defines a type for the subslotBlock structure
type SubslotBlock *subslotBlock

// Blockchain gets the ChainHandler stored in the ConsensusCore
func (sr *subslotBlock) BlockChain() data.ChainHandler {
	return sr.Blockchain()
}

// DoBlockJob method does the job of the subslot Block
func (sr *subslotBlock) DoBlockJob() bool {
	return sr.doBlockJob()
}

// ProcessReceivedBlock method processes the received proposed block in the subslot Block
func (sr *subslotBlock) ProcessReceivedBlock(cnsDta *consensus.Message) bool {
	return sr.processReceivedBlock(cnsDta)
}

// DoBlockConsensusCheck method checks if the consensus in the subslot Block is achieved
func (sr *subslotBlock) DoBlockConsensusCheck() bool {
	return sr.doBlockConsensusCheck()
}

// IsBlockReceived method checks if the block was received from the leader in the current slot
func (sr *subslotBlock) IsBlockReceived(threshold int) bool {
	return sr.isBlockReceived(threshold)
}

// CreateHeader method creates the proposed block header in the subslot Block
func (sr *subslotBlock) CreateHeader() (data.HeaderHandler, error) {
	return sr.createHeader()
}

// CreateBody method creates the proposed block header in the subslot Block
func (sr *subslotBlock) CreateBlock(hdr data.HeaderHandler) (data.HeaderHandler, error) {
	return sr.createBlock(hdr)
}

// SendBlockHeader method sends the proposed block header in the subslot Block
func (sr *subslotBlock) SendBlockHeader(header data.HeaderHandler, headerHash []byte, marshalizedHeader []byte) bool {
	return sr.sendBlockHeader(header, headerHash, marshalizedHeader)
}

// ComputeSubslotProcessingMetric computes processing metric related to the subslot Block
func (sr *subslotBlock) ComputeSubslotProcessingMetric(startTime time.Time, metric string) {
	sr.computeSubslotProcessingMetric(startTime, metric)
}

// ReceivedBlockHeader method is called when a block header is received through the block header channel
func (sr *subslotBlock) ReceivedBlockHeader(cnsDta *consensus.Message) bool {
	return sr.receivedBlockHeader(cnsDta)
}

// subslotSignature

// SubslotSignature defines a type for the subslotSignature structure
type SubslotSignature *subslotSignature

// DoSignatureJob method does the job of the subslot Signature
func (sr *subslotSignature) DoSignatureJob() bool {
	return sr.doSignatureJob()
}

// ReceivedSignature method is called when a signature is received through the signature channel
func (sr *subslotSignature) ReceivedSignature(cnsDta *consensus.Message) bool {
	return sr.receivedSignature(cnsDta)
}

// DoSignatureConsensusCheck method checks if the consensus in the subslot Signature is achieved
func (sr *subslotSignature) DoSignatureConsensusCheck() bool {
	return sr.doSignatureConsensusCheck()
}

// AreSignaturesCollected method checks if the number of signatures received from the nodes are more than the given threshold
func (sr *subslotSignature) AreSignaturesCollected(threshold int) (bool, int) {
	return sr.areSignaturesCollected(threshold)
}

// subslotEndSlot

// SubslotEndSlot defines a type for the subslotEndSlot structure
type SubslotEndSlot *subslotEndSlot

// DoEndSlotJob method does the job of the subslot EndSlot
func (sr *subslotEndSlot) DoEndSlotJob() bool {
	return sr.doEndSlotJob()
}

// DoEndSlotConsensusCheck method checks if the consensus is achieved
func (sr *subslotEndSlot) DoEndSlotConsensusCheck() bool {
	return sr.doEndSlotConsensusCheck()
}

// CheckSignaturesValidity method checks the signature validity for the nodes included in bitmap
func (sr *subslotEndSlot) CheckSignaturesValidity(bitmap []byte) error {
	return sr.checkSignaturesValidity(bitmap)
}

func (sr *subslotEndSlot) DoEndSlotJobByParticipant(cnsDta *consensus.Message) bool {
	return sr.doEndSlotJobByParticipant(cnsDta)
}

func (sr *subslotEndSlot) HaveConsensusHeaderWithFullInfo(cnsDta *consensus.Message) (bool, data.HeaderHandler) {
	return sr.haveConsensusHeaderWithFullInfo(cnsDta)
}

func (sr *subslotEndSlot) CreateAndBroadcastHeaderFinalInfo(header data.HeaderHandler) {
	sr.createAndBroadcastHeaderFinalInfo(header)
}

func (sr *subslotEndSlot) ReceivedBlockHeaderFinalInfo(cnsDta *consensus.Message) bool {
	return sr.receivedBlockHeaderFinalInfo(cnsDta)
}

func (sr *subslotEndSlot) IsBlockHeaderFinalInfoValid(cnsDta *consensus.Message) bool {
	return sr.isBlockHeaderFinalInfoValid(cnsDta)
}

func (sr *subslotEndSlot) IsConsensusHeaderReceived() (bool, data.HeaderHandler) {
	return sr.isConsensusHeaderReceived()
}

func (sr *subslotEndSlot) IsOutOfTime() bool {
	return sr.isOutOfTime()
}

// GetStringValue gets the name of the message type
func GetStringValue(messageType consensus.MessageType) string {
	return getStringValue(messageType)
}
