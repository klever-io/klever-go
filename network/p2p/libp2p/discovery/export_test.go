package discovery

import (
	"context"
	"time"
)

const KadDhtName = kadDhtName
const NullName = nilName

//------- ContinuousKadDhtDiscoverer

func (ckdd *ContinuousKadDhtDiscoverer) ConnectToOnePeerFromInitialPeersList(
	durationBetweenAttempts time.Duration,
	initialPeersList []string) <-chan struct{} {

	return ckdd.connectToOnePeerFromInitialPeersList(durationBetweenAttempts, initialPeersList)
}

func (ckdd *ContinuousKadDhtDiscoverer) StopDHT() error {
	ckdd.mutKadDht.Lock()
	err := ckdd.stopDHT()
	ckdd.mutKadDht.Unlock()

	return err
}

//------- optimizedKadDhtDiscoverer

const OptimizedKadDhtName = optimizedKadDhtName

// NumSeederCycles returns how many seeder top-ups have finished
func (okdd *optimizedKadDhtDiscoverer) NumSeederCycles() uint64 {
	return okdd.numSeederCycles.Load()
}

// LastNumCountedSeeders returns how many seeders the last top-up counted (-1 before the first)
func (okdd *optimizedKadDhtDiscoverer) LastNumCountedSeeders() int {
	return int(okdd.lastNumCountedSeeders.Load())
}

// NewOptimizedKadDhtDiscovererWithKadDht creates an optimized discoverer whose Bootstrap uses the
// provided kad-dht handler instead of a real one, then starts its process loop
func NewOptimizedKadDhtDiscovererWithKadDht(arg ArgKadDht, kadDht KadDhtHandler) (*optimizedKadDhtDiscoverer, error) {
	okdd, err := newOptimizedKadDhtDiscoverer(arg)
	if err != nil {
		return nil, err
	}

	okdd.createKadDhtHandler = func(_ context.Context) (KadDhtHandler, error) {
		return kadDht, nil
	}
	go okdd.processLoop(arg.Context)

	return okdd, nil
}
