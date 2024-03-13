package p2p

import (
	"context"

	"github.com/klever-io/klever-go/core"
)

func newTestP2PDebugger(
	selfPeerID core.PeerID,
	shouldProcessDataFn func() bool,
	printStringFn func(string),
) *p2pDebugger {
	pd := &p2pDebugger{
		selfPeerID: selfPeerID,
		data:       make(map[string]*metric),
	}
	pd.shouldProcessDataFn = shouldProcessDataFn
	pd.printStringFn = printStringFn

	ctx, cancelFunc := context.WithCancel(context.Background())
	pd.cancelFunc = cancelFunc

	go pd.continuouslyPrintStatistics(ctx)

	return pd
}

func (pd *p2pDebugger) GetClonedMetric(topic string) *metric {
	pd.mut.Lock()
	defer pd.mut.Unlock()

	m := pd.data[topic]
	if m == nil {
		return nil
	}

	clonedMetric := *m

	return &clonedMetric
}
