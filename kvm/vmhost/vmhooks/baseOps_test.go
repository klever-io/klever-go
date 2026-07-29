package vmhooks_test

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/kapps"
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
	protectedPrefix := [][]byte{
		[]byte("prefix1"),
		[]byte("prefix2"),
	}

	blockchainHooks := &contextmock.BlockchainHookStub{}
	storageContext, err := contexts.NewStorageContext(host, blockchainHooks, protectedPrefix)
	if err != nil {
		return err
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

	return nil
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
			gasUsed := (uint64(loopGas) * uint64(len(vmInput.KDATransfers))) + callValueGas

			assert.Equal(
				t,
				host.MeteringContext.(*contextmock.MeteringContextMock).GasUsedForExecution(),
				gasUsed,
			)
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
