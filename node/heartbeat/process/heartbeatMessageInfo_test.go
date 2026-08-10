package process_test

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/node/heartbeat"
	"github.com/klever-io/klever-go/node/heartbeat/mock"
	"github.com/klever-io/klever-go/node/heartbeat/process"
	"github.com/stretchr/testify/assert"
)

const dummyPeerType = "dummy peer type"
const dummyIdentity = "dummy identity"
const dummyNodeDisplayName = "dummy node display name"

//------- newHeartbeatMessageInfo

func TestNewHeartbeatMessageInfo_InvalidDurationShouldErr(t *testing.T) {
	t.Parallel()

	hbmi, err := process.NewHeartbeatMessageInfo(
		0,
		dummyPeerType,
		time.Time{},
		mock.NewTimerMock(),
	)

	assert.Nil(t, hbmi)
	assert.Equal(t, heartbeat.ErrInvalidMaxDurationPeerUnresponsive, err)
}

func TestNewHeartbeatMessageInfo_NilGetTimeHandlerShouldErr(t *testing.T) {
	t.Parallel()

	hbmi, err := process.NewHeartbeatMessageInfo(
		1,
		dummyPeerType,
		time.Time{},
		nil,
	)

	assert.Nil(t, hbmi)
	assert.Equal(t, heartbeat.ErrNilTimer, err)
}

func TestNewHeartbeatMessageInfo_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	hbmi, err := process.NewHeartbeatMessageInfo(
		1,
		dummyPeerType,
		time.Time{},
		mock.NewTimerMock(),
	)

	assert.NotNil(t, hbmi)
	assert.Nil(t, err)
}

//------- HeartbeatReceived

func TestHeartbeatMessageInfo_HeartbeatReceivedShouldUpdate(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := mockTimer.Now()

	hbmi, _ := process.NewHeartbeatMessageInfo(
		10*time.Second,
		dummyPeerType,
		genesisTime,
		mockTimer,
	)

	assert.Equal(t, genesisTime, hbmi.GetTimestamp())

	mockTimer.IncrementSeconds(1)

	expectedTime := time.Unix(1, 0)
	hbmi.HeartbeatReceived("v0.1", dummyNodeDisplayName, dummyIdentity, dummyPeerType, 0, 1)
	assert.Equal(t, expectedTime, hbmi.GetTimestamp())

	mockTimer.IncrementSeconds(1)
	expectedTime = time.Unix(2, 0)
	hbmi.HeartbeatReceived("v0.1", dummyNodeDisplayName, dummyIdentity, dummyPeerType, 0, 1)
	assert.Equal(t, expectedTime, hbmi.GetTimestamp())
}

func TestHeartbeatMessageInfo_HeartbeatUpdateFieldsShouldWork(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := mockTimer.Now()
	hbmi, _ := process.NewHeartbeatMessageInfo(
		100*time.Second,
		dummyPeerType,
		genesisTime,
		mockTimer,
	)

	assert.Equal(t, genesisTime, hbmi.GetTimestamp())

	mockTimer.IncrementSeconds(1)

	expectedTime := time.Unix(1, 0)
	expectedUptime := time.Duration(0)
	expectedDownTime := 1 * time.Second
	nonce := uint64(4455)
	hbmi.HeartbeatReceived("v0.1", dummyNodeDisplayName, dummyIdentity, dummyPeerType, nonce, 1)
	assert.Equal(t, expectedTime, hbmi.GetTimestamp())
	assert.Equal(t, true, hbmi.GetIsActive())
	assert.Equal(t, expectedUptime, hbmi.GetTotalUpTime())
	assert.Equal(t, expectedDownTime, hbmi.GetTotalDownTime())
	assert.Equal(t, nonce, hbmi.GetNonce())
}

func TestHeartbeatMessageInfo_HeartbeatShouldUpdateUpDownTime(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := mockTimer.Now()
	hbmi, _ := process.NewHeartbeatMessageInfo(
		100*time.Second,
		dummyPeerType,
		genesisTime,
		mockTimer,
	)

	assert.Equal(t, genesisTime, hbmi.GetTimestamp())

	// send heartbeat twice in order to calculate the duration between thm
	mockTimer.IncrementSeconds(1)
	hbmi.HeartbeatReceived("v0.1", dummyNodeDisplayName, dummyIdentity, dummyPeerType, 0, 1)
	mockTimer.IncrementSeconds(1)
	hbmi.HeartbeatReceived("v0.1", dummyNodeDisplayName, dummyIdentity, dummyPeerType, 0, 1)

	expectedDownDuration := 1 * time.Second
	expectedUpDuration := 1 * time.Second
	assert.Equal(t, expectedUpDuration, hbmi.GetTotalUpTime())
	assert.Equal(t, expectedDownDuration, hbmi.GetTotalDownTime())
	expectedTime := time.Unix(2, 0)
	assert.Equal(t, expectedTime, hbmi.GetTimestamp())
}

func TestHeartbeatMessageInfo_HeartbeatLongerDurationThanMaxShouldUpdateDownTime(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := mockTimer.Now()
	maxUnresponsiveTime := 500 * time.Millisecond
	hbmi, _ := process.NewHeartbeatMessageInfo(
		maxUnresponsiveTime,
		"eligible",
		genesisTime,
		mockTimer,
	)

	assert.Equal(t, genesisTime, hbmi.GetTimestamp())

	// send heartbeat twice in order to calculate the duration between thm
	mockTimer.IncrementSeconds(1)
	hbmi.HeartbeatReceived("v0.1", dummyNodeDisplayName, dummyIdentity, dummyPeerType, 0, 1)
	mockTimer.IncrementSeconds(1)
	hbmi.HeartbeatReceived("v0.1", dummyNodeDisplayName, dummyIdentity, dummyPeerType, 0, 1)

	expectedDownDuration := 1500 * time.Millisecond
	expectedUpDuration := maxUnresponsiveTime
	assert.Equal(t, expectedDownDuration, hbmi.GetTotalDownTime())
	assert.Equal(t, expectedUpDuration, hbmi.GetTotalUpTime())
	expectedTime := time.Unix(2, 0)
	assert.Equal(t, expectedTime, hbmi.GetTimestamp())
}

func TestHeartbeatMessageInfo_HeartbeatBeforeGenesisShouldNotUpdateUpDownTime(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := time.Unix(5, 0)
	hbmi, _ := process.NewHeartbeatMessageInfo(
		100*time.Second,
		dummyPeerType,
		genesisTime,
		mockTimer,
	)

	assert.Equal(t, genesisTime, hbmi.GetTimestamp())

	// send heartbeat twice in order to calculate the duration between thm
	mockTimer.IncrementSeconds(1)
	hbmi.HeartbeatReceived("v0.1", dummyNodeDisplayName, dummyIdentity, dummyPeerType, 0, 1)
	mockTimer.IncrementSeconds(1)
	hbmi.HeartbeatReceived("v0.1", dummyNodeDisplayName, dummyIdentity, dummyPeerType, 0, 1)

	expectedDuration := time.Duration(0)
	assert.Equal(t, expectedDuration, hbmi.GetTotalDownTime())
	assert.Equal(t, expectedDuration, hbmi.GetTotalUpTime())
	expectedTime := time.Unix(2, 0)
	assert.Equal(t, expectedTime, hbmi.GetTimestamp())
}

func TestHeartbeatMessageInfo_HeartbeatEqualGenesisShouldHaveUpDownTimeZero(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := time.Unix(1, 0)
	hbmi, _ := process.NewHeartbeatMessageInfo(
		100*time.Second,
		dummyPeerType,
		genesisTime,
		mockTimer,
	)

	assert.Equal(t, genesisTime, hbmi.GetTimestamp())
	mockTimer.IncrementSeconds(1)
	hbmi.HeartbeatReceived("v0.1", dummyNodeDisplayName, dummyIdentity, dummyPeerType, 0, 1)

	expectedDuration := time.Duration(0)
	assert.Equal(t, expectedDuration, hbmi.GetTotalUpTime())
	assert.Equal(t, expectedDuration, hbmi.GetTotalDownTime())
	expectedTime := time.Unix(1, 0)
	assert.Equal(t, expectedTime, hbmi.GetTimestamp())
}

func TestHeartbeatMessageInfo_GetIsValidator_NotValidatorShouldReturnFalse(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := time.Unix(1, 0)
	hbmi, _ := process.NewHeartbeatMessageInfo(
		100*time.Second,
		dummyPeerType,
		genesisTime,
		mockTimer,
	)

	assert.False(t, hbmi.GetIsValidator())
}

func TestHeartbeatMessageInfo_GetIsValidator_PeerTypeElectedShouldReturnTrue(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := time.Unix(1, 0)
	hbmi, _ := process.NewHeartbeatMessageInfo(
		100*time.Second,
		string(core.ElectedList),
		genesisTime,
		mockTimer,
	)

	assert.True(t, hbmi.GetIsValidator())
}

func TestHeartbeatMessageInfo_GetIsValidator_PeerTypeEligibleShouldReturnTrue(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := time.Unix(1, 0)
	hbmi, _ := process.NewHeartbeatMessageInfo(
		100*time.Second,
		string(core.EligibleList),
		genesisTime,
		mockTimer,
	)

	assert.True(t, hbmi.GetIsValidator())
}

func TestHeartbeatMessageInfo_GetIsValidator_PeerTypeWaitingShouldReturnTrue(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := time.Unix(1, 0)
	hbmi, _ := process.NewHeartbeatMessageInfo(
		100*time.Second,
		string(core.WaitingList),
		genesisTime,
		mockTimer,
	)

	assert.True(t, hbmi.GetIsValidator())
}

func TestHeartbeatMessageInfo_GetIsValidator_PeerTypeObserverShouldReturnFalse(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := time.Unix(1, 0)
	hbmi, _ := process.NewHeartbeatMessageInfo(
		100*time.Second,
		string(core.ObserverList),
		genesisTime,
		mockTimer,
	)

	assert.False(t, hbmi.GetIsValidator())
}

func TestHeartbeatMessageInfo_GetIsConsensusCapable_PeerTypeEligibleShouldReturnTrue(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := time.Unix(1, 0)
	hbmi, _ := process.NewHeartbeatMessageInfo(
		100*time.Second,
		string(core.EligibleList),
		genesisTime,
		mockTimer,
	)

	assert.True(t, hbmi.GetIsConsensusCapable())
}

func TestHeartbeatMessageInfo_GetIsConsensusCapable_PeerTypeWaitingShouldReturnFalse(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := time.Unix(1, 0)
	hbmi, _ := process.NewHeartbeatMessageInfo(
		100*time.Second,
		string(core.WaitingList),
		genesisTime,
		mockTimer,
	)

	assert.False(t, hbmi.GetIsConsensusCapable())
}

func TestHeartbeatMessageInfo_GetIsValidator_PeerTypeJailedShouldReturnFalse(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := time.Unix(1, 0)
	hbmi, _ := process.NewHeartbeatMessageInfo(
		100*time.Second,
		string(core.JailedList),
		genesisTime,
		mockTimer,
	)

	// jailed is a registered validator but not part of the working set, so it
	// counts toward neither klv_live_validator_nodes nor the consensus gauge
	assert.False(t, hbmi.GetIsValidator())
	assert.False(t, hbmi.GetIsConsensusCapable())
}

func TestIsRegisteredValidatorPeerType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		peerType string
		expected bool
	}{
		{peerType: string(core.ElectedList), expected: true},
		{peerType: string(core.EligibleList), expected: true},
		{peerType: string(core.WaitingList), expected: true},
		{peerType: string(core.JailedList), expected: true},
		{peerType: string(core.ObserverList), expected: false},
		{peerType: string(core.InactiveList), expected: false},
		// the peer-type cache reports leaving-list members as jailed and never
		// emits "leaving", so the raw string is deliberately not registered
		{peerType: string(core.LeavingList), expected: false},
	}

	for _, tc := range testCases {
		t.Run(tc.peerType, func(t *testing.T) {
			assert.Equal(t, tc.expected, process.IsRegisteredValidatorPeerType(tc.peerType))
		})
	}
}

//------- UpdatePeerType

func TestHeartbeatMessageInfo_Update(t *testing.T) {
	t.Parallel()

	mockTimer := mock.NewTimerMock()
	genesisTime := mockTimer.Now()

	hbmi, _ := process.NewHeartbeatMessageInfo(
		10*time.Second,
		dummyPeerType,
		genesisTime,
		mockTimer,
	)

	peerType := dummyPeerType

	hbmi.HeartbeatReceived("v0.1", dummyNodeDisplayName, dummyIdentity, peerType, 0, 1)
	assert.Equal(t, peerType, hbmi.GetPeerType())

	peerType = "new peer type"
	hbmi.UpdatePeerType(peerType)
	assert.Equal(t, peerType, hbmi.GetPeerType())
}
