package process_test

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/core"
	coreProcess "github.com/klever-io/klever-go/core/process"
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

func TestMonitor_ProcessReceivedMessageNilMessageShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	mon, _ := process.NewMonitor(arg)

	err := mon.ProcessReceivedMessage(nil, fromConnectedPeerId)
	assert.Equal(t, heartbeat.ErrNilMessage, err)
}

func TestMonitor_ProcessReceivedMessageNilDataShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	mon, _ := process.NewMonitor(arg)

	err := mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: nil}, fromConnectedPeerId)
	assert.Equal(t, heartbeat.ErrNilDataToProcess, err)
}

func TestMonitor_ProcessReceivedMessageAntifloodCanNotProcessMessageShouldErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("can not process message")
	arg := createMockArgHeartbeatMonitor()
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		CanProcessMessageCalled: func(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
			return expectedErr
		},
	}
	arg.MessageHandler = &mock.MessageHandlerStub{
		CreateHeartbeatFromP2PMessageCalled: func(message p2p.MessageP2P) (*data.Heartbeat, error) {
			assert.Fail(t, "should have not created the heartbeat")
			return nil, nil
		},
	}
	mon, _ := process.NewMonitor(arg)

	err := mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: []byte("data")}, fromConnectedPeerId)
	assert.Equal(t, expectedErr, err)
}

func TestMonitor_ProcessReceivedMessageAntifloodCanNotProcessMessagesOnTopicShouldErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("can not process messages on topic")
	arg := createMockArgHeartbeatMonitor()
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		CanProcessMessagesOnTopicCalled: func(peer core.PeerID, topic string, numMessages uint32, totalSize uint64, sequence []byte) error {
			return expectedErr
		},
	}
	arg.MessageHandler = &mock.MessageHandlerStub{
		CreateHeartbeatFromP2PMessageCalled: func(message p2p.MessageP2P) (*data.Heartbeat, error) {
			assert.Fail(t, "should have not created the heartbeat")
			return nil, nil
		},
	}
	mon, _ := process.NewMonitor(arg)

	err := mon.ProcessReceivedMessage(&mock.P2PMessageStub{DataField: []byte("data")}, fromConnectedPeerId)
	assert.Equal(t, expectedErr, err)
}

func TestMonitor_ProcessReceivedMessageInvalidHeartbeatShouldBlacklistBothPeers(t *testing.T) {
	t.Parallel()

	originator := core.PeerID("message originator")
	expectedErr := errors.New("invalid heartbeat")
	arg := createMockArgHeartbeatMonitor()
	arg.MessageHandler = &mock.MessageHandlerStub{
		CreateHeartbeatFromP2PMessageCalled: func(message p2p.MessageP2P) (*data.Heartbeat, error) {
			return nil, expectedErr
		},
	}
	blacklistReasons := make(map[core.PeerID]string)
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		BlacklistPeerCalled: func(pid core.PeerID, reason string, duration time.Duration) {
			blacklistReasons[pid] = reason
			assert.Equal(t, core.InvalidMessageBlacklistDuration, duration)
		},
	}
	mon, _ := process.NewMonitor(arg)

	message := &mock.P2PMessageStub{
		DataField: []byte("data"),
		PeerField: originator,
	}

	err := mon.ProcessReceivedMessage(message, fromConnectedPeerId)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, coreProcess.BlacklistReasonInvalidHeartbeat, blacklistReasons[originator])
	assert.Equal(t, coreProcess.BlacklistReasonInvalidHeartbeat, blacklistReasons[fromConnectedPeerId])
}

func TestMonitor_ProcessReceivedMessagePidMismatchOnCreateShouldBlacklistAsInconsistent(t *testing.T) {
	t.Parallel()

	originator := core.PeerID("message originator")
	expectedErr := fmt.Errorf("%w heartbeat pid %s, message pid %s",
		heartbeat.ErrHeartbeatPidMismatch, "hb pid", originator)
	arg := createMockArgHeartbeatMonitor()
	arg.MessageHandler = &mock.MessageHandlerStub{
		CreateHeartbeatFromP2PMessageCalled: func(message p2p.MessageP2P) (*data.Heartbeat, error) {
			return nil, expectedErr
		},
	}
	blacklistReasons := make(map[core.PeerID]string)
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		BlacklistPeerCalled: func(pid core.PeerID, reason string, duration time.Duration) {
			blacklistReasons[pid] = reason
			assert.Equal(t, core.InvalidMessageBlacklistDuration, duration)
		},
	}
	mon, _ := process.NewMonitor(arg)

	message := &mock.P2PMessageStub{
		DataField: []byte("data"),
		PeerField: originator,
	}

	err := mon.ProcessReceivedMessage(message, fromConnectedPeerId)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, coreProcess.BlacklistReasonInconsistentHeartbeat, blacklistReasons[originator])
	assert.Equal(t, coreProcess.BlacklistReasonInconsistentHeartbeat, blacklistReasons[fromConnectedPeerId])
}

//------- SetAppStatusHandler

func TestMonitor_SetAppStatusHandlerNilShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	mon, _ := process.NewMonitor(arg)

	err := mon.SetAppStatusHandler(nil)
	assert.Equal(t, heartbeat.ErrNilAppStatusHandler, err)
}

func TestMonitor_SetAppStatusHandlerShouldWork(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	mon, _ := process.NewMonitor(arg)

	err := mon.SetAppStatusHandler(&mock.AppStatusHandlerStub{})
	assert.Nil(t, err)
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

func TestMonitor_UnknownIdentitiesStayWithinCapBeforeCleanup(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = []string{}
	arg.MaxDurationPeerUnresponsive = 10 * time.Second
	arg.MaxUnknownHeartbeatPubKeys = 4
	arg.MaxUnknownHeartbeatPubKeysPerOrigin = 2

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	defer mon.Close()

	for origin := 0; origin < 10; origin++ {
		for idx := 0; idx < 2; idx++ {
			hb := &data.Heartbeat{
				Pubkey: []byte(fmt.Sprintf("unknown-%d-%d", origin, idx)),
				Pid:    []byte(fmt.Sprintf("pid-%d-%d", origin, idx)),
			}
			mon.AddHeartbeatMessageFromOrigin(hb, core.PeerID(fmt.Sprintf("origin-%d", origin)))
		}
	}

	assert.LessOrEqual(t, mon.GetNumHearbeatMessages(), int(arg.MaxUnknownHeartbeatPubKeys))
	assert.LessOrEqual(t, mon.GetNumDoubleSignerPeers(), int(arg.MaxUnknownHeartbeatPubKeys))
}

func TestMonitor_UnknownIdentitiesAreNeverPersisted(t *testing.T) {
	t.Parallel()

	var mutPersisted sync.Mutex
	persistedPubKeys := make([]string, 0)

	timer := mock.NewTimerMock()

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = []string{}
	arg.Timer = timer
	arg.MaxDurationPeerUnresponsive = 10 * time.Second
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
			mutPersisted.Lock()
			persistedPubKeys = append(persistedPubKeys, string(pubkey))
			mutPersisted.Unlock()
			return nil
		},
		RemovePubkeyDataCalled: func(pubkey []byte) error {
			return nil
		},
		SaveKeysCalled: func(peersSlice [][]byte) error {
			return nil
		},
	}

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	defer mon.Close()

	admittedPubKey := "admitted-validator-pk"
	unknownPubKey := "unknown-external-pk"

	mon.AddTrustedHeartbeatMessageToMap(&data.Heartbeat{Pubkey: []byte(admittedPubKey), Pid: []byte("pid-admitted")})
	mon.AddHeartbeatMessageFromOrigin(
		&data.Heartbeat{Pubkey: []byte(unknownPubKey), Pid: []byte("pid-unknown")},
		core.PeerID("origin-a"),
	)

	mutPersisted.Lock()
	persistedPubKeys = persistedPubKeys[:0]
	mutPersisted.Unlock()

	timer.IncrementSeconds(60)
	mon.RefreshHeartbeatMessageInfo()

	mutPersisted.Lock()
	defer mutPersisted.Unlock()

	assert.Contains(t, persistedPubKeys, admittedPubKey)
	assert.NotContains(t, persistedPubKeys, unknownPubKey)
}

func newWalkSignalHandler(t *testing.T, walks chan<- struct{}, gate <-chan struct{}, gateWalks *atomic.Bool, entered chan<- struct{}) *mock.AppStatusHandlerStub {
	t.Helper()

	return &mock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {
			if key != core.MetricConnectedNodes {
				return
			}
			if gateWalks != nil && gateWalks.Load() {
				select {
				case entered <- struct{}{}:
				default:
				}
				<-gate
			}
			select {
			case walks <- struct{}{}:
			default:
			}
		},
	}
}

func waitForWalk(t *testing.T, walks <-chan struct{}) {
	t.Helper()

	select {
	case <-walks:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "recompute did not run")
	}
}

func requireNoWalkQueued(t *testing.T, walks <-chan struct{}) {
	t.Helper()

	select {
	case <-walks:
		require.FailNow(t, "unexpected recompute")
	default:
	}
}

func drainWalks(walks <-chan struct{}) {
	for {
		select {
		case <-walks:
		default:
			return
		}
	}
}

func TestMonitor_RejectedUnknownHeartbeatDoesNotTriggerRecompute(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = []string{}
	arg.HeartbeatRefreshIntervalInSec = 3600
	arg.MaxDurationPeerUnresponsive = 10 * time.Second
	arg.MaxUnknownHeartbeatPubKeys = 1
	arg.MaxUnknownHeartbeatPubKeysPerOrigin = 1

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mon.Close() })

	walks := make(chan struct{}, 16)
	require.NoError(t, mon.SetAppStatusHandler(newWalkSignalHandler(t, walks, nil, nil, nil)))
	drainWalks(walks)

	origin := core.PeerID("origin-a")
	mon.ProcessValidatedHeartbeat(&data.Heartbeat{Pubkey: []byte("unknown-accepted"), Pid: []byte("pid-1")}, origin)
	waitForWalk(t, walks)
	requireNoWalkQueued(t, walks)

	mon.ProcessValidatedHeartbeat(&data.Heartbeat{Pubkey: []byte("unknown-rejected"), Pid: []byte("pid-2")}, origin)
	assert.False(t, mon.HasPendingRecompute())
	requireNoWalkQueued(t, walks)
}

func TestMonitor_BurstOfAcceptedHeartbeatsCoalescesRecomputes(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = []string{}
	arg.HeartbeatRefreshIntervalInSec = 3600
	arg.MaxDurationPeerUnresponsive = 10 * time.Second
	arg.MaxUnknownHeartbeatPubKeys = 64
	arg.MaxUnknownHeartbeatPubKeysPerOrigin = 64

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mon.Close() })

	walks := make(chan struct{}, 64)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	var gateWalks atomic.Bool

	require.NoError(t, mon.SetAppStatusHandler(newWalkSignalHandler(t, walks, release, &gateWalks, entered)))
	drainWalks(walks)
	gateWalks.Store(true)

	origin := core.PeerID("origin-a")
	mon.ProcessValidatedHeartbeat(&data.Heartbeat{Pubkey: []byte("unknown-0"), Pid: []byte("pid-0")}, origin)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "first recompute never started")
	}

	const burst = 25
	for i := 1; i <= burst; i++ {
		mon.ProcessValidatedHeartbeat(
			&data.Heartbeat{Pubkey: []byte(fmt.Sprintf("unknown-%d", i)), Pid: []byte(fmt.Sprintf("pid-%d", i))},
			origin,
		)
	}
	require.True(t, mon.HasPendingRecompute())

	gateWalks.Store(false)
	releaseOnce.Do(func() { close(release) })
	waitForWalk(t, walks)
	waitForWalk(t, walks)

	assert.False(t, mon.HasPendingRecompute())
	requireNoWalkQueued(t, walks)
}

func TestMonitor_LiveUnknownStateMatchesTrackedSetUnderConcurrentChurn(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = []string{}
	arg.HeartbeatRefreshIntervalInSec = 3600
	arg.MaxDurationPeerUnresponsive = time.Hour
	arg.MaxUnknownHeartbeatPubKeys = 16
	arg.MaxUnknownHeartbeatPubKeysPerOrigin = 4

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	defer mon.Close()

	const (
		senders   = 32
		perSender = 100
		origins   = 8
		keyPool   = 64
	)

	var stopCleanup atomic.Bool
	var cleanupWg sync.WaitGroup
	for i := 0; i < 4; i++ {
		cleanupWg.Add(1)
		go func() {
			defer cleanupWg.Done()
			for !stopCleanup.Load() {
				mon.Cleanup()
			}
		}()
	}

	var sendWg sync.WaitGroup
	for s := 0; s < senders; s++ {
		sendWg.Add(1)
		go func(s int) {
			defer sendWg.Done()
			for i := 0; i < perSender; i++ {
				idx := (s*perSender + i) % keyPool
				hb := &data.Heartbeat{
					Pubkey: []byte(fmt.Sprintf("unknown-%d", idx)),
					Pid:    []byte(fmt.Sprintf("pid-%d", idx)),
				}
				mon.AddHeartbeatMessageFromOrigin(hb, core.PeerID(fmt.Sprintf("origin-%d", (s+i)%origins)))
			}
		}(s)
	}
	sendWg.Wait()
	stopCleanup.Store(true)
	cleanupWg.Wait()

	tracked := mon.GetNumTransientUnknownHeartbeatPubKeys()
	assert.LessOrEqual(t, tracked, int(arg.MaxUnknownHeartbeatPubKeys))
	assert.Equal(t, tracked, mon.GetNumHearbeatMessages())
	assert.Equal(t, tracked, mon.GetNumDoubleSignerPeers())

	mon.Cleanup()

	assert.Equal(t, tracked, mon.GetNumTransientUnknownHeartbeatPubKeys())
	assert.Equal(t, tracked, mon.GetNumHearbeatMessages())
	assert.Equal(t, tracked, mon.GetNumDoubleSignerPeers())
}

func TestMonitor_ScheduledRecomputesStopAfterClose(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = []string{}
	arg.HeartbeatRefreshIntervalInSec = 3600
	arg.MaxDurationPeerUnresponsive = 10 * time.Second
	arg.MaxUnknownHeartbeatPubKeys = 64
	arg.MaxUnknownHeartbeatPubKeysPerOrigin = 64

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mon.Close() })

	walks := make(chan struct{}, 64)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	var gateWalks atomic.Bool

	require.NoError(t, mon.SetAppStatusHandler(newWalkSignalHandler(t, walks, release, &gateWalks, entered)))
	drainWalks(walks)
	gateWalks.Store(true)

	origin := core.PeerID("origin-a")
	mon.ProcessValidatedHeartbeat(&data.Heartbeat{Pubkey: []byte("unknown-0"), Pid: []byte("pid-0")}, origin)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "first recompute never started")
	}

	mon.ProcessValidatedHeartbeat(&data.Heartbeat{Pubkey: []byte("unknown-1"), Pid: []byte("pid-1")}, origin)
	require.True(t, mon.HasPendingRecompute())

	closed := make(chan error, 1)
	go func() { closed <- mon.Close() }()
	select {
	case <-mon.StopSignal():
	case <-time.After(5 * time.Second):
		require.FailNow(t, "close never signalled stop")
	}

	gateWalks.Store(false)
	releaseOnce.Do(func() { close(release) })
	select {
	case err = <-closed:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "close did not return")
	}

	waitForWalk(t, walks)
	requireNoWalkQueued(t, walks)
}

func TestMonitor_AdmittedIdentityNeverHoldsTransientSlot(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = []string{}
	arg.HeartbeatRefreshIntervalInSec = 3600
	arg.MaxDurationPeerUnresponsive = time.Hour
	arg.MaxUnknownHeartbeatPubKeys = 8192
	arg.MaxUnknownHeartbeatPubKeysPerOrigin = 8192

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mon.Close() })

	const iterations = 3000
	origin := core.PeerID("origin-a")
	for i := 0; i < iterations; i++ {
		pubKey := fmt.Sprintf("racing-%d", i)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			mon.AddHeartbeatMessageFromOrigin(&data.Heartbeat{Pubkey: []byte(pubKey), Pid: []byte("pid")}, origin)
		}()
		go func() {
			defer wg.Done()
			mon.MarkHeartbeatPubKeyAsAdmitted(pubKey)
		}()
		wg.Wait()
	}

	assert.Equal(t, 0, mon.GetNumTransientUnknownHeartbeatPubKeys())
}

func TestMonitor_HeartbeatFromAdmittedIdentityReleasesTransientSlot(t *testing.T) {
	t.Parallel()

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = []string{}
	arg.HeartbeatRefreshIntervalInSec = 3600
	arg.MaxDurationPeerUnresponsive = time.Hour

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mon.Close() })

	pubKey := "admitted-with-stale-slot"
	origin := core.PeerID("origin-a")
	mon.MarkHeartbeatPubKeyAsAdmitted(pubKey)
	require.True(t, mon.TrackTransientUnknownHeartbeatPubKey(pubKey, origin))
	require.Equal(t, 1, mon.GetNumTransientUnknownHeartbeatPubKeys())

	mon.AddHeartbeatMessageFromOrigin(&data.Heartbeat{Pubkey: []byte(pubKey), Pid: []byte("pid")}, origin)

	assert.Equal(t, 0, mon.GetNumTransientUnknownHeartbeatPubKeys())
	assert.Equal(t, 1, mon.GetNumHearbeatMessages())
}

func TestMonitor_RefreshPersistsIdentityAdmittedAfterFirstHeartbeat(t *testing.T) {
	t.Parallel()

	var mutStore sync.Mutex
	persisted := make([]string, 0)
	savedKeyLists := make([][][]byte, 0)
	var listed atomic.Bool
	admittedPubKey := "late-validator"

	arg := createMockArgHeartbeatMonitor()
	arg.PubKeysList = []string{}
	arg.HeartbeatRefreshIntervalInSec = 3600
	arg.MaxDurationPeerUnresponsive = time.Hour
	arg.PeerTypeProvider = &mock.PeerTypeProviderStub{
		ComputeForPubKeyCalled: func(pubKey []byte) (core.PeerType, uint32, error) {
			return "", 1, nil
		},
		GetAllPeerTypeInfosCalled: func() []*state.PeerTypeInfo {
			if !listed.Load() {
				return nil
			}
			return []*state.PeerTypeInfo{{PublicKey: admittedPubKey, PeerType: string(core.EligibleList)}}
		},
	}
	arg.Storer = &mock.HeartbeatStorerStub{
		UpdateGenesisTimeCalled: func(genesisTime time.Time) error { return nil },
		LoadHeartBeatDTOCalled:  func(pubKey string) (*data.HeartbeatDTO, error) { return nil, errors.New("not found") },
		LoadKeysCalled:          func() ([][]byte, error) { return nil, nil },
		SavePubkeyDataCalled: func(pubkey []byte, heartbeat *data.HeartbeatDTO) error {
			mutStore.Lock()
			persisted = append(persisted, string(pubkey))
			mutStore.Unlock()
			return nil
		},
		RemovePubkeyDataCalled: func(pubkey []byte) error { return nil },
		SaveKeysCalled: func(peersSlice [][]byte) error {
			mutStore.Lock()
			savedKeyLists = append(savedKeyLists, peersSlice)
			mutStore.Unlock()
			return nil
		},
	}

	mon, err := process.NewMonitor(arg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mon.Close() })

	mon.AddHeartbeatMessageFromOrigin(&data.Heartbeat{Pubkey: []byte(admittedPubKey), Pid: []byte("pid")}, core.PeerID("origin-a"))
	mutStore.Lock()
	require.Empty(t, persisted)
	mutStore.Unlock()

	listed.Store(true)
	mon.RefreshHeartbeatMessageInfo()

	mutStore.Lock()
	defer mutStore.Unlock()
	assert.Contains(t, persisted, admittedPubKey)
	require.NotEmpty(t, savedKeyLists)
	assert.Contains(t, savedKeyLists[len(savedKeyLists)-1], []byte(admittedPubKey))
	assert.Equal(t, 0, mon.GetNumTransientUnknownHeartbeatPubKeys())
}
