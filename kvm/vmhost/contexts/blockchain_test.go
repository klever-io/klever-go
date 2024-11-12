package contexts

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/data/transaction"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errTestError = errors.New("some test error")

var testAccounts = []*worldmock.Account{
	{Address: []byte("account_old"), Nonce: 12, Balance: big.NewInt(0)},
	{Address: []byte("account_newer"), Nonce: 8, Balance: big.NewInt(0)},
	{Address: []byte("account_new"), Nonce: 0, Balance: big.NewInt(0)},
	{Address: []byte("account_new_with_money"), Nonce: 0, Balance: big.NewInt(1000)},
	{Address: []byte("account_with_code"), Nonce: 4, Balance: big.NewInt(512), Code: []byte("somecode")},
	{Address: []byte("account_old_with_money"), Nonce: 56, Balance: big.NewInt(1024)},
}

func TestNewBlockchainContext(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostStub{}
	mockWorld := worldmock.NewMockWorld()

	blockchainContext, err := NewBlockchainContext(host, mockWorld)
	require.Nil(t, err)
	require.NotNil(t, blockchainContext)
}

func TestBlockchainContext_AccountExists(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostStub{}
	mockWorld := worldmock.NewMockWorld()
	mockWorld.PutAccounts(testAccounts)

	blockchainContext, _ := NewBlockchainContext(host, mockWorld)

	require.False(t, blockchainContext.AccountExists([]byte("account_missing")))
	require.False(t, blockchainContext.AccountExists([]byte("account_faulty")))
	require.True(t, blockchainContext.AccountExists([]byte("account_old")))

	mockWorld.Err = errTestError
	require.False(t, blockchainContext.AccountExists([]byte("account_something")))
}

func TestBlockchainContext_GetBalance(t *testing.T) {
	t.Parallel()

	mockWorld := worldmock.NewMockWorld()
	mockWorld.PutAccounts(testAccounts)
	mockOutput := &contextmock.OutputContextMock{}
	host := &contextmock.VMHostMock{}
	host.OutputContext = mockOutput
	blockchainContext, _ := NewBlockchainContext(host, mockWorld)

	// Act as if the OutputContext has no OutputAccounts cached
	// (mockOutput.GetOutputAccount() always returns "is new")
	account := &vmcommon.OutputAccount{}
	mockOutput.OutputAccountMock = account
	mockOutput.OutputAccountIsNew = true

	// Test if error is propagated from BlockchainHook
	mockWorld.Err = errTestError
	balanceBytes := blockchainContext.GetBalance([]byte("account_new_with_money"))
	value := big.NewInt(0).SetBytes(balanceBytes)
	require.Equal(t, vmhost.Zero, value)
	mockWorld.Err = nil

	// Test that an account that doesn't exist will not be updated with any kind
	// of balance in the OutputAccounts, but GetBalance() must return 0
	balanceBytes = blockchainContext.GetBalance([]byte("account_missing"))
	value = big.NewInt(0).SetBytes(balanceBytes)
	require.Equal(t, vmhost.Zero, value)

	// Act as if the OutputContext has the requested OutputAccount cached
	mockOutput.OutputAccountIsNew = false
	mockWorld.PutAccount(&worldmock.Account{Address: []byte("any account"), Balance: big.NewInt(42)})
	balanceBytes = blockchainContext.GetBalance([]byte("any account"))
	value = big.NewInt(0).SetBytes(balanceBytes)
	require.Equal(t, big.NewInt(42), value)
}

func TestBlockchainContext_GetBalance_Updates(t *testing.T) {
	t.Parallel()

	mockWorld := worldmock.NewMockWorld()
	mockWorld.PutAccounts(testAccounts)
	mockOutput := &contextmock.OutputContextMock{}
	host := &contextmock.VMHostMock{}
	host.OutputContext = mockOutput
	blockchainContext, _ := NewBlockchainContext(host, mockWorld)

	// Act as if the OutputContext has no OutputAccounts cached
	// (mockOutput.GetOutputAccount() always returns "is new")
	account := &vmcommon.OutputAccount{
		Address:        []byte("account_new_with_money"),
		StorageUpdates: make(map[string]*vmcommon.StorageUpdate),
	}

	mockOutput.OutputAccountMock = account
	mockOutput.OutputAccountIsNew = false

	balanceBytes := blockchainContext.GetBalance([]byte("account_new_with_money"))
	value := big.NewInt(0).SetBytes(balanceBytes)
	require.Equal(t, big.NewInt(1000), value)
}
func TestBlockchainContext_GetCodeHashAndSize(t *testing.T) {
	t.Parallel()

	mockCrypto := &contextmock.CryptoHookMock{}

	mockWorld := worldmock.NewMockWorld()
	mockWorld.PutAccounts(testAccounts)

	outputContext := &contextmock.OutputContextMock{}

	host := &contextmock.VMHostMock{}
	host.CryptoHook = mockCrypto
	host.OutputContext = outputContext

	blockchainContext, _ := NewBlockchainContext(host, mockWorld)

	address := []byte("account_with_code")
	expectedCode := []byte("somecode")
	expectedCodeHash := defaultHasher.Compute(string(expectedCode))

	// GetCode: Test if error is propagated from blockchain hook
	outputContext.OutputAccountIsNew = true
	outputContext.OutputAccountMock = &vmcommon.OutputAccount{}
	mockWorld.Err = errTestError
	code, err := blockchainContext.GetCode(address)
	require.NotNil(t, err)
	require.Nil(t, code)
	mockWorld.Err = nil

	// GetCode: Test success
	outputContext.OutputAccountIsNew = true
	outputContext.OutputAccountMock = &vmcommon.OutputAccount{}
	code, err = blockchainContext.GetCode(address)
	require.Nil(t, err)
	require.Equal(t, expectedCode, code)

	// GetCodeHash: Test if error is propagated from blockchain hook
	outputContext.OutputAccountIsNew = true
	outputContext.OutputAccountMock = &vmcommon.OutputAccount{}
	mockWorld.Err = errTestError
	codeHash := blockchainContext.GetCodeHash(address)
	require.Nil(t, codeHash)
	mockWorld.Err = nil

	// GetCodeHash: Test success
	outputContext.OutputAccountIsNew = true
	outputContext.OutputAccountMock = &vmcommon.OutputAccount{}
	codeHash = blockchainContext.GetCodeHash(address)

	require.Equal(t, len(expectedCodeHash), len(codeHash))
	require.Nil(t, err)

	// GetCodeSize: Test if error is propagated from blockchain hook
	outputContext.OutputAccountIsNew = true
	outputContext.OutputAccountMock = &vmcommon.OutputAccount{}
	mockWorld.Err = errTestError
	size, err := blockchainContext.GetCodeSize(address)
	require.Equal(t, errTestError, err)
	require.Equal(t, int32(0), size)
	mockWorld.Err = nil

	// GetCodeSize: Test success
	outputContext.OutputAccountIsNew = true
	outputContext.OutputAccountMock = &vmcommon.OutputAccount{}
	size, err = blockchainContext.GetCodeSize(address)
	require.Nil(t, err)
	require.Equal(t, int32(len(expectedCode)), size)
}

func TestBlockchainContext_NewAddress(t *testing.T) {
	t.Parallel()

	mockOutput := &contextmock.OutputContextMock{}

	mockWorld := worldmock.NewMockWorld()
	mockWorld.PutAccounts(testAccounts)

	mockRuntime := &contextmock.RuntimeContextMock{}
	mockRuntime.VMType = []byte{0xF, 0xF}

	host := &contextmock.VMHostMock{
		OutputContext:  mockOutput,
		RuntimeContext: mockRuntime,
	}

	// Test error propagation from GetNonce()
	blockchainContext, _ := NewBlockchainContext(host, mockWorld)
	creatorAddress := []byte("account_new")
	creatorAccount := mockWorld.GetAccount(creatorAddress)
	creatorOutputAccount := mockOutput.NewVMOutputAccountFromMockAccount(creatorAccount)
	mockOutput.OutputAccountMock = creatorOutputAccount
	mockOutput.OutputAccountIsNew = true
	mockWorld.Err = errTestError

	address, err := blockchainContext.NewAddress(creatorAddress)
	require.Equal(t, errTestError, err)
	require.Nil(t, address)

	mockWorld.Err = nil

	// Test if nonce is not deducted if 0, before calling BlockchainHook.NewAddres()
	creatorAddress = []byte("account_new")
	creatorAccount = mockWorld.GetAccount(creatorAddress)
	creatorOutputAccount = mockOutput.NewVMOutputAccountFromMockAccount(creatorAccount)
	mockOutput.OutputAccountMock = creatorOutputAccount
	mockOutput.OutputAccountIsNew = true

	expectedCreatorAddres := creatorAddress
	stubBlockchain := &contextmock.BlockchainHookStub{
		GetUserAccountCalled: mockWorld.GetUserAccount,
		NewAddressCalled: func(creatorAddress []byte, creatorNonce uint64, vmType []byte, randomSeed []byte) ([]byte, error) {
			require.Equal(t, expectedCreatorAddres, creatorAddress)
			require.Equal(t, uint64(0), creatorNonce)
			require.Equal(t, mockRuntime.VMType, vmType)
			require.Equal(t, []byte("seed"), randomSeed)
			return []byte("new_address"), nil
		},
	}
	blockchainContext, _ = NewBlockchainContext(host, stubBlockchain)

	address, err = blockchainContext.NewAddress(creatorAddress)
	require.Nil(t, err)
	require.Equal(t, []byte("new_address"), address)

	// Test if nonce is correctly deducted if greater than 0, before calling BlockchainHook.NewAddres()
	creatorAddress = []byte("account_old_with_money")
	creatorAccount = mockWorld.GetAccount(creatorAddress)
	creatorOutputAccount = mockOutput.NewVMOutputAccountFromMockAccount(creatorAccount)
	mockOutput.OutputAccountMock = creatorOutputAccount
	mockOutput.OutputAccountIsNew = false

	expectedCreatorAddres = creatorAddress
	stubBlockchain = &contextmock.BlockchainHookStub{
		GetUserAccountCalled: mockWorld.GetUserAccount,
		NewAddressCalled: func(creatorAddress []byte, creatorNonce uint64, vmType []byte, randomSeed []byte) ([]byte, error) {
			require.Equal(t, expectedCreatorAddres, creatorAddress)
			require.Equal(t, uint64(56), creatorNonce)
			require.Equal(t, mockRuntime.VMType, vmType)
			require.Equal(t, []byte("seed"), randomSeed)
			return []byte("new_address"), nil
		},
	}
	blockchainContext, _ = NewBlockchainContext(host, stubBlockchain)

	address, err = blockchainContext.NewAddress(creatorAddress)
	require.Nil(t, err)
	require.Equal(t, []byte("new_address"), address)

	// Test if error is propagated from Blockchain.NewAddress
	creatorAddress = []byte("account_with_code")
	creatorAccount = mockWorld.GetAccount(creatorAddress)
	creatorOutputAccount = mockOutput.NewVMOutputAccountFromMockAccount(creatorAccount)
	mockOutput.OutputAccountMock = creatorOutputAccount
	mockOutput.OutputAccountIsNew = false

	expectedCreatorAddres = creatorAddress
	stubBlockchain = &contextmock.BlockchainHookStub{
		GetUserAccountCalled: mockWorld.GetUserAccount,
		NewAddressCalled: func(creatorAddress []byte, creatorNonce uint64, vmType []byte, randomSeed []byte) ([]byte, error) {
			require.Equal(t, expectedCreatorAddres, creatorAddress)
			require.Equal(t, uint64(4), creatorNonce)
			require.Equal(t, mockRuntime.VMType, vmType)
			require.Equal(t, []byte("seed"), randomSeed)
			return nil, errTestError
		},
	}
	blockchainContext, _ = NewBlockchainContext(host, stubBlockchain)

	address, err = blockchainContext.NewAddress(creatorAddress)
	require.Equal(t, errTestError, err)
	require.Nil(t, address)
}

func TestBlockchainContext_BlockHash(t *testing.T) {
	t.Parallel()

	// FIXME rewrite this test to use absolute block nonces
	host := &contextmock.VMHostMock{}
	mockWorld := worldmock.NewMockWorld()
	blockchainContext, _ := NewBlockchainContext(host, mockWorld)

	mockWorld.Err = errTestError
	hash := blockchainContext.BlockHash(42)
	require.Nil(t, hash)
	mockWorld.Err = nil

	mockWorld.SetCurrentBlockHash([]byte("1234fa"))
	hash = blockchainContext.BlockHash(3)
	require.Nil(t, hash)

	mockWorld.SetCurrentBlockHash([]byte("1234fb"))
	hash = blockchainContext.BlockHash(0)
	require.Equal(t, []byte("1234fb"), hash)

	mockWorld.SetCurrentBlockHash([]byte("1234fc"))
	hash = blockchainContext.BlockHash(42)
	require.Nil(t, hash)
}

func TestBlockchainContext_IsPayable(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostMock{}
	mockWorld := worldmock.NewMockWorld()
	accounts := []*worldmock.Account{
		{Address: []byte("test"), CodeMetadata: []byte{0, vmcommon.MetadataPayable}},
	}
	mockWorld.PutAccounts(accounts)

	bc, _ := NewBlockchainContext(host, mockWorld)

	isPayable, err := bc.IsPayable(nil, []byte("test"))
	require.Nil(t, err)
	require.True(t, isPayable)
}

func TestBlockchainContext_KDATransfer(t *testing.T) {
	t.Parallel()
	t.Run("Should call KDATransfer Hook", func(t *testing.T) {
		host := &contextmock.VMHostMock{}
		stub := &contextmock.BlockchainHookStub{}
		bc, _ := NewBlockchainContext(host, stub)
		called := false
		stub.KDATransferCalled = func(sender []byte, tc *transaction.TransferContract) error {
			called = true
			return nil
		}
		err := bc.KDATransfer([]byte("sender"), &transaction.TransferContract{})
		assert.Nil(t, err)
		assert.True(t, called)
	})

	t.Run("Should propagate errors", func(t *testing.T) {
		host := &contextmock.VMHostMock{}
		stub := &contextmock.BlockchainHookStub{}
		bc, _ := NewBlockchainContext(host, stub)
		stub.KDATransferCalled = func(sender []byte, tc *transaction.TransferContract) error {
			return fmt.Errorf("KDATransfer error")
		}
		err := bc.KDATransfer([]byte("sender"), &transaction.TransferContract{})
		assert.NotNil(t, err)
		assert.Equal(t, "KDATransfer error", err.Error())
	})
}

func TestBlockchainContext_Getters(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostMock{}
	var randomSeed1 [48]byte
	copy(randomSeed1[:], "last random seed                                ")
	var randomSeed2 [48]byte
	copy(randomSeed2[:], "current random seed                             ")

	mockWorld := &worldmock.MockWorld{
		PreviousBlockInfo: &worldmock.BlockInfo{
			BlockTimestamp: 6749,
			BlockNonce:     90,
			BlockSlot:      96,
			BlockEpoch:     3,
			RandomSeed:     &randomSeed1,
		},
		CurrentBlockInfo: &worldmock.BlockInfo{
			BlockTimestamp: 6800,
			BlockNonce:     98,
			BlockSlot:      99,
			BlockEpoch:     4,
			RandomSeed:     &randomSeed2,
		},
		StateRootHash: []byte("root hash"),
	}

	blockchainContext, _ := NewBlockchainContext(host, mockWorld)

	require.Equal(t, uint32(3), blockchainContext.LastEpoch())
	require.Equal(t, uint32(4), blockchainContext.CurrentEpoch())

	require.Equal(t, uint64(90), blockchainContext.LastNonce())
	require.Equal(t, uint64(98), blockchainContext.CurrentNonce())

	require.Equal(t, uint64(96), blockchainContext.LastSlot())
	require.Equal(t, uint64(99), blockchainContext.CurrentSlot())

	require.Equal(t, int64(6749), blockchainContext.LastTimeStamp())
	require.Equal(t, int64(6800), blockchainContext.CurrentTimeStamp())

	require.Equal(t, []byte("root hash"), blockchainContext.GetStateRootHash())
	require.Equal(t, randomSeed1[:], blockchainContext.LastRandomSeed())
	require.Equal(t, randomSeed2[:], blockchainContext.CurrentRandomSeed())
}
