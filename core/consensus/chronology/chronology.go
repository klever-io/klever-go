package chronology

import (
	"context"
	"fmt"
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/closing"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/ntp"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/display"
)

var _ consensus.ChronologyHandler = (*chronology)(nil)
var _ closing.Closer = (*chronology)(nil)

var log = logger.GetOrCreate("consensus/chronology")

const slotTimeout = 10
const chronologyAlarmID = "chronology"

// slBeforeStartSlot defines the state which exist before the start of the slot
const slBeforeStartSlot = -1

// NotSlotMiner signals that node is not current slot miner
const NotSlotMiner = ("not slot miner")

// chronology defines the data needed by the chronology
type chronology struct {
	genesisTime      time.Time
	slotManager      consensus.SlotManager
	syncTimer        ntp.SyncTimer
	appStatusHandler core.AppStatusHandler

	blk              data.ChainHandler
	nodesCoordinator sharding.NodesCoordinator

	blockProcessor     process.BlockProcessor
	broadcastMessenger consensus.BroadcastMessenger

	currentSlot int64
	cancelFunc  func()
	watchdog    core.WatchdogTimer

	subslotID       int
	subslots        map[int]int
	subslotHandlers []consensus.SubslotHandler
	mutSubslots     sync.RWMutex
}

// NewChronology creates a new chronology object
func NewChronology(
	slotManager consensus.SlotManager,
	blk data.ChainHandler,
	nodesCoordinator sharding.NodesCoordinator,
	blockProcessor process.BlockProcessor,
	broadcastMessenger consensus.BroadcastMessenger,
	syncTimer ntp.SyncTimer,
	watchdog core.WatchdogTimer,
) (*chronology, error) {

	if check.IfNil(slotManager) {
		return nil, ErrNilSlotManager
	}
	if check.IfNil(syncTimer) {
		return nil, ErrNilSyncTimer
	}
	if check.IfNil(watchdog) {
		return nil, ErrNilWatchdog
	}
	if check.IfNil(blk) {
		return nil, ErrNilBlockchain
	}
	if check.IfNil(nodesCoordinator) {
		return nil, common.ErrNilNodesCoordinator
	}
	if check.IfNil(blockProcessor) {
		return nil, common.ErrNilBlockProcessor
	}
	if check.IfNil(broadcastMessenger) {
		return nil, common.ErrNilBroadcastMessenger
	}

	chr := chronology{
		genesisTime:        slotManager.GenesisTimestamp(),
		slotManager:        slotManager,
		blk:                blk,
		nodesCoordinator:   nodesCoordinator,
		blockProcessor:     blockProcessor,
		broadcastMessenger: broadcastMessenger,
		syncTimer:          syncTimer,
		watchdog:           watchdog,
		currentSlot:        -1,

		subslotID:       slBeforeStartSlot,
		subslots:        make(map[int]int),
		subslotHandlers: make([]consensus.SubslotHandler, 0),
	}

	return &chr, nil
}

// AddSubslot adds new SubslotHandler implementation to the chronology
func (chr *chronology) AddSubslot(subslotHandler consensus.SubslotHandler) {
	chr.mutSubslots.Lock()

	chr.subslots[subslotHandler.Current()] = len(chr.subslotHandlers)
	chr.subslotHandlers = append(chr.subslotHandlers, subslotHandler)

	chr.mutSubslots.Unlock()
}

// RemoveAllSubslots removes all the SubslotHandler implementations added to the chronology
func (chr *chronology) RemoveAllSubslots() {
	chr.mutSubslots.Lock()

	chr.subslots = make(map[int]int)
	chr.subslotHandlers = make([]consensus.SubslotHandler, 0)

	chr.mutSubslots.Unlock()
}

// StartSlots starts the chronology and calls the DoWork() method of the slotHandlers loaded
func (chr *chronology) StartSlots() {
	watchdogAlarmDuration := chr.slotManager.TimeDuration() * slotTimeout
	chr.watchdog.SetDefault(watchdogAlarmDuration, chronologyAlarmID)

	var ctx context.Context
	ctx, chr.cancelFunc = context.WithCancel(context.Background())
	go chr.startSlots(ctx)
}

func (chr *chronology) startSlots(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Debug("chronology's go routine is stopping...")
			return
		case <-time.After(time.Millisecond):
		}

		chr.startSlot()
	}
}

// startSlot calls the current slot, given by the finished tasks
func (chr *chronology) startSlot() {
	if chr.subslotID == slBeforeStartSlot {
		chr.updateSlot()
	}

	if chr.slotManager.BeforeGenesis() {
		return
	}

	ss := chr.loadSubslotHandler(chr.subslotID)
	if ss == nil {
		return
	}

	msg := fmt.Sprintf("SUBSLOT %s BEGINS", ss.Name())
	log.Debug(display.Headline(msg, chr.syncTimer.FormattedCurrentTime(), "."))
	logger.SetCorrelationSubSlot(ss.Name())

	if !ss.DoWork(chr.slotManager) {
		chr.subslotID = slBeforeStartSlot
		return
	}

	chr.subslotID = ss.Next()
}

// updateSlot updates slots depending of the current time
func (chr *chronology) updateSlot() {
	oldSlotIndex := chr.slotManager.Index()
	chr.slotManager.UpdateSlot(chr.syncTimer.CurrentTime())

	if oldSlotIndex != chr.slotManager.Index() {
		chr.watchdog.Reset(chronologyAlarmID)
		msg := fmt.Sprintf("SLOT %d BEGINS (%d)", chr.slotManager.Index(), chr.slotManager.Timestamp().Unix())
		log.Info(display.Headline(msg, chr.syncTimer.FormattedCurrentTime(), "#"))
		logger.SetCorrelationSlot(chr.slotManager.Index())

		chr.initSlot()
	}
}

// initSlot is called when a new slot begins and it does the necessary initialization
func (chr *chronology) initSlot() {
	chr.subslotID = slBeforeStartSlot

	chr.mutSubslots.RLock()

	hasSubslotsAndGenesisTimePassed := !chr.slotManager.BeforeGenesis() && len(chr.subslotHandlers) > 0

	if hasSubslotsAndGenesisTimePassed {
		chr.subslotID = chr.subslotHandlers[0].Current()
		chr.appStatusHandler.SetUInt64Value(core.MetricCurrentSlot, tools.SafeI64ToU64(chr.slotManager.Index()))
		chr.appStatusHandler.SetUInt64Value(core.MetricCurrentSlotTimestamp, tools.SafeI64ToU64(chr.slotManager.Timestamp().Unix()))
	}

	chr.mutSubslots.RUnlock()
}

// loadSubslotHandler returns the implementation of SubslotHandler given by the subslotID
func (chr *chronology) loadSubslotHandler(subslotID int) consensus.SubslotHandler {

	chr.mutSubslots.RLock()
	defer chr.mutSubslots.RUnlock()

	index, exist := chr.subslots[subslotID]

	if !exist {
		return nil
	}

	indexIsOutOfBounds := index < 0 || index >= len(chr.subslotHandlers)

	if indexIsOutOfBounds {
		return nil
	}

	return chr.subslotHandlers[index]
}

// Close will close the endless running go routine
func (chr *chronology) Close() error {
	if chr.cancelFunc != nil {
		chr.cancelFunc()
	}

	chr.watchdog.Stop(chronologyAlarmID)

	return nil
}

// SetAppStatusHandler will set the AppStatusHandler which will be used for monitoring
func (chr *chronology) SetAppStatusHandler(ash core.AppStatusHandler) error {
	if ash == nil || ash.IsInterfaceNil() {
		return ErrNilAppStatusHandler
	}

	chr.appStatusHandler = ash
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (chr *chronology) IsInterfaceNil() bool {
	return chr == nil
}
