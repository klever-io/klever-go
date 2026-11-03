package vmhooks_test

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	goruntime "runtime"
	"testing"
	"time"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/contexts"
	hostmock "github.com/klever-io/klever-go/kvm/vmhost/mock"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVMHooksImpl_GetGasLeft(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		initialGas    uint64
		expectedGas   int64
		shouldFailExe bool
	}{
		{
			name:        "normal gas remaining",
			initialGas:  1000,
			expectedGas: 999,
		},
		{
			name:        "zero gas remaining",
			initialGas:  0,
			expectedGas: 0,
		},
		{
			name:        "large gas value",
			initialGas:  math.MaxUint64,
			expectedGas: math.MaxInt64, // max value int64
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			host.MeteringContext.(*contextmock.MeteringContextMock).GasLeftMock = tt.initialGas
			hooks := vmhooks.NewVMHooksImpl(host)

			result := hooks.GetGasLeft()
			assert.Equal(t, tt.expectedGas, result)
		})
	}
}

func TestVMHooksImpl_GetSCAddress(t *testing.T) {
	t.Parallel()

	host := newMockVMHost()
	hooks := vmhooks.NewVMHooksImpl(host)

	testAddress := []byte("testcontractaddress..")
	host.RuntimeContext.(*contextmock.RuntimeContextMock).SCAddress = testAddress

	hooks.GetSCAddress(executor.MemPtr(0))

	result, err := host.MemLoadFromMock(executor.MemPtr(0), len(testAddress))
	require.Nil(t, err)
	assert.Equal(t, testAddress, result)
	assert.Equal(t, uint64(999), host.MeteringContext.GasLeft())
}

func TestVMHooksImpl_GetOwnerAddress(t *testing.T) {
	t.Parallel()

	host := newMockVMHost()
	hooks := vmhooks.NewVMHooksImpl(host)

	testAddress := []byte("owneraddresstest123456")
	host.BlockchainContext.(*hostmock.BlockchainContextStub).GetOwnerAddressCalled = func() ([]byte, error) {
		return testAddress, nil
	}

	hooks.GetOwnerAddress(executor.MemPtr(0))

	result, err := host.MemLoadFromMock(executor.MemPtr(0), len(testAddress))
	require.Nil(t, err)
	assert.Equal(t, testAddress, result)
	assert.Equal(t, uint64(999), host.MeteringContext.GasLeft())
}

func TestVMHooksImpl_IsSmartContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		address        []byte
		isSmartCont    bool
		expectedValue  int32
		shouldFailExec bool
	}{
		{
			name:          "is smart contract",
			address:       []byte("smartcontractaddr123"),
			isSmartCont:   true,
			expectedValue: 1,
		},
		{
			name:          "not smart contract",
			address:       []byte("regularaddress12345"),
			isSmartCont:   false,
			expectedValue: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)

			host.BlockchainContext.(*hostmock.BlockchainContextStub).IsSmartContractCalled = func(address []byte) bool {
				return tt.isSmartCont
			}

			err := host.MemStoreToMock(executor.MemPtr(0), tt.address)
			require.Nil(t, err)

			result := hooks.IsSmartContract(executor.MemPtr(0))
			assert.Equal(t, tt.expectedValue, result)
		})
	}
}

func TestVMHooksImpl_SignalError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		message        []byte
		expectedError  error
		shouldFailExec bool
	}{
		{
			name:           "normal error message",
			message:        []byte("error occurred"),
			expectedError:  nil,
			shouldFailExec: false,
		},
		{
			name:           "empty message",
			message:        []byte{},
			expectedError:  nil,
			shouldFailExec: false,
		},
		{
			name:           "very long message",
			message:        make([]byte, 1000000),
			expectedError:  vmhost.ErrNotEnoughGas,
			shouldFailExec: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)

			_ = host.MemStoreToMock(executor.MemPtr(0), tt.message)
			host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBaseOpsAPI = tt.shouldFailExec

			hooks.SignalError(executor.MemPtr(0), executor.MemLength(len(tt.message)))

			if tt.shouldFailExec {
				assert.Empty(t, host.RuntimeContext.(*contextmock.RuntimeContextMock).SignalErrorMessage)
				assert.Equal(t, tt.expectedError, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
			} else {
				assert.Equal(t, string(tt.message), host.RuntimeContext.(*contextmock.RuntimeContextMock).SignalErrorMessage)
			}
		})
	}
}

func makeAddress(addr string) []byte {
	result := make([]byte, vmhost.AddressLen)
	copy(result, addr)
	return result
}

func TestVMHooksImpl_GetExternalBalance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		address        []byte
		balance        *big.Int
		expectedError  error
		shouldFailExec bool
	}{
		{
			name:    "normal balance",
			address: makeAddress("address123456789012"),
			balance: big.NewInt(100),
		},
		{
			name:    "zero balance",
			address: makeAddress("address123456789012"),
			balance: big.NewInt(0),
		},
		{
			name:           "invalid address",
			address:        []byte("invalid"),
			balance:        big.NewInt(100),
			shouldFailExec: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)

			host.BlockchainContext.(*hostmock.BlockchainContextStub).GetBalanceCalled = func(address []byte) []byte {
				if bytes.Equal(address, tt.address) {
					return tt.balance.Bytes()
				}

				return nil
			}

			err := host.MemStoreToMock(executor.MemPtr(0), tt.address)
			require.Nil(t, err)

			hooks.GetExternalBalance(executor.MemPtr(0), executor.MemPtr(32))

			if !tt.shouldFailExec {
				// TODO: temporary load size from expected balance
				// must fix this with return of function
				expectedBytes := tt.balance.Bytes()
				result, err := host.MemLoadFromMock(executor.MemPtr(32), len(expectedBytes))
				require.Nil(t, err)

				assert.Equal(t, expectedBytes, result)
			} else {
				// TODO: length should be 0
				result, err := host.MemLoadFromMock(executor.MemPtr(32), 1)
				require.Nil(t, err)
				assert.Equal(t, []byte{0}, result)
			}
		})
	}
}

func TestVMHooksImpl_GetBlockHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		nonce          int64
		hash           []byte
		expectedError  error
		shouldFailExec bool
	}{
		{
			name:  "valid block hash",
			nonce: 100,
			hash:  []byte("blockhash123456789012"),
		},
		{
			name:           "invalid nonce",
			nonce:          -1,
			shouldFailExec: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)

			host.BlockchainContext.(*hostmock.BlockchainContextStub).BlockHashCalled = func(u uint64) []byte {
				if u == uint64(tt.nonce) {
					return tt.hash
				}
				return nil
			}

			result := hooks.GetBlockHash(tt.nonce, executor.MemPtr(0))

			if !tt.shouldFailExec {
				assert.Equal(t, int32(0), result)

				hashResult, err := host.MemLoadFromMock(executor.MemPtr(0), len(tt.hash))
				require.Nil(t, err)
				assert.Equal(t, tt.hash, hashResult)
			} else {
				assert.Equal(t, int32(0), result)
			}
		})
	}
}

func TestVMHooksImpl_WriteLog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		data           []byte
		topics         [][]byte
		numTopics      int32
		fixAuditV4     bool
		meteringErr    error
		shouldFailExec bool
		expectedError  error
	}{
		{
			name:      "valid log",
			data:      []byte("log message"),
			topics:    [][]byte{makeAddress("topic1"), makeAddress("topic2")},
			numTopics: 2,
		},
		{
			name:      "log without topics",
			data:      []byte("log message"),
			topics:    [][]byte{},
			numTopics: 0,
		},
		{
			name:           "invalid number of topics",
			data:           []byte("log message"),
			numTopics:      -1,
			shouldFailExec: true,
			expectedError:  vmhost.ErrNegativeLength,
		},
		{
			name:       "valid log with audit fix",
			data:       []byte("log message"),
			topics:     [][]byte{makeAddress("topic1"), makeAddress("topic2")},
			numTopics:  2,
			fixAuditV4: true,
		},
		{
			name:           "negative topics with audit fix",
			data:           []byte("log message"),
			numTopics:      -1,
			fixAuditV4:     true,
			shouldFailExec: true,
			expectedError:  vmhost.ErrNegativeLength,
		},
		{
			name:           "insufficient gas for topics with audit fix",
			data:           []byte("log message"),
			numTopics:      1,
			fixAuditV4:     true,
			meteringErr:    vmhost.ErrNotEnoughGas,
			shouldFailExec: true,
			expectedError:  vmhost.ErrNotEnoughGas,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			host.ForkControllerContext = &commonMock.ForkControllerStub{FixAuditChangesV4Value: tt.fixAuditV4}
			if tt.meteringErr != nil {
				host.MeteringContext.(*contextmock.MeteringContextMock).Err = tt.meteringErr
			}
			hooks := vmhooks.NewVMHooksImpl(host)
			host.OutputContext = &contextmock.OutputContextStub{
				WriteLogCalled: func(address []byte, topics [][]byte, data [][]byte) {
					assert.Equal(t, tt.data, data[0])
					assert.Equal(t, tt.topics, topics)
				},
			}
			host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBaseOpsAPI = tt.shouldFailExec

			err := host.MemStoreToMock(executor.MemPtr(0), tt.data)
			require.Nil(t, err)

			topicPtr := executor.MemPtr(100)
			for i, topic := range tt.topics {
				err = host.MemStoreToMock(topicPtr.Offset(int32(i)*vmhost.HashLen), topic)
				require.Nil(t, err)
			}

			hooks.WriteLog(
				executor.MemPtr(0),
				executor.MemLength(len(tt.data)),
				topicPtr,
				tt.numTopics,
			)

			assert.Equal(t, tt.expectedError, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
		})
	}
}

// UseGasBoundedAndAddTracedGas used to record the charge twice in the diagnostic gas trace: once
// into whichever trace was current (the previous hook's) and once as a fresh writeLog entry, while
// the meter itself only deducted once. A real meteringContext is used so both the trace and the
// deduction are observed end to end through the hook.
func TestVMHooksImpl_WriteLog_TracesGasOnce(t *testing.T) {
	t.Parallel()

	const gasProvided = uint64(1000)
	scAddress := []byte("scAddress1")

	host := newMockVMHost()
	host.ForkControllerContext = &commonMock.ForkControllerStub{FixAuditChangesV4Value: true}
	host.OutputContext = &contextmock.OutputContextStub{}
	host.RuntimeContext.(*contextmock.RuntimeContextMock).SCAddress = scAddress

	metering, err := contexts.NewMeteringContext(host, config.MakeGasMapForTests())
	require.NoError(t, err)
	metering.InitStateFromContractCallInput(&vmcommon.VMInput{GasProvided: gasProvided})
	metering.SetGasTracing(true)
	host.MeteringContext = metering

	// the trace the leaked copy used to land in
	metering.StartGasTracing("previousHook")

	data := []byte("log message")
	topics := [][]byte{makeAddress("topic1"), makeAddress("topic2")}
	require.NoError(t, host.MemStoreToMock(executor.MemPtr(0), data))
	topicPtr := executor.MemPtr(100)
	for i, topic := range topics {
		require.NoError(t, host.MemStoreToMock(topicPtr.Offset(int32(i)*vmhost.HashLen), topic))
	}

	schedule := metering.GasSchedule()
	expectedGas := schedule.BaseOpsAPICost.Log +
		schedule.BaseOperationCost.PersistPerByte*uint64(len(topics)*vmhost.HashLen+len(data))

	vmhooks.NewVMHooksImpl(host).WriteLog(executor.MemPtr(0), executor.MemLength(len(data)), topicPtr, int32(len(topics)))

	require.Nil(t, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
	// the meter deducts the charge exactly once
	assert.Equal(t, gasProvided-expectedGas, metering.GasLeft())
	// the trace attributes it exactly once, under writeLog, leaving the previous trace untouched
	expectedTrace := map[string]map[string][]uint64{string(scAddress): {
		"previousHook": {0},
		"writeLog":     {expectedGas},
	}}
	assert.Equal(t, expectedTrace, metering.GetGasTrace())
}

// From FixAuditChangesV6 WriteEventLog charges in four places: the lengths array and the topics inside
// getArgumentsFromMemory, the data in memLoadCharged, then the Log base cost. The copy charges go into
// the trace the hook already opened, so the whole call must still show up as a single writeEventLog
// entry: a helper that started its own trace would split it, one that did not trace would lose gas.
func TestVMHooksImpl_WriteEventLog_TracesGasOnce(t *testing.T) {
	t.Parallel()

	const gasProvided = uint64(1000)
	scAddress := []byte("scAddress1")

	host, metering := newMockVMHostWithRealMetering(t, true, config.MakeGasMapForTests(), gasProvided)
	host.RuntimeContext.(*contextmock.RuntimeContextMock).SCAddress = scAddress
	metering.SetGasTracing(true)

	metering.StartGasTracing("previousHook")

	topics := [][]byte{[]byte("topic1"), []byte("longerTopic2")}
	data := []byte("event data")
	topicLengths := make([]byte, 4*len(topics))
	for i, topic := range topics {
		topicLengths[i*4] = byte(len(topic))
	}
	require.NoError(t, host.MemStoreToMock(executor.MemPtr(0), topicLengths))
	require.NoError(t, host.MemStoreToMock(executor.MemPtr(100), bytes.Join(topics, nil)))
	require.NoError(t, host.MemStoreToMock(executor.MemPtr(200), data))

	schedule := metering.GasSchedule()
	copiedBytes := uint64(len(topicLengths) + len(topics[0]) + len(topics[1]) + len(data))
	expectedGas := schedule.BaseOpsAPICost.Log + schedule.BaseOperationCost.DataCopyPerByte*copiedBytes

	vmhooks.NewVMHooksImpl(host).WriteEventLog(
		int32(len(topics)), executor.MemPtr(0), executor.MemPtr(100), executor.MemPtr(200), executor.MemLength(len(data)))

	require.Nil(t, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
	assert.Equal(t, gasProvided-expectedGas, metering.GasLeft())
	expectedTrace := map[string]map[string][]uint64{string(scAddress): {
		"previousHook":  {0},
		"writeEventLog": {expectedGas},
	}}
	assert.Equal(t, expectedTrace, metering.GetGasTrace())
}

// newMockVMHostWithRealMetering is the mock host every gas test in this file needs: a real
// meteringContext and a real managedTypesContext, so a bounded charge actually refuses instead of
// overdrawing, and BaseOpsErrorShouldFailExecution true as in production, so a shortfall is
// observable as the error the hook fails the execution with.
func newMockVMHostWithRealMetering(
	t testing.TB,
	fixAuditChangesV6 bool,
	gasMap map[string]map[string]uint64,
	gasProvided uint64,
) (*contextmock.VMHostMock, vmhost.MeteringContext) {
	t.Helper()

	host := newMockVMHost()
	host.ForkControllerContext = &commonMock.ForkControllerStub{FixAuditChangesV4Value: true, FixAuditChangesV6Value: fixAuditChangesV6}
	host.OutputContext = &contextmock.OutputContextStub{}
	host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBaseOpsAPI = true
	host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBigIntAPI = true

	metering, err := contexts.NewMeteringContext(host, gasMap)
	require.NoError(t, err)
	metering.InitStateFromContractCallInput(&vmcommon.VMInput{GasProvided: gasProvided})
	host.MeteringContext = metering

	managedTypes, err := contexts.NewManagedTypesContext(host)
	require.NoError(t, err)
	host.ManagedTypesContext = managedTypes

	return host, metering
}

// Before FixAuditChangesV6 these hooks load the argument data (and the upgrade code, the event log data)
// and charge it afterwards; from it they charge it before the load. The copy total must not move, or
// blocks from before the fork would stop replaying: a caller that dropped its pre-fork charge, or a
// helper that charged it a second time, fails here.
//
// The deploy hooks move one more charge: from the fork they pay the deployment's CompilePerByte here,
// which CreateNewContract used to pay after BoundGasLimit had already fixed the new contract's budget.
// That is postV6ExtraGas, and it is the one total that is meant to differ between the two sides; the
// managed getters put their whole copy charge there, having copied their buffers for free pre-fork.
// gasLimit 0 stops each hook right after its charges.
//
// Every row also runs with a budget one unit short of its fixed cost. From the fork that cost is
// bounded and charged first, so the hook must refuse without copying anything or reaching the
// blockchain, the output or the new contract; pre-fork it overdrew and carried on. That half is what
// holds each conversion in place: reverting one hook to the unbounded charge fails its own row.
func TestVMHooksImpl_CopyGas_SameBeforeAndFromV6(t *testing.T) {
	t.Parallel()

	const (
		lengthsPtr = executor.MemPtr(0)
		argsPtr    = executor.MemPtr(100)
		codePtr    = executor.MemPtr(200)
		valuePtr   = executor.MemPtr(300)
		metaPtr    = executor.MemPtr(400)
		addressPtr = executor.MemPtr(500)
		resultPtr  = executor.MemPtr(600)

		// the managed rows take their inputs from buffers rather than from contract memory; they
		// are seeded with the same bytes, so a managed copy charge is sized like its legacy sibling's
		tokenIDHandle = int32(1)
		addressHandle = int32(2)
		outHandle     = int32(3)
		metaHandle    = int32(4)
		argsHandle    = int32(5)
		valueHandle   = int32(6)
	)
	args := [][]byte{[]byte("arg1"), []byte("longerArgument2")}
	argLengths := make([]byte, 4*len(args))
	for i, arg := range args {
		argLengths[i*4] = byte(len(arg))
	}
	code := []byte("contract code")
	codeLen := executor.MemLength(len(code))
	address := []byte("addressBytes")
	codeMetadata := []byte{0, 0}
	gasMap := config.MakeGasMapForTests()
	bigIntCost := gasMap["BigIntAPICost"]
	createCost := gasMap["BaseOpsAPICost"]["CreateContract"]
	argBytes := uint64(len(argLengths) + len(args[0]) + len(args[1]))

	// the deploy hooks charge this per code byte from the fork; the upgrade ones still leave it to the
	// child frame's DeductInitialGasForIndirectDeployment, which this mock host never reaches
	deployGas := gasMap["BaseOperationCost"]["CompilePerByte"] * uint64(len(code))
	// the KDA getters and GetCallValueByTokenName charge the token id they copy; code doubles as
	// that id here, so they reuse codePtr/codeLen and tokenIDHandle
	tokenIDGas := gasMap["BaseOperationCost"]["DataCopyPerByte"] * uint64(len(code))
	addressGas := gasMap["BaseOperationCost"]["DataCopyPerByte"] * uint64(len(address))
	// the managed deploy hooks also pay for the code-metadata buffer they clone
	metadataGas := gasMap["BaseOperationCost"]["DataCopyPerByte"] * uint64(len(codeMetadata))
	// the KDA reads also pay for the records the stub below hands back, as decoded; the roles read
	// passes no address, so for the KDAData alone
	stubKDA := func() *kapps.KDAData {
		// Properties and Attributes are dereferenced unconditionally
		return &kapps.KDAData{Properties: &kapps.PropertiesData{}, Attributes: &kapps.AttributesData{}}
	}
	stubUser := func() *kapps.UserKDA { return &kapps.UserKDA{Balance: 1} }
	// literal encoded sizes: the stub KDAData (empty Properties and Attributes messages) is 6 bytes,
	// and the stub UserKDA (Balance 1) 2 more, so a proto.Size change breaks this table
	kdaReadGas := gasMap["BaseOperationCost"]["DataCopyPerByte"] * 8
	kdaRolesGas := gasMap["BaseOperationCost"]["DataCopyPerByte"] * 6

	tests := []struct {
		name           string
		baseCost       uint64
		copiedBytes    uint64
		postV6ExtraGas uint64
		// boundedFixedCost marks the hooks whose fixed cost this fork converted, and so the ones
		// the shortfall half applies to. The upgrade and delete hooks still charge theirs
		// unbounded and are a declared follow-up
		boundedFixedCost bool
		// firstBoundedCost is the first bounded charge the hook makes, which the shortfall half
		// runs one unit short of. 0 means the whole of baseCost is charged in one step; a hook
		// that charges it in two, as BigIntGetKDAExternalBalance does, names the first here
		firstBoundedCost uint64
		call             func(hooks *vmhooks.VMHooksImpl)
	}{
		{"DeleteContract", 0, argBytes, 0, false, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.DeleteContract(addressPtr, 0, int32(len(args)), lengthsPtr, argsPtr)
		}},
		{"UpgradeContract", createCost, argBytes + uint64(len(code)), 0, false, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.UpgradeContract(addressPtr, 0, valuePtr, codePtr, metaPtr, codeLen, int32(len(args)), lengthsPtr, argsPtr)
		}},
		{"UpgradeFromSourceContract", createCost, argBytes, 0, false, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.UpgradeFromSourceContract(addressPtr, 0, valuePtr, addressPtr, metaPtr, int32(len(args)), lengthsPtr, argsPtr)
		}},
		{"CreateContract", createCost, argBytes, deployGas, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.CreateContract(0, valuePtr, codePtr, metaPtr, codeLen, resultPtr, int32(len(args)), lengthsPtr, argsPtr)
		}},
		{"DeployFromSourceContract", createCost, argBytes, deployGas, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.DeployFromSourceContract(0, valuePtr, addressPtr, metaPtr, resultPtr, int32(len(args)), lengthsPtr, argsPtr)
		}},
		{"WriteEventLog", gasMap["BaseOpsAPICost"]["Log"], argBytes + uint64(len(code)), 0, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.WriteEventLog(int32(len(args)), lengthsPtr, argsPtr, codePtr, codeLen)
		}},
		{"GetCallValueByTokenName", gasMap["BaseOpsAPICost"]["GetCallValue"], 0, tokenIDGas, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.GetCallValueByTokenName(resultPtr, codePtr, codeLen)
		}},
		{"GetKDABalance", gasMap["BaseOpsAPICost"]["GetExternalBalance"], 0, tokenIDGas + kdaReadGas, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.GetKDABalance(addressPtr, codePtr, codeLen, 0, resultPtr)
		}},
		{"ManagedGetKDACallValue", gasMap["BaseOpsAPICost"]["GetCallValue"], 0, tokenIDGas, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.ManagedGetKDACallValue(outHandle, tokenIDHandle)
		}},
		{"ManagedGetKDABalance", gasMap["BaseOpsAPICost"]["GetExternalBalance"], 0, addressGas + tokenIDGas + kdaReadGas, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.ManagedGetKDABalance(addressHandle, tokenIDHandle, 0, outHandle)
		}},
		{"ManagedGetUserKDA", gasMap["BaseOpsAPICost"]["GetUserKDA"], 0, addressGas + tokenIDGas + kdaReadGas, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.ManagedGetUserKDA(addressHandle, tokenIDHandle, 0,
				outHandle, outHandle, outHandle, outHandle, outHandle, outHandle)
		}},
		{"ManagedGetKDATokenData", gasMap["BaseOpsAPICost"]["GetKDATokenData"], 0, addressGas + tokenIDGas + kdaReadGas, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.ManagedGetKDATokenData(addressHandle, tokenIDHandle, 0,
				outHandle, outHandle, outHandle, outHandle, outHandle, outHandle, outHandle, outHandle,
				outHandle, outHandle, outHandle, outHandle, outHandle, outHandle, outHandle, outHandle, outHandle)
		}},
		{"ManagedGetKDARoles", gasMap["BaseOpsAPICost"]["GetKDARoles"], 0, tokenIDGas + kdaRolesGas, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.ManagedGetKDARoles(tokenIDHandle, outHandle)
		}},
		{"ManagedGetSftMetadata", gasMap["BaseOpsAPICost"]["GetSFTMetadata"], 0, tokenIDGas, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.ManagedGetSftMetadata(tokenIDHandle, 0, outHandle)
		}},
		{"ManagedCreateContract", createCost, 0, deployGas + metadataGas, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.ManagedCreateContract(0, valueHandle, tokenIDHandle, metaHandle, argsHandle, outHandle, outHandle)
		}},
		{"ManagedDeployFromSourceContract", createCost, 0, deployGas + metadataGas, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.ManagedDeployFromSourceContract(0, valueHandle, addressHandle, metaHandle, argsHandle, outHandle, outHandle)
		}},
		// the bigInt setters charge the same copy on both sides: pre-fork after the load, on what
		// MemLoad returned, and from the fork before it, on the length asked for. An in-bounds
		// length makes the two the same, which is what keeps historical blocks replaying
		{"BigIntSetUnsignedBytes", bigIntCost["BigIntSetUnsignedBytes"], uint64(len(code)), 0, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.BigIntSetUnsignedBytes(valueHandle, codePtr, codeLen)
		}},
		{"BigIntSetSignedBytes", bigIntCost["BigIntSetSignedBytes"], uint64(len(code)), 0, true, 0, func(hooks *vmhooks.VMHooksImpl) {
			hooks.BigIntSetSignedBytes(valueHandle, codePtr, codeLen)
		}},
		// this one charges its own fixed cost and then the shared KDA read's, both bounded, so a
		// budget one short of the first is what the shortfall half needs
		{"BigIntGetKDAExternalBalance",
			bigIntCost["BigIntGetExternalBalance"] + gasMap["BaseOpsAPICost"]["GetExternalBalance"],
			0, tokenIDGas + kdaReadGas, true, bigIntCost["BigIntGetExternalBalance"],
			func(hooks *vmhooks.VMHooksImpl) {
				hooks.BigIntGetKDAExternalBalance(addressPtr, codePtr, codeLen, 0, valueHandle)
			}},
	}

	for _, tt := range tests {
		for _, fixAuditChangesV6 := range []bool{false, true} {
			for _, shortfall := range []bool{false, true} {
				// the shortfall half is a post-fork claim about a converted hook: pre-fork these
				// charged whatever they liked, which the exact totals already pin
				if shortfall && (!fixAuditChangesV6 || !tt.boundedFixedCost) {
					continue
				}

				t.Run(fmt.Sprintf("%s/FixAuditChangesV6=%v/shortfall=%v", tt.name, fixAuditChangesV6, shortfall), func(t *testing.T) {
					t.Parallel()

					gasProvided := uint64(1000)
					if shortfall {
						gasProvided = tt.baseCost - 1
						if tt.firstBoundedCost > 0 {
							gasProvided = tt.firstBoundedCost - 1
						}
					}

					host, metering := newMockVMHostWithRealMetering(t, fixAuditChangesV6, gasMap, gasProvided)

					// every effect a hook here can have outside the meter, so a shortfall can be
					// held to having none of them
					sideEffects := 0
					host.OutputContext = &contextmock.OutputContextStub{
						WriteLogCalled: func(_ []byte, _ [][]byte, _ [][]byte) { sideEffects++ },
					}
					blockchain := host.BlockchainContext.(*hostmock.BlockchainContextStub)
					// serve the same code to the from-source hooks, so their deployment charge is
					// sized like the ones that take the code out of contract memory
					blockchain.GetCodeCalled = func(_ []byte) ([]byte, error) {
						sideEffects++
						return code, nil
					}
					blockchain.GetKDATokenCalled =
						func(_ []byte, _ []byte, _ uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
							sideEffects++
							return stubKDA(), stubUser(), nil
						}
					blockchain.GetSFTMetaCalled = func(_ []byte, _ uint64) (*kapps.MetaV2, error) {
						sideEffects++
						return &kapps.MetaV2{Metadata: &kapps.MetaV2Data{}}, nil
					}
					host.CreateNewContractCalled = func(_ *vmcommon.ContractCreateInput, _ int) ([]byte, error) {
						sideEffects++
						return nil, nil
					}
					// ComputeGetCallValueGas reads the transfers, so the input cannot stay nil
					host.RuntimeContext.(*contextmock.RuntimeContextMock).VMInput = &vmcommon.ContractCallInput{}

					require.NoError(t, host.MemStoreToMock(lengthsPtr, argLengths))
					require.NoError(t, host.MemStoreToMock(argsPtr, bytes.Join(args, nil)))
					require.NoError(t, host.MemStoreToMock(codePtr, code))
					// seeding buffers and handles charges nothing, so it can happen after the
					// budget is set; an empty argument vector decodes to no arguments for free
					host.ManagedTypesContext.SetBytes(tokenIDHandle, code)
					host.ManagedTypesContext.SetBytes(addressHandle, address)
					host.ManagedTypesContext.SetBytes(metaHandle, codeMetadata)
					host.ManagedTypesContext.SetBytes(argsHandle, nil)
					host.ManagedTypesContext.GetBigIntOrCreate(valueHandle)

					tt.call(vmhooks.NewVMHooksImpl(host))

					failErr := host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr
					if shortfall {
						require.ErrorIs(t, failErr, vmhost.ErrNotEnoughGas)
						// the fault drains whatever is left, which is nothing here, so the meter is
						// read through the points: the bounded charge refuses and takes nothing,
						// where the unbounded one took the fixed cost and went negative
						assert.Zero(t, host.Runtime().GetPointsUsed(), "a refused charge must take nothing")
						assert.Zero(t, sideEffects, "nothing may happen for a hook that was not paid for")
						return
					}

					require.Nil(t, failErr)
					dataCopyPerByte := metering.GasSchedule().BaseOperationCost.DataCopyPerByte
					expectedGas := tt.baseCost + dataCopyPerByte*tt.copiedBytes
					if fixAuditChangesV6 {
						expectedGas += tt.postV6ExtraGas
					}
					assert.Equal(t, expectedGas, gasProvided-metering.GasLeft())
				})
			}
		}
	}
}

// From FixAuditChangesV6 these hooks price a user-supplied input on the length the contract asks for,
// charged before the copy, which is what makes an unaffordable length unaffordable instead of merely
// expensive: MemLoadFromMemory truncates a range running past the end of memory rather than rejecting
// it, so pre-fork the hook copies whatever is there for free and whatever follows is priced on the
// truncated result, or not priced at all.
func TestVMHooksImpl_LengthChargedBeforeTheCopy(t *testing.T) {
	t.Parallel()

	const (
		addressPtr  = executor.MemPtr(100)
		codePtr     = executor.MemPtr(200)
		valuePtr    = executor.MemPtr(300)
		metaPtr     = executor.MemPtr(400)
		resultPtr   = executor.MemPtr(600)
		gasProvided = uint64(1000)
		// past the end of the two-page mock memory, and far past what gasProvided can pay for
		length = executor.MemLength(200_000)
	)
	gasMap := config.MakeGasMapForTests()

	tests := []struct {
		name string
		// baseCost is all the hook charges pre-fork, the oversized copy having been free
		baseCost uint64
		// failResult and preForkResult are the hook's own return values, which differ per hook
		failResult    int32
		preForkResult int32
		call          func(hooks *vmhooks.VMHooksImpl) int32
	}{
		{"CreateContract", gasMap["BaseOpsAPICost"]["CreateContract"], 1, 0, func(hooks *vmhooks.VMHooksImpl) int32 {
			return hooks.CreateContract(0, valuePtr, codePtr, metaPtr, length, resultPtr, 0, 0, 0)
		}},
		{"GetKDABalance", gasMap["BaseOpsAPICost"]["GetExternalBalance"], -1, 1, func(hooks *vmhooks.VMHooksImpl) int32 {
			return hooks.GetKDABalance(addressPtr, addressPtr, length, 0, resultPtr)
		}},
	}

	for _, tt := range tests {
		for _, fixAuditChangesV6 := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/FixAuditChangesV6=%v", tt.name, fixAuditChangesV6), func(t *testing.T) {
				t.Parallel()

				host, metering := newMockVMHostWithRealMetering(t, fixAuditChangesV6, gasMap, gasProvided)

				reads := 0
				host.BlockchainContext.(*hostmock.BlockchainContextStub).GetKDATokenCalled =
					func(_ []byte, _ []byte, _ uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
						reads++
						return &kapps.KDAData{}, &kapps.UserKDA{Balance: 1}, nil
					}

				result := tt.call(vmhooks.NewVMHooksImpl(host))

				failErr := host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr
				if fixAuditChangesV6 {
					require.ErrorIs(t, failErr, vmhost.ErrNotEnoughGas)
					assert.Equal(t, tt.failResult, result)
					assert.Zero(t, reads, "the blockchain must not be read for an unaffordable length")
					return
				}

				require.NoError(t, failErr)
				assert.Equal(t, tt.preForkResult, result)
				// the copy went through for free and only the fixed cost was charged
				assert.Equal(t, tt.baseCost, gasProvided-metering.GasLeft())
			})
		}
	}
}

// The hooks that write storage or build a bigInt from contract memory copy a contract-sized input
// the same way StorageStore does, and used to copy it for free: MemLoadFromMemory truncates a length
// running past the end of memory instead of rejecting it, so the memcpy happened, the key reached
// SetStorage and was read off the trie, and only what came after was priced. Their fixed cost was
// unbounded too, and charged before all of that. From FixAuditChangesV6 the fixed cost is bounded
// and the copy is paid for on the length the contract asks for, before the copy, so neither an
// unaffordable length nor an unaffordable hook gets any work done for free.
//
// The fault drains whatever gas is left, so the meter says nothing after a refusal; what the hook
// did or did not do is the observation. The exact totals live in the copy-gas table above for the
// bigInt setters, and in TestVMHooksImpl_StorageHooks_GasSameBeforeAndFromV6 below for the storage
// hooks, whose totals run on through SetStorage.
func TestVMHooksImpl_SiblingHooks_ChargedBeforeTheCopy(t *testing.T) {
	t.Parallel()

	const (
		keyPtr  = executor.MemPtr(0)
		dataPtr = executor.MemPtr(100)
		// GetBigIntOrCreate makes the handle on first use and charges nothing, so the rows that
		// need one do not spend gas before the hook under test is reached
		destHandle = int32(1)
	)
	gasMap := config.MakeGasMapForTests()

	// call reports whether the hook's effect landed: a storage status for the ones that write, and
	// a non-zero destination for the ones that build a bigInt out of the copied bytes
	tests := []struct {
		name        string
		needStorage bool
		// preForkCopyCharged marks the rows whose pre-fork charge is the fixed cost plus the copy,
		// on whatever MemLoad returned after truncating. Their exact pre-fork total is asserted
		// below, which is what pins the charge to the truncated length rather than the requested
		// one; the storage rows' pre-fork total runs on through SetStorage, so it is pinned on an
		// affordable key instead, by TestVMHooksImpl_StorageHooks_GasSameBeforeAndFromV6
		preForkCopyCharged bool
		call               func(hooks *vmhooks.VMHooksImpl, keyLength executor.MemLength) bool
	}{
		{"StorageStore", true, false, func(hooks *vmhooks.VMHooksImpl, keyLength executor.MemLength) bool {
			return hooks.StorageStore(keyPtr, keyLength, dataPtr, 4) >= 0
		}},
		{"SetStorageLock", true, false, func(hooks *vmhooks.VMHooksImpl, keyLength executor.MemLength) bool {
			return hooks.SetStorageLock(keyPtr, keyLength, 1) >= 0
		}},
		{"BigIntStorageStoreUnsigned", true, false, func(hooks *vmhooks.VMHooksImpl, keyLength executor.MemLength) bool {
			return hooks.BigIntStorageStoreUnsigned(keyPtr, keyLength, destHandle) >= 0
		}},
		{"SmallIntStorageStoreUnsigned", true, false, func(hooks *vmhooks.VMHooksImpl, keyLength executor.MemLength) bool {
			return hooks.SmallIntStorageStoreUnsigned(keyPtr, keyLength, 1) >= 0
		}},
		{"SmallIntStorageStoreSigned", true, false, func(hooks *vmhooks.VMHooksImpl, keyLength executor.MemLength) bool {
			return hooks.SmallIntStorageStoreSigned(keyPtr, keyLength, 1) >= 0
		}},
		{"BigIntSetUnsignedBytes", false, true, func(hooks *vmhooks.VMHooksImpl, keyLength executor.MemLength) bool {
			hooks.BigIntSetUnsignedBytes(destHandle, keyPtr, keyLength)
			return hooks.GetManagedTypesContext().GetBigIntOrCreate(destHandle).BitLen() > 0
		}},
		{"BigIntSetSignedBytes", false, true, func(hooks *vmhooks.VMHooksImpl, keyLength executor.MemLength) bool {
			hooks.BigIntSetSignedBytes(destHandle, keyPtr, keyLength)
			return hooks.GetManagedTypesContext().GetBigIntOrCreate(destHandle).BitLen() > 0
		}},
	}

	// every cost in MakeGasMapForTests is 1, so no budget at all is short of any fixed cost
	budgets := []struct {
		name        string
		gasProvided uint64
		keyLength   executor.MemLength
	}{
		// past the end of the two-page mock memory, and far past what the budget can pay for
		{"unaffordableKey", 1000, executor.MemLength(200_000)},
		{"unaffordableHook", 0, executor.MemLength(8)},
	}

	for _, tt := range tests {
		for _, budget := range budgets {
			for _, fixAuditChangesV6 := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/FixAuditChangesV6=%v", tt.name, budget.name, fixAuditChangesV6), func(t *testing.T) {
					t.Parallel()

					host, _ := newMockVMHostWithRealMetering(t, fixAuditChangesV6, gasMap, budget.gasProvided)
					host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBigIntAPI = true
					trieReads := 0
					if tt.needStorage {
						blockchainHook, err := addStorageContextWithHook(host)
						require.NoError(t, err)
						blockchainHook.GetStorageDataCalled = func(_ []byte, _ []byte) ([]byte, uint32, error) {
							trieReads++
							return nil, 0, nil
						}
					}
					// the copy has to carry something, or the bigInt rows cannot tell a refused
					// copy from a copy of the zeroed memory
					require.NoError(t, host.MemStoreToMock(keyPtr, []byte("a key the contract cannot pay for")))

					landed := tt.call(vmhooks.NewVMHooksImpl(host), budget.keyLength)

					failErr := host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr
					if fixAuditChangesV6 {
						require.ErrorIs(t, failErr, vmhost.ErrNotEnoughGas)
						assert.False(t, landed, "the charge was refused, so nothing may have come of it")
						assert.Zero(t, trieReads, "the trie must not be read for a key that was not paid for")
						// with no budget at all the bounded fixed cost takes nothing; the fault
						// drains nothing either, so any points used mean it was charged unbounded
						if budget.gasProvided == 0 {
							assert.Zero(t, host.Runtime().GetPointsUsed(), "a refused charge must take nothing")
						}
						return
					}

					// pre-fork the hook ran on gas it did not have: either its work landed, or
					// SetStorage read the old value off the trie first and only then found out
					assert.True(t, landed || trieReads > 0)

					if tt.preForkCopyCharged {
						// MemLoadFromMemory truncates a range running past the end of memory
						// rather than rejecting it, and the pre-fork charge is on what came back
						truncated, err := host.MemLoadFromMock(keyPtr, int(budget.keyLength))
						require.NoError(t, err)
						expectedGas := gasMap["BigIntAPICost"][tt.name] +
							gasMap["BaseOperationCost"]["DataCopyPerByte"]*uint64(len(truncated))
						assert.Equal(t, expectedGas, host.Runtime().GetPointsUsed())
					}
				})
			}
		}
	}
}

// The exact gas of the storage hooks, on both sides of the fork, which the shortfall rows above
// cannot pin: those all end in a refusal. Pre-fork the key is copied for free, so the whole
// difference between the two totals is the charge the fork adds for copying it (and, for
// StorageStore, for copying the value). The totals are literals: recomputing SetStorage's
// arithmetic here would pin nothing. Every cost in MakeGasMapForTests is 1, so each total is a
// byte count — the fixed cost, plus what SetStorage charges for the key beyond AddressLen and for
// the value stored, plus the copies from the fork.
func TestVMHooksImpl_StorageHooks_GasSameBeforeAndFromV6(t *testing.T) {
	t.Parallel()

	const (
		keyPtr     = executor.MemPtr(0)
		dataPtr    = executor.MemPtr(100)
		keyLength  = executor.MemLength(8)
		destHandle = int32(1)
	)
	gasMap := config.MakeGasMapForTests()

	tests := []struct {
		name        string
		preForkGas  uint64
		postForkGas uint64
		call        func(hooks *vmhooks.VMHooksImpl) int32
	}{
		// 1 fixed + 4 stored value bytes; from the fork also the 8-byte key and the 4-byte value
		{"StorageStore", 5, 17, func(hooks *vmhooks.VMHooksImpl) int32 {
			return hooks.StorageStore(keyPtr, keyLength, dataPtr, 4)
		}},
		// 1 fixed + 1 stored value byte; from the fork also the 8-byte key. The time-lock key the
		// hook builds stays within AddressLen, so SetStorage charges nothing for it either way
		{"SetStorageLock", 2, 10, func(hooks *vmhooks.VMHooksImpl) int32 {
			return hooks.SetStorageLock(keyPtr, keyLength, 1)
		}},
		{"BigIntStorageStoreUnsigned", 2, 10, func(hooks *vmhooks.VMHooksImpl) int32 {
			return hooks.BigIntStorageStoreUnsigned(keyPtr, keyLength, destHandle)
		}},
		{"SmallIntStorageStoreUnsigned", 2, 10, func(hooks *vmhooks.VMHooksImpl) int32 {
			return hooks.SmallIntStorageStoreUnsigned(keyPtr, keyLength, 7)
		}},
		{"SmallIntStorageStoreSigned", 2, 10, func(hooks *vmhooks.VMHooksImpl) int32 {
			return hooks.SmallIntStorageStoreSigned(keyPtr, keyLength, 7)
		}},
	}

	for _, tt := range tests {
		for _, fixAuditChangesV6 := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/FixAuditChangesV6=%v", tt.name, fixAuditChangesV6), func(t *testing.T) {
				t.Parallel()

				const gasProvided = uint64(1000)
				host, metering := newMockVMHostWithRealMetering(t, fixAuditChangesV6, gasMap, gasProvided)
				_, err := addStorageContextWithHook(host)
				require.NoError(t, err)
				require.NoError(t, host.MemStoreToMock(keyPtr, []byte("a key123")))
				require.NoError(t, host.MemStoreToMock(dataPtr, []byte("data")))
				// a non-zero bigInt, so the row that stores one writes a byte like the others;
				// seeding it charges nothing
				host.ManagedTypesContext.GetBigIntOrCreate(destHandle).SetInt64(7)

				assert.GreaterOrEqual(t, tt.call(vmhooks.NewVMHooksImpl(host)), int32(0))
				require.Nil(t, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)

				expectedGas := tt.preForkGas
				if fixAuditChangesV6 {
					expectedGas = tt.postForkGas
				}
				assert.Equal(t, expectedGas, gasProvided-metering.GasLeft())
			})
		}
	}
}

const (
	storageLoadKeyPtr     = executor.MemPtr(0)
	storageLoadAddressPtr = executor.MemPtr(100)
	storageLoadDataPtr    = executor.MemPtr(200)
	storageLoadKeyLength  = executor.MemLength(8)
	storageLoadDestHandle = int32(1)
)

var (
	storageLoadKey     = []byte("a key123")
	storageLoadValue   = []byte("data")
	storageLoadAddress = bytes.Repeat([]byte{7}, vmhost.AddressLen)
)

func newStorageLoadHost(
	t *testing.T,
	fixAuditChangesV6 bool,
	gasProvided uint64,
	cached bool,
) (*contextmock.VMHostMock, vmhost.MeteringContext, *int) {
	t.Helper()

	host, metering := newMockVMHostWithRealMetering(t, fixAuditChangesV6, config.MakeGasMapForTests(), gasProvided)
	host.ForkControllerContext.(*commonMock.ForkControllerStub).FixAuditChangesV5Value = true
	blockchainHook, err := addStorageContextWithHook(host)
	require.NoError(t, err)
	trieReads := 0
	blockchainHook.GetStorageDataCalled = func(_ []byte, _ []byte) ([]byte, uint32, error) {
		trieReads++
		return storageLoadValue, 0, nil
	}
	host.StorageContext.SetAddress(storageLoadAddress)

	if cached {
		account, _ := host.Output().GetOutputAccount(storageLoadAddress)
		account.StorageUpdates[string(storageLoadKey)] = &vmcommon.StorageUpdate{
			Offset: storageLoadKey,
			Data:   storageLoadValue,
		}
	}

	require.NoError(t, host.MemStoreToMock(storageLoadKeyPtr, storageLoadKey))
	require.NoError(t, host.MemStoreToMock(storageLoadAddressPtr, storageLoadAddress))

	return host, metering, &trieReads
}

func storageLoadValueInMemory(host *contextmock.VMHostMock) bool {
	data, err := host.MemLoadFromMock(storageLoadDataPtr, len(storageLoadValue))
	return err == nil && bytes.Equal(data, storageLoadValue)
}

func TestVMHooksImpl_StorageLoadHooks_GasBeforeAndFromV6(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		cachedPreForkGas    uint64
		cachedPostForkGas   uint64
		uncachedPreForkGas  uint64
		uncachedPostForkGas uint64
		call                func(hooks *vmhooks.VMHooksImpl)
	}{
		{"StorageLoad", 1, 13, 15291, 15299, func(hooks *vmhooks.VMHooksImpl) {
			hooks.StorageLoad(storageLoadKeyPtr, storageLoadKeyLength, storageLoadDataPtr)
		}},
		{"StorageLoadFromAddress", 1, 13, 15291, 15299, func(hooks *vmhooks.VMHooksImpl) {
			hooks.StorageLoadFromAddress(storageLoadAddressPtr, storageLoadKeyPtr, storageLoadKeyLength, storageLoadDataPtr)
		}},
		{"BigIntStorageLoadUnsigned", 1, 13, 15291, 15299, func(hooks *vmhooks.VMHooksImpl) {
			hooks.BigIntStorageLoadUnsigned(storageLoadKeyPtr, storageLoadKeyLength, storageLoadDestHandle)
		}},
		{"SmallIntStorageLoadUnsigned", 1, 13, 15291, 15299, func(hooks *vmhooks.VMHooksImpl) {
			hooks.SmallIntStorageLoadUnsigned(storageLoadKeyPtr, storageLoadKeyLength)
		}},
		{"SmallIntStorageLoadSigned", 1, 13, 15291, 15299, func(hooks *vmhooks.VMHooksImpl) {
			hooks.SmallIntStorageLoadSigned(storageLoadKeyPtr, storageLoadKeyLength)
		}},
		{"StorageLoadLength", 1, 9, 15287, 15299, func(hooks *vmhooks.VMHooksImpl) {
			hooks.StorageLoadLength(storageLoadKeyPtr, storageLoadKeyLength)
		}},
		{"MBufferStorageLoad", 13, 13, 15303, 15303, func(hooks *vmhooks.VMHooksImpl) {
			managedType := hooks.GetManagedTypesContext()
			keyHandle := managedType.NewManagedBufferFromBytes(storageLoadKey)
			hooks.MBufferStorageLoad(keyHandle, managedType.NewManagedBuffer())
		}},
		{"MBufferStorageLoadFromAddress", 45, 45, 15335, 15335, func(hooks *vmhooks.VMHooksImpl) {
			managedType := hooks.GetManagedTypesContext()
			addressHandle := managedType.NewManagedBufferFromBytes(storageLoadAddress)
			keyHandle := managedType.NewManagedBufferFromBytes(storageLoadKey)
			hooks.MBufferStorageLoadFromAddress(addressHandle, keyHandle, managedType.NewManagedBuffer())
		}},
	}

	for _, tt := range tests {
		for _, cached := range []bool{false, true} {
			for _, fixAuditChangesV6 := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/cached=%v/FixAuditChangesV6=%v", tt.name, cached, fixAuditChangesV6), func(t *testing.T) {
					t.Parallel()

					const gasProvided = uint64(100_000)
					host, metering, _ := newStorageLoadHost(t, fixAuditChangesV6, gasProvided, cached)

					tt.call(vmhooks.NewVMHooksImpl(host))
					require.Nil(t, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)

					expectedGas := tt.uncachedPreForkGas
					switch {
					case cached && fixAuditChangesV6:
						expectedGas = tt.cachedPostForkGas
					case cached:
						expectedGas = tt.cachedPreForkGas
					case fixAuditChangesV6:
						expectedGas = tt.uncachedPostForkGas
					}
					assert.Equal(t, expectedGas, gasProvided-metering.GasLeft())
				})
			}
		}
	}
}

func TestVMHooksImpl_StorageLoadHooks_CachedValueChargedBeforeTheCopy(t *testing.T) {
	t.Parallel()

	storedNumber := big.NewInt(0).SetBytes(storageLoadValue).Int64()

	tests := []struct {
		name string
		call func(hooks *vmhooks.VMHooksImpl, host *contextmock.VMHostMock) bool
	}{
		{"StorageLoad", func(hooks *vmhooks.VMHooksImpl, host *contextmock.VMHostMock) bool {
			hooks.StorageLoad(storageLoadKeyPtr, storageLoadKeyLength, storageLoadDataPtr)
			return storageLoadValueInMemory(host)
		}},
		{"StorageLoadFromAddress", func(hooks *vmhooks.VMHooksImpl, host *contextmock.VMHostMock) bool {
			hooks.StorageLoadFromAddress(storageLoadAddressPtr, storageLoadKeyPtr, storageLoadKeyLength, storageLoadDataPtr)
			return storageLoadValueInMemory(host)
		}},
		{"BigIntStorageLoadUnsigned", func(hooks *vmhooks.VMHooksImpl, host *contextmock.VMHostMock) bool {
			hooks.BigIntStorageLoadUnsigned(storageLoadKeyPtr, storageLoadKeyLength, storageLoadDestHandle)
			return host.ManagedTypesContext.GetBigIntOrCreate(storageLoadDestHandle).Int64() == storedNumber
		}},
		{"SmallIntStorageLoadUnsigned", func(hooks *vmhooks.VMHooksImpl, _ *contextmock.VMHostMock) bool {
			return hooks.SmallIntStorageLoadUnsigned(storageLoadKeyPtr, storageLoadKeyLength) == storedNumber
		}},
		{"SmallIntStorageLoadSigned", func(hooks *vmhooks.VMHooksImpl, _ *contextmock.VMHostMock) bool {
			return hooks.SmallIntStorageLoadSigned(storageLoadKeyPtr, storageLoadKeyLength) == storedNumber
		}},
	}

	for _, tt := range tests {
		for _, fixAuditChangesV6 := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/FixAuditChangesV6=%v", tt.name, fixAuditChangesV6), func(t *testing.T) {
				t.Parallel()

				gasMap := config.MakeGasMapForTests()
				cachedLoadGas := gasMap["BaseOpsAPICost"]["CachedStorageLoad"]
				gasProvided := cachedLoadGas + gasMap["BaseOperationCost"]["DataCopyPerByte"]*uint64(len(storageLoadKey))
				host, metering, _ := newStorageLoadHost(t, fixAuditChangesV6, gasProvided, true)

				landed := tt.call(vmhooks.NewVMHooksImpl(host), host)

				failErr := host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr
				if fixAuditChangesV6 {
					require.ErrorIs(t, failErr, vmhost.ErrNotEnoughGas)
					assert.False(t, landed)
					return
				}

				require.NoError(t, failErr)
				assert.True(t, landed)
				assert.Equal(t, cachedLoadGas, gasProvided-metering.GasLeft())
			})
		}
	}
}

func TestVMHooksImpl_StorageLoadHooks_KeyChargedBeforeTheLoad(t *testing.T) {
	t.Parallel()

	const unaffordableKeyLength = executor.MemLength(200_000)

	tests := []struct {
		name string
		call func(hooks *vmhooks.VMHooksImpl)
	}{
		{"StorageLoad", func(hooks *vmhooks.VMHooksImpl) {
			hooks.StorageLoad(storageLoadKeyPtr, unaffordableKeyLength, storageLoadDataPtr)
		}},
		{"StorageLoadFromAddress", func(hooks *vmhooks.VMHooksImpl) {
			hooks.StorageLoadFromAddress(storageLoadAddressPtr, storageLoadKeyPtr, unaffordableKeyLength, storageLoadDataPtr)
		}},
		{"StorageLoadLength", func(hooks *vmhooks.VMHooksImpl) {
			hooks.StorageLoadLength(storageLoadKeyPtr, unaffordableKeyLength)
		}},
		{"GetStorageLock", func(hooks *vmhooks.VMHooksImpl) {
			hooks.GetStorageLock(storageLoadKeyPtr, unaffordableKeyLength)
		}},
		{"BigIntStorageLoadUnsigned", func(hooks *vmhooks.VMHooksImpl) {
			hooks.BigIntStorageLoadUnsigned(storageLoadKeyPtr, unaffordableKeyLength, storageLoadDestHandle)
		}},
		{"SmallIntStorageLoadUnsigned", func(hooks *vmhooks.VMHooksImpl) {
			hooks.SmallIntStorageLoadUnsigned(storageLoadKeyPtr, unaffordableKeyLength)
		}},
		{"SmallIntStorageLoadSigned", func(hooks *vmhooks.VMHooksImpl) {
			hooks.SmallIntStorageLoadSigned(storageLoadKeyPtr, unaffordableKeyLength)
		}},
	}

	for _, tt := range tests {
		for _, fixAuditChangesV6 := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/FixAuditChangesV6=%v", tt.name, fixAuditChangesV6), func(t *testing.T) {
				t.Parallel()

				host, _, trieReads := newStorageLoadHost(t, fixAuditChangesV6, 100_000, false)

				tt.call(vmhooks.NewVMHooksImpl(host))

				if fixAuditChangesV6 {
					require.ErrorIs(t, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr, vmhost.ErrNotEnoughGas)
					assert.Zero(t, *trieReads)
					return
				}

				assert.Positive(t, *trieReads)
			})
		}
	}
}

func TestVMHooksImpl_GetStorageLock_FixedCostChargedBeforeTheLoad(t *testing.T) {
	t.Parallel()

	for _, fixAuditChangesV6 := range []bool{false, true} {
		t.Run(fmt.Sprintf("FixAuditChangesV6=%v", fixAuditChangesV6), func(t *testing.T) {
			t.Parallel()

			host, _, trieReads := newStorageLoadHost(t, fixAuditChangesV6, 0, false)

			vmhooks.NewVMHooksImpl(host).GetStorageLock(storageLoadKeyPtr, 0)

			require.ErrorIs(t, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr, vmhost.ErrNotEnoughGas)
			if fixAuditChangesV6 {
				assert.Zero(t, *trieReads)
				assert.Zero(t, host.Runtime().GetPointsUsed())
				return
			}

			assert.Positive(t, *trieReads)
		})
	}
}

// StorageStore charges its fixed cost before it copies, and the key and the value separately, so
// each shortfall must leave the trie untouched. StorageStoreWithTypedArgs has no copies to make but
// charges the same fixed cost for its own callers, so it owns that branch too.
func TestVMHooksImpl_StorageStore_FaultsBeforeWriting(t *testing.T) {
	t.Parallel()

	const (
		keyPtr  = executor.MemPtr(0)
		dataPtr = executor.MemPtr(100)
		// past the end of the two-page mock memory, and far past any budget here
		oversized = executor.MemLength(200_000)
	)
	gasMap := config.MakeGasMapForTests()

	// every cost in MakeGasMapForTests is 1, so no budget at all is short of the fixed cost, while
	// 1000 covers it and an eight-byte key but not an oversized value
	tests := []struct {
		name        string
		gasProvided uint64
		call        func(host *contextmock.VMHostMock) int32
	}{
		{"typedArgsCannotPayTheFixedCost", 0, func(host *contextmock.VMHostMock) int32 {
			return vmhooks.StorageStoreWithTypedArgs(host, []byte("a key"), []byte("a value"))
		}},
		{"hookCannotPayForTheValue", 1000, func(host *contextmock.VMHostMock) int32 {
			return vmhooks.NewVMHooksImpl(host).StorageStore(keyPtr, 8, dataPtr, oversized)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			host, _ := newMockVMHostWithRealMetering(t, true, gasMap, tt.gasProvided)
			blockchainHook, err := addStorageContextWithHook(host)
			require.NoError(t, err)
			trieReads := 0
			blockchainHook.GetStorageDataCalled = func(_ []byte, _ []byte) ([]byte, uint32, error) {
				trieReads++
				return nil, 0, nil
			}

			assert.Equal(t, int32(-1), tt.call(host))
			require.ErrorIs(t, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr,
				vmhost.ErrNotEnoughGas)
			assert.Zero(t, trieReads, "nothing may be written for a store that was not paid for")
		})
	}
}

func TestVMHooksImpl_GetArgumentLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           [][]byte
		index          int32
		expected       int32
		shouldFailExec bool
		expectedError  error
	}{
		{
			name:     "valid argument",
			args:     [][]byte{[]byte("arg1"), []byte("argument2")},
			index:    0,
			expected: 4,
		},
		{
			name:     "second argument",
			args:     [][]byte{[]byte("arg1"), []byte("argument2")},
			index:    1,
			expected: 9,
		},
		{
			name:           "invalid index",
			args:           [][]byte{[]byte("arg1")},
			index:          1,
			expected:       -1,
			shouldFailExec: true,
			expectedError:  vmhost.ErrInvalidArgument,
		},
		{
			name:           "negative index",
			args:           [][]byte{[]byte("arg1")},
			index:          -1,
			expected:       -1,
			shouldFailExec: true,
			expectedError:  vmhost.ErrInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)

			host.RuntimeContext.(*contextmock.RuntimeContextMock).VMInput = &vmcommon.ContractCallInput{VMInput: vmcommon.VMInput{Arguments: tt.args}}
			host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBaseOpsAPI = tt.shouldFailExec

			result := hooks.GetArgumentLength(tt.index)
			assert.Equal(t, tt.expected, result)

			assert.Equal(t, tt.expectedError, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
		})
	}
}

func TestVMHooksImpl_GetKDABalance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		address        []byte
		tokenID        []byte
		nonce          int64
		balance        int64
		expectedLen    int32
		shouldFailExec bool
	}{
		{
			name:        "valid KDA balance",
			address:     makeAddress("address123456789012"),
			tokenID:     []byte("TKN"),
			nonce:       1,
			balance:     1000,
			expectedLen: 2, // length of balance bytes
		},
		{
			name:           "invalid address",
			address:        []byte("invalid"),
			tokenID:        []byte("TKN"),
			nonce:          1,
			shouldFailExec: true,
			expectedLen:    -1,
		},
		{
			name:           "invalid token ID",
			address:        makeAddress("address123456789012"),
			tokenID:        []byte{},
			nonce:          1,
			shouldFailExec: true,
			expectedLen:    -1,
		},
		{
			name:        "zero balance",
			address:     makeAddress("address123456789012"),
			tokenID:     []byte("TKN"),
			nonce:       1,
			balance:     0,
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)

			// Setup KDA token data
			kdaData := &kapps.KDAData{
				Name:      []byte("TestToken"),
				Precision: 8,
			}
			userKDA := &kapps.UserKDA{
				Balance: tt.balance,
			}

			host.BlockchainContext.(*hostmock.BlockchainContextStub).GetKDATokenCalled = func([]byte, []byte, uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
				if bytes.Equal(tt.address, []byte("invalid")) || len(tt.tokenID) == 0 {
					return nil, nil, vmhost.ErrInvalidArgument
				}

				return kdaData, userKDA, nil
			}

			// Store address and tokenID in memory
			err := host.Runtime().GetInstance().MemStore(executor.MemPtr(0), tt.address)
			require.Nil(t, err)
			err = host.Runtime().GetInstance().MemStore(executor.MemPtr(32), tt.tokenID)
			require.Nil(t, err)

			result := hooks.GetKDABalance(
				executor.MemPtr(0),
				executor.MemPtr(32),
				executor.MemLength(len(tt.tokenID)),
				tt.nonce,
				executor.MemPtr(64),
			)

			assert.Equal(t, tt.expectedLen, result)

			if !tt.shouldFailExec {
				// Verify stored balance
				stored, err := host.Runtime().GetInstance().MemLoad(
					executor.MemPtr(64),
					executor.MemLength(tt.expectedLen),
				)
				require.Nil(t, err)
				assert.Equal(t, big.NewInt(tt.balance).Bytes(), stored)
			}
		})
	}
}

// TestVMHooksImpl_GetKDABalance_ReadPastTheTrieReadsLimitFailsExecution pins that the
// non-managed KDA getters pass the hook's ErrMaxCallsReached through unchanged (they never
// remapped errors, unlike faultKDAArg); the count itself is the test's counter wrapper.
func TestVMHooksImpl_GetKDABalance_ReadPastTheTrieReadsLimitFailsExecution(t *testing.T) {
	t.Parallel()

	host := newKDAGetterHostWithFixV7(t, kdaGetterGasMap(), true)
	runtime := host.RuntimeContext.(*contextmock.RuntimeContextMock)
	kdaGetterStubToken(host, &kapps.KDAData{}, &kapps.UserKDA{Balance: 5})
	kdaGetterLimitTrieReads(t, host)

	err := host.Runtime().GetInstance().MemStore(executor.MemPtr(0), kdaGetterAddress)
	require.Nil(t, err)
	err = host.Runtime().GetInstance().MemStore(executor.MemPtr(32), kdaGetterTicker)
	require.Nil(t, err)
	hooks := vmhooks.NewVMHooksImpl(host)
	getBalance := func() int32 {
		return hooks.GetKDABalance(
			executor.MemPtr(0),
			executor.MemPtr(32),
			executor.MemLength(len(kdaGetterTicker)),
			0,
			executor.MemPtr(64),
		)
	}

	assert.Equal(t, int32(1), getBalance())
	require.NoError(t, runtime.FailExecutionErr)

	assert.Equal(t, int32(-1), getBalance())
	require.ErrorIs(t, runtime.FailExecutionErr, process.ErrMaxCallsReached)
}

func TestVMHooksImpl_GetKDANameLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		address        []byte
		tokenID        []byte
		nftName        string
		nonce          int64
		expectedLen    int32
		shouldFailExec bool
	}{
		{
			name:        "valid name",
			address:     []byte("address123456789012"),
			tokenID:     []byte("NFT-123"),
			nftName:     "CoolNFT",
			nonce:       1,
			expectedLen: 7,
		},
		{
			name:        "empty name",
			address:     []byte("address123456789012"),
			tokenID:     []byte("NFT-123"),
			nftName:     "",
			nonce:       1,
			expectedLen: 0,
		},
		{
			name:           "invalid address",
			address:        []byte("invalid"),
			tokenID:        []byte("NFT-123"),
			nftName:        "CoolNFT",
			nonce:          1,
			shouldFailExec: true,
			expectedLen:    -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)

			// Setup KDA token data
			kdaData := &kapps.KDAData{
				Name: []byte(tt.nftName),
			}
			userKDA := &kapps.UserKDA{}

			host.BlockchainContext.(*hostmock.BlockchainContextStub).GetKDATokenCalled = func([]byte, []byte, uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
				if bytes.Equal(tt.address, []byte("invalid")) || len(tt.tokenID) == 0 {
					return nil, nil, vmhost.ErrInvalidArgument
				}

				return kdaData, userKDA, nil
			}

			// Store address and tokenID in memory
			err := host.Runtime().GetInstance().MemStore(executor.MemPtr(0), tt.address)
			require.Nil(t, err)
			err = host.Runtime().GetInstance().MemStore(executor.MemPtr(32), tt.tokenID)
			require.Nil(t, err)

			result := hooks.GetKDANameLength(
				executor.MemPtr(0),
				executor.MemPtr(32),
				executor.MemLength(len(tt.tokenID)),
				tt.nonce,
			)

			assert.Equal(t, tt.expectedLen, result)
		})
	}
}

func TestVMHooksImpl_GetKDAURILength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		address        []byte
		tokenID        []byte
		uris           map[string]string
		nonce          int64
		expectedLen    int32
		shouldFailExec bool
	}{
		{
			name:        "valid URI",
			address:     []byte("address123456789012"),
			tokenID:     []byte("NFT-123"),
			uris:        map[string]string{"ipfs": "ipfs://QmX"},
			nonce:       1,
			expectedLen: 10,
		},
		{
			name:        "empty URIs",
			address:     []byte("address123456789012"),
			tokenID:     []byte("NFT-123"),
			uris:        map[string]string{},
			nonce:       1,
			expectedLen: 0,
		},
		{
			name:        "multiple URIs",
			address:     []byte("address123456789012"),
			tokenID:     []byte("NFT-123"),
			uris:        map[string]string{"uri1": "uri1", "uri2": "uri2"},
			nonce:       1,
			expectedLen: 4, // Should return length of first URI
		},
		{
			name:           "invalid address",
			address:        []byte("invalid"),
			tokenID:        []byte("NFT-123"),
			uris:           map[string]string{"uri": "uri1"},
			nonce:          1,
			shouldFailExec: true,
			expectedLen:    -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)

			// Setup KDA token data
			kdaData := &kapps.KDAData{
				URIs: tt.uris,
			}
			userKDA := &kapps.UserKDA{}

			host.BlockchainContext.(*hostmock.BlockchainContextStub).GetKDATokenCalled = func([]byte, []byte, uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
				if bytes.Equal(tt.address, []byte("invalid")) || len(tt.tokenID) == 0 {
					return nil, nil, vmhost.ErrInvalidArgument
				}

				return kdaData, userKDA, nil
			}

			// Store address and tokenID in memory
			err := host.Runtime().GetInstance().MemStore(executor.MemPtr(0), tt.address)
			require.Nil(t, err)
			err = host.Runtime().GetInstance().MemStore(executor.MemPtr(32), tt.tokenID)
			require.Nil(t, err)

			result := hooks.GetKDAURILength(
				executor.MemPtr(0),
				executor.MemPtr(32),
				executor.MemLength(len(tt.tokenID)),
				tt.nonce,
			)

			assert.Equal(t, tt.expectedLen, result)
		})
	}
}

func TestVMHooksImpl_GetArgument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           [][]byte
		index          int32
		expectedLen    int32
		shouldFailExec bool
	}{
		{
			name:        "valid argument",
			args:        [][]byte{[]byte("arg1"), []byte("argument2")},
			index:       0,
			expectedLen: 4,
		},
		{
			name:        "second argument",
			args:        [][]byte{[]byte("arg1"), []byte("argument2")},
			index:       1,
			expectedLen: 9,
		},
		{
			name:           "invalid index",
			args:           [][]byte{[]byte("arg1")},
			index:          1,
			expectedLen:    -1,
			shouldFailExec: true,
		},
		{
			name:           "negative index",
			args:           [][]byte{[]byte("arg1")},
			index:          -1,
			expectedLen:    -1,
			shouldFailExec: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)

			host.RuntimeContext.(*contextmock.RuntimeContextMock).VMInput = &vmcommon.ContractCallInput{VMInput: vmcommon.VMInput{Arguments: tt.args}}

			result := hooks.GetArgument(tt.index, executor.MemPtr(0))

			assert.Equal(t, tt.expectedLen, result)

			if !tt.shouldFailExec {
				// Verify stored argument
				stored, err := host.Runtime().GetInstance().MemLoad(
					executor.MemPtr(0),
					executor.MemLength(tt.expectedLen),
				)
				require.Nil(t, err)
				assert.Equal(t, tt.args[tt.index], stored)
			}
		})
	}
}

func TestVMHooksImpl_GetFunction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		function       string
		expectedLen    int32
		shouldFailExec bool
	}{
		{
			name:        "valid function name",
			function:    "transfer",
			expectedLen: 8,
		},
		{
			name:        "empty function name",
			function:    "",
			expectedLen: 0,
		},
		{
			name:        "long function name",
			function:    "verylongfunctionnameherewithmorethan40characters",
			expectedLen: 48,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)

			host.RuntimeContext.(*contextmock.RuntimeContextMock).CallFunction = tt.function

			result := hooks.GetFunction(executor.MemPtr(0))

			assert.Equal(t, tt.expectedLen, result)

			if !tt.shouldFailExec {
				// Verify stored function name
				stored, err := host.Runtime().GetInstance().MemLoad(
					executor.MemPtr(0),
					executor.MemLength(tt.expectedLen),
				)
				require.Nil(t, err)
				assert.Equal(t, []byte(tt.function), stored)
			}
		})
	}
}

func TestVMHooksImpl_GetNumArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     [][]byte
		expected int32
	}{
		{
			name:     "multiple arguments",
			args:     [][]byte{[]byte("arg1"), []byte("arg2"), []byte("arg3")},
			expected: 3,
		},
		{
			name:     "single argument",
			args:     [][]byte{[]byte("arg1")},
			expected: 1,
		},
		{
			name:     "no arguments",
			args:     [][]byte{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)

			host.RuntimeContext.(*contextmock.RuntimeContextMock).VMInput = &vmcommon.ContractCallInput{VMInput: vmcommon.VMInput{Arguments: tt.args}}

			result := hooks.GetNumArguments()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func addStorageContext(host *contextmock.VMHostMock) error {
	_, err := addStorageContextWithHook(host)
	return err
}

// addStorageContext, handing back the blockchain hook the storage context reads through, so a test
// can observe whether the trie was touched
func addStorageContextWithHook(host *contextmock.VMHostMock) (*contextmock.BlockchainHookStub, error) {
	protectedPrefix := [][]byte{
		[]byte("prefix1"),
		[]byte("prefix2"),
	}

	blockchainHooks := &contextmock.BlockchainHookStub{}
	storageContext, err := contexts.NewStorageContext(host, blockchainHooks, protectedPrefix)
	if err != nil {
		return nil, err
	}

	host.StorageContext = storageContext

	outputAccounts := make(map[string]*vmcommon.OutputAccount)

	// output context
	host.OutputContext = &contextmock.OutputContextStub{
		GetOutputAccountsCalled: func() map[string]*vmcommon.OutputAccount {
			return outputAccounts
		},
		GetOutputAccountCalled: func(address []byte) (*vmcommon.OutputAccount, bool) {
			acnt, exists := outputAccounts[string(address)]
			if !exists {
				acnt = &vmcommon.OutputAccount{
					Address:         address,
					StorageUpdates:  make(map[string]*vmcommon.StorageUpdate),
					OutputTransfers: make([]vmcommon.OutputTransfer, 0),
				}
				outputAccounts[string(address)] = acnt
			}
			return acnt, exists
		},
	}

	return blockchainHooks, nil
}

func TestVMHooksImpl_SetStorageLock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		key            []byte
		lockTimestamp  int64
		expectedStatus int32
		shouldFailExec bool
	}{
		{
			name:           "valid lock",
			key:            []byte("key1"),
			lockTimestamp:  time.Now().Unix() + 3600, // lock for 1 hour
			expectedStatus: 2,                        // storage added
		},
		{
			name:           "clear lock",
			key:            []byte("key1"),
			lockTimestamp:  0,
			expectedStatus: 0, // storage unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			err := addStorageContext(host)
			require.Nil(t, err)

			hooks := vmhooks.NewVMHooksImpl(host)

			err = host.Runtime().GetInstance().MemStore(executor.MemPtr(0), tt.key)
			require.Nil(t, err)

			result := hooks.SetStorageLock(
				executor.MemPtr(0),
				executor.MemLength(len(tt.key)),
				tt.lockTimestamp,
			)

			assert.Equal(t, tt.expectedStatus, result)

			if !tt.shouldFailExec {
				// Verify lock was set
				timeLock := hooks.GetStorageLock(
					executor.MemPtr(0),
					executor.MemLength(len(tt.key)),
				)
				assert.Equal(t, tt.lockTimestamp, timeLock)
			}
		})
	}
}

func TestVMHooksImpl_GetCallValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		callValue   *big.Int
		expectedLen int32
	}{
		{
			name:        "non-zero value",
			callValue:   big.NewInt(1000),
			expectedLen: 32,
		},
		{
			name:        "zero value",
			callValue:   big.NewInt(0),
			expectedLen: 0,
		},
		{
			name:        "large value",
			callValue:   big.NewInt(9999999999),
			expectedLen: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)

			vmInput := vmcommon.VMInput{
				KDATransfers: []*vmcommon.KDATransfer{
					{
						KDAValue: tt.callValue,
					},
				},
			}
			host.RuntimeContext.(*contextmock.RuntimeContextMock).VMInput = &vmcommon.ContractCallInput{VMInput: vmInput}

			result := hooks.GetCallValue(executor.MemPtr(0))
			assert.Equal(t, tt.expectedLen, result)

			// Calculate gas used
			loopGas := hooks.GetMeteringContext().GasSchedule().WASMOpcodeCost.Loop
			callValueGas := hooks.GetMeteringContext().GasSchedule().BaseOpsAPICost.GetCallValue
			gasUsed := (uint64(loopGas) * uint64(len(vmInput.KDATransfers))) + callValueGas

			assert.Equal(t, host.MeteringContext.(*contextmock.MeteringContextMock).GasUsedForExecution(), gasUsed)
		})
	}
}

func TestVMHooksImpl_GetKDAValueByIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		transfers      []*vmcommon.KDATransfer
		index          int32
		expectedLen    int32
		shouldFailExec bool
		expectedError  error
	}{
		{
			name: "valid index",
			transfers: []*vmcommon.KDATransfer{
				{KDAValue: big.NewInt(100)},
				{KDAValue: big.NewInt(200)},
			},
			index:       0,
			expectedLen: 32,
		},
		{
			name: "second index",
			transfers: []*vmcommon.KDATransfer{
				{KDAValue: big.NewInt(100)},
				{KDAValue: big.NewInt(200)},
			},
			index:       1,
			expectedLen: 32,
		},
		{
			name: "invalid index",
			transfers: []*vmcommon.KDATransfer{
				{KDAValue: big.NewInt(100)},
			},
			index:          1,
			expectedLen:    0,
			shouldFailExec: true,
			expectedError:  vmhost.ErrInvalidTokenIndex,
		},
		{
			name:           "no transfers",
			transfers:      []*vmcommon.KDATransfer{},
			index:          0,
			expectedLen:    0,
			shouldFailExec: true,
			expectedError:  vmhost.ErrInvalidTokenIndex,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)
			host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBaseOpsAPI = tt.shouldFailExec

			vmInput := vmcommon.VMInput{
				KDATransfers: tt.transfers,
			}
			host.RuntimeContext.(*contextmock.RuntimeContextMock).VMInput = &vmcommon.ContractCallInput{VMInput: vmInput}

			result := hooks.GetKDAValueByIndex(executor.MemPtr(0), tt.index)
			assert.Equal(t, tt.expectedLen, result)

			if !tt.shouldFailExec && len(tt.transfers) > int(tt.index) {
				// Verify stored value
				stored, err := host.Runtime().GetInstance().MemLoad(
					executor.MemPtr(0),
					executor.MemLength(tt.expectedLen),
				)
				require.Nil(t, err)

				expectedBytes := vmhost.PadBytesLeft(tt.transfers[tt.index].KDAValue.Bytes(), 32)
				assert.Equal(t, expectedBytes, stored)
			}

			assert.Equal(t, tt.expectedError, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
		})
	}
}

func TestVMHooksImpl_GetCallValueByTokenName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		transfers   []*vmcommon.KDATransfer
		tokenName   []byte
		expectedLen int32
	}{
		{
			name: "existing token",
			transfers: []*vmcommon.KDATransfer{
				{
					KDAValue:     big.NewInt(100),
					KDATokenName: []byte("TKN-123"),
				},
			},
			tokenName:   []byte("TKN-123"),
			expectedLen: 32,
		},
		{
			name: "non-existing token",
			transfers: []*vmcommon.KDATransfer{
				{
					KDAValue:     big.NewInt(100),
					KDATokenName: []byte("TKN-123"),
				},
			},
			tokenName:   []byte("OTHER-TOKEN"),
			expectedLen: 0,
		},
		{
			name: "multiple transfers",
			transfers: []*vmcommon.KDATransfer{
				{
					KDAValue:     big.NewInt(100),
					KDATokenName: []byte("TKN-123"),
				},
				{
					KDAValue:     big.NewInt(200),
					KDATokenName: []byte("TKN-456"),
				},
			},
			tokenName:   []byte("TKN-123"),
			expectedLen: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)

			// the hook's fixed cost now goes through startTracingAndUseGas, and the mock's
			// UseAndTraceGas deducts nothing, so the charge is recorded instead - the same
			// convention the loadMemBytes tests use
			var charged uint64
			host.MeteringContext.(*contextmock.MeteringContextMock).UseAndTraceGasCalled =
				func(gas uint64) { charged += gas }

			vmInput := vmcommon.VMInput{
				KDATransfers: tt.transfers,
			}
			host.RuntimeContext.(*contextmock.RuntimeContextMock).VMInput = &vmcommon.ContractCallInput{VMInput: vmInput}

			// Store token name in memory
			err := host.Runtime().GetInstance().MemStore(executor.MemPtr(32), tt.tokenName)
			require.Nil(t, err)

			result := hooks.GetCallValueByTokenName(
				executor.MemPtr(0),  // value result
				executor.MemPtr(32), // token name
				executor.MemLength(len(tt.tokenName)),
			)

			assert.Equal(t, tt.expectedLen, result)

			// Verify stored value
			stored, err := host.Runtime().GetInstance().MemLoad(
				executor.MemPtr(0),
				executor.MemLength(tt.expectedLen),
			)
			require.Nil(t, err)

			// Calculate expected value (sum of all transfers for this token)
			expectedValue := big.NewInt(0)
			for _, transfer := range tt.transfers {
				if string(transfer.KDATokenName) == string(tt.tokenName) {
					expectedValue.Add(expectedValue, transfer.KDAValue)
				}
			}

			expectedBytes := vmhost.PadBytesLeft(expectedValue.Bytes(), 32)
			assert.Equal(t, expectedBytes, stored)

			// Calculate gas used
			loopGas := hooks.GetMeteringContext().GasSchedule().WASMOpcodeCost.Loop
			callValueGas := hooks.GetMeteringContext().GasSchedule().BaseOpsAPICost.GetCallValue
			// pre-fork the token name is still free, so this is the fixed cost alone
			gasUsed := (uint64(loopGas) * uint64(len(vmInput.KDATransfers))) + callValueGas

			assert.Equal(t, gasUsed, charged)
		})
	}
}

func TestVMHooksImpl_GetKDATokenNonceByIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		transfers      []*vmcommon.KDATransfer
		index          int32
		expectedNonce  int64
		shouldFailExec bool
		expectedError  error
	}{
		{
			name: "valid nonce",
			transfers: []*vmcommon.KDATransfer{
				{KDATokenNonce: 1},
				{KDATokenNonce: 2},
			},
			index:         0,
			expectedNonce: 1,
		},
		{
			name: "second transfer",
			transfers: []*vmcommon.KDATransfer{
				{KDATokenNonce: 1},
				{KDATokenNonce: 2},
			},
			index:         1,
			expectedNonce: 2,
		},
		{
			name: "invalid index",
			transfers: []*vmcommon.KDATransfer{
				{KDATokenNonce: 1},
			},
			index:          2,
			expectedNonce:  0,
			shouldFailExec: true,
			expectedError:  vmhost.ErrInvalidTokenIndex,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)
			host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBaseOpsAPI = tt.shouldFailExec

			vmInput := vmcommon.VMInput{
				KDATransfers: tt.transfers,
			}
			host.RuntimeContext.(*contextmock.RuntimeContextMock).VMInput = &vmcommon.ContractCallInput{VMInput: vmInput}

			result := hooks.GetKDATokenNonceByIndex(tt.index)
			assert.Equal(t, tt.expectedNonce, result)

			assert.Equal(t, tt.expectedError, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
		})
	}
}

func TestVMHooksImpl_GetKDATokenTypeByIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		transfers      []*vmcommon.KDATransfer
		index          int32
		expectedType   int32
		shouldFailExec bool
		expectedError  error
	}{
		{
			name: "fungible token",
			transfers: []*vmcommon.KDATransfer{
				{KDATokenType: uint32(kapps.KDAData_Fungible)},
			},
			index:        0,
			expectedType: int32(kapps.KDAData_Fungible),
		},
		{
			name: "non-fungible token",
			transfers: []*vmcommon.KDATransfer{
				{KDATokenType: uint32(kapps.KDAData_NonFungible)},
			},
			index:        0,
			expectedType: int32(kapps.KDAData_NonFungible),
		},
		{
			name: "invalid index",
			transfers: []*vmcommon.KDATransfer{
				{KDATokenType: uint32(kapps.KDAData_Fungible)},
			},
			index:          1,
			expectedType:   0,
			shouldFailExec: true,
			expectedError:  vmhost.ErrInvalidTokenIndex,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)
			host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBaseOpsAPI = tt.shouldFailExec

			vmInput := vmcommon.VMInput{
				KDATransfers: tt.transfers,
			}
			host.RuntimeContext.(*contextmock.RuntimeContextMock).VMInput = &vmcommon.ContractCallInput{VMInput: vmInput}

			result := hooks.GetKDATokenTypeByIndex(tt.index)
			assert.Equal(t, tt.expectedType, result)

			assert.Equal(t, tt.expectedError, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
		})
	}
}

func TestVMHooksImpl_WriteEventLog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		numTopics      int32
		topics         [][]byte
		data           []byte
		shouldFailExec bool
		expectedError  error
	}{
		{
			name:      "valid event",
			numTopics: 2,
			topics:    [][]byte{[]byte("topic1"), []byte("topic2")},
			data:      []byte("event data"),
		},
		{
			name:      "no topics",
			numTopics: 0,
			topics:    [][]byte{},
			data:      []byte("event data"),
		},
		{
			name:           "invalid number of topics",
			numTopics:      -1,
			topics:         [][]byte{},
			data:           []byte("event data"),
			shouldFailExec: true,
			expectedError:  fmt.Errorf("invalid numArguments (-1)"),
		},
		{
			name:      "large data",
			numTopics: 1,
			topics:    [][]byte{[]byte("topic1")},
			data:      make([]byte, 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)
			require.NoError(t, addStorageContext(host))
			host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBaseOpsAPI = tt.shouldFailExec

			var resultAddress []byte
			var resultTopics [][]byte
			var resultData [][]byte

			host.OutputContext.(*contextmock.OutputContextStub).WriteLogCalled = func(address []byte, topics [][]byte, data [][]byte) {
				resultAddress = address
				resultTopics = topics
				resultData = data
			}

			// Store topics lengths
			if tt.numTopics >= 0 {
				topicLengths := make([]byte, tt.numTopics*4)
				for i, topic := range tt.topics {
					topicLengths[i*4] = byte(len(topic))
				}

				// Store topic lengths in memory
				err := host.Runtime().GetInstance().MemStore(executor.MemPtr(0), topicLengths)
				require.Nil(t, err)
			}

			// Store topics data
			topicData := make([]byte, 0)
			for _, topic := range tt.topics {
				topicData = append(topicData, topic...)
			}
			err := host.Runtime().GetInstance().MemStore(executor.MemPtr(100), topicData)
			require.Nil(t, err)

			// Store event data
			err = host.Runtime().GetInstance().MemStore(executor.MemPtr(200), tt.data)
			require.Nil(t, err)

			hooks.WriteEventLog(
				tt.numTopics,
				executor.MemPtr(0),
				executor.MemPtr(100),
				executor.MemPtr(200),
				executor.MemLength(len(tt.data)),
			)

			if tt.shouldFailExec {
				assert.Equal(t, tt.expectedError, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
			} else {
				assert.Equal(t, resultAddress, host.RuntimeContext.(*contextmock.RuntimeContextMock).SCAddress)
				assert.Equal(t, resultTopics, tt.topics)
				assert.Equal(t, resultData, [][]byte{tt.data})
			}
		})
	}
}

func TestVMHooksImpl_GetNumReturnData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		returnData  [][]byte
		expectedNum int32
	}{
		{
			name:        "multiple return values",
			returnData:  [][]byte{[]byte("data1"), []byte("data2"), []byte("data3")},
			expectedNum: 3,
		},
		{
			name:        "single return value",
			returnData:  [][]byte{[]byte("data1")},
			expectedNum: 1,
		},
		{
			name:        "no return data",
			returnData:  [][]byte{},
			expectedNum: 0,
		},
		{
			name:        "empty return values",
			returnData:  [][]byte{[]byte{}, []byte{}},
			expectedNum: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)
			require.NoError(t, addStorageContext(host))

			host.OutputContext.(*contextmock.OutputContextStub).ReturnDataCalled = func() [][]byte {
				return tt.returnData
			}

			result := hooks.GetNumReturnData()
			assert.Equal(t, tt.expectedNum, result)
		})
	}
}

func TestVMHooksImpl_GetReturnDataSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		returnData     [][]byte
		index          int32
		expectedSize   int32
		shouldFailExec bool
		expectedError  error
	}{
		{
			name:         "valid index",
			returnData:   [][]byte{[]byte("small"), []byte("medium data"), []byte("large data here")},
			index:        1,
			expectedSize: 11, // len("medium data")
		},
		{
			name:         "zero index",
			returnData:   [][]byte{[]byte("data")},
			index:        0,
			expectedSize: 4,
		},
		{
			name:           "invalid index",
			returnData:     [][]byte{[]byte("data")},
			index:          1,
			expectedSize:   0,
			shouldFailExec: true,
			expectedError:  vmhost.ErrInvalidArgument,
		},
		{
			name:           "negative index",
			returnData:     [][]byte{[]byte("data")},
			index:          -1,
			expectedSize:   0,
			shouldFailExec: true,
			expectedError:  vmhost.ErrInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)
			require.NoError(t, addStorageContext(host))
			host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBaseOpsAPI = tt.shouldFailExec

			host.OutputContext.(*contextmock.OutputContextStub).ReturnDataCalled = func() [][]byte {
				return tt.returnData
			}

			result := hooks.GetReturnDataSize(tt.index)

			if tt.shouldFailExec {
				assert.Equal(t, tt.expectedError, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
			} else {
				assert.Equal(t, tt.expectedSize, result)
			}
		})
	}
}

func TestVMHooksImpl_GetReturnData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		returnData     [][]byte
		index          int32
		expectedSize   int32
		shouldFailExec bool
		expectedError  error
	}{
		{
			name:         "get first value",
			returnData:   [][]byte{[]byte("first"), []byte("second")},
			index:        0,
			expectedSize: 5,
		},
		{
			name:         "get second value",
			returnData:   [][]byte{[]byte("first"), []byte("second")},
			index:        1,
			expectedSize: 6,
		},
		{
			name:           "invalid index",
			returnData:     [][]byte{[]byte("data")},
			index:          1,
			expectedSize:   0,
			shouldFailExec: true,
			expectedError:  vmhost.ErrInvalidArgument,
		},
		{
			name:         "empty value",
			returnData:   [][]byte{[]byte("")},
			index:        0,
			expectedSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := newMockVMHost()
			hooks := vmhooks.NewVMHooksImpl(host)
			require.NoError(t, addStorageContext(host))
			host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBaseOpsAPI = tt.shouldFailExec

			host.OutputContext.(*contextmock.OutputContextStub).ReturnDataCalled = func() [][]byte {
				return tt.returnData
			}

			result := hooks.GetReturnData(tt.index, executor.MemPtr(0))

			if tt.shouldFailExec {
				assert.Equal(t, tt.expectedError, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
			} else {
				assert.Equal(t, tt.expectedSize, result)

				// Verify stored data
				stored, err := host.Runtime().GetInstance().MemLoad(
					executor.MemPtr(0),
					executor.MemLength(tt.expectedSize),
				)
				require.Nil(t, err)
				assert.Equal(t, tt.returnData[tt.index], stored)
			}
		})
	}
}

// argumentGetterGasMap prices the argument and return data getters with distinct values, so a test
// can tell a fixed cost from a per-byte one and DataCopyPerByte from CopyPerByteForTooBig
func argumentGetterGasMap() config.GasScheduleMap {
	gasMap := config.MakeGasMapForTests()
	gasMap["BigIntAPICost"]["BigIntGetUnsignedArgument"] = 11
	gasMap["BigIntAPICost"]["BigIntGetSignedArgument"] = 13
	gasMap["BigIntAPICost"]["CopyPerByteForTooBig"] = 7
	gasMap["BaseOpsAPICost"]["Int64GetArgument"] = 17
	gasMap["BaseOpsAPICost"]["GetArgument"] = 19
	gasMap["BaseOpsAPICost"]["GetReturnData"] = 23
	gasMap["BaseOperationCost"]["DataCopyPerByte"] = 50
	return gasMap
}

// argumentGetterGasCase is one hook that copies a whole call argument or return data item, sized
// by the caller rather than by the contract, for runArgumentGetterGasCases
type argumentGetterGasCase struct {
	fixedCost uint64
	// perByteFromV6 is what the hook charges, from FixAuditChangesV6, for an item of argLen bytes
	perByteFromV6 func(argLen int) uint64
	// badIndexErr is the fault an out-of-range index raises; nil for a hook that returns silently
	badIndexErr error
	// failed is what the hook returns when it copies nothing
	failed int64
	// returnData serves the item as return data instead of as a call argument
	returnData bool
	call       func(hooks *vmhooks.VMHooksImpl, id int32) int64
	// assertCopied checks the hook's result and destination after it copied arg
	assertCopied func(t *testing.T, host *contextmock.VMHostMock, result int64, arg []byte)
	// assertUntouched checks the destination holds nothing the hook could have copied
	assertUntouched func(t *testing.T, host *contextmock.VMHostMock)
}

// argumentGetterDestPtr is where the legacy getters write; it is pre-filled with
// argumentGetterSentinel, sized to hold the oversized argument, so an unpaid copy shows up
const argumentGetterDestPtr = executor.MemPtr(0)

var (
	argumentGetterSmallArg = []byte{0x01, 0x02, 0x03}
	// 100 KB of leading zeros: still the small value, so the int getters parse it as before, but
	// far past the 32 bytes CopyPerByteForTooBig lets through for free
	argumentGetterOversizedArg = append(make([]byte, 100_000), argumentGetterSmallArg...)
	argumentGetterSentinel     = bytes.Repeat([]byte{0xEE}, len(argumentGetterOversizedArg))
)

func assertArgumentGetterMemoryCopied(t *testing.T, host *contextmock.VMHostMock, result int64, arg []byte) {
	t.Helper()
	assert.Equal(t, int64(len(arg)), result)
	stored, err := host.MemLoadFromMock(argumentGetterDestPtr, len(arg))
	require.NoError(t, err)
	assert.Equal(t, arg, stored)
}

func assertArgumentGetterMemoryUntouched(t *testing.T, host *contextmock.VMHostMock) {
	t.Helper()
	stored, err := host.MemLoadFromMock(argumentGetterDestPtr, len(argumentGetterSentinel))
	require.NoError(t, err)
	assert.Equal(t, argumentGetterSentinel, stored, "contract memory must not change for an unpaid copy")
}

// Before FixAuditChangesV6 the argument and return data getters charge a flat cost whatever the size
// of what they copy, and charge it unbounded; from it the flat cost is bounded and the copy is paid
// for, bounded, before it happens. Pre-fork every total and every effect must stay as it was, or
// blocks from before the fork would stop replaying. From the fork a budget one short of either
// charge must fault with ErrNotEnoughGas and leave the destination as it was.
func runArgumentGetterGasCases(t *testing.T, tc argumentGetterGasCase) {
	t.Helper()

	const plentyOfGas = uint64(100_000_000)
	small, oversized := argumentGetterSmallArg, argumentGetterOversizedArg

	type scenario struct {
		name        string
		arg         []byte
		id          int32
		gasProvided uint64
		// failErr is the fault expected, nil for none; a fault drains the gas left, so the
		// points are only checked when there is none
		failErr error
		// pointsUsed is checked when failErr is nil
		pointsUsed uint64
		copied     bool
	}

	for _, fixV6 := range []bool{false, true} {
		perByte := func(arg []byte) uint64 {
			if !fixV6 {
				return 0
			}
			return tc.perByteFromV6(len(arg))
		}

		scenarios := []scenario{
			{"small", small, 0, plentyOfGas, nil, tc.fixedCost + perByte(small), true},
			{"oversized", oversized, 0, plentyOfGas, nil, tc.fixedCost + perByte(oversized), true},
			{"badIndex", small, 1, plentyOfGas, tc.badIndexErr, tc.fixedCost, false},
		}
		if fixV6 {
			scenarios = append(scenarios,
				scenario{"fixedShortfall", small, 0, tc.fixedCost - 1, vmhost.ErrNotEnoughGas, 0, false},
				scenario{"perByteShortfall", oversized, 0, tc.fixedCost + tc.perByteFromV6(len(oversized)) - 1,
					vmhost.ErrNotEnoughGas, 0, false},
			)
		} else {
			// pre-fork the flat cost overdraws and the copy is free, so neither budget stops it
			scenarios = append(scenarios,
				scenario{"fixedShortfall", small, 0, tc.fixedCost - 1, nil, tc.fixedCost, true},
				scenario{"perByteShortfall", oversized, 0, tc.fixedCost + tc.perByteFromV6(len(oversized)) - 1,
					nil, tc.fixedCost, true},
			)
		}

		for _, sc := range scenarios {
			t.Run(fmt.Sprintf("FixAuditChangesV6=%v/%s", fixV6, sc.name), func(t *testing.T) {
				host, _ := newMockVMHostWithRealMetering(t, fixV6, argumentGetterGasMap(), sc.gasProvided)
				runtime := host.RuntimeContext.(*contextmock.RuntimeContextMock)
				if tc.returnData {
					host.OutputContext.(*contextmock.OutputContextStub).ReturnDataCalled = func() [][]byte {
						return [][]byte{sc.arg}
					}
				} else {
					runtime.VMInput = &vmcommon.ContractCallInput{VMInput: vmcommon.VMInput{Arguments: [][]byte{sc.arg}}}
				}
				require.NoError(t, host.MemStoreToMock(argumentGetterDestPtr, argumentGetterSentinel))

				result := tc.call(vmhooks.NewVMHooksImpl(host), sc.id)

				if sc.failErr != nil {
					require.ErrorIs(t, runtime.FailExecutionErr, sc.failErr)
				} else {
					require.Nil(t, runtime.FailExecutionErr)
					assert.Equal(t, sc.pointsUsed, runtime.GetPointsUsed())
				}

				if sc.copied {
					tc.assertCopied(t, host, result, sc.arg)
					return
				}
				assert.Equal(t, tc.failed, result)
				tc.assertUntouched(t, host)
			})
		}
	}
}

func TestVMHooksImpl_GetArgument_GasPerByteFromV6(t *testing.T) {
	t.Parallel()

	gasMap := argumentGetterGasMap()
	runArgumentGetterGasCases(t, argumentGetterGasCase{
		fixedCost: gasMap["BaseOpsAPICost"]["GetArgument"],
		perByteFromV6: func(argLen int) uint64 {
			return uint64(argLen) * gasMap["BaseOperationCost"]["DataCopyPerByte"]
		},
		badIndexErr: vmhost.ErrInvalidArgument,
		failed:      -1,
		call: func(hooks *vmhooks.VMHooksImpl, id int32) int64 {
			return int64(hooks.GetArgument(id, argumentGetterDestPtr))
		},
		assertCopied:    assertArgumentGetterMemoryCopied,
		assertUntouched: assertArgumentGetterMemoryUntouched,
	})
}

func TestVMHooksImpl_GetReturnData_GasPerByteFromV6(t *testing.T) {
	t.Parallel()

	gasMap := argumentGetterGasMap()
	runArgumentGetterGasCases(t, argumentGetterGasCase{
		fixedCost: gasMap["BaseOpsAPICost"]["GetReturnData"],
		perByteFromV6: func(argLen int) uint64 {
			return uint64(argLen) * gasMap["BaseOperationCost"]["DataCopyPerByte"]
		},
		badIndexErr: vmhost.ErrInvalidArgument,
		failed:      0,
		returnData:  true,
		call: func(hooks *vmhooks.VMHooksImpl, id int32) int64 {
			return int64(hooks.GetReturnData(id, argumentGetterDestPtr))
		},
		assertCopied:    assertArgumentGetterMemoryCopied,
		assertUntouched: assertArgumentGetterMemoryUntouched,
	})
}

// flatCloneGasMap prices the hooks that used to clone a caller-sized handle for a flat cost with
// distinct values, so a test can tell each fixed cost apart from DataCopyPerByte
func flatCloneGasMap() config.GasScheduleMap {
	gasMap := config.MakeGasMapForTests()
	gasMap["BigIntAPICost"]["BigIntUnsignedByteLength"] = 29
	gasMap["BigIntAPICost"]["BigIntSignedByteLength"] = 31
	gasMap["BaseOpsAPICost"]["GetArgument"] = 19
	gasMap["ManagedMapAPICost"]["ManagedMapContains"] = 37
	gasMap["BaseOpsAPICost"]["GetCodeMetadata"] = 41
	gasMap["ManagedBufferAPICost"]["MBufferSetBytes"] = 43
	gasMap["BaseOpsAPICost"]["Int64Finish"] = 47
	gasMap["BaseOpsAPICost"]["GetOwnerAddress"] = 53
	gasMap["BaseOperationCost"]["DataCopyPerByte"] = 50
	return gasMap
}

const (
	// flatCloneOversizedLen is the size of the oversized inputs: a clone of it is plain in the
	// allocation count, which flatCloneNoCloneBelow is far below
	flatCloneOversizedLen = 1 << 20
	flatCloneNoCloneBelow = flatCloneOversizedLen / 4
	flatCloneGasProvided  = uint64(10_000_000_000)
)

// newFlatCloneHost is a mock host on the real metering and managed types, priced by
// flatCloneGasMap, where every API predicate the flat clone hooks use fails execution on an error
func newFlatCloneHost(t *testing.T, fixV6 bool) (*contextmock.VMHostMock, *contextmock.RuntimeContextMock) {
	t.Helper()
	host, _ := newMockVMHostWithRealMetering(t, fixV6, flatCloneGasMap(), flatCloneGasProvided)
	runtime := host.RuntimeContext.(*contextmock.RuntimeContextMock)
	runtime.FailManagedMapAPI = true
	return host, runtime
}

// perByteHookGasMap prices the hooks that charge per byte after the work (until FixAuditChangesV6)
// with distinct fixed costs on top of flatCloneGasMap, and PersistPerByte as in gasScheduleV1
func perByteHookGasMap() config.GasScheduleMap {
	gasMap := flatCloneGasMap()
	gasMap["ManagedMapAPICost"]["ManagedMapPut"] = 59
	gasMap["ManagedMapAPICost"]["ManagedMapGet"] = 61
	gasMap["ManagedMapAPICost"]["ManagedMapRemove"] = 67
	gasMap["BigIntAPICost"]["BigIntFinishUnsigned"] = 71
	gasMap["BigIntAPICost"]["BigIntFinishSigned"] = 73
	gasMap["BaseOpsAPICost"]["SignalError"] = 79
	gasMap["BaseOperationCost"]["PersistPerByte"] = 1_000
	// the real schedule prices an oversized BigInt operand well above a copied byte; keeping that
	// ratio lets BigIntToString's operandShortfall be refused by the operand charge, not the digits
	gasMap["BigIntAPICost"]["CopyPerByteForTooBig"] = 1_000
	return gasMap
}

// newPerByteHost is newFlatCloneHost priced by perByteHookGasMap, with the managed buffer API
// failing execution on an error too, and an output that records what is finished
func newPerByteHost(t *testing.T, fixV6 bool) (*contextmock.VMHostMock, *contextmock.RuntimeContextMock, *[][]byte) {
	t.Helper()
	host, _ := newMockVMHostWithRealMetering(t, fixV6, perByteHookGasMap(), flatCloneGasProvided)
	runtime := host.RuntimeContext.(*contextmock.RuntimeContextMock)
	runtime.FailManagedMapAPI = true
	runtime.FailManagedBuffersAPI = true
	finished := &[][]byte{}
	host.OutputContext = &contextmock.OutputContextStub{FinishCalled: func(data []byte) {
		*finished = append(*finished, data)
	}}
	return host, runtime, finished
}

// runFlatCloneHook calls hook with budget gas left, and returns the gas it used and the bytes it
// allocated. Only for tests that are not parallel, or the other tests' allocations count too
func runFlatCloneHook(host vmhost.VMHost, budget uint64, hook func()) (uint64, uint64) {
	metering := host.Metering()
	metering.UseGas(metering.GasLeft() - budget)
	pointsBefore := host.Runtime().GetPointsUsed()

	var before, after goruntime.MemStats
	goruntime.GC()
	goruntime.ReadMemStats(&before)
	hook()
	goruntime.ReadMemStats(&after)

	return host.Runtime().GetPointsUsed() - pointsBefore, after.TotalAlloc - before.TotalAlloc
}

// ValidateTokenIdentifier cloned the whole buffer to check it, for a flat cost. From
// FixAuditChangesV6 the flat cost is bounded and a buffer longer than any token identifier is
// answered 0 without the clone: every result and every gas total stay as they were. Not parallel:
// it counts allocations
func TestVMHooksImpl_ValidateTokenIdentifier_NoCloneFromV6(t *testing.T) {
	fixedCost := flatCloneGasMap()["BaseOpsAPICost"]["GetArgument"]
	maxLen := vmcommon.TokenIdentifierMaxLength
	oversized := bytes.Repeat([]byte{'A'}, flatCloneOversizedLen)

	type scenario struct {
		name    string
		tokenID []byte
		missing bool
		budget  uint64
		failErr error
		result  int32
		cloned  bool
	}
	validate := func(tokenID []byte) int32 {
		if vmcommon.ValidateToken(tokenID) {
			return 1
		}
		return 0
	}

	for _, fixV6 := range []bool{false, true} {
		scenarios := []scenario{
			{name: "official", tokenID: []byte("KLV"), result: 1},
			{name: "generated", tokenID: generateAssetIdentifier(), result: 1},
			{name: "maxLength", tokenID: []byte("ABCDEFGHIJ-1A2B")},
			{name: "maxLengthBadTicker", tokenID: append(bytes.Repeat([]byte{'a'}, maxLen-5), []byte("-1A2B")...)},
			{name: "maxLengthPlusOne", tokenID: []byte("ABCDEFGHIJK-1A2B")},
			{name: "oversized", tokenID: oversized, cloned: !fixV6},
			{name: "missing", missing: true, failErr: vmhost.ErrNoManagedBufferUnderThisHandle, result: -1},
		}
		if fixV6 {
			scenarios = append(scenarios,
				scenario{name: "fixedShortfall", tokenID: oversized, budget: fixedCost - 1,
					failErr: vmhost.ErrNotEnoughGas, result: -1})
		} else {
			// pre-fork the flat cost overdraws and the hook carries on
			scenarios = append(scenarios,
				scenario{name: "fixedShortfall", tokenID: oversized, budget: fixedCost - 1, cloned: true})
		}

		for _, sc := range scenarios {
			t.Run(fmt.Sprintf("FixAuditChangesV6=%v/%s", fixV6, sc.name), func(t *testing.T) {
				host, runtime := newFlatCloneHost(t, fixV6)
				handle := int32(1234)
				if !sc.missing {
					handle = host.ManagedTypes().NewManagedBufferFromBytes(sc.tokenID)
					if sc.failErr == nil {
						sc.result = validate(sc.tokenID)
					}
				}
				budget := sc.budget
				if budget == 0 {
					budget = 1_000_000
				}

				var result int32
				gasUsed, allocated := runFlatCloneHook(host, budget, func() {
					result = vmhooks.NewVMHooksImpl(host).ValidateTokenIdentifier(handle)
				})

				assert.Equal(t, sc.result, result)
				if sc.failErr != nil {
					require.ErrorIs(t, runtime.FailExecutionErr, sc.failErr)
				} else {
					require.Nil(t, runtime.FailExecutionErr)
					assert.Equal(t, fixedCost, gasUsed)
				}
				if len(sc.tokenID) == flatCloneOversizedLen {
					if sc.cloned {
						assert.GreaterOrEqual(t, allocated, uint64(flatCloneOversizedLen))
					} else {
						assert.Less(t, allocated, uint64(flatCloneNoCloneBelow), "the buffer must not be cloned")
					}
				}
			})
		}
	}
}
