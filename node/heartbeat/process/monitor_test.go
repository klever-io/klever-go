package process_test

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/node/heartbeat"
	"github.com/klever-io/klever-go/node/heartbeat/data"
	"github.com/klever-io/klever-go/node/heartbeat/mock"
	"github.com/klever-io/klever-go/node/heartbeat/process"
	"github.com/klever-io/klever-go/node/heartbeat/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fromConnectedPeerId = core.PeerID("from connected peer Id")

func createMockP2PAntifloodHandler() *mock.P2PAntifloodHandlerStub {
	return &mock.P2PAntifloodHandlerStub{
		CanProcessMessageCalled: func(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
			return nil
		},
		CanProcessMessagesOnTopicCalled: func(peer core.PeerID, topic string, numMessages uint32, totalSize uint64, sequence []byte) error {
			return nil
		},
	}
}

func createMockStorer() heartbeat.HeartbeatStorageHandler {
	return &mock.HeartbeatStorerStub{
		UpdateGenesisTimeCalled: func(genesisTime time.Time) error {
			return nil
		},
		LoadHeartBeatDTOCalled: func(pubKey string) (*data.HeartbeatDTO, error) {
			return nil, errors.New("not found")
		},
		LoadKeysCalled: func() ([][]byte, error) {
			return nil, nil
		},
		SavePubkeyDataCalled: func(pubkey []byte, heartbeat *data.HeartbeatDTO) error {
			return nil
		},
		SaveKeysCalled: func(peersSlice [][]byte) error {
			return nil
		},
	}
}

func createMockArgHeartbeatMonitor() process.ArgHeartbeatMonitor {
	return process.ArgHeartbeatMonitor{
		Marshalizer:                 &mock.MarshalizerStub{},
		MaxDurationPeerUnresponsive: 1,
		PubKeysList:                 []string{""},
		GenesisTime:                 time.Now(),
		MessageHandler:              &mock.MessageHandlerStub{},
		Storer:                      createMockStorer(),
		PeerTypeProvider: &mock.PeerTypeProviderStub{
			ComputeForPubKeyCalled: func(pubKey []byte) (core.PeerType, uint32, error) {
				if string(pubKey) == "pk0" {
					return "", 0, nil
				}

				return "", 1, nil
			},
		},
		Timer:                              mock.NewTimerMock(),
		AntifloodHandler:                   createMockP2PAntifloodHandler(),
		ValidatorPubkeyConverter:           mock.NewPubkeyConverterMock(96),
		HeartbeatRefreshIntervalInSec:      1,
		HideInactiveValidatorIntervalInSec: 600,
	}
}

//------- NewMonitor

func TestNewMonitor_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.Marshalizer = nil
	mon, err := process.NewMonitor(arg)

	assert.Nil(t, mon)
	assert.Equal(t, heartbeat.ErrNilMarshalizer, err)
}

func TestNewMonitor_NilPublicKeyListShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = nil
	mon, err := process.NewMonitor(arg)

	assert.Nil(t, mon)
	assert.Equal(t, heartbeat.ErrNilPublicKeysMap, err)
}

func TestNewMonitor_NilMessageHandlerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.MessageHandler = nil
	mon, err := process.NewMonitor(arg)

	assert.Nil(t, mon)
	assert.Equal(t, heartbeat.ErrNilMessageHandler, err)
}

func TestNewMonitor_NilHeartbeatStorerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.Storer = nil
	mon, err := process.NewMonitor(arg)

	assert.Nil(t, mon)
	assert.Equal(t, heartbeat.ErrNilHeartbeatStorer, err)
}

func TestNewMonitor_NilPeerTypeProviderShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.PeerTypeProvider = nil
	mon, err := process.NewMonitor(arg)

	assert.Nil(t, mon)
	assert.Equal(t, heartbeat.ErrNilPeerTypeProvider, err)
}

func TestNewMonitor_NilTimeHandlerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.Timer = nil
	mon, err := process.NewMonitor(arg)

	assert.Nil(t, mon)
	assert.Equal(t, heartbeat.ErrNilTimer, err)
}

func TestNewMonitor_NilAntifloodHandlerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.AntifloodHandler = nil
	mon, err := process.NewMonitor(arg)

	assert.Nil(t, mon)
	assert.Equal(t, heartbeat.ErrNilAntifloodHandler, err)
}

func TestNewMonitor_NilValidatorPubkeyConverterShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.ValidatorPubkeyConverter = nil
	mon, err := process.NewMonitor(arg)

	assert.Nil(t, mon)
	assert.True(t, errors.Is(err, heartbeat.ErrNilPubkeyConverter))
}

func TestNewMonitor_ZeroHbmiRefreshIntervalShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.HeartbeatRefreshIntervalInSec = 0
	mon, err := process.NewMonitor(arg)

	assert.Nil(t, mon)
	assert.True(t, errors.Is(err, heartbeat.ErrZeroHeartbeatRefreshIntervalInSec))
}

func TestNewMonitor_ZeroHideInactiveVlidatorIntervalInHoursShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.HideInactiveValidatorIntervalInSec = 0
	mon, err := process.NewMonitor(arg)

	assert.Nil(t, mon)
	assert.True(t, errors.Is(err, heartbeat.ErrZeroHideInactiveValidatorIntervalInSec))
}

func TestNewMonitor_OkValsShouldCreatePubkeyMap(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = []string{"pk1", "pk2"}
	mon, err := process.NewMonitor(arg)

	assert.Nil(t, err)
	defer mon.Close()
	assert.False(t, check.IfNil(mon))

	hbStatus := mon.GetHeartbeats()
	assert.Equal(t, 2, len(hbStatus))
}

//------- ProcessReceivedMessage

func TestMonitor_ProcessReceivedMessageShouldWork(t *testing.T) {
	t.Parallel()

	pubKey := "pk1"

	arg := createMockArgHeartbeatMonitor()
	arg.Marshalizer = &mock.MarshalizerStub{
		UnmarshalHandler: func(obj interface{}, buff []byte) error {
			(obj.(*data.Heartbeat)).Pubkey = []byte(pubKey)
			return nil
		},
	}
	arg.MaxDurationPeerUnresponsive = time.Second * 1000
	arg.PubKeysList = []string{pubKey}
	arg.MessageHandler = &mock.MessageHandlerStub{
		CreateHeartbeatFromP2PMessageCalled: func(message p2p.MessageP2P) (*data.Heartbeat, error) {
			var rcvHb data.Heartbeat
			_ = json.Unmarshal(message.Data(), &rcvHb)
			return &rcvHb, nil
		},
	}
	mon, _ := process.NewMonitor(arg)

	hb := data.Heartbeat{
		Pubkey: []byte(pubKey),
	}
	hbBytes, _ := json.Marshal(&hb)
	err := mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: hbBytes}, fromConnectedPeerId)
	assert.Nil(t, err)

	//a delay is mandatory for the go routine to finish its job
	time.Sleep(time.Second)

	hbStatus := mon.GetHeartbeats()
	assert.Equal(t, 1, len(hbStatus))
	assert.Equal(t, hex.EncodeToString([]byte(pubKey)), hbStatus[0].PublicKey)
}

func TestMonitor_ProcessReceivedMessageWithNewPublicKey(t *testing.T) {
	t.Parallel()

	pubKey := "pk1"

	arg := createMockArgHeartbeatMonitor()
	arg.Marshalizer = &mock.MarshalizerStub{
		UnmarshalHandler: func(obj interface{}, buff []byte) error {
			(obj.(*data.Heartbeat)).Pubkey = []byte(pubKey)
			return nil
		},
	}
	arg.MaxDurationPeerUnresponsive = time.Second * 1000
	arg.PubKeysList = []string{"pk2"}
	arg.MessageHandler = &mock.MessageHandlerStub{
		CreateHeartbeatFromP2PMessageCalled: func(message p2p.MessageP2P) (*data.Heartbeat, error) {
			var rcvHb data.Heartbeat
			_ = json.Unmarshal(message.Data(), &rcvHb)
			return &rcvHb, nil
		},
	}
	mon, _ := process.NewMonitor(arg)

	hb := data.Heartbeat{
		Pubkey: []byte(pubKey),
	}
	hbBytes, _ := json.Marshal(&hb)
	err := mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: hbBytes}, fromConnectedPeerId)
	assert.Nil(t, err)

	//a delay is mandatory for the go routine to finish its job
	time.Sleep(time.Second)

	//there should be 2 heartbeats, because a new one should have been added with pk2
	hbStatus := mon.GetHeartbeats()
	assert.Equal(t, 2, len(hbStatus))
	assert.Equal(t, hex.EncodeToString([]byte(pubKey)), hbStatus[0].PublicKey)
}

func TestMonitor_ProcessReceivedMessageWithNewShardID(t *testing.T) {
	t.Parallel()

	pubKey := []byte("pk1")

	arg := createMockArgHeartbeatMonitor()
	arg.Marshalizer = &mock.MarshalizerStub{
		UnmarshalHandler: func(obj interface{}, buff []byte) error {
			var rcvdHb data.Heartbeat
			_ = json.Unmarshal(buff, &rcvdHb)
			(obj.(*data.Heartbeat)).Pubkey = rcvdHb.Pubkey
			return nil
		},
	}
	arg.MaxDurationPeerUnresponsive = time.Second * 1000
	arg.PubKeysList = []string{"pk1"}
	arg.MessageHandler = &mock.MessageHandlerStub{
		CreateHeartbeatFromP2PMessageCalled: func(message p2p.MessageP2P) (*data.Heartbeat, error) {
			var rcvHb data.Heartbeat
			_ = json.Unmarshal(message.Data(), &rcvHb)
			return &rcvHb, nil
		},
	}

	mon, _ := process.NewMonitor(arg)

	// First send from pk1 from shard 0
	hb := &data.Heartbeat{
		Pubkey: pubKey,
	}

	buffToSend, err := json.Marshal(hb)
	assert.Nil(t, err)

	err = mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: buffToSend}, fromConnectedPeerId)
	assert.Nil(t, err)

	//a delay is mandatory for the go routine to finish its job
	time.Sleep(time.Second)

	// now we send a new heartbeat which will contain a new shard id
	hb = &data.Heartbeat{
		Pubkey: pubKey,
	}

	buffToSend, err = json.Marshal(hb)
	assert.Nil(t, err)

	err = mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: buffToSend}, fromConnectedPeerId)
	assert.Nil(t, err)

	time.Sleep(1 * time.Second)

	hbStatus := mon.GetHeartbeats()

	assert.Equal(t, 1, len(hbStatus))
}

func TestMonitor_ProcessReceivedMessageShouldSetPeerInactive(t *testing.T) {
	t.Parallel()

	th := mock.NewTimerMock()
	pubKey1 := "pk1-should-stay-online"
	pubKey2 := "pk2-should-go-offline"
	storer, _ := storage.NewHeartbeatDbStorer(mock.NewStorerMock(), &mock.MarshalizerMock{})
	arg := createMockArgHeartbeatMonitor()
	arg.Marshalizer = &mock.MarshalizerStub{
		UnmarshalHandler: func(obj interface{}, buff []byte) error {
			var rcvdHb data.Heartbeat
			_ = json.Unmarshal(buff, &rcvdHb)
			(obj.(*data.Heartbeat)).Pubkey = rcvdHb.Pubkey
			return nil
		},
	}
	arg.MessageHandler = &mock.MessageHandlerStub{
		CreateHeartbeatFromP2PMessageCalled: func(message p2p.MessageP2P) (*data.Heartbeat, error) {
			var rcvHb data.Heartbeat
			_ = json.Unmarshal(message.Data(), &rcvHb)
			return &rcvHb, nil
		},
	}
	arg.MaxDurationPeerUnresponsive = time.Second * 5
	arg.PubKeysList = []string{pubKey1, pubKey2}
	arg.Storer = storer
	arg.Timer = th
	arg.HideInactiveValidatorIntervalInSec = 600
	mon, _ := process.NewMonitor(arg)

	// First send from pk1
	err := sendHbMessageFromPubKey(pubKey1, mon)
	assert.Nil(t, err)

	// Send from pk2
	err = sendHbMessageFromPubKey(pubKey2, mon)
	assert.Nil(t, err)

	// set pk2 to inactive as max inactive time is lower
	time.Sleep(10 * time.Millisecond)
	th.IncrementSeconds(6)

	// Check that both are added
	mon.RefreshHeartbeatMessageInfo()
	hbStatus := mon.GetHeartbeats()
	assert.Equal(t, 2, len(hbStatus))
	//assert.False(t, hbStatus[1].IsActive)

	// Now send a message from pk1 in order to see that pk2 is not active anymore
	err = sendHbMessageFromPubKey(pubKey1, mon)
	time.Sleep(5 * time.Millisecond)
	assert.Nil(t, err)

	th.IncrementSeconds(4)
	mon.RefreshHeartbeatMessageInfo()
	hbStatus = mon.GetHeartbeats()

	// check if pk1 is still on
	assert.True(t, hbStatus[0].IsActive)
	// check if pk2 was set to offline by pk1
	assert.False(t, hbStatus[1].IsActive)
}

func TestMonitor_RemoveInactiveValidatorsIfIntervalExceeded(t *testing.T) {
	t.Parallel()
	pubKey1 := "pk1-elected"
	pubKey2 := "pk2-eligible"
	pubKey3 := "pk3-observer"
	pubKey4 := "pk4-inactive"
	pubKey5 := "pk5-waiting"

	storer, _ := storage.NewHeartbeatDbStorer(mock.NewStorerMock(), &mock.MarshalizerMock{})

	timer := mock.NewTimerMock()
	genesisTime := timer.Now()

	arg := process.ArgHeartbeatMonitor{
		Marshalizer:                 &mock.MarshalizerMock{},
		MaxDurationPeerUnresponsive: unresponsiveDuration,
		PubKeysList: []string{
			pkValidator,
			pubKey1,
		},
		GenesisTime:    genesisTime,
		MessageHandler: &mock.MessageHandlerStub{},
		Storer:         storer,
		PeerTypeProvider: &mock.PeerTypeProviderStub{
			ComputeForPubKeyCalled: func(pubKey []byte) (core.PeerType, uint32, error) {
				switch string(pubKey) {
				case pubKey1:
					return core.ElectedList, 0, nil
				case pubKey2:
					return core.EligibleList, 0, nil
				case pubKey3:
					return core.ObserverList, 0, nil
				case pubKey4:
					return core.InactiveList, 0, nil
				case pubKey5:
					return core.WaitingList, 0, nil
				}
				return core.ObserverList, 0, nil
			},
		},
		Timer:                              timer,
		AntifloodHandler:                   createMockP2PAntifloodHandler(),
		ValidatorPubkeyConverter:           mock.NewPubkeyConverterMock(32),
		HeartbeatRefreshIntervalInSec:      1,
		HideInactiveValidatorIntervalInSec: 600,
	}
	mon, _ := process.NewMonitor(arg)
	t.Cleanup(func() {
		require.NoError(t, mon.Close())
	})
	mon.SendHeartbeatMessage(&data.Heartbeat{Pubkey: []byte(pkValidator)})
	mon.SendHeartbeatMessage(&data.Heartbeat{Pubkey: []byte(pubKey1)})
	mon.SendHeartbeatMessage(&data.Heartbeat{Pubkey: []byte(pubKey2)})
	mon.SendHeartbeatMessage(&data.Heartbeat{Pubkey: []byte(pubKey3)})
	mon.SendHeartbeatMessage(&data.Heartbeat{Pubkey: []byte(pubKey4)})
	mon.SendHeartbeatMessage(&data.Heartbeat{Pubkey: []byte(pubKey5)})

	// Check that all are added
	mon.RefreshHeartbeatMessageInfo()
	hbStatus := mon.GetHeartbeats()
	assert.Equal(t, 6, len(hbStatus))

	timer.IncrementSeconds(int(arg.HideInactiveValidatorIntervalInSec) - 20)
	mon.RefreshHeartbeatMessageInfo()
	hbStatus = mon.GetHeartbeats()
	assert.Equal(t, 6, len(hbStatus))

	// increase to over HideInactiveValidatorIntervalInSec ~ 10 min
	timer.IncrementSeconds(int(arg.HideInactiveValidatorIntervalInSec) + 10)
	mon.RefreshHeartbeatMessageInfo()
	hbStatus = mon.GetHeartbeats()
	// check if pk1, pk2 and pk5 are still on
	assert.Equal(t, 3, len(hbStatus))
}

func TestMonitor_ProcessReceivedMessageImpersonatedMessageShouldErr(t *testing.T) {
	t.Parallel()

	pubKey := "pk1"
	originator := core.PeerID("message originator")

	arg := createMockArgHeartbeatMonitor()
	arg.Marshalizer = &mock.MarshalizerStub{
		UnmarshalHandler: func(obj interface{}, buff []byte) error {
			(obj.(*data.Heartbeat)).Pubkey = []byte(pubKey)
			return nil
		},
	}
	arg.MaxDurationPeerUnresponsive = time.Second * 1000
	arg.PubKeysList = []string{"pk2"}
	arg.MessageHandler = &mock.MessageHandlerStub{
		CreateHeartbeatFromP2PMessageCalled: func(message p2p.MessageP2P) (*data.Heartbeat, error) {
			var rcvHb data.Heartbeat
			_ = json.Unmarshal(message.Data(), &rcvHb)
			return &rcvHb, nil
		},
	}
	originatorWasBlacklisted := false
	connectedPeerWasBlacklisted := false
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		BlacklistPeerCalled: func(pid core.PeerID, reason string, duration time.Duration) {
			if pid == originator {
				originatorWasBlacklisted = true
			}
			if pid == fromConnectedPeerId {
				connectedPeerWasBlacklisted = true
			}
		},
	}
	mon, _ := process.NewMonitor(arg)

	hb := data.Heartbeat{
		Pubkey: []byte(pubKey),
	}
	hbBytes, _ := json.Marshal(&hb)
	message := &mock.P2PMessageStub{
		DataField: hbBytes,
		PeerField: originator,
	}

	err := mon.ProcessReceivedMessage(message, fromConnectedPeerId)
	assert.True(t, errors.Is(err, heartbeat.ErrHeartbeatPidMismatch))
	assert.True(t, originatorWasBlacklisted)
	assert.True(t, connectedPeerWasBlacklisted)
}

func sendHbMessageFromPubKey(pubKey string, mon *process.Monitor) error {
	hb := &data.Heartbeat{
		Pubkey: []byte(pubKey),
	}
	buffToSend, _ := json.Marshal(hb)
	err := mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: buffToSend}, fromConnectedPeerId)
	return err
}

func TestMonitor_AddAndGetDoubleSignerPeersShouldWork(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.MaxDurationPeerUnresponsive = time.Millisecond * 100
	mon, _ := process.NewMonitor(arg)

	assert.Equal(t, uint64(0), mon.GetNumInstancesOfPublicKey(string("pk0")))

	mon.AddDoubleSignerPeers(&data.Heartbeat{Pubkey: []byte("pk1"), Pid: []byte("pid1")})
	assert.Equal(t, uint64(1), mon.GetNumInstancesOfPublicKey(string("pk1")))

	mon.AddDoubleSignerPeers(&data.Heartbeat{Pubkey: []byte("pk2"), Pid: []byte("pid2.1")})
	mon.AddDoubleSignerPeers(&data.Heartbeat{Pubkey: []byte("pk2"), Pid: []byte("pid2.2")})
	assert.Equal(t, uint64(2), mon.GetNumInstancesOfPublicKey(string("pk2")))

	mon.AddDoubleSignerPeers(&data.Heartbeat{Pubkey: []byte("pk3"), Pid: []byte("pid3.1")})
	mon.AddDoubleSignerPeers(&data.Heartbeat{Pubkey: []byte("pk3"), Pid: []byte("pid3.2")})
	mon.AddDoubleSignerPeers(&data.Heartbeat{Pubkey: []byte("pk3"), Pid: []byte("pid3.3")})
	assert.Equal(t, uint64(3), mon.GetNumInstancesOfPublicKey(string("pk3")))

	time.Sleep(time.Millisecond * 100)

	mon.AddDoubleSignerPeers(&data.Heartbeat{Pubkey: []byte("pk3"), Pid: []byte("pid3.4")})
	assert.Equal(t, uint64(1), mon.GetNumInstancesOfPublicKey(string("pk3")))
}

func TestMonitor_WaitingNodeSurvivesRefreshAndCleanupRounds(t *testing.T) {
	t.Parallel()

	pkWaiting := "pk-waiting"

	timer := mock.NewTimerMock()
	genesisTime := timer.Now()

	arg := createMockArgHeartbeatMonitor()
	arg.GenesisTime = genesisTime
	arg.Timer = timer
	arg.PubKeysList = []string{}
	arg.PeerTypeProvider = &mock.PeerTypeProviderStub{
		ComputeForPubKeyCalled: func(pubKey []byte) (core.PeerType, uint32, error) {
			if string(pubKey) == pkWaiting {
				return core.WaitingList, 0, nil
			}
			return core.ObserverList, 0, nil
		},
		GetAllPeerTypeInfosCalled: func() []*state.PeerTypeInfo {
			return []*state.PeerTypeInfo{
				{PublicKey: pkWaiting, PeerType: string(core.WaitingList)},
			}
		},
	}
	mon, err := process.NewMonitor(arg)
	require.Nil(t, err)
	t.Cleanup(func() {
		require.NoError(t, mon.Close())
	})

	// move far past the hide interval so an unshielded inactive entry would be
	// deleted by Cleanup and recreated by the next refresh (the churn cycle)
	timer.SetSeconds(int(arg.HideInactiveValidatorIntervalInSec) + 100)

	mon.RefreshHeartbeatMessageInfo()
	assert.Equal(t, 1, mon.GetNumHearbeatMessages())

	for i := 0; i < 3; i++ {
		mon.Cleanup()
		assert.Equal(t, 1, mon.GetNumHearbeatMessages())
		mon.RefreshHeartbeatMessageInfo()
		assert.Equal(t, 1, mon.GetNumHearbeatMessages())
	}
}

func TestMonitor_ActiveWaitingNodeCountsAsLiveValidator(t *testing.T) {
	t.Parallel()

	pkWaiting := "pk-waiting"
	pkObserver := "pk-observer"

	arg := createMockArgHeartbeatMonitor()
	arg.MaxDurationPeerUnresponsive = time.Second * 1000
	arg.PubKeysList = []string{}
	arg.PeerTypeProvider = &mock.PeerTypeProviderStub{
		ComputeForPubKeyCalled: func(pubKey []byte) (core.PeerType, uint32, error) {
			if string(pubKey) == pkWaiting {
				return core.WaitingList, 0, nil
			}
			return core.ObserverList, 0, nil
		},
	}
	mon, err := process.NewMonitor(arg)
	require.Nil(t, err)
	// stop the background refresher before installing the status handler so a
	// stale initial refresh can never overwrite the asserted metric values;
	// the test drives the refresh manually
	require.NoError(t, mon.Close())

	liveValidators := uint64(0)
	connectedNodes := uint64(0)
	require.NoError(t, mon.SetAppStatusHandler(&mock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {
			switch key {
			case core.MetricLiveValidatorNodes:
				liveValidators = value
			case core.MetricConnectedNodes:
				connectedNodes = value
			}
		},
	}))

	mon.SendHeartbeatMessage(&data.Heartbeat{Pubkey: []byte(pkWaiting)})
	mon.SendHeartbeatMessage(&data.Heartbeat{Pubkey: []byte(pkObserver)})

	mon.RefreshHeartbeatMessageInfo()

	assert.Equal(t, uint64(1), liveValidators)
	assert.Equal(t, uint64(2), connectedNodes)
}

func TestMonitor_UnstakedWaitingNodeIsDemotedAndCleaned(t *testing.T) {
	t.Parallel()

	pkWaiting := "pk-waiting"

	timer := mock.NewTimerMock()
	genesisTime := timer.Now()

	stillWaiting := true

	arg := createMockArgHeartbeatMonitor()
	arg.GenesisTime = genesisTime
	arg.Timer = timer
	arg.PubKeysList = []string{}
	arg.PeerTypeProvider = &mock.PeerTypeProviderStub{
		ComputeForPubKeyCalled: func(pubKey []byte) (core.PeerType, uint32, error) {
			if stillWaiting && string(pubKey) == pkWaiting {
				return core.WaitingList, 0, nil
			}
			return core.ObserverList, 0, nil
		},
		GetAllPeerTypeInfosCalled: func() []*state.PeerTypeInfo {
			if stillWaiting {
				return []*state.PeerTypeInfo{
					{PublicKey: pkWaiting, PeerType: string(core.WaitingList)},
				}
			}
			return nil
		},
	}
	mon, err := process.NewMonitor(arg)
	require.Nil(t, err)
	// stop the background refresher: the test drives refresh and cleanup
	// manually, and flips the stub without synchronization
	require.NoError(t, mon.Close())

	timer.SetSeconds(int(arg.HideInactiveValidatorIntervalInSec) + 100)

	mon.RefreshHeartbeatMessageInfo()
	mon.Cleanup()
	assert.Equal(t, 1, mon.GetNumHearbeatMessages())

	// the key leaves the waiting list (e.g. unstaked before promotion): the
	// cache rebuild drops it and ComputeForPubKey falls back to observer, so
	// the entry must be demoted by the next refresh and removed by Cleanup
	stillWaiting = false

	mon.RefreshHeartbeatMessageInfo()
	mon.Cleanup()
	assert.Equal(t, 0, mon.GetNumHearbeatMessages())
}

func TestMonitor_CleanupShouldWork(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()

	currentTime := time.Now()
	timer := &mock.TimerMock{
		NowCalled: func() time.Time {
			return currentTime.Add(time.Second * time.Duration(arg.HideInactiveValidatorIntervalInSec+1))
		},
	}

	arg.Timer = timer
	mon, _ := process.NewMonitor(arg)

	assert.Equal(t, 1, mon.GetNumHearbeatMessages())
	assert.Equal(t, 0, mon.GetNumDoubleSignerPeers())

	hbmi, _ := process.NewHeartbeatMessageInfo(time.Second, "1", currentTime, timer)
	mon.AddHeartbeatMessage("pk1", hbmi)
	mon.AddDoubleSignerPeers(&data.Heartbeat{Pubkey: []byte("pk1"), Pid: []byte("pid1")})
	assert.Equal(t, 2, mon.GetNumHearbeatMessages())
	assert.Equal(t, 1, mon.GetNumDoubleSignerPeers())

	hbmi, _ = process.NewHeartbeatMessageInfo(time.Second, "2", currentTime, timer)
	mon.AddHeartbeatMessage("pk2", hbmi)
	mon.AddDoubleSignerPeers(&data.Heartbeat{Pubkey: []byte("pk2"), Pid: []byte("pid1")})
	assert.Equal(t, 3, mon.GetNumHearbeatMessages())
	assert.Equal(t, 2, mon.GetNumDoubleSignerPeers())

	mon.Cleanup()

	assert.Equal(t, 0, mon.GetNumHearbeatMessages())
	assert.Equal(t, 0, mon.GetNumDoubleSignerPeers())
}
