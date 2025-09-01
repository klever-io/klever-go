package slot

import (
	"strconv"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/tracing"
)

var _ consensus.SubslotHandler = (*Subslot)(nil)

const lastSubslotID = -1

// Subslot struct contains the needed data for one Subslot and the Subslot properties. It defines a Subslot
// with it's properties (it's ID, next Subslot ID, it's duration, it's name) and also it has some handler functions
// which should be set. Job function will be the main function of this Subslot, Extend function will handle the overtime
// situation of the Subslot and Check function will decide if in this Subslot the consensus is achieved
type Subslot struct {
	ConsensusCoreHandler
	*ConsensusState

	previous   int
	current    int
	next       int
	startTime  int64
	endTime    int64
	name       string
	chainID    []byte
	currentPid core.PeerID

	consensusStateChangedChannel chan bool
	executeStoredMessages        func()
	appStatusHandler             core.AppStatusHandler

	Job    func() bool         // method does the Subslot Job and send the result to the peers
	Check  func() bool         // method checks if the consensus of the Subslot is done
	Extend func(subslotId int) // method is called when slot time is out
}

// NewSubslot creates a new SubslotId object
func NewSubslot(
	previous int,
	current int,
	next int,
	startTime int64,
	endTime int64,
	name string,
	consensusState *ConsensusState,
	consensusStateChangedChannel chan bool,
	executeStoredMessages func(),
	container ConsensusCoreHandler,
	chainID []byte,
	currentPid core.PeerID,
) (*Subslot, error) {
	err := checkNewSubslotParams(
		consensusState,
		consensusStateChangedChannel,
		executeStoredMessages,
		container,
		chainID,
	)
	if err != nil {
		return nil, err
	}

	sr := Subslot{
		ConsensusCoreHandler:         container,
		ConsensusState:               consensusState,
		previous:                     previous,
		current:                      current,
		next:                         next,
		startTime:                    startTime,
		endTime:                      endTime,
		name:                         name,
		chainID:                      chainID,
		consensusStateChangedChannel: consensusStateChangedChannel,
		executeStoredMessages:        executeStoredMessages,
		Job:                          nil,
		Check:                        nil,
		Extend:                       nil,
		appStatusHandler:             statusHandler.NewNilStatusHandler(),
		currentPid:                   currentPid,
	}

	return &sr, nil
}

func checkNewSubslotParams(
	state *ConsensusState,
	consensusStateChangedChannel chan bool,
	executeStoredMessages func(),
	container ConsensusCoreHandler,
	chainID []byte,
) error {
	err := ValidateConsensusCore(container)
	if err != nil {
		return err
	}
	if consensusStateChangedChannel == nil {
		return ErrNilChannel
	}
	if state == nil {
		return ErrNilConsensusState
	}
	if executeStoredMessages == nil {
		return ErrNilExecuteStoredMessages
	}
	if len(chainID) == 0 {
		return ErrInvalidChainID
	}

	return nil
}

// DoWork method actually does the work of this Subslot. First it tries to do the Job of the Subslot then it will
// Check the consensus. If the upper time limit of this Subslot is reached, the Extend method will be called before
// returning. If this method returns true the chronology will advance to the next Subslot.
func (sr *Subslot) DoWork(slotManager consensus.SlotManager) bool {
	if sr.Job == nil || sr.Check == nil {
		return false
	}

	// Stop slot-level tracing when the last subslot (END_SLOT) completes
	// The last subslot is identified by having no next subslot (next == -1)
	if sr.next == lastSubslotID {
		defer func() {
			tracing.StopSpan("consensus.slot." + strconv.FormatInt(slotManager.Index(), 10))
		}()
	}

	// Start subslot tracing
	defer tracing.StartSpan(
		"consensus.subslot."+sr.name,
		"subslot.name", sr.name,
		"slot.index", strconv.FormatInt(slotManager.Index(), 10),
	)()

	// execute stored messages which were received in this new slot but before this initialization
	go sr.executeStoredMessages()

	startTime := slotManager.Timestamp()
	maxTime := slotManager.TimeDuration() * MaxThresholdPercent / 100

	sr.Job()
	if sr.Check() {
		return true
	}

	timer := time.NewTimer(slotManager.RemainingTime(startTime, maxTime))
	defer timer.Stop()
	for {
		select {
		case <-sr.consensusStateChangedChannel:
			if sr.Check() {
				return true
			}
		case <-timer.C:
			if sr.Extend != nil {
				sr.SlotCanceled = true
				sr.Extend(sr.current)
			}

			return false
		}
	}
}

// Previous method returns the ID of the previous Subslot
func (sr *Subslot) Previous() int {
	return sr.previous
}

// Current method returns the ID of the current Subslot
func (sr *Subslot) Current() int {
	return sr.current
}

// Next method returns the ID of the next Subslot
func (sr *Subslot) Next() int {
	return sr.next
}

// StartTime method returns the start time of the Subslot
func (sr *Subslot) StartTime() int64 {
	return sr.startTime
}

// EndTime method returns the upper time limit of the Subslot
func (sr *Subslot) EndTime() int64 {
	return sr.endTime
}

// Name method returns the name of the Subslot
func (sr *Subslot) Name() string {
	return sr.name
}

// ChainID method returns the current chain ID
func (sr *Subslot) ChainID() []byte {
	return sr.chainID
}

// CurrentPid returns the current p2p peer ID
func (sr *Subslot) CurrentPid() core.PeerID {
	return sr.currentPid
}

// SetAppStatusHandler method sets appStatusHandler
func (sr *Subslot) SetAppStatusHandler(ash core.AppStatusHandler) error {
	if check.IfNil(ash) {
		return ErrNilAppStatusHandler
	}
	sr.appStatusHandler = ash

	return nil
}

// AppStatusHandler method returns the appStatusHandler instance
func (sr *Subslot) AppStatusHandler() core.AppStatusHandler {
	return sr.appStatusHandler
}

// ConsensusChannel method returns the consensus channel
func (sr *Subslot) ConsensusChannel() chan bool {
	return sr.consensusStateChangedChannel
}

// IsInterfaceNil returns true if there is no value under the interface
func (sr *Subslot) IsInterfaceNil() bool {
	return sr == nil
}
