package process_test

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
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
		RemovePubkeyDataCalled: func(pubkey []byte) error {
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

	require.NoError(t, err)
	require.NotNil(t, mon)
	t.Cleanup(func() {
		require.NoError(t, mon.Close())
	})
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
	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	require.NotNil(t, mon)
	t.Cleanup(func() {
		require.NoError(t, mon.Close())
	})

	hb := data.Heartbeat{
		Pubkey: []byte(pubKey),
	}
	hbBytes, _ := json.Marshal(&hb)
	err = mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: hbBytes}, fromConnectedPeerId)
	assert.Nil(t, err)

	// wait for the processing goroutine to register the heartbeat
	require.Eventually(t, func() bool {
		hb := mon.GetHeartbeats()
		return len(hb) == 1 && hb[0].IsActive
	}, 5*time.Second, 10*time.Millisecond)

	hbStatus := mon.GetHeartbeats()
	assert.Equal(t, 1, len(hbStatus))
	assert.Equal(t, hex.EncodeToString([]byte(pubKey)), hbStatus[0].PublicKey)
}

func TestMonitor_ProcessReceivedMessageWithNewPublicKey(t *testing.T) {
	t.Parallel()

	pubKey := "pk1"
	savedPubkeyData := 0
	savedKeys := 0

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
	arg.Storer = &mock.HeartbeatStorerStub{
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
			savedPubkeyData++
			return nil
		},
		RemovePubkeyDataCalled: func(pubkey []byte) error {
			return nil
		},
		SaveKeysCalled: func(peersSlice [][]byte) error {
			savedKeys++
			return nil
		},
	}
	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	require.NotNil(t, mon)
	t.Cleanup(func() {
		require.NoError(t, mon.Close())
	})

	hb := data.Heartbeat{
		Pubkey: []byte(pubKey),
	}
	hbBytes, _ := json.Marshal(&hb)
	err = mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: hbBytes}, fromConnectedPeerId)
	assert.Nil(t, err)

	// unknown public keys remain in memory only and must not create durable monitor state
	require.Eventually(t, func() bool {
		return len(mon.GetHeartbeats()) == 2
	}, 5*time.Second, 10*time.Millisecond)
	hbStatus := mon.GetHeartbeats()
	assert.Equal(t, 2, len(hbStatus))
	assert.Equal(t, 1, savedPubkeyData)
	assert.Equal(t, 0, savedKeys)
}

func TestMonitor_ProcessReceivedMessageWithNewPublicKeyIsTransient(t *testing.T) {
	t.Parallel()

	pubKey := "pk1"
	timer := mock.NewTimerMock()
	savedKeys := 0

	arg := createMockArgHeartbeatMonitor()
	arg.MaxDurationPeerUnresponsive = 5 * time.Second
	arg.Timer = timer
	arg.PubKeysList = []string{"pk2"}
	arg.MessageHandler = &mock.MessageHandlerStub{
		CreateHeartbeatFromP2PMessageCalled: func(message p2p.MessageP2P) (*data.Heartbeat, error) {
			var rcvHb data.Heartbeat
			_ = json.Unmarshal(message.Data(), &rcvHb)
			return &rcvHb, nil
		},
	}
	arg.Storer = &mock.HeartbeatStorerStub{
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
		RemovePubkeyDataCalled: func(pubkey []byte) error {
			return nil
		},
		SaveKeysCalled: func(peersSlice [][]byte) error {
			savedKeys++
			return nil
		},
	}
	mon, _ := process.NewMonitor(arg)

	hb := data.Heartbeat{Pubkey: []byte(pubKey)}
	hbBytes, _ := json.Marshal(&hb)
	err := mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: hbBytes}, fromConnectedPeerId)
	assert.Nil(t, err)

	time.Sleep(time.Second)
	hbStatus := mon.GetHeartbeats()
	assert.Equal(t, 2, len(hbStatus))
	assert.Equal(t, 0, savedKeys)

	timer.IncrementSeconds(6)
	mon.Cleanup()
	hbStatus = mon.GetHeartbeats()
	assert.Equal(t, 1, len(hbStatus))
	assert.Equal(t, hex.EncodeToString([]byte("pk2")), hbStatus[0].PublicKey)
}

func TestMonitor_LoadRestOfPubKeysFromStoragePrunesUnknownPersistedKeys(t *testing.T) {
	t.Parallel()

	knownPubKey := "known-validator"
	unknownPubKey := "legacy-unknown"
	removedUnknown := 0
	savedFiltered := make([][]byte, 0)

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = []string{knownPubKey}
	arg.MaxDurationPeerUnresponsive = 5 * time.Second
	arg.Storer = &mock.HeartbeatStorerStub{
		UpdateGenesisTimeCalled: func(genesisTime time.Time) error {
			return nil
		},
		LoadHeartBeatDTOCalled: func(pubKey string) (*data.HeartbeatDTO, error) {
			return nil, errors.New("not found")
		},
		SavePubkeyDataCalled: func(pubkey []byte, heartbeat *data.HeartbeatDTO) error {
			return nil
		},
		RemovePubkeyDataCalled: func(pubkey []byte) error {
			if string(pubkey) == unknownPubKey {
				removedUnknown++
			}
			return nil
		},
		LoadKeysCalled: func() ([][]byte, error) {
			return [][]byte{[]byte(knownPubKey), []byte(unknownPubKey)}, nil
		},
		SaveKeysCalled: func(peersSlice [][]byte) error {
			savedFiltered = append(savedFiltered[:0], peersSlice...)
			return nil
		},
	}

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	defer mon.Close()

	status := mon.GetHeartbeats()
	require.Len(t, status, 1)
	assert.Equal(t, hex.EncodeToString([]byte(knownPubKey)), status[0].PublicKey)
	assert.Equal(t, 1, removedUnknown)
	require.Len(t, savedFiltered, 1)
	assert.Equal(t, knownPubKey, string(savedFiltered[0]))
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

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	require.NotNil(t, mon)
	t.Cleanup(func() {
		require.NoError(t, mon.Close())
	})

	// First send from pk1 from shard 0
	hb := &data.Heartbeat{
		Pubkey: pubKey,
	}

	buffToSend, err := json.Marshal(hb)
	assert.Nil(t, err)

	err = mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: buffToSend}, fromConnectedPeerId)
	assert.Nil(t, err)

	// wait for the processing goroutine to register the first heartbeat
	require.Eventually(t, func() bool {
		hb := mon.GetHeartbeats()
		return len(hb) == 1 && hb[0].IsActive
	}, 5*time.Second, 10*time.Millisecond)

	// now we send a new heartbeat which will contain a new shard id
	hb = &data.Heartbeat{
		Pubkey: pubKey,
	}

	buffToSend, err = json.Marshal(hb)
	assert.Nil(t, err)

	err = mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: buffToSend}, fromConnectedPeerId)
	assert.Nil(t, err)

	// the second heartbeat reuses the same pubkey, so no new entry may appear
	assert.Never(t, func() bool {
		return len(mon.GetHeartbeats()) != 1
	}, time.Second, 50*time.Millisecond)
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
	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	require.NotNil(t, mon)
	t.Cleanup(func() {
		require.NoError(t, mon.Close())
	})

	// First send from pk1
	err = sendHbMessageFromPubKey(pubKey1, mon)
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
	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	require.NotNil(t, mon)
	t.Cleanup(func() {
		require.NoError(t, mon.Close())
	})
	mon.AddTrustedHeartbeatMessageToMap(&data.Heartbeat{Pubkey: []byte(pkValidator)})
	mon.AddTrustedHeartbeatMessageToMap(&data.Heartbeat{Pubkey: []byte(pubKey1)})
	mon.AddTrustedHeartbeatMessageToMap(&data.Heartbeat{Pubkey: []byte(pubKey2)})
	mon.AddTrustedHeartbeatMessageToMap(&data.Heartbeat{Pubkey: []byte(pubKey3)})
	mon.AddTrustedHeartbeatMessageToMap(&data.Heartbeat{Pubkey: []byte(pubKey4)})
	mon.AddTrustedHeartbeatMessageToMap(&data.Heartbeat{Pubkey: []byte(pubKey5)})

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
	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	require.NotNil(t, mon)
	t.Cleanup(func() {
		require.NoError(t, mon.Close())
	})

	hb := data.Heartbeat{
		Pubkey: []byte(pubKey),
	}
	hbBytes, _ := json.Marshal(&hb)
	message := &mock.P2PMessageStub{
		DataField: hbBytes,
		PeerField: originator,
	}

	err = mon.ProcessReceivedMessage(message, fromConnectedPeerId)
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
	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	require.NotNil(t, mon)
	t.Cleanup(func() {
		require.NoError(t, mon.Close())
	})

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
	// stop the background refresher before the Cleanup/assert rounds: if it
	// kept running it could recreate an evicted entry between Cleanup and the
	// assertion, masking a shielding regression
	require.NoError(t, mon.Close())

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
	pkEligible := "pk-eligible"
	pkObserver := "pk-observer"

	arg := createMockArgHeartbeatMonitor()
	arg.MaxDurationPeerUnresponsive = time.Second * 1000
	arg.PubKeysList = []string{}
	arg.PeerTypeProvider = &mock.PeerTypeProviderStub{
		ComputeForPubKeyCalled: func(pubKey []byte) (core.PeerType, uint32, error) {
			switch string(pubKey) {
			case pkWaiting:
				return core.WaitingList, 0, nil
			case pkEligible:
				return core.EligibleList, 0, nil
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
	liveConsensusValidators := uint64(0)
	connectedNodes := uint64(0)
	require.NoError(t, mon.SetAppStatusHandler(&mock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {
			switch key {
			case core.MetricLiveValidatorNodes:
				liveValidators = value
			case core.MetricLiveConsensusValidatorNodes:
				liveConsensusValidators = value
			case core.MetricConnectedNodes:
				connectedNodes = value
			}
		},
	}))

	mon.SendHeartbeatMessage(&data.Heartbeat{Pubkey: []byte(pkWaiting)})
	mon.SendHeartbeatMessage(&data.Heartbeat{Pubkey: []byte(pkEligible)})
	mon.SendHeartbeatMessage(&data.Heartbeat{Pubkey: []byte(pkObserver)})

	mon.RefreshHeartbeatMessageInfo()

	// waiting and eligible count as live validators, only eligible as
	// consensus-capable, all three as connected
	assert.Equal(t, uint64(2), liveValidators)
	assert.Equal(t, uint64(1), liveConsensusValidators)
	assert.Equal(t, uint64(3), connectedNodes)
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
	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	require.NotNil(t, mon)
	t.Cleanup(func() {
		require.NoError(t, mon.Close())
	})

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

func TestMonitor_SetAppStatusHandlerRepublishesStartupMetrics(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	require.NotNil(t, mon)
	t.Cleanup(func() {
		require.NoError(t, mon.Close())
	})

	// the constructor's initial refresh ran against the placeholder handler;
	// wiring the real one must re-publish the computed metrics
	var mut sync.Mutex
	published := make(map[string]uint64)
	err = mon.SetAppStatusHandler(&mock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {
			mut.Lock()
			published[key] = value
			mut.Unlock()
		},
	})
	require.NoError(t, err)

	mut.Lock()
	defer mut.Unlock()
	_, hasLiveValidators := published[core.MetricLiveValidatorNodes]
	_, hasLiveConsensusValidators := published[core.MetricLiveConsensusValidatorNodes]
	_, hasConnectedNodes := published[core.MetricConnectedNodes]
	assert.True(t, hasLiveValidators)
	assert.True(t, hasLiveConsensusValidators)
	assert.True(t, hasConnectedNodes)
}

func makeHeartbeatMessage(pubKey string) []byte {
	hb := data.Heartbeat{
		Pubkey: []byte(pubKey),
	}
	hbBytes, _ := json.Marshal(&hb)
	return hbBytes
}

func monitorStatusContainsPubKey(status []data.PubKeyHeartbeat, pubKey string) bool {
	encoded := hex.EncodeToString([]byte(pubKey))
	for _, hb := range status {
		if hb.PublicKey == encoded {
			return true
		}
	}
	return false
}

func TestMonitor_ProcessReceivedMessagePerOriginLimitIsEnforced(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = []string{}
	arg.MaxDurationPeerUnresponsive = 5 * time.Second
	arg.MaxUnknownHeartbeatPubKeys = 10
	arg.MaxUnknownHeartbeatPubKeysPerOrigin = 2
	arg.MessageHandler = &mock.MessageHandlerStub{
		CreateHeartbeatFromP2PMessageCalled: func(message p2p.MessageP2P) (*data.Heartbeat, error) {
			var rcvHb data.Heartbeat
			_ = json.Unmarshal(message.Data(), &rcvHb)
			return &rcvHb, nil
		},
	}

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	defer mon.Close()

	for i := 0; i < 3; i++ {
		err = mon.ProcessReceivedMessage(
			&mock.P2PMessageStub{DataField: makeHeartbeatMessage(fmt.Sprintf("pk%d", i))},
			core.PeerID("same-origin"),
		)
		assert.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)
	status := mon.GetHeartbeats()
	assert.Len(t, status, 2)
}

func TestMonitor_ProcessReceivedMessageGlobalCapEvictsOldestTransientUnknown(t *testing.T) {
	t.Parallel()

	timer := mock.NewTimerMock()
	arg := createMockArgHeartbeatMonitor()
	arg.Timer = timer
	arg.PubKeysList = []string{}
	arg.MaxDurationPeerUnresponsive = 10 * time.Second
	arg.MaxUnknownHeartbeatPubKeys = 2
	arg.MaxUnknownHeartbeatPubKeysPerOrigin = 1

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	defer mon.Close()

	mon.AddHeartbeatMessageFromOrigin(&data.Heartbeat{Pubkey: []byte("pk-oldest")}, core.PeerID("origin-1"))
	timer.IncrementSeconds(1)

	mon.AddHeartbeatMessageFromOrigin(&data.Heartbeat{Pubkey: []byte("pk-middle")}, core.PeerID("origin-2"))
	timer.IncrementSeconds(1)

	mon.AddHeartbeatMessageFromOrigin(&data.Heartbeat{Pubkey: []byte("pk-newest")}, core.PeerID("origin-3"))

	mon.Cleanup()
	status := mon.GetHeartbeats()

	assert.Len(t, status, 2)
	assert.False(t, monitorStatusContainsPubKey(status, "pk-oldest"))
	assert.True(t, monitorStatusContainsPubKey(status, "pk-middle"))
	assert.True(t, monitorStatusContainsPubKey(status, "pk-newest"))
}

func TestMonitor_TransientUnknownEntriesExpireOnCleanup(t *testing.T) {
	t.Parallel()

	timer := mock.NewTimerMock()
	arg := createMockArgHeartbeatMonitor()
	arg.Timer = timer
	arg.PubKeysList = []string{}
	arg.MaxDurationPeerUnresponsive = 2 * time.Second
	arg.MaxUnknownHeartbeatPubKeys = 10
	arg.MaxUnknownHeartbeatPubKeysPerOrigin = 10
	arg.MessageHandler = &mock.MessageHandlerStub{
		CreateHeartbeatFromP2PMessageCalled: func(message p2p.MessageP2P) (*data.Heartbeat, error) {
			var rcvHb data.Heartbeat
			_ = json.Unmarshal(message.Data(), &rcvHb)
			return &rcvHb, nil
		},
	}

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	defer mon.Close()

	err = mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: makeHeartbeatMessage("pk-expire")}, core.PeerID("origin-1"))
	assert.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	assert.Len(t, mon.GetHeartbeats(), 1)

	timer.IncrementSeconds(3)
	mon.Cleanup()
	assert.Len(t, mon.GetHeartbeats(), 0)
}

func TestMonitor_ProcessReceivedMessageConcurrentNearCap(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = []string{}
	arg.MaxDurationPeerUnresponsive = 10 * time.Second
	arg.MaxUnknownHeartbeatPubKeys = 64
	arg.MaxUnknownHeartbeatPubKeysPerOrigin = 16
	arg.MessageHandler = &mock.MessageHandlerStub{
		CreateHeartbeatFromP2PMessageCalled: func(message p2p.MessageP2P) (*data.Heartbeat, error) {
			var rcvHb data.Heartbeat
			_ = json.Unmarshal(message.Data(), &rcvHb)
			return &rcvHb, nil
		},
	}

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	defer mon.Close()

	var wg sync.WaitGroup
	for i := 0; i < 120; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pubKey := fmt.Sprintf("pk-%d", i)
			origin := core.PeerID(fmt.Sprintf("origin-%d", i%8))
			_ = mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: makeHeartbeatMessage(pubKey)}, origin)
		}(i)
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mon.Cleanup()
		}()
	}
	wg.Wait()

	time.Sleep(200 * time.Millisecond)
	mon.Cleanup()
	assert.LessOrEqual(t, len(mon.GetHeartbeats()), int(arg.MaxUnknownHeartbeatPubKeys))
}
