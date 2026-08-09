package componentHandler

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process/peer"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/node/heartbeat"
	"github.com/klever-io/klever-go/node/heartbeat/process"
	heartbeatStorage "github.com/klever-io/klever-go/node/heartbeat/storage"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var log = logger.GetOrCreate("heartbeat/componenthandler")

// ArgHeartbeat represents the heartbeat creation argument
type ArgHeartbeat struct {
	HeartbeatConfig          config.HeartbeatConfig
	PrefsConfig              config.PreferencesConfig
	Marshalizer              marshal.Marshalizer
	Messenger                heartbeat.P2PMessenger
	NodesCoordinator         sharding.NodesCoordinator
	AppStatusHandler         core.AppStatusHandler
	Storer                   storage.Storer
	ValidatorStatistics      heartbeat.ValidatorStatisticsProcessor
	PeerSignatureHandler     crypto.PeerSignatureHandler
	PrivKey                  crypto.PrivateKey
	AntifloodHandler         heartbeat.P2PAntifloodHandler
	ValidatorPubkeyConverter core.PubkeyConverter
	EpochStartTrigger        sharding.EpochHandler
	EpochStartRegistration   sharding.EpochStartEventNotifier
	Timer                    heartbeat.Timer
	GenesisTime              time.Time
	VersionNumber            string
	PeerShardMapper          heartbeat.NetworkShardingCollector
	CurrentBlockProvider     heartbeat.CurrentBlockProvider
	RedundancyHandler        consensus.NodeRedundancyHandler
}

// HeartbeatHandler is the struct used to manage heartbeat subsystem consisting of a heartbeat sender and monitor
// wired on a dedicated p2p topic
type HeartbeatHandler struct {
	monitor          *process.Monitor
	sender           *process.Sender
	arg              ArgHeartbeat
	peerTypeProvider *peer.PeerTypeProvider
	cancelFunc       func()
	senderDone       chan struct{}
}

// NewHeartbeatHandler will create a heartbeat handler containing both a monitor and a sender
func NewHeartbeatHandler(arg ArgHeartbeat) (*HeartbeatHandler, error) {
	hbh := &HeartbeatHandler{
		arg: arg,
	}

	err := hbh.create()
	if err != nil {
		return nil, err
	}

	return hbh, nil
}

func (hbh *HeartbeatHandler) create() error {
	arg := hbh.arg

	var ctx context.Context
	ctx, hbh.cancelFunc = context.WithCancel(context.Background())

	err := hbh.checkConfigParams(arg.HeartbeatConfig)
	if err != nil {
		return err
	}

	if check.IfNil(arg.Messenger) {
		return heartbeat.ErrNilMessenger
	}

	if arg.Messenger.HasTopicValidator(common.HeartbeatTopic) {
		return heartbeat.ErrValidatorAlreadySet
	}

	if !arg.Messenger.HasTopic(common.HeartbeatTopic) {
		err = arg.Messenger.CreateTopic(common.HeartbeatTopic, true)
		if err != nil {
			return err
		}
	}
	argPeerTypeProvider := peer.ArgPeerTypeProvider{
		NodesCoordinator:        arg.NodesCoordinator,
		StartEpoch:              arg.EpochStartTrigger.Epoch(),
		EpochStartEventNotifier: arg.EpochStartRegistration,
	}
	peerTypeProvider, err := peer.NewPeerTypeProvider(argPeerTypeProvider)
	if err != nil {
		return err
	}
	hbh.peerTypeProvider = peerTypeProvider
	argSender := process.ArgHeartbeatSender{
		PeerMessenger:        arg.Messenger,
		PeerSignatureHandler: arg.PeerSignatureHandler,
		PrivKey:              arg.PrivKey,
		Marshalizer:          arg.Marshalizer,
		Topic:                common.HeartbeatTopic,
		PeerTypeProvider:     peerTypeProvider,
		StatusHandler:        arg.AppStatusHandler,
		VersionNumber:        arg.VersionNumber,
		NodeDisplayName:      arg.PrefsConfig.NodeDisplayName,
		KeyBaseIdentity:      arg.PrefsConfig.Identity,
		CurrentBlockProvider: arg.CurrentBlockProvider,
		RedundancyHandler:    arg.RedundancyHandler,
	}

	hbh.sender, err = process.NewSender(argSender)
	if err != nil {
		return err
	}

	log.Debug("heartbeat's sender component has been instantiated")

	heartBeatMsgProcessor, err := process.NewMessageProcessor(
		arg.PeerSignatureHandler,
		arg.Marshalizer,
		arg.PeerShardMapper,
	)
	if err != nil {
		return err
	}

	heartbeatStorer, err := heartbeatStorage.NewHeartbeatDbStorer(arg.Storer, arg.Marshalizer)
	if err != nil {
		return err
	}

	timer := &process.RealTimer{}
	netInputMarshalizer := arg.Marshalizer
	allValidators, _, _ := hbh.getLatestValidators()
	pubKeysList := make([]string, 0)

	for _, val := range allValidators {
		pubKeysList = append(pubKeysList, string(val.PublicKey))
	}

	argMonitor := process.ArgHeartbeatMonitor{
		Marshalizer:                        netInputMarshalizer,
		MaxDurationPeerUnresponsive:        time.Second * time.Duration(arg.HeartbeatConfig.DurationToConsiderUnresponsiveInSec),
		PubKeysList:                        pubKeysList,
		GenesisTime:                        arg.GenesisTime,
		MessageHandler:                     heartBeatMsgProcessor,
		Storer:                             heartbeatStorer,
		PeerTypeProvider:                   peerTypeProvider,
		Timer:                              timer,
		AntifloodHandler:                   arg.AntifloodHandler,
		ValidatorPubkeyConverter:           arg.ValidatorPubkeyConverter,
		HeartbeatRefreshIntervalInSec:      arg.HeartbeatConfig.HeartbeatRefreshIntervalInSec,
		HideInactiveValidatorIntervalInSec: arg.HeartbeatConfig.HideInactiveValidatorIntervalInSec,
	}
	hbh.monitor, err = process.NewMonitor(argMonitor)
	if err != nil {
		return err
	}

	log.Debug("heartbeat's monitor component has been instantiated")

	err = hbh.monitor.SetAppStatusHandler(arg.AppStatusHandler)
	if err != nil {
		return err
	}

	err = arg.Messenger.RegisterMessageProcessor(common.HeartbeatTopic, hbh.monitor)
	if err != nil {
		return err
	}

	hbh.senderDone = make(chan struct{})
	go hbh.startSendingHeartbeats(ctx)

	return nil
}

func (hbh *HeartbeatHandler) getLatestValidators() ([]*state.ValidatorInfo, map[string]*state.ValidatorApiResponse, error) {
	latestHash, err := hbh.arg.ValidatorStatistics.RootHash()
	if err != nil {
		return nil, nil, err
	}

	validators, err := hbh.arg.ValidatorStatistics.GetValidatorInfoForRootHash(latestHash)
	if err != nil {
		return nil, nil, err
	}

	return validators, nil, nil
}

func (hbh *HeartbeatHandler) startSendingHeartbeats(ctx context.Context) {
	defer close(hbh.senderDone)
	// #nosec G404: required for randomness
	r := rand.New(rand.NewSource(time.Now().Unix()))
	cfg := hbh.arg.HeartbeatConfig

	log.Debug("heartbeat's endless sending go routine started")

	diffSeconds := cfg.MaxTimeToWaitBetweenBroadcastsInSec - cfg.MinTimeToWaitBetweenBroadcastsInSec
	diffNanos := int64(diffSeconds) * time.Second.Nanoseconds()

	for {
		randomNanos := r.Int63n(diffNanos)
		timeToWait := time.Second*time.Duration(cfg.MinTimeToWaitBetweenBroadcastsInSec) + time.Duration(randomNanos)

		select {
		case <-ctx.Done():
			log.Debug("heartbeat's go routine is stopping...")
			return
		case <-time.After(timeToWait):
		}

		err := hbh.sender.SendHeartbeat()
		if err != nil {
			log.Debug("SendHeartbeat", "error", err.Error())
		}

		hbh.monitor.Cleanup()
	}
}

func (hbh *HeartbeatHandler) checkConfigParams(config config.HeartbeatConfig) error {
	if config.DurationToConsiderUnresponsiveInSec < 1 {
		return heartbeat.ErrInvalidDurationToConsiderUnresponsiveInSec
	}
	if config.MaxTimeToWaitBetweenBroadcastsInSec < 1 {
		return heartbeat.ErrNegativeMaxTimeToWaitBetweenBroadcastsInSec
	}
	if config.MinTimeToWaitBetweenBroadcastsInSec < 1 {
		return heartbeat.ErrNegativeMinTimeToWaitBetweenBroadcastsInSec
	}
	if config.MaxTimeToWaitBetweenBroadcastsInSec <= config.MinTimeToWaitBetweenBroadcastsInSec {
		return fmt.Errorf("%w for MaxTimeToWaitBetweenBroadcastsInSec", heartbeat.ErrWrongValues)
	}
	if config.DurationToConsiderUnresponsiveInSec <= config.MaxTimeToWaitBetweenBroadcastsInSec {
		return fmt.Errorf("%w for DurationToConsiderUnresponsiveInSec", heartbeat.ErrWrongValues)
	}

	return nil
}

// Monitor returns the monitor component
func (hbh *HeartbeatHandler) Monitor() *process.Monitor {
	return hbh.monitor
}

// Sender returns the sender component
func (hbh *HeartbeatHandler) Sender() *process.Sender {
	return hbh.sender
}

// Close stops the heartbeat sender and monitor background goroutines and waits
// for both to exit. Idempotent: each step (cancelFunc, channel receive on closed
// channel, monitor.Close) is safe to repeat.
func (hbh *HeartbeatHandler) Close() error {
	hbh.cancelFunc()
	log.Debug("calling close on heartbeat system")

	<-hbh.senderDone

	if hbh.monitor != nil {
		return hbh.monitor.Close()
	}
	return nil
}

// RefreshPeerTypeCache forces the peer-type cache to rebuild for the given epoch.
func (hbh *HeartbeatHandler) RefreshPeerTypeCache(epoch uint32) {
	if hbh.peerTypeProvider == nil {
		return
	}
	hbh.peerTypeProvider.UpdateCache(epoch)
}

// IsInterfaceNil returns true if there is no value under the interface
func (hbh *HeartbeatHandler) IsInterfaceNil() bool {
	return hbh == nil
}
