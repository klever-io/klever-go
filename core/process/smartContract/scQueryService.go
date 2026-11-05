package smartContract

import (
	"bytes"
	"math"
	"math/big"
	"time"

	"github.com/klever-io/klever-go/common/types"
	"github.com/klever-io/klever-go/core"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/kapp/disabled"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data"
	vmData "github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

var _ process.SCQueryService = (*SCQueryService)(nil)

const (
	UnlimitedGasValue = uint64(math.MaxInt64)

	// DefaultMaxQueryWait is how long a query waits for its query VM when
	// VirtualMachine.Querying.maxQueryWaitInMilliseconds is unset
	DefaultMaxQueryWait = time.Second
)

// SCQueryService can execute Get functions over SC to fetch stored values
type SCQueryService struct {
	vmContainer              process.VirtualMachinesContainer
	economicsFee             process.EconomicsDataHandler
	runSlot                  chan struct{}
	maxQueryWait             time.Duration
	blockChainHook           process.BlockChainHookHandler
	blockChain               data.ChainHandler
	getNodeState             func() core.NodeState
	numQueries               int
	gasForQuery              uint64
	wasmVMChangeLocker       common.Locker
	allowExternalQueriesChan chan struct{}
}

// ArgsNewSCQueryService defines the arguments needed for the sc query service
type ArgsNewSCQueryService struct {
	VmContainer              process.VirtualMachinesContainer
	EconomicsFee             process.EconomicsDataHandler
	BlockChainHook           process.BlockChainHookHandler
	BlockChain               data.ChainHandler
	GetNodeState             func() core.NodeState
	WasmVMChangeLocker       common.Locker
	AllowExternalQueriesChan chan struct{}
	MaxGasLimitPerQuery      uint64
	// MaxQueryWait bounds how long a query waits for the query VM before it is
	// refused with process.ErrQueryServiceBusy. 0 means DefaultMaxQueryWait.
	MaxQueryWait time.Duration
}

// NewSCQueryService returns a new instance of SCQueryService
func NewSCQueryService(
	args ArgsNewSCQueryService,
) (*SCQueryService, error) {
	if check.IfNil(args.VmContainer) {
		return nil, process.ErrNoVM
	}
	if check.IfNil(args.EconomicsFee) {
		return nil, process.ErrNilEconomicsFeeHandler
	}
	if check.IfNil(args.BlockChainHook) {
		return nil, process.ErrNilBlockChainHook
	}
	if check.IfNil(args.BlockChain) {
		return nil, process.ErrNilBlockChain
	}
	if check.IfNilReflect(args.WasmVMChangeLocker) {
		return nil, process.ErrNilLocker
	}
	if args.AllowExternalQueriesChan == nil {
		return nil, process.ErrNilAllowExternalQueriesChan
	}
	if args.GetNodeState == nil {
		return nil, process.ErrNilBootstrapper
	}

	gasForQuery := validateAndGetQueryGas(args.MaxGasLimitPerQuery)

	maxQueryWait := args.MaxQueryWait
	if maxQueryWait <= 0 {
		maxQueryWait = DefaultMaxQueryWait
	}

	return &SCQueryService{
		vmContainer:              args.VmContainer,
		economicsFee:             args.EconomicsFee,
		runSlot:                  make(chan struct{}, 1),
		maxQueryWait:             maxQueryWait,
		blockChain:               args.BlockChain,
		blockChainHook:           args.BlockChainHook,
		wasmVMChangeLocker:       args.WasmVMChangeLocker,
		gasForQuery:              gasForQuery,
		allowExternalQueriesChan: args.AllowExternalQueriesChan,
		getNodeState:             args.GetNodeState,
	}, nil
}

func validateAndGetQueryGas(configuredGas uint64) uint64 {
	if configuredGas == 0 {
		log.Warn("VM query running with unlimited gas limit",
			"configured_value", "0",
			"effective_limit", UnlimitedGasValue,
			"security_note", "Node is configured to allow unlimited gas for VM queries. This may increase vulnerability to DOS attacks",
			"config_key", "metaMaxGasPerVmQuery")
		return UnlimitedGasValue
	}

	return configuredGas
}

// ExecuteQuery returns the VMOutput resulted upon running the function on the smart contract
func (service *SCQueryService) ExecuteQuery(query *process.SCQuery) (*vmcommon.VMOutput, error) {
	if !service.shouldAllowQueriesExecution() {
		return nil, process.ErrQueriesNotAllowedYet
	}

	if query.ScAddress == nil {
		return nil, process.ErrNilScAddress
	}
	if len(query.FuncName) == 0 {
		return nil, process.ErrEmptyFunctionName
	}

	if !service.acquireRunSlot() {
		return nil, process.ErrQueryServiceBusy
	}
	defer service.releaseRunSlot()

	return service.executeScCall(query)
}

// acquireRunSlot waits at most maxQueryWait for the query VM, so queries
// queued behind a long-running one are refused instead of piling up
func (service *SCQueryService) acquireRunSlot() bool {
	select {
	case service.runSlot <- struct{}{}:
		return true
	default:
	}

	timer := time.NewTimer(service.maxQueryWait)
	defer timer.Stop()

	select {
	case service.runSlot <- struct{}{}:
		return true
	case <-timer.C:
		return false
	}
}

func (service *SCQueryService) releaseRunSlot() {
	<-service.runSlot
}

func (service *SCQueryService) shouldAllowQueriesExecution() bool {
	select {
	case <-service.allowExternalQueriesChan:
		return true
	default:
		return false
	}
}

func (service *SCQueryService) executeScCall(query *process.SCQuery) (*vmcommon.VMOutput, error) {
	log.Trace("executeScCall", "function", query.FuncName, "numQueries", service.numQueries)
	service.numQueries++

	shouldEarlyExitBecauseOfSyncState := query.ShouldBeSynced &&
		service.getNodeState() == core.NsNotSynchronized
	if shouldEarlyExitBecauseOfSyncState {
		return nil, process.ErrNodeIsNotSynced
	}

	shouldCheckRootHashChanges := query.SameScState
	rootHashBeforeExecution := make([]byte, 0)

	if shouldCheckRootHashChanges {
		rootHashBeforeExecution = service.blockChain.GetCurrentBlockRootHash()
	}

	service.blockChainHook.SetCurrentHeader(service.blockChain.GetCurrentBlockHeader())

	// A KApp context is a per-request object everywhere else in the tree (see
	// txProcess.go, which builds a fresh one after every tx), but the query
	// controller is built once per query element at boot and lives for the whole
	// process. Its receipt slice is append-only, so without this reset every
	// query permanently leaks its receipts and each built-in dispatch pays to
	// copy the whole accumulated slice. Only reset for read-only controllers so
	// a service wired to the production controller is never clobbered.
	kappController := service.blockChainHook.GetKAppController()
	if !check.IfNil(kappController) && kappController.IsReadOnly() {
		kappController.SetCurrentKAppContext(disabled.NewDisabledKappContext())
	}

	service.wasmVMChangeLocker.RLock()
	vm, err := FindVMByScAddress(service.vmContainer, query.ScAddress)
	if err != nil {
		service.wasmVMChangeLocker.RUnlock()
		return nil, err
	}

	query = prepareScQuery(query)
	vmInput, err := service.createVMCallInput(query)
	if err != nil {
		service.wasmVMChangeLocker.RUnlock()
		return nil, err
	}

	vmOutput, err := vm.RunSmartContractCall(vmInput)
	service.wasmVMChangeLocker.RUnlock()
	if err != nil {
		return nil, err
	}

	if query.SameScState {
		err = service.checkForRootHashChanges(rootHashBeforeExecution)
		if err != nil {
			return nil, err
		}
	}

	return vmOutput, nil
}

func (service *SCQueryService) checkForRootHashChanges(rootHashBefore []byte) error {
	rootHashAfter := service.blockChain.GetCurrentBlockRootHash()

	if bytes.Equal(rootHashBefore, rootHashAfter) {
		return nil
	}

	return process.ErrStateChangedWhileExecutingVmQuery
}

func prepareScQuery(query *process.SCQuery) *process.SCQuery {
	if query.CallerAddr == nil {
		query.CallerAddr = query.ScAddress
	}
	if query.CallValue == nil {
		query.CallValue = make(map[string]int64)
	}

	return query
}

func (service *SCQueryService) createVMCallInput(query *process.SCQuery) (*vmcommon.ContractCallInput, error) {
	vmInput := vmcommon.VMInput{
		CallerAddr:   query.CallerAddr,
		GasProvided:  service.gasForQuery,
		Arguments:    query.Arguments,
		CallType:     vmData.DirectCall,
		KDATransfers: make([]*vmcommon.KDATransfer, 0),
	}

	// load other tokens calls
	dm := types.NewDeterministicMap(query.CallValue)
	err := dm.Each(func(token string, amount int64) error {
		tokenID, nonce, _, err := kdautils.ExtractAssetIDAndNonce([]byte(token))
		if err != nil {
			log.Error("Error extracting asset ID and nonce", "error", err)
			return err
		}
		vmInput.KDATransfers = append(vmInput.KDATransfers, &vmcommon.KDATransfer{
			KDAValue:      big.NewInt(amount),
			KDATokenName:  tokenID,
			KDATokenNonce: nonce,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	vmContractCallInput := &vmcommon.ContractCallInput{
		RecipientAddr: query.ScAddress,
		Function:      query.FuncName,
		VMInput:       vmInput,
	}

	return vmContractCallInput, nil
}

// Close closes all underlying components
func (service *SCQueryService) Close() error {
	return service.vmContainer.Close()
}

// IsInterfaceNil returns true if there is no value under the interface
func (service *SCQueryService) IsInterfaceNil() bool {
	return service == nil
}
