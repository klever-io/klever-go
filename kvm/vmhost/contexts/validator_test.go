package contexts

import (
	"strings"
	"testing"

	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost/mock"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

func testImportNames() vmcommon.FunctionNames {
	importNames := make(vmcommon.FunctionNames)
	var empty struct{}
	importNames["getArgument"] = empty
	return importNames
}

func TestFunctionsGuard_isValidFunctionName(t *testing.T) {
	builtInFuncContainer := builtInFunctions.NewBuiltInFunctionContainer()
	_ = builtInFuncContainer.Add("protocolFunctionFoo", &mock.BuiltInFunctionStub{})
	_ = builtInFuncContainer.Add("protocolFunctionBar", &mock.BuiltInFunctionStub{})

	validator := newWASMValidator(testImportNames(), builtInFuncContainer)

	require.Nil(t, validator.verifyValidFunctionName("foo"))
	require.Nil(t, validator.verifyValidFunctionName("_"))
	require.Nil(t, validator.verifyValidFunctionName("a"))
	require.Nil(t, validator.verifyValidFunctionName("i"))

	require.NotNil(t, validator.verifyValidFunctionName(""))
	require.NotNil(t, validator.verifyValidFunctionName("3"))
	require.NotNil(t, validator.verifyValidFunctionName("π"))
	require.NotNil(t, validator.verifyValidFunctionName("2foo"))
	require.NotNil(t, validator.verifyValidFunctionName("-"))
	require.NotNil(t, validator.verifyValidFunctionName("â"))
	require.NotNil(t, validator.verifyValidFunctionName("ș"))
	require.NotNil(t, validator.verifyValidFunctionName("Ä"))

	require.NotNil(t, validator.verifyValidFunctionName("protocolFunctionFoo"))
	require.NotNil(t, validator.verifyValidFunctionName("protocolFunctionBar"))

	require.Nil(t, validator.verifyValidFunctionName(strings.Repeat("_", 255)))
	require.NotNil(t, validator.verifyValidFunctionName(strings.Repeat("_", 256)))

	require.NotNil(t, validator.verifyValidFunctionName("getArgument"))
	require.Nil(t, validator.verifyValidFunctionName("getArgument55"))
}

func TestFunctionsProtected(t *testing.T) {
	host := InitializeVMAndWasmer()

	validator := newWASMValidator(testImportNames(), builtInFunctions.NewBuiltInFunctionContainer())

	world := worldmock.NewMockWorld()
	imb := contextmock.NewExecutorMock(world)
	instance := imb.CreateAndStoreInstanceMock(t, host, []byte{}, []byte{}, []byte{}, []byte{}, 0, false)

	instance.AddMockMethod("transferValueOnly", func() *contextmock.InstanceMock {
		testHost := instance.Host
		testInstance := contextmock.GetMockInstance(testHost)
		return testInstance
	})

	err := validator.verifyProtectedFunctions(instance)
	require.NotNil(t, err)
}
