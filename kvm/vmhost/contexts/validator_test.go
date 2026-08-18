package contexts

import (
	"math"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost"
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
	_ = builtInFuncContainer.Add("protocolFunctionFoo", &contextmock.BuiltInFunctionStub{})
	_ = builtInFuncContainer.Add("protocolFunctionBar", &contextmock.BuiltInFunctionStub{})

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

func TestTableDeclaration_verifyTableDeclaration(t *testing.T) {
	validator := newWASMValidator(testImportNames(), builtInFunctions.NewBuiltInFunctionContainer())
	instance := contextmock.NewInstanceMock([]byte{})

	instance.MaxDeclaredTableSizeMock = 500
	require.NoError(t, validator.verifyTableDeclaration(instance, 500))
	require.NoError(t, validator.verifyTableDeclaration(instance, 1000))

	instance.MaxDeclaredTableSizeMock = 501
	require.ErrorIs(t, validator.verifyTableDeclaration(instance, 500), vmhost.ErrDeclaredTableSizeExceedsMaximum)

	instance.MaxDeclaredTableSizeMock = math.MaxUint32 // no declared maximum, as reported by Wasmer
	require.ErrorIs(t, validator.verifyTableDeclaration(instance, 10000), vmhost.ErrDeclaredTableSizeExceedsMaximum)

	// A configured cap of exactly math.MaxUint32 collides numerically with the
	// "no declared maximum" sentinel; a plain > comparison would have made this
	// specific cap value a silent no-op for the case it most needs to catch.
	require.ErrorIs(t, validator.verifyTableDeclaration(instance, math.MaxUint32), vmhost.ErrDeclaredTableSizeExceedsMaximum)
}
