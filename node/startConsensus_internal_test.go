package node

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/watchdog"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	heartbeatProcess "github.com/klever-io/klever-go/node/heartbeat/process"
	"github.com/stretchr/testify/require"
)

// liveIndexer reports a running indexer, which the chronology watchdog cannot
// coexist with
type liveIndexer struct {
	*indexer.NilIndexer
}

func (li *liveIndexer) IsNilIndexer() bool {
	return false
}

// heartbeatStub records the epoch the peer type cache was refreshed with
type heartbeatStub struct {
	refreshedEpoch int64
}

func (h *heartbeatStub) Monitor() *heartbeatProcess.Monitor { return nil }
func (h *heartbeatStub) Sender() *heartbeatProcess.Sender   { return nil }
func (h *heartbeatStub) Close() error                       { return nil }
func (h *heartbeatStub) IsInterfaceNil() bool               { return h == nil }
func (h *heartbeatStub) RefreshPeerTypeCache(epoch uint32) {
	h.refreshedEpoch = int64(epoch)
}

func TestNode_DisableIncompatibleWatchdog(t *testing.T) {
	t.Parallel()

	t.Run("indexer running", func(t *testing.T) {
		t.Parallel()

		n := &Node{indexer: &liveIndexer{}, watchdog: &consensusMock.WatchdogMock{}}
		n.disableIncompatibleWatchdog()
		require.IsType(t, &watchdog.DisabledWatchdog{}, n.watchdog)
	})

	t.Run("import mode", func(t *testing.T) {
		t.Parallel()

		n := &Node{indexer: indexer.NewNilIndexer(), isInImportMode: true, watchdog: &consensusMock.WatchdogMock{}}
		n.disableIncompatibleWatchdog()
		require.IsType(t, &watchdog.DisabledWatchdog{}, n.watchdog)
	})

	t.Run("neither, the watchdog stays", func(t *testing.T) {
		t.Parallel()

		configured := &consensusMock.WatchdogMock{}
		n := &Node{indexer: indexer.NewNilIndexer(), watchdog: configured}
		n.disableIncompatibleWatchdog()
		require.Same(t, configured, n.watchdog)
	})
}

func TestNode_CurrentEpoch(t *testing.T) {
	t.Parallel()

	t.Run("no current header falls back to genesis", func(t *testing.T) {
		t.Parallel()

		heartbeat := &heartbeatStub{refreshedEpoch: -1}
		n := &Node{
			heartbeatHandler: heartbeat,
			blkc: &commonMock.BlockChainMock{
				GetGenesisHeaderCalled:      func() data.HeaderHandler { return &block.Block{Header: &block.BlockHeader{Epoch: 3}} },
				GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return nil },
			},
		}

		require.Equal(t, uint32(3), n.currentEpoch())
		// nothing to refresh before the first block is committed
		require.Equal(t, int64(-1), heartbeat.refreshedEpoch)
	})

	t.Run("current header wins and refreshes the peer type cache", func(t *testing.T) {
		t.Parallel()

		heartbeat := &heartbeatStub{refreshedEpoch: -1}
		n := &Node{
			heartbeatHandler: heartbeat,
			blkc: &commonMock.BlockChainMock{
				GetGenesisHeaderCalled:      func() data.HeaderHandler { return &block.Block{Header: &block.BlockHeader{Epoch: 3}} },
				GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return &block.Block{Header: &block.BlockHeader{Epoch: 9}} },
			},
		}

		require.Equal(t, uint32(9), n.currentEpoch())
		require.Equal(t, int64(9), heartbeat.refreshedEpoch)
	})

	t.Run("no heartbeat handler", func(t *testing.T) {
		t.Parallel()

		n := &Node{
			blkc: &commonMock.BlockChainMock{
				GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return &block.Block{Header: &block.BlockHeader{Epoch: 9}} },
			},
		}

		require.Equal(t, uint32(9), n.currentEpoch())
	})
}

func TestNode_StartProposalController(t *testing.T) {
	t.Parallel()

	// newNode wires the processing KApps adapter and the five consumers
	// publishProposalController fans out to
	newNode := func(t *testing.T, acnt state.AccountHandler, loadErr, saveErr error) *Node {
		forkController, err := fork.NewForkController(config.EnableEpochs{}, &commonMock.EpochNotifierStub{})
		require.Nil(t, err)

		return &Node{
			forkController: forkController,
			processingKApps: &commonMock.AccountsStub{
				LoadAccountCalled: func([]byte) (state.AccountHandler, error) { return acnt, loadErr },
				SaveAccountCalled: func(state.AccountHandler) error { return saveErr },
			},
			feeHandler:               &commonMock.FeeHandlerStub{},
			blockProcessor:           &consensusMock.BlockProcessorMock{},
			processingKAppController: &stub.KAppControllerStub{},
			kappController:           &stub.KAppControllerStub{},
			kappControllerSimulator:  &stub.KAppControllerStub{},
		}
	}

	proposalKApp := func(t *testing.T) state.AccountHandler {
		acnt, err := state.NewKAppAccount(kapps.ProposalKAppAddress)
		require.Nil(t, err)

		return acnt
	}

	t.Run("publishes the controller it started", func(t *testing.T) {
		t.Parallel()

		n := newNode(t, proposalKApp(t), nil, nil)
		require.Nil(t, n.startProposalController(4))

		params, err := n.GetProposalParameters()
		require.Nil(t, err)
		require.NotEmpty(t, params)
	})

	t.Run("load error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("trie is gone")
		n := newNode(t, nil, expectedErr, nil)
		require.Equal(t, expectedErr, n.startProposalController(4))
	})

	t.Run("the loaded account is not a kapp account", func(t *testing.T) {
		t.Parallel()

		userAcnt, err := state.NewUserAccount([]byte("not a kapp"))
		require.Nil(t, err)

		n := newNode(t, userAcnt, nil, nil)
		require.Equal(t, common.ErrWrongTypeAssertion, n.startProposalController(4))
	})

	t.Run("save error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("cannot save")
		n := newNode(t, proposalKApp(t), nil, expectedErr)
		require.Equal(t, expectedErr, n.startProposalController(4))

		// the consumers were already published to, which is why the caller
		// aborts startup on this error
		params, err := n.GetProposalParameters()
		require.Nil(t, err)
		require.NotEmpty(t, params)
	})
}
