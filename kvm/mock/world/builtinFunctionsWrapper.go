package worldmock

import (
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
)

// WorldMarshalizer is the global marshalizer to be used by the components of
// the BuiltinFunctionsWrapper.
var WorldMarshalizer = &marshal.ProtoMarshalizer{}

// BuiltinFunctionsWrapper manages and initializes a BuiltInFunctionContainer
// along with its dependencies
type BuiltinFunctionsWrapper struct {
	Container       vmcommon.BuiltInFunctionContainer
	MapDNSAddresses map[string]struct{}
	World           *MockWorld
	Marshalizer     vmcommon.Marshalizer
}

// NewBuiltinFunctionsWrapper creates a new BuiltinFunctionsWrapper with
// default dependencies.
func NewBuiltinFunctionsWrapper(
	world *MockWorld,
	gasMap config.GasScheduleMap,
	forkController core.ForkController,
) (*BuiltinFunctionsWrapper, error) {
	kappController := getKAppController(world.AccountsCacher, forkController)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: []byte("sender1"),
		ContractID:     -1,
		ContractType:   -1,
		Block:          &block.Block{},
	})

	kappController.SetCurrentKAppContext(ctx)

	argsBuiltIn := builtInFunctions.ArgsCreateBuiltInFunctionContainer{
		GasMap:                           gasMap,
		Marshalizer:                      WorldMarshalizer,
		MaxNumOfAddressesForTransferRole: 100,
		AccountsCacher:                   world.AccountsCacher,
		KAppController:                   kappController,
		MapDNSAddresses:                  make(map[string]struct{}),
		ForkController:                   forkController,
	}
	builtinFuncFactory, err := builtInFunctions.NewBuiltInFunctionsCreator(argsBuiltIn)
	if err != nil {
		return nil, err
	}
	err = builtinFuncFactory.CreateBuiltInFunctionContainer()
	if err != nil {
		return nil, err
	}
	err = builtinFuncFactory.SetPayableHandler(world)
	if err != nil {
		return nil, err
	}
	builtinFuncsWrapper := &BuiltinFunctionsWrapper{
		Container:       builtinFuncFactory.BuiltInFunctionContainer(),
		MapDNSAddresses: argsBuiltIn.MapDNSAddresses,
		World:           world,
	}
	return builtinFuncsWrapper, nil
}

// ProcessBuiltInFunction delegates the execution of a real builtin function to
// the inner BuiltInFunctionContainer.
func (bf *BuiltinFunctionsWrapper) ProcessBuiltInFunction(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	function, err := bf.Container.Get(input.Function)
	if err != nil {
		return nil, err
	}

	vmOutput, err := function.ProcessBuiltinFunction(input)
	if err != nil {
		return nil, err
	}

	return vmOutput, nil
}

// GetBuiltinFunctionNames returns the list of defined builtin-in functions.
func (bf *BuiltinFunctionsWrapper) GetBuiltinFunctionNames() vmcommon.FunctionNames {
	return bf.Container.Keys()
}

func (bf *BuiltinFunctionsWrapper) getAccount(address []byte) state.UserAccountHandler {
	u, _ := bf.World.AccountsCacher.LoadUser(address)
	return u
}

func getKAppController(accCacher state.AccountsCacher, forkController core.ForkController) kapp.KAppController {
	marshalizerMock := &mock.ProtoMarshalizerMock{}

	argsKapp := kappcontroller.ArgsNewKApp{
		Hasher:         &mock.HasherStub{},
		Marshalizer:    marshalizerMock,
		PubkeyConv:     mock.NewPubkeyConverterMock(32),
		ForkController: forkController,
		AccountsCacher: accCacher,
		RatingsData:    &mock.RatingsInfoMock{},
	}

	kAppController, err := kappcontroller.NewKappController(argsKapp)
	if err != nil {
		panic(err)
	}
	err = kAppController.InitKApps(accCacher)
	if err != nil {
		panic(err)
	}

	return kAppController
}
