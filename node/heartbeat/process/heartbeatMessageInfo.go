package process

import (
	"sync"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/node/heartbeat"
	"github.com/klever-io/klever-go/tools/check"
)

// heartbeatMessageInfo retain the message info received from another node (identified by a public key)
type heartbeatMessageInfo struct {
	maxDurationPeerUnresponsive time.Duration
	maxInactiveTime             time.Duration
	totalUpTime                 time.Duration
	totalDownTime               time.Duration
	timestamp                   time.Time
	lastUptimeDowntime          time.Time
	genesisTime                 time.Time
	versionNumber               string
	nodeDisplayName             string
	identity                    string
	peerType                    string
	updateMutex                 sync.RWMutex
	getTimeHandler              func() time.Time
	isActive                    bool
	nonce                       uint64
	numInstances                uint64
}

// newHeartbeatMessageInfo returns a new instance of a heartbeatMessageInfo
func newHeartbeatMessageInfo(
	maxDurationPeerUnresponsive time.Duration,
	peerType string,
	genesisTime time.Time,
	timer heartbeat.Timer,
) (*heartbeatMessageInfo, error) {

	if maxDurationPeerUnresponsive == 0 {
		return nil, heartbeat.ErrInvalidMaxDurationPeerUnresponsive
	}
	if check.IfNil(timer) {
		return nil, heartbeat.ErrNilTimer
	}

	hbmi := &heartbeatMessageInfo{
		maxDurationPeerUnresponsive: maxDurationPeerUnresponsive,
		maxInactiveTime:             time.Duration(0),
		isActive:                    false,
		timestamp:                   genesisTime,
		lastUptimeDowntime:          timer.Now(),
		totalUpTime:                 time.Duration(0),
		totalDownTime:               time.Duration(0),
		versionNumber:               "",
		nodeDisplayName:             "",
		identity:                    "",
		peerType:                    peerType,
		genesisTime:                 genesisTime,
		getTimeHandler:              timer.Now,
		nonce:                       0,
		numInstances:                0,
	}

	return hbmi, nil
}

// ComputeActive will update the isActive field
func (hbmi *heartbeatMessageInfo) ComputeActive(crtTime time.Time) {
	hbmi.updateMutex.Lock()
	defer hbmi.updateMutex.Unlock()
	validDuration := computeValidDuration(crtTime, hbmi)
	hbmi.isActive = hbmi.isActive && validDuration
	hbmi.updateTimes(crtTime)
}

func (hbmi *heartbeatMessageInfo) updateTimes(crtTime time.Time) {
	if crtTime.Sub(hbmi.genesisTime) < 0 {
		return
	}
	hbmi.updateMaxInactiveTimeDuration(crtTime)
	hbmi.updateUpAndDownTime(crtTime)
}

func computeValidDuration(crtTime time.Time, hbmi *heartbeatMessageInfo) bool {
	crtDuration := crtTime.Sub(hbmi.timestamp)
	crtDuration = maxDuration(0, crtDuration)
	validDuration := crtDuration <= hbmi.maxDurationPeerUnresponsive
	return validDuration
}

// Will update the total time a node was up and down
func (hbmi *heartbeatMessageInfo) updateUpAndDownTime(crtTime time.Time) {
	if hbmi.lastUptimeDowntime.Sub(hbmi.genesisTime) < 0 {
		hbmi.lastUptimeDowntime = hbmi.genesisTime
	}

	if crtTime.Sub(hbmi.timestamp) < 0 {
		return
	}

	lastDuration := crtTime.Sub(hbmi.lastUptimeDowntime)
	lastDuration = maxDuration(0, lastDuration)

	if lastDuration == 0 {
		return
	}

	uptime, downTime := hbmi.computeUptimeDowntime(crtTime, lastDuration)

	hbmi.totalUpTime += uptime
	hbmi.totalDownTime += downTime

	hbmi.isActive = uptime == lastDuration
	hbmi.lastUptimeDowntime = crtTime
}

func (hbmi *heartbeatMessageInfo) computeUptimeDowntime(
	crtTime time.Time,
	lastDuration time.Duration,
) (time.Duration, time.Duration) {
	durationSinceLastHeartbeat := crtTime.Sub(hbmi.timestamp)
	insideActiveWindowAfterHeartheat := durationSinceLastHeartbeat <= hbmi.maxDurationPeerUnresponsive
	noHeartbeatReceived := hbmi.timestamp.Equal(hbmi.genesisTime) && !hbmi.isActive
	outSideActiveWindowAfterHeartbeat := durationSinceLastHeartbeat-lastDuration > hbmi.maxDurationPeerUnresponsive

	uptime := time.Duration(0)
	downtime := time.Duration(0)

	if noHeartbeatReceived || outSideActiveWindowAfterHeartbeat {
		downtime = lastDuration
		return uptime, downtime
	}

	if insideActiveWindowAfterHeartheat {
		uptime = lastDuration
		return uptime, downtime
	}

	downtime = durationSinceLastHeartbeat - hbmi.maxDurationPeerUnresponsive
	uptime = lastDuration - downtime

	return uptime, downtime
}

// HeartbeatReceived processes a new message arrived from a peer
func (hbmi *heartbeatMessageInfo) HeartbeatReceived(
	version string,
	nodeDisplayName string,
	identity string,
	peerType string,
	nonce uint64,
	numInstances uint64,
) {
	hbmi.updateMutex.Lock()
	defer hbmi.updateMutex.Unlock()
	crtTime := hbmi.getTimeHandler()

	hbmi.versionNumber = version
	hbmi.nodeDisplayName = nodeDisplayName
	hbmi.identity = identity
	hbmi.peerType = peerType

	hbmi.updateTimes(crtTime)
	hbmi.timestamp = crtTime
	hbmi.isActive = true
	hbmi.nonce = nonce
	hbmi.numInstances = numInstances
}

// UpdatePeerType - updates peerType only for a heartbeat message info
func (hbmi *heartbeatMessageInfo) UpdatePeerType(
	peerType string,
) {
	hbmi.updateMutex.Lock()
	defer hbmi.updateMutex.Unlock()

	hbmi.peerType = peerType
}

func (hbmi *heartbeatMessageInfo) updateMaxInactiveTimeDuration(currentTime time.Time) {
	crtDuration := currentTime.Sub(hbmi.timestamp)
	crtDuration = maxDuration(0, crtDuration)

	greaterDurationThanMax := hbmi.maxInactiveTime < crtDuration
	currentTimeAfterGenesis := hbmi.genesisTime.Sub(currentTime) < 0

	if greaterDurationThanMax && currentTimeAfterGenesis {
		hbmi.maxInactiveTime = crtDuration
	}
}

func maxDuration(first, second time.Duration) time.Duration {
	if first > second {
		return first
	}

	return second
}

// GetIsActive will return true if the peer is set as active
func (hbmi *heartbeatMessageInfo) GetIsActive() bool {
	hbmi.updateMutex.Lock()
	defer hbmi.updateMutex.Unlock()
	isActive := hbmi.isActive
	return isActive
}

// The peer-type predicates below form three nested tiers. Every consumer that
// classifies a peer must pick its tier from here instead of comparing peer
// types itself, so the classifications cannot diverge:
//   - consensus-capable (elected, eligible): klv_live_consensus_validator_nodes
//   - working validators (+ waiting): klv_live_validator_nodes
//   - registered validators (+ jailed): Cleanup shielding, klv_node_type self-reporting

// isConsensusCapablePeerType reports whether the peer type can participate in
// the consensus rotation: elected and eligible.
func isConsensusCapablePeerType(peerType string) bool {
	return peerType == string(core.ElectedList) ||
		peerType == string(core.EligibleList)
}

// isValidatorPeerType reports whether the peer type belongs to the working
// validator set: waiting plus the consensus-capable types. Punished (jailed)
// validators are deliberately not part of this tier, so they do not count
// toward klv_live_validator_nodes.
func isValidatorPeerType(peerType string) bool {
	return isConsensusCapablePeerType(peerType) ||
		peerType == string(core.WaitingList)
}

// isRegisteredValidatorPeerType reports whether the peer type belongs to a
// registered validator at all, including punished (jailed) ones. A jailed
// validator must stay visible in the heartbeat status and keep self-reporting
// as a validator node. core.LeavingList is deliberately absent: the peer-type
// cache reports leaving-list members as jailed and never emits "leaving".
func isRegisteredValidatorPeerType(peerType string) bool {
	return isValidatorPeerType(peerType) ||
		peerType == string(core.JailedList)
}

// GetIsValidator will return true is the peer is a validator
func (hbmi *heartbeatMessageInfo) GetIsValidator() bool {
	hbmi.updateMutex.RLock()
	defer hbmi.updateMutex.RUnlock()
	return isValidatorPeerType(hbmi.peerType)
}

// GetIsConsensusCapable will return true if the peer can participate in consensus
func (hbmi *heartbeatMessageInfo) GetIsConsensusCapable() bool {
	hbmi.updateMutex.RLock()
	defer hbmi.updateMutex.RUnlock()
	return isConsensusCapablePeerType(hbmi.peerType)
}
