package counters_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/smartContract/hooks/counters"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	maxBuiltinCalls = "MaxBuiltInCallsPerTx"
	maxTransfers    = "MaxNumberOfTransfersPerTx"
	maxTrieReads    = "MaxNumberOfTrieReadsPerTx"
	crtBuiltinCalls = "CrtBuiltInCallsPerTx"
	crtTransfers    = "CrtNumberOfTransfersPerTx"
	crtTrieReads    = "CrtNumberOfTrieReadsPerTx"
)

func generateMapsOfValues(value uint64) map[string]uint64 {
	return map[string]uint64{
		maxTrieReads:    value,
		maxTransfers:    value,
		maxBuiltinCalls: value,
	}
}

func generatePendingTransfers(numTransfers int) []*vmcommon.KDATransfer {
	transfers := make([]*vmcommon.KDATransfer, 0, numTransfers)
	for i := 0; i < numTransfers; i++ {
		transfers = append(transfers, &vmcommon.KDATransfer{
			KDATokenName: []byte("KLV"),
			KDAValue:     big.NewInt(1),
		})
	}

	return transfers
}

func argumentsForTwoTransfers() [][]byte {
	return [][]byte{
		{2},
		[]byte("KLV"), {}, {1},
		[]byte("KFI"), {}, {1},
	}
}

func TestCheckMaximumValues(t *testing.T) {
	t.Parallel()

	assert.Nil(t, counters.CheckMaximumValues(generateMapsOfValues(1)))
	assert.ErrorIs(t, counters.CheckMaximumValues(nil), counters.ErrInvalidMaximumValue)

	for _, identifier := range []string{maxBuiltinCalls, maxTransfers, maxTrieReads} {
		missing := generateMapsOfValues(1)
		delete(missing, identifier)
		err := counters.CheckMaximumValues(missing)
		assert.ErrorIs(t, err, counters.ErrInvalidMaximumValue)
		assert.Contains(t, err.Error(), identifier)

		zero := generateMapsOfValues(1)
		zero[identifier] = 0
		assert.ErrorIs(t, counters.CheckMaximumValues(zero), counters.ErrInvalidMaximumValue)
	}
}

func TestNewUsageCounter(t *testing.T) {
	t.Parallel()

	t.Run("nil kda transfer parser should error", func(t *testing.T) {
		t.Parallel()

		counter, err := counters.NewUsageCounter(nil, mock.NewForkControllerStub())
		assert.True(t, check.IfNil(counter))
		assert.Equal(t, process.ErrNilKDATransferParser, err)
	})
	t.Run("nil fork controller should error", func(t *testing.T) {
		t.Parallel()

		counter, err := counters.NewUsageCounter(&KDATransferParserStub{}, nil)
		assert.True(t, check.IfNil(counter))
		assert.Equal(t, process.ErrNilForkController, err)
	})
	t.Run("should work", func(t *testing.T) {
		t.Parallel()

		counter, err := counters.NewUsageCounter(&KDATransferParserStub{}, mock.NewForkControllerStub())
		assert.False(t, check.IfNil(counter))
		assert.Nil(t, err)
	})
}

func TestUsageCounter_ProcessCrtNumberOfTrieReadsCounter(t *testing.T) {
	t.Parallel()

	counter, _ := counters.NewUsageCounter(&KDATransferParserStub{}, mock.NewForkControllerStub())
	counter.SetMaximumValues(generateMapsOfValues(3))

	assert.Nil(t, counter.ProcessCrtNumberOfTrieReadsCounter())                                 // counter is now 1
	assert.Nil(t, counter.ProcessCrtNumberOfTrieReadsCounter())                                 // counter is now 2
	assert.Nil(t, counter.ProcessCrtNumberOfTrieReadsCounter())                                 // counter is now 3
	assert.ErrorIs(t, counter.ProcessCrtNumberOfTrieReadsCounter(), process.ErrMaxCallsReached) // counter is now 4, error signalled
	err := counter.ProcessCrtNumberOfTrieReadsCounter()                                         // counter is now 5, error signalled
	assert.ErrorIs(t, err, process.ErrMaxCallsReached)
	t.Run("backwards compatibility on error message string", func(t *testing.T) {
		assert.Equal(t, "max calls reached: too many reads from trie", err.Error())
	})

	countersMap := counter.GetCounterValues()
	expectedMap := map[string]uint64{
		crtBuiltinCalls: 0,
		crtTransfers:    0,
		crtTrieReads:    5,
	}
	assert.Equal(t, expectedMap, countersMap)

	counter.ResetCounters()

	assert.Nil(t, counter.ProcessCrtNumberOfTrieReadsCounter()) // counter is now 1
	countersMap = counter.GetCounterValues()
	expectedMap = map[string]uint64{
		crtBuiltinCalls: 0,
		crtTransfers:    0,
		crtTrieReads:    1,
	}
	assert.Equal(t, expectedMap, countersMap)
}

func TestUsageCounter_ProcessMaxBuiltInCounters(t *testing.T) {
	t.Parallel()

	t.Run("builtin functions exceeded", func(t *testing.T) {
		t.Parallel()

		counter, _ := counters.NewUsageCounter(&KDATransferParserStub{}, mock.NewForkControllerStub())
		counter.SetMaximumValues(generateMapsOfValues(3))

		vmInput := &vmcommon.ContractCallInput{}

		assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))                                 // counter is now 1
		assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))                                 // counter is now 2
		assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))                                 // counter is now 3
		assert.ErrorIs(t, counter.ProcessMaxBuiltInCounters(vmInput), process.ErrMaxCallsReached) // counter is now 4, error signalled
		err := counter.ProcessMaxBuiltInCounters(vmInput)                                         // counter is now 5, error signalled
		require.ErrorIs(t, err, process.ErrMaxCallsReached)
		t.Run("backwards compatibility on error message string", func(t *testing.T) {
			assert.Equal(t, "max calls reached: too many built-in functions calls", err.Error())
		})

		countersMap := counter.GetCounterValues()
		expectedMap := map[string]uint64{
			crtBuiltinCalls: 5,
			crtTransfers:    0,
			crtTrieReads:    0,
		}
		assert.Equal(t, expectedMap, countersMap)

		counter.ResetCounters()

		assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput)) // counter is now 1
		countersMap = counter.GetCounterValues()
		expectedMap = map[string]uint64{
			crtBuiltinCalls: 1,
			crtTransfers:    0,
			crtTrieReads:    0,
		}
		assert.Equal(t, expectedMap, countersMap)
	})
	t.Run("number of transfers exceeded", func(t *testing.T) {
		t.Parallel()

		counter, _ := counters.NewUsageCounter(&KDATransferParserStub{
			ParseKDATransfersCalled: func(rcvAddr []byte, args [][]byte) (*vmcommon.ParsedKDATransfers, error) {
				t.Fatal("arguments must not be parsed")
				return nil, nil
			},
		}, mock.NewForkControllerStub())
		values := generateMapsOfValues(6)
		values[maxBuiltinCalls] = 1000
		counter.SetMaximumValues(values)

		vmInput := &vmcommon.ContractCallInput{
			VMInput:  vmcommon.VMInput{KDATransfers: generatePendingTransfers(2)},
			Function: core.BuiltInFunctionTransfer,
		}

		assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))                                 // counter is now 2
		assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))                                 // counter is now 4
		assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))                                 // counter is now 6
		assert.ErrorIs(t, counter.ProcessMaxBuiltInCounters(vmInput), process.ErrMaxCallsReached) // counter is now 8, error signalled
		err := counter.ProcessMaxBuiltInCounters(vmInput)                                         // counter is now 10, error signalled
		require.ErrorIs(t, err, process.ErrMaxCallsReached)
		t.Run("backwards compatibility on error message string", func(t *testing.T) {
			assert.Equal(t, "max calls reached: too many KDA transfers", err.Error())
		})

		countersMap := counter.GetCounterValues()
		expectedMap := map[string]uint64{
			crtBuiltinCalls: 5,
			crtTransfers:    10,
			crtTrieReads:    0,
		}
		assert.Equal(t, expectedMap, countersMap)

		counter.ResetCounters()

		assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput)) // counter is now 2
		countersMap = counter.GetCounterValues()
		expectedMap = map[string]uint64{
			crtBuiltinCalls: 1,
			crtTransfers:    2,
			crtTrieReads:    0,
		}
		assert.Equal(t, expectedMap, countersMap)
	})
}

func TestNonTransferBuiltinNotCountedAsTransfer(t *testing.T) {
	t.Parallel()

	counter, _ := counters.NewUsageCounter(&KDATransferParserStub{
		ParseKDATransfersCalled: func(rcvAddr []byte, args [][]byte) (*vmcommon.ParsedKDATransfers, error) {
			return &vmcommon.ParsedKDATransfers{
				KDATransfers: make([]*vmcommon.KDATransfer, 2),
			}, nil
		},
	}, mock.NewForkControllerStub().SetFork("FixAuditChangesV3", true).SetFork("FixAuditChangesV7", false))
	counter.SetMaximumValues(generateMapsOfValues(100))

	vmInput := &vmcommon.ContractCallInput{Function: core.BuiltInFunctionFreeze}

	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))
	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))

	countersMap := counter.GetCounterValues()
	expectedMap := map[string]uint64{
		crtBuiltinCalls: 2,
		crtTransfers:    0,
		crtTrieReads:    0,
	}
	assert.Equal(t, expectedMap, countersMap)
}

func TestBeforeForkTransferShapedBuiltinIsCountedAsTransfer(t *testing.T) {
	t.Parallel()

	counter, _ := counters.NewUsageCounter(&KDATransferParserStub{
		ParseKDATransfersCalled: func(rcvAddr []byte, args [][]byte) (*vmcommon.ParsedKDATransfers, error) {
			return &vmcommon.ParsedKDATransfers{
				KDATransfers: make([]*vmcommon.KDATransfer, 2),
			}, nil
		},
	}, mock.NewForkControllerStub().SetFork("FixAuditChangesV3", false).SetFork("FixAuditChangesV7", false))
	counter.SetMaximumValues(generateMapsOfValues(100))

	vmInput := &vmcommon.ContractCallInput{Function: core.BuiltInFunctionFreeze}

	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))
	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))

	countersMap := counter.GetCounterValues()
	expectedMap := map[string]uint64{
		crtBuiltinCalls: 2,
		crtTransfers:    4,
		crtTrieReads:    0,
	}
	assert.Equal(t, expectedMap, countersMap)
}

func TestMultiTransferStillCounted(t *testing.T) {
	t.Parallel()

	counter, _ := counters.NewUsageCounter(&KDATransferParserStub{
		ParseKDATransfersCalled: func(rcvAddr []byte, args [][]byte) (*vmcommon.ParsedKDATransfers, error) {
			t.Fatal("arguments must not be parsed")
			return nil, nil
		},
	}, mock.NewForkControllerStub())
	values := generateMapsOfValues(6)
	values[maxBuiltinCalls] = 1000
	counter.SetMaximumValues(values)

	vmInput := &vmcommon.ContractCallInput{
		VMInput:  vmcommon.VMInput{KDATransfers: generatePendingTransfers(2)},
		Function: core.BuiltInFunctionTransfer,
	}

	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))                                 // transfers now 2
	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))                                 // transfers now 4
	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))                                 // transfers now 6
	assert.ErrorIs(t, counter.ProcessMaxBuiltInCounters(vmInput), process.ErrMaxCallsReached) // transfers now 8, over cap

	countersMap := counter.GetCounterValues()
	expectedMap := map[string]uint64{
		crtBuiltinCalls: 4,
		crtTransfers:    8,
		crtTrieReads:    0,
	}
	assert.Equal(t, expectedMap, countersMap)
}

func TestBeforeForkMultiTransferStillCounted(t *testing.T) {
	t.Parallel()

	counter, _ := counters.NewUsageCounter(&KDATransferParserStub{
		ParseKDATransfersCalled: func(rcvAddr []byte, args [][]byte) (*vmcommon.ParsedKDATransfers, error) {
			return &vmcommon.ParsedKDATransfers{
				KDATransfers: make([]*vmcommon.KDATransfer, 2),
			}, nil
		},
	}, mock.NewForkControllerStub().SetFork("FixAuditChangesV3", false).SetFork("FixAuditChangesV7", false))
	values := generateMapsOfValues(6)
	values[maxBuiltinCalls] = 1000
	counter.SetMaximumValues(values)

	vmInput := &vmcommon.ContractCallInput{Function: core.BuiltInFunctionTransfer}

	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))                                 // transfers now 2
	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))                                 // transfers now 4
	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))                                 // transfers now 6
	assert.ErrorIs(t, counter.ProcessMaxBuiltInCounters(vmInput), process.ErrMaxCallsReached) // transfers now 8, over cap

	countersMap := counter.GetCounterValues()
	expectedMap := map[string]uint64{
		crtBuiltinCalls: 4,
		crtTransfers:    8,
		crtTrieReads:    0,
	}
	assert.Equal(t, expectedMap, countersMap)
}

func TestBeforeForkTransferCountBeyondArgumentsIsNotCounted(t *testing.T) {
	t.Parallel()

	parser, err := parsers.NewKDATransferParser(&marshal.ProtoMarshalizer{})
	assert.Nil(t, err)
	counter, _ := counters.NewUsageCounter(parser, mock.NewForkControllerStub().SetFork("FixAuditChangesV3", false).SetFork("FixAuditChangesV7", false))
	counter.SetMaximumValues(generateMapsOfValues(100))

	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			Arguments: [][]byte{
				big.NewInt(0).SetUint64(0x5555555555555556).Bytes(),
				[]byte("KLV"), {}, {1},
			},
		},
		Function: core.BuiltInFunctionFreeze,
	}

	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))

	countersMap := counter.GetCounterValues()
	expectedMap := map[string]uint64{
		crtBuiltinCalls: 1,
		crtTransfers:    0,
		crtTrieReads:    0,
	}
	assert.Equal(t, expectedMap, countersMap)
}

func TestPendingTransfersCountedAsTransfers(t *testing.T) {
	t.Parallel()

	parser, err := parsers.NewKDATransferParser(&marshal.ProtoMarshalizer{})
	assert.Nil(t, err)
	counter, _ := counters.NewUsageCounter(parser, mock.NewForkControllerStub())
	values := generateMapsOfValues(6)
	values[maxBuiltinCalls] = 1000
	counter.SetMaximumValues(values)

	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			Arguments:    make([][]byte, 0),
			KDATransfers: generatePendingTransfers(3),
		},
		Function: core.BuiltInFunctionTransfer,
	}

	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))
	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))
	err = counter.ProcessMaxBuiltInCounters(vmInput)
	require.ErrorIs(t, err, process.ErrMaxCallsReached)
	assert.Equal(t, "max calls reached: too many KDA transfers", err.Error())

	countersMap := counter.GetCounterValues()
	expectedMap := map[string]uint64{
		crtBuiltinCalls: 3,
		crtTransfers:    9,
		crtTrieReads:    0,
	}
	assert.Equal(t, expectedMap, countersMap)
}

func TestTransfersUpToMaximumAreAllowed(t *testing.T) {
	t.Parallel()

	counter, _ := counters.NewUsageCounter(&KDATransferParserStub{}, mock.NewForkControllerStub())
	values := generateMapsOfValues(250)
	values[maxBuiltinCalls] = 1000
	counter.SetMaximumValues(values)

	atMaximum := &vmcommon.ContractCallInput{
		VMInput:  vmcommon.VMInput{KDATransfers: generatePendingTransfers(250)},
		Function: core.BuiltInFunctionTransfer,
	}
	oneMore := &vmcommon.ContractCallInput{
		VMInput:  vmcommon.VMInput{KDATransfers: generatePendingTransfers(1)},
		Function: core.BuiltInFunctionTransfer,
	}

	assert.Nil(t, counter.ProcessMaxBuiltInCounters(atMaximum))
	require.ErrorIs(t, counter.ProcessMaxBuiltInCounters(oneMore), process.ErrMaxCallsReached)

	countersMap := counter.GetCounterValues()
	expectedMap := map[string]uint64{
		crtBuiltinCalls: 2,
		crtTransfers:    251,
		crtTrieReads:    0,
	}
	assert.Equal(t, expectedMap, countersMap)
}

func TestExecutedTransfersNotCounted(t *testing.T) {
	t.Parallel()

	counter, _ := counters.NewUsageCounter(&KDATransferParserStub{}, mock.NewForkControllerStub())
	counter.SetMaximumValues(generateMapsOfValues(100))

	transfers := generatePendingTransfers(3)
	transfers[0].SetExecuted()
	transfers[2].SetExecuted()
	vmInput := &vmcommon.ContractCallInput{
		VMInput:  vmcommon.VMInput{KDATransfers: transfers},
		Function: core.BuiltInFunctionTransfer,
	}

	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))

	countersMap := counter.GetCounterValues()
	expectedMap := map[string]uint64{
		crtBuiltinCalls: 1,
		crtTransfers:    1,
		crtTrieReads:    0,
	}
	assert.Equal(t, expectedMap, countersMap)
}

func TestArgumentTransfersCountedOnce(t *testing.T) {
	t.Parallel()

	parser, err := parsers.NewKDATransferParser(&marshal.ProtoMarshalizer{})
	assert.Nil(t, err)
	counter, _ := counters.NewUsageCounter(parser, mock.NewForkControllerStub())
	counter.SetMaximumValues(generateMapsOfValues(100))

	settledTransfers := generatePendingTransfers(1)
	settledTransfers[0].SetExecuted()
	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			Arguments:    argumentsForTwoTransfers(),
			KDATransfers: settledTransfers,
		},
		Function: core.BuiltInFunctionTransfer,
	}
	parsedTransfers, err := parser.ParseKDATransfers(vmInput.RecipientAddr, vmInput.Arguments)
	assert.Nil(t, err)
	vmInput.KDATransfers = append(vmInput.KDATransfers, parsedTransfers.KDATransfers...)

	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))

	countersMap := counter.GetCounterValues()
	expectedMap := map[string]uint64{
		crtBuiltinCalls: 1,
		crtTransfers:    2,
		crtTrieReads:    0,
	}
	assert.Equal(t, expectedMap, countersMap)
}

func TestNonTransferBuiltinPendingTransfersNotCounted(t *testing.T) {
	t.Parallel()

	counter, _ := counters.NewUsageCounter(&KDATransferParserStub{}, mock.NewForkControllerStub())
	counter.SetMaximumValues(generateMapsOfValues(100))

	vmInput := &vmcommon.ContractCallInput{
		VMInput:  vmcommon.VMInput{KDATransfers: generatePendingTransfers(2)},
		Function: core.BuiltInFunctionFreeze,
	}

	assert.Nil(t, counter.ProcessMaxBuiltInCounters(vmInput))

	countersMap := counter.GetCounterValues()
	expectedMap := map[string]uint64{
		crtBuiltinCalls: 1,
		crtTransfers:    0,
		crtTrieReads:    0,
	}
	assert.Equal(t, expectedMap, countersMap)
}

func TestBeforeForkOnlyArgumentTransfersCounted(t *testing.T) {
	t.Parallel()

	parser, err := parsers.NewKDATransferParser(&marshal.ProtoMarshalizer{})
	assert.Nil(t, err)
	counter, _ := counters.NewUsageCounter(parser, mock.NewForkControllerStub().SetFork("FixAuditChangesV7", false))
	counter.SetMaximumValues(generateMapsOfValues(100))

	typedInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			Arguments:    make([][]byte, 0),
			KDATransfers: generatePendingTransfers(3),
		},
		Function: core.BuiltInFunctionTransfer,
	}
	assert.Nil(t, counter.ProcessMaxBuiltInCounters(typedInput))

	argumentsInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			Arguments:    argumentsForTwoTransfers(),
			KDATransfers: generatePendingTransfers(1),
		},
		Function: core.BuiltInFunctionTransfer,
	}
	assert.Nil(t, counter.ProcessMaxBuiltInCounters(argumentsInput))

	countersMap := counter.GetCounterValues()
	expectedMap := map[string]uint64{
		crtBuiltinCalls: 2,
		crtTransfers:    2,
		crtTrieReads:    0,
	}
	assert.Equal(t, expectedMap, countersMap)
}

// KDATransferParserStub -
type KDATransferParserStub struct {
	ParseKDATransfersCalled func(rcvAddr []byte, args [][]byte) (*vmcommon.ParsedKDATransfers, error)
}

// ParseESDTTransfers -
func (stub *KDATransferParserStub) ParseKDATransfers(rcvAddr []byte, args [][]byte) (*vmcommon.ParsedKDATransfers, error) {
	if stub.ParseKDATransfersCalled != nil {
		return stub.ParseKDATransfersCalled(rcvAddr, args)
	}

	return nil, errors.New("not implemented")
}

// IsInterfaceNil -
func (stub *KDATransferParserStub) IsInterfaceNil() bool {
	return stub == nil
}
