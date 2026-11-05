package txsimulator_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	processMock "github.com/klever-io/klever-go/core/process/mock"
	"github.com/klever-io/klever-go/core/process/txsimulator"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingTxProcessor holds the first transaction until release is closed; later ones return at once
type blockingTxProcessor struct {
	started  chan struct{}
	release  chan struct{}
	numCalls int32
}

func newBlockingTxProcessor() *blockingTxProcessor {
	return &blockingTxProcessor{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (b *blockingTxProcessor) ProcessTransaction(_ *transaction.Transaction) error {
	if atomic.AddInt32(&b.numCalls, 1) == 1 {
		close(b.started)
		<-b.release
	}
	return nil
}

func (b *blockingTxProcessor) IsInterfaceNil() bool {
	return b == nil
}

func createSimulator(t *testing.T, txProcessor txsimulator.TransactionProcessor, maxWait time.Duration) txsimulator.TransactionSimulatorProcessor {
	simulator, err := txsimulator.NewTransactionSimulator(txsimulator.ArgsTxSimulator{
		TransactionProcessor:   txProcessor,
		AddressPubKeyConverter: processMock.NewPubkeyConverterMock(32),
		VMOutputCacher:         commonMock.NewCacherStub(),
		Hasher:                 &commonMock.HasherMock{},
		Marshalizer:            &commonMock.MarshalizerMock{},
		MaxWait:                maxWait,
	})
	require.NoError(t, err)

	return simulator
}

func newTestTx() *transaction.Transaction {
	return &transaction.Transaction{RawData: &transaction.Transaction_Raw{}}
}

func TestTransactionSimulator_ProcessTx_RefusedAsBusyWithinBoundWhileSimulatorIsHeld(t *testing.T) {
	t.Parallel()

	const maxWait = 50 * time.Millisecond
	txProcessor := newBlockingTxProcessor()
	simulator := createSimulator(t, txProcessor, maxWait)

	heavyDone := make(chan error, 1)
	go func() {
		_, err := simulator.ProcessTx(newTestTx())
		heavyDone <- err
	}()
	<-txProcessor.started

	const numWaiting = 8
	var wg sync.WaitGroup
	errs := make([]error, numWaiting)
	elapsed := make([]time.Duration, numWaiting)
	for i := 0; i < numWaiting; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			start := time.Now()
			_, errs[idx] = simulator.ProcessTx(newTestTx())
			elapsed[idx] = time.Since(start)
		}(i)
	}
	wg.Wait()

	for i := 0; i < numWaiting; i++ {
		assert.True(t, errors.Is(errs[i], common.ErrTxSimulatorBusy), "simulation %d: %v", i, errs[i])
		assert.GreaterOrEqual(t, elapsed[i], maxWait)
		assert.Less(t, elapsed[i], maxWait+time.Second, "simulation %d waited past the bound", i)
	}

	close(txProcessor.release)
	require.NoError(t, <-heavyDone)

	// once the simulator is free, a simulation runs normally
	results, err := simulator.ProcessTx(newTestTx())
	require.NoError(t, err)
	assert.Equal(t, transaction.Transaction_SUCCESS, results.Result)
}

func TestTransactionSimulator_ProcessTx_WaitingSimulationRunsWhenSimulatorFreesWithinBound(t *testing.T) {
	t.Parallel()

	txProcessor := newBlockingTxProcessor()
	simulator := createSimulator(t, txProcessor, time.Minute)

	heavyDone := make(chan error, 1)
	go func() {
		_, err := simulator.ProcessTx(newTestTx())
		heavyDone <- err
	}()
	<-txProcessor.started

	lightDone := make(chan error, 1)
	go func() {
		_, err := simulator.ProcessTx(newTestTx())
		lightDone <- err
	}()

	select {
	case err := <-lightDone:
		t.Fatalf("simulation ran while the simulator was held: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	close(txProcessor.release)
	require.NoError(t, <-heavyDone)
	require.NoError(t, <-lightDone)
	assert.Equal(t, int32(2), atomic.LoadInt32(&txProcessor.numCalls))
}

func TestTransactionSimulator_ProcessTx_UnsetMaxWaitStillBoundsTheWait(t *testing.T) {
	t.Parallel()

	txProcessor := newBlockingTxProcessor()
	simulator := createSimulator(t, txProcessor, 0)

	heavyDone := make(chan error, 1)
	go func() {
		_, err := simulator.ProcessTx(newTestTx())
		heavyDone <- err
	}()
	<-txProcessor.started

	start := time.Now()
	_, err := simulator.ProcessTx(newTestTx())
	elapsed := time.Since(start)

	assert.True(t, errors.Is(err, common.ErrTxSimulatorBusy), "%v", err)
	assert.GreaterOrEqual(t, elapsed, txsimulator.DefaultMaxWait)
	assert.Less(t, elapsed, txsimulator.DefaultMaxWait+time.Second)

	close(txProcessor.release)
	require.NoError(t, <-heavyDone)
}
