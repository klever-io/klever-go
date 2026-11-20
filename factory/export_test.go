package factory

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/smartContract/warmup"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
)

// CreateCompiledCodeWarmerForTest calls createCompiledCodeWarmer with only the components it reads
func CreateCompiledCodeWarmerForTest(
	accounts state.AccountsAdapter,
	datapool retriever.PoolsHolder,
	compiledCode warmup.CompiledCodeCache,
	gasScheduleNotifier core.GasScheduleNotifier,
	forkController core.ForkController,
	hasher hashing.Hasher,
	statusHandler core.AppStatusHandler,
) (warmup.CompiledCodeWarmer, error) {
	args := &ProcessComponentsFactoryArgs{
		state:          &StateComponents{AccountsAdapter: accounts},
		data:           &DataComponents{Datapool: datapool},
		coreData:       &CoreComponents{Hasher: hasher, StatusHandler: statusHandler},
		forkController: forkController,
	}
	return createCompiledCodeWarmer(args, compiledCode, gasScheduleNotifier, nil)
}

// SetSkPkProviderHandler updates the handler for testing reasons
func (cspf *cryptoSigningParamsLoader) SetSkPkProviderHandler(handler func() ([]byte, []byte, error)) {
	cspf.skPkProviderHandler = handler
}

// GetSkPk will call the inner function
func (cspf *cryptoSigningParamsLoader) GetSkPk() ([]byte, []byte, error) {
	return cspf.getSkPk()
}

// SetListenAddress will update the listen address for testing reasons
func (ncf *networkComponentsFactory) SetListenAddress(address string) {
	ncf.listenAddress = address
}

// LiveSCProcessor returns the block-processing SC processor for testing reasons
func (p *Process) LiveSCProcessor() process.SmartContractProcessor {
	return p.liveSCProcessor
}
