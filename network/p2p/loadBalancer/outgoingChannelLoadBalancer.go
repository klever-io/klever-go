package loadBalancer

import (
	"context"
	"sync"

	"github.com/klever-io/klever-go/network/p2p"
)

var _ p2p.ChannelLoadBalancer = (*OutgoingChannelLoadBalancer)(nil)

const defaultSendChannel = "default send channel"

// OutgoingChannelLoadBalancer is a component that evenly balances requests to be sent
type OutgoingChannelLoadBalancer struct {
	mut      sync.RWMutex
	chans    []chan *p2p.SendableData
	mainChan chan *p2p.SendableData
	names    []string
	//namesChans is defined only for performance purposes as to fast search by name
	//iteration is done directly on slices as that is used very often and is about 50x
	//faster then an iteration over a map
	namesChans map[string]chan *p2p.SendableData
	cancelFunc context.CancelFunc
	ctx        context.Context //we need the context saved here in order to call appendChannel from exported func AddChannel
}

// NewOutgoingChannelLoadBalancer creates a new instance of a ChannelLoadBalancer instance
func NewOutgoingChannelLoadBalancer() *OutgoingChannelLoadBalancer {
	ctx, cancelFunc := context.WithCancel(context.Background())

	oclb := &OutgoingChannelLoadBalancer{
		chans:      make([]chan *p2p.SendableData, 0),
		names:      make([]string, 0),
		namesChans: make(map[string]chan *p2p.SendableData),
		mainChan:   make(chan *p2p.SendableData),
		cancelFunc: cancelFunc,
		ctx:        ctx,
	}

	oclb.appendChannel(defaultSendChannel)

	return oclb
}

func (oplb *OutgoingChannelLoadBalancer) appendChannel(channel string) {
	oplb.names = append(oplb.names, channel)
	ch := make(chan *p2p.SendableData)
	oplb.chans = append(oplb.chans, ch)
	oplb.namesChans[channel] = ch

	go func() {
		for {
			var obj *p2p.SendableData

			select {
			case obj = <-ch:
			case <-oplb.ctx.Done():
				return
			}

			select {
			case oplb.mainChan <- obj:
			case <-oplb.ctx.Done():
				return
			}
		}
	}()
}

// AddChannel adds a new channel to the throttler, if it does not exists
func (oplb *OutgoingChannelLoadBalancer) AddChannel(channel string) error {
	if channel == defaultSendChannel {
		return p2p.ErrChannelCanNotBeReAdded
	}

	oplb.mut.Lock()
	defer oplb.mut.Unlock()

	for _, name := range oplb.names {
		if name == channel {
			return nil
		}
	}

	oplb.appendChannel(channel)

	return nil
}

// GetChannelOrDefault fetches the required channel or the default if the channel is not present
func (oplb *OutgoingChannelLoadBalancer) GetChannelOrDefault(channel string) chan *p2p.SendableData {
	oplb.mut.RLock()
	defer oplb.mut.RUnlock()

	ch := oplb.namesChans[channel]
	if ch != nil {
		return ch
	}

	return oplb.chans[0]
}

// CollectOneElementFromChannels gets the waiting object from mainChan. It blocks until one arrives or Close is called, then returns nil
func (oplb *OutgoingChannelLoadBalancer) CollectOneElementFromChannels() *p2p.SendableData {
	select {
	case obj := <-oplb.mainChan:
		return obj
	case <-oplb.ctx.Done():
		return nil
	}
}

// Close finishes all started go routines in this instance
func (oplb *OutgoingChannelLoadBalancer) Close() error {
	oplb.cancelFunc()
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (oplb *OutgoingChannelLoadBalancer) IsInterfaceNil() bool {
	return oplb == nil
}
