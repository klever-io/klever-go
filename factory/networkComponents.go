package factory

import (
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	antifloodFactory "github.com/klever-io/klever-go/core/process/throttle/antiflood/factory"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/libp2p"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/debug/antiflood"
	"github.com/klever-io/klever-go/tools/marshal"
)

type networkComponentsFactory struct {
	p2pConfig     config.P2PConfig
	mainConfig    config.Config
	statusHandler core.AppStatusHandler
	listenAddress string
	marshalizer   marshal.Marshalizer
	syncer        p2p.SyncTimer
}

// NewNetworkComponentsFactory returns a new instance of a network components factory
func NewNetworkComponentsFactory(
	mainConfig config.Config,
	statusHandler core.AppStatusHandler,
	marshalizer marshal.Marshalizer,
	syncer p2p.SyncTimer,
) (*networkComponentsFactory, error) {
	if check.IfNil(statusHandler) {
		return nil, common.ErrNilStatusHandler
	}
	if check.IfNil(marshalizer) {
		return nil, fmt.Errorf("%w in NewNetworkComponentsFactory", common.ErrNilMarshalizer)
	}

	return &networkComponentsFactory{
		p2pConfig:     mainConfig.P2P,
		marshalizer:   marshalizer,
		mainConfig:    mainConfig,
		statusHandler: statusHandler,
		listenAddress: libp2p.ListenAddrWithIp4AndTcp,
		syncer:        syncer,
	}, nil
}

// Create creates and returns the network components
func (ncf *networkComponentsFactory) Create() (*NetworkComponents, error) {
	arg := libp2p.ArgsNetworkMessenger{
		Marshalizer:   ncf.marshalizer,
		ListenAddress: ncf.listenAddress,
		P2pConfig:     ncf.p2pConfig,
		SyncTimer:     ncf.syncer,
	}

	netMessenger, err := libp2p.NewNetworkMessenger(arg)
	if err != nil {
		return nil, err
	}

	inAntifloodHandler, peerIdBlackList, pkTimeCache, errNewAntiflood := antifloodFactory.NewP2PAntiFloodAndBlackList(
		ncf.mainConfig,
		ncf.statusHandler,
		netMessenger.ID(),
	)
	if errNewAntiflood != nil {
		return nil, errNewAntiflood
	}

	if ncf.mainConfig.Debug.Antiflood.Enabled {
		var debugger process.AntifloodDebugger
		debugger, err = antiflood.NewAntifloodDebugger(ncf.mainConfig.Debug.Antiflood)
		if err != nil {
			return nil, err
		}

		err = inAntifloodHandler.SetDebugger(debugger)
		if err != nil {
			return nil, err
		}
	}

	inputAntifloodHandler, ok := inAntifloodHandler.(P2PAntifloodHandler)
	if !ok {
		return nil, fmt.Errorf("%w when casting input antiflood handler to structs/P2PAntifloodHandler", common.ErrWrongTypeAssertion)
	}

	outAntifloodHandler, errOutputAntiflood := antifloodFactory.NewP2POutputAntiFlood(ncf.mainConfig)
	if errOutputAntiflood != nil {
		return nil, errOutputAntiflood
	}

	outputAntifloodHandler, ok := outAntifloodHandler.(P2PAntifloodHandler)
	if !ok {
		return nil, fmt.Errorf("%w when casting output antiflood handler to structs/P2PAntifloodHandler", common.ErrWrongTypeAssertion)
	}

	return &NetworkComponents{
		NetMessenger:           netMessenger,
		InputAntifloodHandler:  inputAntifloodHandler,
		OutputAntifloodHandler: outputAntifloodHandler,
		PeerBlackListHandler:   peerIdBlackList,
		PkTimeCache:            pkTimeCache,
	}, nil
}
