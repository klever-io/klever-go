package smartContract

import (
	"bytes"
	"math"
	"math/big"
	"sync"

	"github.com/klever-io/klever-go/common/types"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data"
	vmData "github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

var _ process.SCQueryService = (*SCQueryService)(nil)

// SCQueryService can execute Get functions over SC to fetch stored values
type SCQueryService struct {
	vmContainer              process.VirtualMachinesContainer
	economicsFee             process.EconomicsDataHandler
	mutRunSc                 sync.Mutex
	blockChainHook           process.BlockChainHookHandler
	blockChain               data.ChainHandler
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
	WasmVMChangeLocker       common.Locker
	AllowExternalQueriesChan chan struct{}
	MaxGasLimitPerQuery      uint64
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

	gasForQuery := uint64(math.MaxInt64)
	if args.MaxGasLimitPerQuery > 0 {
		gasForQuery = args.MaxGasLimitPerQuery
	}
	return &SCQueryService{
		vmContainer:              args.VmContainer,
		economicsFee:             args.EconomicsFee,
		blockChain:               args.BlockChain,
		blockChainHook:           args.BlockChainHook,
		wasmVMChangeLocker:       args.WasmVMChangeLocker,
		gasForQuery:              gasForQuery,
		allowExternalQueriesChan: args.AllowExternalQueriesChan,
	}, nil
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

	service.mutRunSc.Lock()
	defer service.mutRunSc.Unlock()

	return service.executeScCall(query)
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

	shouldEarlyExitBecauseOfSyncState := query.ShouldBeSynced
	if shouldEarlyExitBecauseOfSyncState {
		return nil, process.ErrNodeIsNotSynced
	}

	shouldCheckRootHashChanges := query.SameScState
	rootHashBeforeExecution := make([]byte, 0)

	if shouldCheckRootHashChanges {
		rootHashBeforeExecution = service.blockChain.GetCurrentBlockHeaderHash()
	}

	service.blockChainHook.SetCurrentHeader(service.blockChain.GetCurrentBlockHeader())

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

	if service.hasRetriableExecutionError(vmOutput) {
		log.Error("Retriable execution error detected. Will retry (once) executeScCall()", "returnCode", vmOutput.ReturnCode, "returnMessage", vmOutput.ReturnMessage)

		vmOutput, err = vm.RunSmartContractCall(vmInput)
		if err != nil {
			return nil, err
		}
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
	rootHashAfter := service.blockChain.GetCurrentBlockHeaderHash()

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

func (service *SCQueryService) hasRetriableExecutionError(vmOutput *vmcommon.VMOutput) bool {
	return vmOutput.ReturnMessage == "allocation error"
}

// Close closes all underlying components
func (service *SCQueryService) Close() error {
	return service.vmContainer.Close()
}

// IsInterfaceNil returns true if there is no value under the interface
func (service *SCQueryService) IsInterfaceNil() bool {
	return service == nil
}
