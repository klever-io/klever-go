package process

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/node/heartbeat"
	"github.com/klever-io/klever-go/node/heartbeat/data"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/storage/timecache"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var log = logger.GetOrCreate("heartbeat/process")

const defaultMaxUnknownHeartbeatPubKeys = 1024
const defaultMaxUnknownHeartbeatPubKeysPerOrigin = 32

type transientUnknownHeartbeatInfo struct {
	originPeer core.PeerID
	lastSeen   time.Time
}

// ArgHeartbeatMonitor represents the arguments for the heartbeat monitor
type ArgHeartbeatMonitor struct {
	Marshalizer                         marshal.Marshalizer
	MaxDurationPeerUnresponsive         time.Duration
	PubKeysList                         []string
	GenesisTime                         time.Time
	MessageHandler                      heartbeat.MessageHandler
	Storer                              heartbeat.HeartbeatStorageHandler
	PeerTypeProvider                    heartbeat.PeerTypeProviderHandler
	Timer                               heartbeat.Timer
	AntifloodHandler                    heartbeat.P2PAntifloodHandler
	ValidatorPubkeyConverter            core.PubkeyConverter
	HeartbeatRefreshIntervalInSec       uint32
	HideInactiveValidatorIntervalInSec  uint32
	MaxUnknownHeartbeatPubKeys          uint32
	MaxUnknownHeartbeatPubKeysPerOrigin uint32
}

// Monitor represents the heartbeat component that processes received heartbeat messages
type Monitor struct {
	maxDurationPeerUnresponsive         time.Duration
	marshalizer                         marshal.Marshalizer
	peerTypeProvider                    heartbeat.PeerTypeProviderHandler
	mutHeartbeatMessages                sync.RWMutex
	mutAdmittedHeartbeatPubKeys         sync.RWMutex
	mutTransientUnknownHeartbeatPubKeys sync.Mutex
	mutAppStatusHandler                 sync.Mutex
	heartbeatMessages                   map[string]*heartbeatMessageInfo
	admittedHeartbeatPubKeys            map[string]struct{}
	transientUnknownHeartbeatPubKeys    map[string]transientUnknownHeartbeatInfo
	recomputeCh                         chan struct{}
	doubleSignerPeers                   map[string]process.TimeCacher
	pubKeysList                         []string
	mutFullPeersSlice                   sync.RWMutex
	fullPeersSlice                      [][]byte
	appStatusHandler                    core.AppStatusHandler
	genesisTime                         time.Time
	messageHandler                      heartbeat.MessageHandler
	storer                              heartbeat.HeartbeatStorageHandler
	timer                               heartbeat.Timer
	antifloodHandler                    heartbeat.P2PAntifloodHandler
	validatorPubkeyConverter            core.PubkeyConverter
	heartbeatRefreshIntervalInSec       uint32
	hideInactiveValidatorIntervalInSec  uint32
	maxUnknownHeartbeatPubKeys          uint32
	maxUnknownHeartbeatPubKeysPerOrigin uint32
	numActiveValidators                 uint64
	numActiveConsensusValidators        uint64
	numConnectedNodes                   uint64
	stopCh                              chan struct{}
	wg                                  sync.WaitGroup
	closeOnce                           sync.Once
}

// NewMonitor returns a new monitor instance
func NewMonitor(arg ArgHeartbeatMonitor) (*Monitor, error) {
	err := checkArgHeartbeatMonitor(arg)
	if err != nil {
		return nil, err
	}

	maxUnknownHeartbeatPubKeys := arg.MaxUnknownHeartbeatPubKeys
	if maxUnknownHeartbeatPubKeys == 0 {
		maxUnknownHeartbeatPubKeys = defaultMaxUnknownHeartbeatPubKeys
	}

	maxUnknownHeartbeatPubKeysPerOrigin := arg.MaxUnknownHeartbeatPubKeysPerOrigin
	if maxUnknownHeartbeatPubKeysPerOrigin == 0 {
		maxUnknownHeartbeatPubKeysPerOrigin = defaultMaxUnknownHeartbeatPubKeysPerOrigin
	}
	if maxUnknownHeartbeatPubKeysPerOrigin > maxUnknownHeartbeatPubKeys {
		return nil, fmt.Errorf("%w for MaxUnknownHeartbeatPubKeysPerOrigin", heartbeat.ErrWrongValues)
	}

	mon := &Monitor{
		marshalizer:                         arg.Marshalizer,
		heartbeatMessages:                   make(map[string]*heartbeatMessageInfo),
		admittedHeartbeatPubKeys:            make(map[string]struct{}),
		transientUnknownHeartbeatPubKeys:    make(map[string]transientUnknownHeartbeatInfo),
		peerTypeProvider:                    arg.PeerTypeProvider,
		maxDurationPeerUnresponsive:         arg.MaxDurationPeerUnresponsive,
		appStatusHandler:                    &statusHandler.NilStatusHandler{},
		genesisTime:                         arg.GenesisTime,
		messageHandler:                      arg.MessageHandler,
		storer:                              arg.Storer,
		timer:                               arg.Timer,
		antifloodHandler:                    arg.AntifloodHandler,
		validatorPubkeyConverter:            arg.ValidatorPubkeyConverter,
		heartbeatRefreshIntervalInSec:       arg.HeartbeatRefreshIntervalInSec,
		hideInactiveValidatorIntervalInSec:  arg.HideInactiveValidatorIntervalInSec,
		maxUnknownHeartbeatPubKeys:          maxUnknownHeartbeatPubKeys,
		maxUnknownHeartbeatPubKeysPerOrigin: maxUnknownHeartbeatPubKeysPerOrigin,
		doubleSignerPeers:                   make(map[string]process.TimeCacher),
		recomputeCh:                         make(chan struct{}, 1),
		stopCh:                              make(chan struct{}),
	}

	err = mon.storer.UpdateGenesisTime(arg.GenesisTime)
	if err != nil {
		return nil, err
	}

	pubKeysToSave, err := mon.initializeHeartbeatMessagesInfo(arg.PubKeysList)
	if err != nil {
		return nil, err
	}

	err = mon.loadRestOfPubKeysFromStorage()
	if err != nil {
		log.Debug("heartbeat can't load public keys from storage", "error", err.Error())
	}

	// Initial refresh runs synchronously, then the newly created records are
	// persisted post-refresh. Deferring either to a goroutine lets it execute
	// at an arbitrary later time, racing callers that advance the timer.
	mon.refreshHeartbeatMessageInfo()
	mon.SaveMultipleHeartbeatMessageInfos(pubKeysToSave)

	mon.startValidatorProcessing()

	return mon, nil
}

func checkArgHeartbeatMonitor(arg ArgHeartbeatMonitor) error {
	if check.IfNil(arg.Marshalizer) {
		return heartbeat.ErrNilMarshalizer
	}
	if check.IfNil(arg.PeerTypeProvider) {
		return heartbeat.ErrNilPeerTypeProvider
	}
	if arg.PubKeysList == nil {
		return heartbeat.ErrNilPublicKeysMap
	}
	if check.IfNil(arg.MessageHandler) {
		return heartbeat.ErrNilMessageHandler
	}
	if check.IfNil(arg.Storer) {
		return heartbeat.ErrNilHeartbeatStorer
	}
	if check.IfNil(arg.Timer) {
		return heartbeat.ErrNilTimer
	}
	if check.IfNil(arg.AntifloodHandler) {
		return heartbeat.ErrNilAntifloodHandler
	}
	if check.IfNil(arg.ValidatorPubkeyConverter) {
		return heartbeat.ErrNilPubkeyConverter
	}
	if arg.HeartbeatRefreshIntervalInSec == 0 {
		return heartbeat.ErrZeroHeartbeatRefreshIntervalInSec
	}
	if arg.HideInactiveValidatorIntervalInSec == 0 {
		return heartbeat.ErrZeroHideInactiveValidatorIntervalInSec
	}
	if arg.MaxDurationPeerUnresponsive == 0 {
		return heartbeat.ErrInvalidMaxDurationPeerUnresponsive
	}

	return nil
}

func (m *Monitor) initializeHeartbeatMessagesInfo(pubKeysList []string) (map[string]*heartbeatMessageInfo, error) {
	pubKeysListCopy := make([]string, 0)
	pubKeysToSave := make(map[string]*heartbeatMessageInfo)

	for _, pubkey := range pubKeysList {
		e := m.initializeHeartBeatForPK(pubkey, pubKeysToSave, &pubKeysListCopy)
		if e != nil {
			return nil, e
		}
	}

	m.pubKeysList = pubKeysListCopy
	return pubKeysToSave, nil
}

func (m *Monitor) initializeHeartBeatForPK(
	pubkey string,
	pubKeysToSave map[string]*heartbeatMessageInfo,
	pubKeysListCopy *[]string,
) error {
	m.markHeartbeatPubKeyAsAdmitted(pubkey)
	hbmi, err := m.loadHeartbeatsFromStorer(pubkey)
	if err != nil { // if pubKey not found in DB, create a new instance
		peerType := m.computePeerType([]byte(pubkey))
		hbmi, err = newHeartbeatMessageInfo(m.maxDurationPeerUnresponsive, peerType, m.genesisTime, m.timer)
		if err != nil {
			return err
		}

		hbmi.genesisTime = m.genesisTime
		pubKeysToSave[pubkey] = hbmi
	}
	m.heartbeatMessages[pubkey] = hbmi
	*pubKeysListCopy = append(*pubKeysListCopy, pubkey)
	return nil
}

// SaveMultipleHeartbeatMessageInfos stores the given heartbeatMessageInfos to the storer,
// skipping any public key that is not admitted (transient/unknown identities are never persisted)
func (m *Monitor) SaveMultipleHeartbeatMessageInfos(pubKeysToSave map[string]*heartbeatMessageInfo) {
	savedPubKeys := make([][]byte, 0, len(pubKeysToSave))

	m.mutHeartbeatMessages.RLock()
	for key, hmbi := range pubKeysToSave {
		if !m.isAdmittedHeartbeatPubKey(key) {
			continue
		}

		hbDTO := m.convertToExportedStruct(hmbi)
		err := m.storer.SavePubkeyData([]byte(key), hbDTO)
		if err != nil {
			log.Debug("cannot save heartbeat to db", "error", err.Error())
			continue
		}
		savedPubKeys = append(savedPubKeys, []byte(key))
	}
	m.mutHeartbeatMessages.RUnlock()

	m.addPeersToFullPeersSlice(savedPubKeys)
}

func (m *Monitor) loadRestOfPubKeysFromStorage() error {
	peersSlice, err := m.storer.LoadKeys()
	if err != nil {
		return err
	}

	allowedPubKeys := make(map[string]struct{}, len(m.pubKeysList))
	for _, pubKey := range m.pubKeysList {
		allowedPubKeys[pubKey] = struct{}{}
	}
	for _, peerTypeInfo := range m.peerTypeProvider.GetAllPeerTypeInfos() {
		if len(peerTypeInfo.PublicKey) > 0 {
			allowedPubKeys[peerTypeInfo.PublicKey] = struct{}{}
		}
	}

	filteredPeersSlice := make([][]byte, 0, len(peersSlice))
	for _, peer := range peersSlice {
		pubKey := string(peer)

		if _, isAllowed := allowedPubKeys[pubKey]; !isAllowed {
			if err = m.storer.RemovePubkeyData(peer); err != nil {
				log.Debug("cannot remove unknown heartbeat from db", "pubkey", pubKey, "error", err.Error())
			}
			continue
		}

		filteredPeersSlice = append(filteredPeersSlice, peer)
		m.markHeartbeatPubKeyAsAdmitted(pubKey)
		_, ok := m.heartbeatMessages[pubKey]
		if !ok { // peer not in nodes map
			hbmi, err1 := m.loadHeartbeatsFromStorer(pubKey)
			if err1 != nil {
				continue
			}
			m.heartbeatMessages[pubKey] = hbmi
		}
	}
	if len(filteredPeersSlice) != len(peersSlice) {
		if err = m.storer.SaveKeys(filteredPeersSlice); err != nil {
			log.Debug("cannot save filtered heartbeat keys", "error", err.Error())
		}
	}
	m.fullPeersSlice = filteredPeersSlice

	return nil
}

func (m *Monitor) loadHeartbeatsFromStorer(pubKey string) (*heartbeatMessageInfo, error) {
	heartbeatDTO, err := m.storer.LoadHeartBeatDTO(pubKey)
	if err != nil {
		return nil, err
	}

	receivedHbmi := m.convertFromExportedStruct(heartbeatDTO, m.maxDurationPeerUnresponsive)
	receivedHbmi.getTimeHandler = m.timer.Now
	crtTime := m.timer.Now()
	crtDuration := crtTime.Sub(receivedHbmi.lastUptimeDowntime)
	crtDuration = maxDuration(0, crtDuration)
	if receivedHbmi.isActive {
		receivedHbmi.totalUpTime += crtDuration
		receivedHbmi.timestamp = crtTime
	} else {
		receivedHbmi.totalDownTime += crtDuration
	}
	receivedHbmi.lastUptimeDowntime = crtTime
	receivedHbmi.genesisTime = m.genesisTime

	return receivedHbmi, nil
}

// SetAppStatusHandler will set the AppStatusHandler which will be used for monitoring
func (m *Monitor) SetAppStatusHandler(ash core.AppStatusHandler) error {
	if check.IfNil(ash) {
		return heartbeat.ErrNilAppStatusHandler
	}

	m.mutAppStatusHandler.Lock()
	m.appStatusHandler = ash
	// re-publish the metrics computed by the constructor's initial refresh,
	// which only reached the placeholder handler
	m.appStatusHandler.SetUInt64Value(core.MetricLiveValidatorNodes, m.numActiveValidators)
	m.appStatusHandler.SetUInt64Value(core.MetricLiveConsensusValidatorNodes, m.numActiveConsensusValidators)
	m.appStatusHandler.SetUInt64Value(core.MetricConnectedNodes, m.numConnectedNodes)
	m.mutAppStatusHandler.Unlock()
	return nil
}

// ProcessReceivedMessage satisfies the p2p.MessageProcessor interface so it can be called
// by the p2p subsystem each time a new heartbeat message arrives
func (m *Monitor) ProcessReceivedMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
	if check.IfNil(message) {
		return heartbeat.ErrNilMessage
	}
	if message.Data() == nil {
		return heartbeat.ErrNilDataToProcess
	}

	err := m.antifloodHandler.CanProcessMessage(message, fromConnectedPeer)
	if err != nil {
		return err
	}
	err = m.antifloodHandler.CanProcessMessagesOnTopic(fromConnectedPeer, common.HeartbeatTopic, 1, uint64(len(message.Data())), message.SeqNo())
	if err != nil {
		return err
	}

	hbRecv, err := m.messageHandler.CreateHeartbeatFromP2PMessage(message)
	if err != nil {
		//this situation is so severe that we have to black list both the message originator and the connected peer
		//that disseminated this message.
		blacklistReason := process.BlacklistReasonInvalidHeartbeat
		if errors.Is(err, heartbeat.ErrHeartbeatPidMismatch) {
			blacklistReason = process.BlacklistReasonInconsistentHeartbeat
		}
		log.Debug("Monitor: invalid heartbeat message",
			"originator", p2p.PeerIDToShortString(message.Peer()),
			"err", process.SanitizeBlacklistReason(err.Error()))
		m.antifloodHandler.BlacklistPeer(message.Peer(), blacklistReason, core.InvalidMessageBlacklistDuration)
		m.antifloodHandler.BlacklistPeer(fromConnectedPeer, blacklistReason, core.InvalidMessageBlacklistDuration)

		return err
	}

	//kept as defense in depth: the message handler already rejects a heartbeat whose pid is not the message
	//originator, before storing any peer id - public key association
	if !bytes.Equal(hbRecv.Pid, message.Peer().Bytes()) {
		//this situation is so severe that we have to black list both the message originator and the connected peer
		//that disseminated this message.
		log.Debug("Monitor: inconsistent heartbeat message",
			"originator", p2p.PeerIDToShortString(message.Peer()),
			"hbPid", p2p.PeerIDToShortString(core.PeerID(hbRecv.Pid)))
		m.antifloodHandler.BlacklistPeer(message.Peer(), process.BlacklistReasonInconsistentHeartbeat, core.InvalidMessageBlacklistDuration)
		m.antifloodHandler.BlacklistPeer(fromConnectedPeer, process.BlacklistReasonInconsistentHeartbeat, core.InvalidMessageBlacklistDuration)

		return fmt.Errorf("%w heartbeat pid %s, message pid %s",
			heartbeat.ErrHeartbeatPidMismatch,
			p2p.PeerIDToShortString(core.PeerID(hbRecv.Pid)),
			p2p.PeerIDToShortString(message.Peer()),
		)
	}

	//message is validated, process should be done async, method can return nil
	go m.processValidatedHeartbeat(hbRecv, fromConnectedPeer)

	return nil
}

func (m *Monitor) processValidatedHeartbeat(hb *data.Heartbeat, fromConnectedPeer core.PeerID) {
	if !m.addHeartbeatMessageToMap(hb, fromConnectedPeer) {
		return
	}

	m.scheduleHeartbeatRecompute()
}

func (m *Monitor) scheduleHeartbeatRecompute() {
	select {
	case m.recomputeCh <- struct{}{}:
	default:
	}
}

func (m *Monitor) addHeartbeatMessageToMap(hb *data.Heartbeat, fromConnectedPeer core.PeerID) bool {
	pubKeyStr := string(hb.Pubkey)
	isAdmittedPubKey, droppedPubKeys, accepted := m.admitHeartbeatPubKey(pubKeyStr, fromConnectedPeer)
	if !accepted {
		return false
	}

	m.mutHeartbeatMessages.Lock()
	m.dropLiveHeartbeatStateLocked(droppedPubKeys)
	isAdmittedPubKey, stillAccepted := m.confirmHeartbeatAdmissionLocked(pubKeyStr, isAdmittedPubKey)
	if !stillAccepted {
		m.mutHeartbeatMessages.Unlock()
		return false
	}
	hbmi, err := m.getOrCreateHeartbeatMessageInfoLocked(hb)
	if err != nil {
		log.Debug("error creating heartbeat message info", "error", err.Error())
		m.mutHeartbeatMessages.Unlock()
		return false
	}
	if len(hb.Pid) > 0 {
		m.addDoubleSignerPeers(hb)
	}
	numInstances := m.getNumInstancesOfPublicKey(pubKeyStr)
	m.mutHeartbeatMessages.Unlock()

	peerType := m.computePeerType(hb.Pubkey)

	hbmi.HeartbeatReceived(
		hb.VersionNumber,
		hb.NodeDisplayName,
		hb.Identity,
		peerType,
		hb.Nonce,
		numInstances,
	)
	if !isAdmittedPubKey {
		return true
	}

	hbDTO := m.convertToExportedStruct(hbmi)

	err = m.storer.SavePubkeyData(hb.Pubkey, hbDTO)
	if err != nil {
		log.Debug("cannot save heartbeat to db", "error", err.Error())
		return true
	}
	m.addPeersToFullPeersSlice([][]byte{hb.Pubkey})

	return true
}

func (m *Monitor) admitHeartbeatPubKey(pubKeyStr string, fromConnectedPeer core.PeerID) (isAdmitted bool, droppedPubKeys []string, accepted bool) {
	if m.isAdmittedHeartbeatPubKey(pubKeyStr) {
		return true, nil, true
	}

	tracked, droppedPubKeys := m.trackTransientUnknownHeartbeatPubKey(pubKeyStr, fromConnectedPeer)
	if tracked {
		return false, droppedPubKeys, true
	}

	if len(droppedPubKeys) > 0 {
		m.mutHeartbeatMessages.Lock()
		m.dropLiveHeartbeatStateLocked(droppedPubKeys)
		m.mutHeartbeatMessages.Unlock()
	}

	return false, nil, false
}

// requires mutHeartbeatMessages held; acquires mutAdmittedHeartbeatPubKeys and mutTransientUnknownHeartbeatPubKeys
func (m *Monitor) confirmHeartbeatAdmissionLocked(pubKeyStr string, isAdmittedPubKey bool) (isAdmitted bool, stillAccepted bool) {
	if isAdmittedPubKey || m.isAdmittedHeartbeatPubKey(pubKeyStr) {
		m.untrackTransientUnknownHeartbeatPubKey(pubKeyStr)
		return true, true
	}

	return false, m.isTransientUnknownHeartbeatPubKeyTracked(pubKeyStr)
}

// requires mutHeartbeatMessages held
func (m *Monitor) getOrCreateHeartbeatMessageInfoLocked(hb *data.Heartbeat) (*heartbeatMessageInfo, error) {
	pubKeyStr := string(hb.Pubkey)
	hbmi, ok := m.heartbeatMessages[pubKeyStr]
	if ok && hbmi != nil {
		return hbmi, nil
	}

	peerType := m.computePeerType(hb.Pubkey)
	hbmi, err := newHeartbeatMessageInfo(m.maxDurationPeerUnresponsive, peerType, m.genesisTime, m.timer)
	if err != nil {
		return nil, err
	}
	m.heartbeatMessages[pubKeyStr] = hbmi

	return hbmi, nil
}

func (m *Monitor) addPeersToFullPeersSlice(pubKeys [][]byte) {
	if len(pubKeys) == 0 {
		return
	}

	m.mutFullPeersSlice.Lock()
	defer m.mutFullPeersSlice.Unlock()

	previousLen := len(m.fullPeersSlice)
	for _, pubKey := range pubKeys {
		if m.isPeerInFullPeersSlice(pubKey) {
			continue
		}
		m.fullPeersSlice = append(m.fullPeersSlice, pubKey)
	}
	if len(m.fullPeersSlice) == previousLen {
		return
	}

	err := m.storer.SaveKeys(m.fullPeersSlice)
	if err != nil {
		m.fullPeersSlice = m.fullPeersSlice[:previousLen]
		log.Debug("can't store the keys slice", "error", err.Error())
	}
}

func (m *Monitor) isAdmittedHeartbeatPubKey(pubKey string) bool {
	m.mutAdmittedHeartbeatPubKeys.RLock()
	_, ok := m.admittedHeartbeatPubKeys[pubKey]
	m.mutAdmittedHeartbeatPubKeys.RUnlock()

	return ok
}

func (m *Monitor) markHeartbeatPubKeyAsAdmitted(pubKey string) bool {
	m.mutAdmittedHeartbeatPubKeys.Lock()
	_, alreadyAdmitted := m.admittedHeartbeatPubKeys[pubKey]
	m.admittedHeartbeatPubKeys[pubKey] = struct{}{}
	m.mutAdmittedHeartbeatPubKeys.Unlock()

	m.untrackTransientUnknownHeartbeatPubKey(pubKey)

	return !alreadyAdmitted
}

func (m *Monitor) untrackTransientUnknownHeartbeatPubKey(pubKey string) {
	m.mutTransientUnknownHeartbeatPubKeys.Lock()
	delete(m.transientUnknownHeartbeatPubKeys, pubKey)
	m.mutTransientUnknownHeartbeatPubKeys.Unlock()
}

func (m *Monitor) trackTransientUnknownHeartbeatPubKey(pubKey string, originPeer core.PeerID) (bool, []string) {
	m.mutTransientUnknownHeartbeatPubKeys.Lock()
	defer m.mutTransientUnknownHeartbeatPubKeys.Unlock()

	droppedPubKeys := m.sweepTransientUnknownHeartbeatPubKeysLocked()

	if existing, ok := m.transientUnknownHeartbeatPubKeys[pubKey]; ok {
		existing.lastSeen = m.timer.Now()
		m.transientUnknownHeartbeatPubKeys[pubKey] = existing
		return true, droppedPubKeys
	}

	if m.countTransientUnknownHeartbeatPubKeysForOriginLocked(originPeer) >= int(m.maxUnknownHeartbeatPubKeysPerOrigin) {
		return false, droppedPubKeys
	}

	if len(m.transientUnknownHeartbeatPubKeys) >= int(m.maxUnknownHeartbeatPubKeys) {
		evictedPubKey, evicted := m.evictOldestTransientUnknownHeartbeatPubKeyLocked()
		if evicted {
			droppedPubKeys = append(droppedPubKeys, evictedPubKey)
		}
	}

	m.transientUnknownHeartbeatPubKeys[pubKey] = transientUnknownHeartbeatInfo{
		originPeer: originPeer,
		lastSeen:   m.timer.Now(),
	}

	return true, droppedPubKeys
}

// requires mutHeartbeatMessages held; acquires mutAdmittedHeartbeatPubKeys and
// mutTransientUnknownHeartbeatPubKeys. Never call with mutTransientUnknownHeartbeatPubKeys held.
func (m *Monitor) dropLiveHeartbeatStateLocked(pubKeys []string) {
	for _, pubKey := range pubKeys {
		if m.isAdmittedHeartbeatPubKey(pubKey) || m.isTransientUnknownHeartbeatPubKeyTracked(pubKey) {
			continue
		}

		delete(m.heartbeatMessages, pubKey)
		delete(m.doubleSignerPeers, pubKey)
	}
}

func (m *Monitor) isTransientUnknownHeartbeatPubKeyTracked(pubKey string) bool {
	m.mutTransientUnknownHeartbeatPubKeys.Lock()
	_, ok := m.transientUnknownHeartbeatPubKeys[pubKey]
	m.mutTransientUnknownHeartbeatPubKeys.Unlock()

	return ok
}

// requires mutTransientUnknownHeartbeatPubKeys held
func (m *Monitor) countTransientUnknownHeartbeatPubKeysForOriginLocked(originPeer core.PeerID) int {
	count := 0
	for _, info := range m.transientUnknownHeartbeatPubKeys {
		if info.originPeer == originPeer {
			count++
		}
	}

	return count
}

// requires mutTransientUnknownHeartbeatPubKeys held
func (m *Monitor) sweepTransientUnknownHeartbeatPubKeysLocked() []string {
	now := m.timer.Now()
	droppedPubKeys := make([]string, 0)
	for pubKey, info := range m.transientUnknownHeartbeatPubKeys {
		if now.Sub(info.lastSeen) > m.maxDurationPeerUnresponsive {
			delete(m.transientUnknownHeartbeatPubKeys, pubKey)
			droppedPubKeys = append(droppedPubKeys, pubKey)
		}
	}

	return droppedPubKeys
}

// requires mutTransientUnknownHeartbeatPubKeys held
func (m *Monitor) evictOldestTransientUnknownHeartbeatPubKeyLocked() (string, bool) {
	var oldestKey string
	var oldestTime time.Time
	hasOldest := false

	for pubKey, info := range m.transientUnknownHeartbeatPubKeys {
		if !hasOldest || info.lastSeen.Before(oldestTime) {
			oldestKey = pubKey
			oldestTime = info.lastSeen
			hasOldest = true
		}
	}

	if hasOldest {
		delete(m.transientUnknownHeartbeatPubKeys, oldestKey)
	}

	return oldestKey, hasOldest
}

func (m *Monitor) isPeerInFullPeersSlice(pubKey []byte) bool {
	for _, peer := range m.fullPeersSlice {
		if bytes.Equal(peer, pubKey) {
			return true
		}
	}

	return false
}

func (m *Monitor) computePeerType(pubkey []byte) string {
	peerType, _, err := m.peerTypeProvider.ComputeForPubKey(pubkey)
	if err != nil {
		log.Warn("monitor: compute peer type and shard", "error", err)
		return string(core.ObserverList)
	}

	return string(peerType)
}

func (m *Monitor) computeAllHeartbeatMessages() {
	m.mutHeartbeatMessages.Lock()
	counterActiveValidators := 0
	counterActiveConsensusValidators := 0
	counterConnectedNodes := 0
	hbChangedStateToInactiveMap := make(map[string]*heartbeatMessageInfo)
	for key, v := range m.heartbeatMessages {
		previousActive := v.GetIsActive()
		v.ComputeActive(m.timer.Now())
		isActive := v.GetIsActive()

		if isActive {
			counterConnectedNodes++

			if v.GetIsValidator() {
				counterActiveValidators++
			}
			if v.GetIsConsensusCapable() {
				counterActiveConsensusValidators++
			}
		}
		changedStateToInactive := previousActive && !isActive
		if changedStateToInactive {
			hbChangedStateToInactiveMap[key] = v
		}
	}

	m.mutHeartbeatMessages.Unlock()
	m.SaveMultipleHeartbeatMessageInfos(hbChangedStateToInactiveMap)

	m.mutAppStatusHandler.Lock()
	m.numActiveValidators = uint64(counterActiveValidators)                   // #nosec G115
	m.numActiveConsensusValidators = uint64(counterActiveConsensusValidators) // #nosec G115
	m.numConnectedNodes = uint64(counterConnectedNodes)                       // #nosec G115
	m.appStatusHandler.SetUInt64Value(core.MetricLiveValidatorNodes, m.numActiveValidators)
	m.appStatusHandler.SetUInt64Value(core.MetricLiveConsensusValidatorNodes, m.numActiveConsensusValidators)
	m.appStatusHandler.SetUInt64Value(core.MetricConnectedNodes, m.numConnectedNodes)
	m.mutAppStatusHandler.Unlock()
}

func (m *Monitor) getValsForUpdate(hbmiKey string, hbmi *heartbeatMessageInfo) (bool, string) {
	hbmi.updateMutex.RLock()
	defer hbmi.updateMutex.RUnlock()

	if hbmi.isActive {
		return false, ""
	}

	peerType := m.computePeerType([]byte(hbmiKey))
	if hbmi.peerType != peerType {
		return true, peerType
	}

	return false, ""
}

func (m *Monitor) computeInactiveHeartbeatMessages() {
	m.mutHeartbeatMessages.Lock()
	inactiveHbChangedMap := make(map[string]*heartbeatMessageInfo)
	for key, v := range m.heartbeatMessages {
		shouldUpdate, peerType := m.getValsForUpdate(key, v)
		if shouldUpdate {
			v.UpdatePeerType(peerType)
			inactiveHbChangedMap[key] = v
		}
	}

	peerTypeInfos := m.peerTypeProvider.GetAllPeerTypeInfos()
	for _, peerTypeInfo := range peerTypeInfos {
		newlyAdmitted := m.markHeartbeatPubKeyAsAdmitted(peerTypeInfo.PublicKey)
		hbmi := m.heartbeatMessages[peerTypeInfo.PublicKey]
		if hbmi == nil {
			var err error
			hbmi, err = newHeartbeatMessageInfo(m.maxDurationPeerUnresponsive, peerTypeInfo.PeerType, m.genesisTime, m.timer)
			if err != nil {
				log.Debug("could not create hbmi ", "err", err)
				continue
			}
			m.heartbeatMessages[peerTypeInfo.PublicKey] = hbmi
			continue
		}
		if newlyAdmitted {
			inactiveHbChangedMap[peerTypeInfo.PublicKey] = hbmi
		}
	}

	m.mutHeartbeatMessages.Unlock()
	m.SaveMultipleHeartbeatMessageInfos(inactiveHbChangedMap)
}

// GetHeartbeats returns the heartbeat status
func (m *Monitor) GetHeartbeats() []data.PubKeyHeartbeat {
	m.Cleanup()

	m.mutHeartbeatMessages.Lock()
	status := make([]data.PubKeyHeartbeat, 0, len(m.heartbeatMessages))
	for k, v := range m.heartbeatMessages {
		v.updateMutex.RLock()
		tmp := data.PubKeyHeartbeat{
			PublicKey: m.validatorPubkeyConverter.Encode([]byte(k)),
			Timestamp: v.timestamp,
			MaxInactiveTime: data.Duration{
				Duration: v.maxInactiveTime,
			},
			IsActive:        v.isActive,
			TotalUpTime:     int64(v.totalUpTime.Seconds()),
			TotalDownTime:   int64(v.totalDownTime.Seconds()),
			VersionNumber:   v.versionNumber,
			NodeDisplayName: v.nodeDisplayName,
			Identity:        v.identity,
			PeerType:        v.peerType,
			Nonce:           v.nonce,
			NumInstances:    v.numInstances,
		}
		v.updateMutex.RUnlock()
		status = append(status, tmp)
	}
	m.mutHeartbeatMessages.Unlock()

	sort.Slice(status, func(i, j int) bool {
		return strings.Compare(status[i].PublicKey, status[j].PublicKey) < 0
	})

	return status
}

func (m *Monitor) shouldSkipValidator(v *heartbeatMessageInfo) bool {
	// snapshot the fields under updateMutex: HeartbeatReceived writes them while
	// holding only that mutex, so raw reads here race with inbound heartbeats
	v.updateMutex.RLock()
	isActive := v.isActive
	peerType := v.peerType
	timestamp := v.timestamp
	v.updateMutex.RUnlock()

	// registered tier: jailed validators keep their heartbeat entry visible
	isInactiveNonValidator := !isActive && !isRegisteredValidatorPeerType(peerType)
	if isInactiveNonValidator {
		lastInactiveInterval := m.timer.Now().Sub(timestamp)
		if lastInactiveInterval.Seconds() > float64(m.hideInactiveValidatorIntervalInSec) {
			return true
		}
	}

	return false
}

// IsInterfaceNil returns true if there is no value under the interface
func (m *Monitor) IsInterfaceNil() bool {
	return m == nil
}

func (m *Monitor) convertToExportedStruct(v *heartbeatMessageInfo) *data.HeartbeatDTO {
	v.updateMutex.Lock()
	defer v.updateMutex.Unlock()
	ret := data.HeartbeatDTO{
		IsActive:        v.isActive,
		VersionNumber:   v.versionNumber,
		NodeDisplayName: v.nodeDisplayName,
		Identity:        v.identity,
		PeerType:        v.peerType,
		Nonce:           v.nonce,
		NumInstances:    v.numInstances,
	}

	ret.Timestamp = v.timestamp.UnixNano()
	ret.MaxInactiveTime = v.maxInactiveTime.Nanoseconds()
	ret.TotalUpTime = v.totalUpTime.Nanoseconds()
	ret.TotalDownTime = v.totalDownTime.Nanoseconds()
	ret.LastUptimeDowntime = v.lastUptimeDowntime.UnixNano()
	ret.GenesisTime = v.genesisTime.UnixNano()

	return &ret
}

func (m *Monitor) convertFromExportedStruct(hbDTO *data.HeartbeatDTO, maxDuration time.Duration) *heartbeatMessageInfo {
	hbmi := &heartbeatMessageInfo{
		maxDurationPeerUnresponsive: maxDuration,
		isActive:                    hbDTO.IsActive,
		versionNumber:               hbDTO.VersionNumber,
		nodeDisplayName:             hbDTO.NodeDisplayName,
		identity:                    hbDTO.Identity,
		peerType:                    hbDTO.PeerType,
		nonce:                       hbDTO.Nonce,
		numInstances:                hbDTO.NumInstances,
	}

	hbmi.maxInactiveTime = time.Duration(hbDTO.MaxInactiveTime)
	hbmi.timestamp = time.Unix(0, hbDTO.Timestamp)
	hbmi.totalUpTime = time.Duration(hbDTO.TotalUpTime)
	hbmi.totalDownTime = time.Duration(hbDTO.TotalDownTime)
	hbmi.lastUptimeDowntime = time.Unix(0, hbDTO.LastUptimeDowntime)
	hbmi.genesisTime = time.Unix(0, hbDTO.GenesisTime)

	return hbmi
}

// startValidatorProcessing starts the periodic refresh of the nodes' information.
// The initial refresh already ran synchronously in NewMonitor; this drives the
// recurring ticker updates and the recomputes requested by received heartbeats.
func (m *Monitor) startValidatorProcessing() {
	m.wg.Add(1)
	go m.runRefreshLoop()
}

func (m *Monitor) runRefreshLoop() {
	defer m.wg.Done()
	refreshInterval := time.Duration(m.heartbeatRefreshIntervalInSec) * time.Second
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			if m.isStopping() {
				return
			}
			m.refreshHeartbeatMessageInfo()
		case <-m.recomputeCh:
			if m.isStopping() {
				return
			}
			m.computeAllHeartbeatMessages()
		}
	}
}

func (m *Monitor) isStopping() bool {
	select {
	case <-m.stopCh:
		return true
	default:
		return false
	}
}

// Close will stop the background processing goroutine and wait for it to exit,
// including any state saves of its in-flight refresh pass or heartbeat-driven
// recompute. Recomputes requested after Close is called are dropped.
// The per-message goroutines spawned by ProcessReceivedMessage are not tracked
// and may still write admitted keys to the storer after Close returns.
// Safe to call multiple times; subsequent calls are no-ops.
func (m *Monitor) Close() error {
	m.closeOnce.Do(func() {
		close(m.stopCh)
	})
	m.wg.Wait()
	return nil
}

func (m *Monitor) refreshHeartbeatMessageInfo() {
	m.computeAllHeartbeatMessages()
	m.computeInactiveHeartbeatMessages()
}

func (m *Monitor) addDoubleSignerPeers(hb *data.Heartbeat) {
	pubKeyStr := string(hb.Pubkey)
	tc, ok := m.doubleSignerPeers[pubKeyStr]
	if !ok {
		tc = timecache.NewTimeCache(m.maxDurationPeerUnresponsive)
		err := tc.Add(string(hb.Pid))
		if err != nil {
			log.Warn("cannot add heartbeat in cache", "peer id", hb.Pid, "error", err)
		}
		m.doubleSignerPeers[pubKeyStr] = tc
		return
	}

	tc.Sweep()
	err := tc.Add(string(hb.Pid))
	if err != nil {
		log.Warn("cannot add heartbeat in cache", "peer id", hb.Pid, "error", err)
	}
}

func (m *Monitor) getNumInstancesOfPublicKey(pubKeyStr string) uint64 {
	tc, ok := m.doubleSignerPeers[pubKeyStr]
	if !ok {
		return 0
	}

	return uint64(tc.Len()) // #nosec G115
}

// Cleanup will delete all the entries in the heartbeatMessages map
func (m *Monitor) Cleanup() {
	m.mutTransientUnknownHeartbeatPubKeys.Lock()
	_ = m.sweepTransientUnknownHeartbeatPubKeysLocked()
	m.mutTransientUnknownHeartbeatPubKeys.Unlock()

	m.mutHeartbeatMessages.Lock()
	for k, v := range m.heartbeatMessages {
		if !m.isAdmittedHeartbeatPubKey(k) {
			if !m.isTransientUnknownHeartbeatPubKeyTracked(k) {
				delete(m.heartbeatMessages, k)
				delete(m.doubleSignerPeers, k)
			}
			continue
		}

		if m.shouldSkipValidator(v) {
			delete(m.heartbeatMessages, k)
			delete(m.doubleSignerPeers, k)
		}
	}
	m.mutHeartbeatMessages.Unlock()
}
