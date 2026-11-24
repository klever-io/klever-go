package contexts

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

func TestNewOutputContext(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostStub{}

	outputContext, err := NewOutputContext(host)
	require.Nil(t, err)
	require.NotNil(t, outputContext)

	require.Equal(t, vmcommon.Ok, outputContext.ReturnCode())
	require.NotNil(t, outputContext.ReturnData())
	require.Equal(t, 0, len(outputContext.ReturnData()))
	require.Equal(t, "", outputContext.ReturnMessage())
	require.Equal(t, 0, len(outputContext.stateStack))

	require.NotNil(t, outputContext.outputState.OutputAccounts)
	require.Equal(t, 0, len(outputContext.outputState.OutputAccounts))

	vmOutput := outputContext.outputState
	require.NotNil(t, vmOutput.DeletedAccounts)
	require.Equal(t, 0, len(vmOutput.DeletedAccounts))
	require.NotNil(t, vmOutput.Logs)
	require.Equal(t, 0, len(vmOutput.Logs))
}

func TestOutputContext_PushPopState(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostStub{}
	host.RuntimeCalled = func() vmhost.RuntimeContext {
		return &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}
	}
	outputContext, _ := NewOutputContext(host)

	address1 := []byte("address1")
	address2 := []byte("address2")

	// Create an account with nonce 99 on the active state.
	_, isNew := outputContext.GetOutputAccount(address1)
	require.True(t, isNew)
	require.Equal(t, 1, len(outputContext.outputState.OutputAccounts))

	// Copy active state onto the stack.
	outputContext.PushState()
	require.Equal(t, 1, len(outputContext.stateStack))

	// Clear the active state and create a new account with the same address as
	// the previous; the new account must not have nonce 99.
	outputContext.InitState()
	_, isNew = outputContext.GetOutputAccount(address1)
	require.True(t, isNew)
	require.Equal(t, 1, len(outputContext.outputState.OutputAccounts))

	// Copy active state onto the stack, then create a new account with nonce 42.
	outputContext.PushState()
	require.Equal(t, 2, len(outputContext.stateStack))

	_, isNew = outputContext.GetOutputAccount(address2)
	require.True(t, isNew)
	require.Equal(t, 2, len(outputContext.outputState.OutputAccounts))

	// Revert to the previous state: account with nonce 42 is lost, and the
	// account with "address1" has nonce 84.
	outputContext.PopSetActiveState()
	_, isNew = outputContext.GetOutputAccount(address1)
	require.False(t, isNew)
	require.Equal(t, 1, len(outputContext.outputState.OutputAccounts))
	require.Equal(t, 1, len(outputContext.stateStack))

	outputContext.PushState()
	require.Equal(t, 2, len(outputContext.stateStack))

	outputContext.PopDiscard()
	require.Equal(t, 1, len(outputContext.stateStack))

	_, isNew = outputContext.GetOutputAccount(address1)
	require.False(t, isNew)
	require.Equal(t, 1, len(outputContext.outputState.OutputAccounts))
	require.Equal(t, 1, len(outputContext.stateStack))

	outputContext.ClearStateStack()
	require.Equal(t, 0, len(outputContext.stateStack))
}

func TestOutputContext_GetOutputAccount(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostStub{}
	outputContext, _ := NewOutputContext(host)
	require.Zero(t, len(outputContext.outputState.OutputAccounts))

	// Request an account that is missing from OutputAccounts
	account, isNew := outputContext.GetOutputAccount([]byte("account"))
	require.Equal(t, 1, len(outputContext.outputState.OutputAccounts))
	require.True(t, isNew)
	require.Equal(t, []byte("account"), account.Address)
	require.Zero(t, len(account.StorageUpdates))

	// Change fields of the OutputAccount to ensure it will be returned on the
	// next call to GetOutputAccount(), from the OutputAccounts cache
	account.Address = []byte("changed address")
	cachedAccount, isNew := outputContext.GetOutputAccount([]byte("account"))
	require.False(t, isNew)
	require.Equal(t, []byte("changed address"), cachedAccount.Address)
	require.Zero(t, len(cachedAccount.StorageUpdates))
}

func TestOutputContext_GettersAndSetters(t *testing.T) {
	host := &contextmock.VMHostStub{}
	outputContext, _ := NewOutputContext(host)

	outputContext.SetReturnCode(vmcommon.VMExecutionFailed)
	require.Equal(t, vmcommon.VMExecutionFailed, outputContext.ReturnCode())

	outputContext.SetReturnMessage("rockets")
	require.Equal(t, "rockets", outputContext.ReturnMessage())
}

func TestOutputContext_FinishReturnData(t *testing.T) {
	host := &contextmock.VMHostStub{}
	outputContext, _ := NewOutputContext(host)

	require.Zero(t, len(outputContext.ReturnData()))

	outputContext.Finish([]byte("something"))
	expectedData := [][]byte{
		[]byte("something"),
	}
	require.Equal(t, expectedData, outputContext.ReturnData())

	outputContext.Finish([]byte("something else"))
	expectedData = append(expectedData, []byte("something else"))
	require.Equal(t, expectedData, outputContext.ReturnData())

	outputContext.Finish(big.NewInt(1234567).Bytes())
	expectedData = append(expectedData, big.NewInt(1234567).Bytes())
	require.Equal(t, expectedData, outputContext.ReturnData())

	outputContext.Finish([]byte{})
	expectedData = append(expectedData, []byte{})
	require.Equal(t, expectedData, outputContext.ReturnData())

	outputContext.ClearReturnData()
	require.Zero(t, len(outputContext.ReturnData()))
}

func TestOutputContext_MergeCompleteAccounts(t *testing.T) {
	t.Parallel()

	transfer1 := vmcommon.OutputTransfer{
		KDATransfers: vmcommon.KDATransfer{
			KDAValue: big.NewInt(1),
		},
	}
	left := &vmcommon.OutputAccount{
		Address:                 []byte("addr1"),
		StorageUpdates:          nil,
		Code:                    []byte("code1"),
		OutputTransfers:         []vmcommon.OutputTransfer{transfer1},
		BytesAddedToStorage:     10,
		BytesDeletedFromStorage: 5,
	}
	right := &vmcommon.OutputAccount{
		Address:                 []byte("addr2"),
		StorageUpdates:          map[string]*vmcommon.StorageUpdate{"key": {Data: []byte("data"), Offset: []byte("offset")}},
		Code:                    []byte("code2"),
		OutputTransfers:         []vmcommon.OutputTransfer{transfer1, transfer1},
		BytesAddedToStorage:     4,
		BytesDeletedFromStorage: 12,
	}

	expected := &vmcommon.OutputAccount{
		Address:                 []byte("addr2"),
		StorageUpdates:          map[string]*vmcommon.StorageUpdate{"key": {Data: []byte("data"), Offset: []byte("offset")}},
		Code:                    []byte("code2"),
		OutputTransfers:         []vmcommon.OutputTransfer{transfer1, transfer1},
		BytesAddedToStorage:     left.BytesAddedToStorage,
		BytesDeletedFromStorage: right.BytesDeletedFromStorage,
	}

	mergeOutputAccounts(left, right, false)
	require.Equal(t, expected, left)
}

func TestOutputContext_MergeIncompleteAccounts(t *testing.T) {
	t.Parallel()

	left := &vmcommon.OutputAccount{}
	right := &vmcommon.OutputAccount{}
	expected := &vmcommon.OutputAccount{
		StorageUpdates: make(map[string]*vmcommon.StorageUpdate),
	}
	mergeOutputAccounts(left, right, false)
	require.Equal(t, expected, left)

	left = &vmcommon.OutputAccount{
		OutputTransfers: []vmcommon.OutputTransfer{{Index: 1}},
	}
	right = &vmcommon.OutputAccount{}
	expected = &vmcommon.OutputAccount{
		OutputTransfers: []vmcommon.OutputTransfer{{Index: 1}},
		StorageUpdates:  make(map[string]*vmcommon.StorageUpdate),
	}
	mergeOutputAccounts(left, right, false)
	require.Equal(t, expected, left)

	left = &vmcommon.OutputAccount{}
	right = &vmcommon.OutputAccount{}
	expected = &vmcommon.OutputAccount{
		StorageUpdates: make(map[string]*vmcommon.StorageUpdate),
	}
	mergeOutputAccounts(left, right, false)
	require.Equal(t, expected, left)

	left = &vmcommon.OutputAccount{
		Address: []byte("left address"),
		Code:    []byte("left code"),
	}
	right = &vmcommon.OutputAccount{}
	expected = &vmcommon.OutputAccount{
		Code:           []byte("left code"),
		StorageUpdates: make(map[string]*vmcommon.StorageUpdate),
		Address:        left.Address,
	}
	mergeOutputAccounts(left, right, false)
	require.Equal(t, expected, left)

	left = &vmcommon.OutputAccount{
		OutputTransfers: []vmcommon.OutputTransfer{{SenderAddress: []byte("left data")}},
	}
	right = &vmcommon.OutputAccount{}
	expected = &vmcommon.OutputAccount{
		OutputTransfers: []vmcommon.OutputTransfer{{SenderAddress: []byte("left data")}},
		StorageUpdates:  make(map[string]*vmcommon.StorageUpdate),
	}
	mergeOutputAccounts(left, right, false)
	require.Equal(t, expected, left)
}

func TestOutputContext_MergeVMOutputs(t *testing.T) {
	t.Parallel()

	left := newVMOutput()
	right := newVMOutput()
	expected := newVMOutput()
	mergeVMOutputs(left, right)
	require.Equal(t, expected, left)

	left = newVMOutput()
	right = newVMOutput()
	right.OutputAccounts["address"] = NewVMOutputAccount([]byte("address"))
	expected = newVMOutput()
	expected.OutputAccounts["address"] = NewVMOutputAccount([]byte("address"))
	mergeVMOutputs(left, right)
	require.Equal(t, expected, left)

	left = newVMOutput()
	left.OutputAccounts["address"] = NewVMOutputAccount([]byte("address"))
	right = newVMOutput()
	right.OutputAccounts["address"] = NewVMOutputAccount([]byte("address"))
	expected = newVMOutput()
	expected.OutputAccounts["address"] = NewVMOutputAccount([]byte("address"))
	mergeVMOutputs(left, right)
	require.Equal(t, expected, left)

	left = newVMOutput()
	left.OutputAccounts["left address"] = NewVMOutputAccount([]byte("left address"))
	right = newVMOutput()
	right.OutputAccounts["right address"] = NewVMOutputAccount([]byte("right address"))
	expected = newVMOutput()
	expected.OutputAccounts["left address"] = NewVMOutputAccount([]byte("left address"))
	expected.OutputAccounts["right address"] = NewVMOutputAccount([]byte("right address"))
	mergeVMOutputs(left, right)
	require.Equal(t, expected, left)

	left = newVMOutput()
	left.GasRemaining = 99
	left.ReturnCode = vmcommon.VMContractNotFound
	left.ReturnMessage = "fireflies"
	right = newVMOutput()
	right.ReturnData = [][]byte{[]byte("rockets"), []byte("albatross")}
	right.GasRemaining = 100
	right.ReturnCode = vmcommon.VMExecutionFailed
	right.ReturnMessage = "turtles all the way down"
	expected = newVMOutput()
	expected.ReturnData = [][]byte{[]byte("rockets"), []byte("albatross")}
	expected.GasRemaining = 100
	expected.ReturnCode = vmcommon.VMExecutionFailed
	expected.ReturnMessage = "turtles all the way down"
	mergeVMOutputs(left, right)
	require.Equal(t, expected, left)
}

func TestOutputContext_VMOutputError(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostMock{
		MeteringContext: &contextmock.MeteringContextMock{},
		RuntimeContext: &contextmock.RuntimeContextMock{
			VMInput: &vmcommon.ContractCallInput{},
		},
	}

	outputContext, _ := NewOutputContext(host)

	returnCode := vmcommon.VMContractNotFound
	returnMessage := vmhost.ErrContractNotFound.Error()

	expected := &vmcommon.VMOutput{
		GasRemaining:  0,
		ReturnCode:    returnCode,
		ReturnMessage: returnMessage,
	}
	vmOutput := outputContext.CreateVMOutputInCaseOfError(vmhost.ErrContractNotFound)
	require.Equal(t, expected, vmOutput)
}

func TestOutputContext_Transfer(t *testing.T) {
	t.Parallel()

	sender := []byte("sender")
	receiver := []byte("receiver")
	balance := big.NewInt(10000)
	valueToTransfer := big.NewInt(1000)

	host := &contextmock.VMHostMock{ForkControllerContext: commonMock.NewForkControllerStub()}
	host.RuntimeContext = &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}
	mockWorld := worldmock.NewMockWorld()
	mockWorld.PutAccount(&worldmock.Account{
		Address: sender,
		Balance: balance,
	})
	mockWorld.PutAccount(&worldmock.Account{
		Address: receiver,
		Balance: balance,
	})

	blockchainContext, _ := NewBlockchainContext(host, mockWorld)
	outputContext, _ := NewOutputContext(host)

	host.OutputContext = outputContext
	host.BlockchainContext = blockchainContext

	err := outputContext.TransferValueOnly(receiver, sender, valueToTransfer, true)
	require.Nil(t, err)

	senderAccount, isNew := outputContext.GetOutputAccount(sender)
	require.False(t, isNew)
	_ = senderAccount

	destAccount, isNew := outputContext.GetOutputAccount(receiver)
	require.False(t, isNew)
	_ = destAccount
}

func TestOutputContext_Transfer_Errors_And_Checks(t *testing.T) {
	t.Parallel()

	sender := []byte("sender")
	receiver := []byte("receiver")

	mockWorld := worldmock.NewMockWorld()
	mockWorld.PutAccount(&worldmock.Account{
		Address: sender,
		Balance: big.NewInt(2000),
	})

	host := &contextmock.VMHostMock{ForkControllerContext: commonMock.NewForkControllerStub()}
	outputContext, _ := NewOutputContext(host)
	blockchainContext, _ := NewBlockchainContext(host, mockWorld)

	host.RuntimeContext = &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}
	host.OutputContext = outputContext
	host.BlockchainContext = blockchainContext

	senderOutputAccount, _ := outputContext.GetOutputAccount(sender)
	_ = senderOutputAccount

	// negative transfers are disallowed
	valueToTransfer := big.NewInt(-1000)
	err := outputContext.TransferValueOnly(receiver, sender, valueToTransfer, true)
	require.Equal(t, vmhost.ErrTransferNegativeValue, err)

	// account must have enough money to transfer
	valueToTransfer = big.NewInt(5000)
	err = outputContext.TransferValueOnly(receiver, sender, valueToTransfer, true)
	require.Equal(t, vmhost.ErrTransferInsufficientFunds, err)

	valueToTransfer = big.NewInt(5000)
	mockWorld.PutAccount(&worldmock.Account{Address: sender, Balance: big.NewInt(6000)})
	err = outputContext.TransferValueOnly(receiver, sender, valueToTransfer, true)
	require.Nil(t, err)

	require.Equal(t, big.NewInt(1000), blockchainContext.GetBalanceBigInt(sender))
}

func TestOutputContext_Transfer_IsAccountPayable(t *testing.T) {
	t.Parallel()

	sender := []byte("sender")
	receiverNonPayable := make([]byte, 32)
	receiverPayable := make([]byte, 32)
	receiverPayable[31] = 1

	mockWorld := worldmock.NewMockWorld()
	mockWorld.PutAccounts([]*worldmock.Account{
		{
			Address: sender,
			Nonce:   0,
			Balance: big.NewInt(2000),
		},
		{
			Address:         receiverNonPayable,
			Nonce:           0,
			Balance:         big.NewInt(0),
			Code:            []byte("contract_code"),
			IsSmartContract: true,
		},
		{
			Address:         receiverPayable,
			Nonce:           0,
			Balance:         big.NewInt(0),
			Code:            []byte("contract_code"),
			CodeMetadata:    []byte{0, vmcommon.MetadataPayable},
			IsSmartContract: true,
		},
	})

	host := &contextmock.VMHostMock{ForkControllerContext: commonMock.NewForkControllerStub()}
	oc, _ := NewOutputContext(host)
	bc, _ := NewBlockchainContext(host, mockWorld)

	host.OutputContext = oc
	host.BlockchainContext = bc
	host.RuntimeContext = &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}

	valueToTransfer := big.NewInt(10)
	err := oc.TransferValueOnly(receiverNonPayable, sender, valueToTransfer, true)

	require.Equal(t, vmhost.ErrAccountNotPayable, err)

	valueToTransfer = big.NewInt(0)
	err = oc.TransferValueOnly(receiverNonPayable, sender, valueToTransfer, true)
	require.Nil(t, err)

	valueToTransfer = big.NewInt(10)
	err = oc.TransferValueOnly(receiverPayable, sender, valueToTransfer, true)

	require.Nil(t, err)
}

// TestOutputContext_TransferValueOnly_FixAuditV6_RecordsACopy pins that, once FixAuditChangesV6 is
// active, the recorded output transfer does not share the caller's big int: a nested call's value
// is the caller's own managed big int, which the caller gets back after the call (KLC-2678).
func TestOutputContext_TransferValueOnly_FixAuditV6_RecordsACopy(t *testing.T) {
	t.Parallel()

	sender := []byte("sender")
	receiver := []byte("receiver")

	recordedTransferValue := func(fixAuditV6 bool, value *big.Int) *big.Int {
		mockWorld := worldmock.NewMockWorld()
		mockWorld.PutAccount(&worldmock.Account{Address: sender, Balance: big.NewInt(10000)})
		mockWorld.PutAccount(&worldmock.Account{Address: receiver, Balance: big.NewInt(0)})

		host := &contextmock.VMHostMock{
			ForkControllerContext: &commonMock.ForkControllerStub{FixAuditChangesV6Value: fixAuditV6},
		}
		host.RuntimeContext = &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}
		outputContext, _ := NewOutputContext(host)
		blockchainContext, _ := NewBlockchainContext(host, mockWorld)
		host.OutputContext = outputContext
		host.BlockchainContext = blockchainContext

		require.NoError(t, outputContext.TransferValueOnly(receiver, sender, value, false))

		receiverAccount, _ := outputContext.GetOutputAccount(receiver)
		require.Len(t, receiverAccount.OutputTransfers, 1)
		return receiverAccount.OutputTransfers[0].KDATransfers.KDAValue
	}

	t.Run("from FixAuditChangesV6 the amount is copied", func(t *testing.T) {
		t.Parallel()

		value := big.NewInt(1000)
		recorded := recordedTransferValue(true, value)
		value.SetInt64(1_000_000)

		require.NotSame(t, value, recorded)
		require.Equal(t, big.NewInt(1000), recorded)
	})

	t.Run("before FixAuditChangesV6 the pointer is recorded", func(t *testing.T) {
		t.Parallel()

		value := big.NewInt(1000)
		require.Same(t, value, recordedTransferValue(false, value))
	})
}

func TestOutputContext_WriteLog(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostMock{
		RuntimeContext: &contextmock.RuntimeContextMock{
			CallFunction: "function",
		},
	}
	outputContext, _ := NewOutputContext(host)

	address := []byte("address")
	data := []byte("data")
	topics := make([][]byte, 0)

	outputContext.WriteLog(address, topics, [][]byte{data})
	require.Equal(t, len(outputContext.outputState.Logs), 1)
	require.Equal(t, outputContext.outputState.Logs[0].Address, address)
	//require.Equal(t, outputContext.outputState.Logs[0].GetFirstDataItem(), data)
	require.Equal(t, outputContext.outputState.Logs[0].Identifier, []byte("function"))
	require.Empty(t, outputContext.outputState.Logs[0].Topics)

	topic := []byte("topic")
	topics = [][]byte{}
	outputContext.WriteLog(address, topics, [][]byte{data})

	require.Equal(t, outputContext.outputState.Logs[1].Identifier, []byte("function"))
	require.Empty(t, outputContext.outputState.Logs[1].Topics)

	topics = append(topics, topic)
	outputContext.WriteLog(address, topics, [][]byte{data})

	require.Equal(t, outputContext.outputState.Logs[2].Topics, [][]byte{topic})
}

func TestOutputContext_PopSetActiveStateIfStackIsEmptyShouldNotPanic(t *testing.T) {
	t.Parallel()

	outputContext, _ := NewOutputContext(&contextmock.VMHostMock{})
	outputContext.PopSetActiveState()

	require.Equal(t, 0, len(outputContext.stateStack))
}

func TestOutputContext_PopMergeActiveStateIfStackIsEmptyShouldNotPanic(t *testing.T) {
	t.Parallel()

	outputContext, _ := NewOutputContext(&contextmock.VMHostMock{})
	outputContext.PopMergeActiveState()

	require.Equal(t, 0, len(outputContext.stateStack))
}

func TestOutputContext_PopDiscardIfStackIsEmptyShouldNotPanic(t *testing.T) {
	t.Parallel()

	outputContext, _ := NewOutputContext(&contextmock.VMHostMock{})
	outputContext.PopDiscard()

	require.Equal(t, 0, len(outputContext.stateStack))
}

func TestOutputContext_SetOutputAccount(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostMock{}
	outputContext, _ := NewOutputContext(host)

	address := []byte("address")
	account := NewVMOutputAccount(address)

	// invalid address, should not save
	outputContext.SetOutputAccount([]byte(""), account)
	accounts := outputContext.GetOutputAccounts()
	_, ok := accounts[""]
	require.False(t, ok)

	// invalid data, should not save
	outputContext.SetOutputAccount(address, nil)
	accounts = outputContext.GetOutputAccounts()
	_, ok = accounts[string(address)]
	require.False(t, ok)

	// valid data
	outputContext.SetOutputAccount(address, account)
	outputAccount, _ := outputContext.GetOutputAccount(address)

	require.Equal(t, account, outputAccount)
}

func TestOutputContext_PendingCodeUpdateLifecycle(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostStub{}
	host.RuntimeCalled = func() vmhost.RuntimeContext {
		return &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}
	}
	outputContext, _ := NewOutputContext(host)

	address := []byte("pending-deploy-address")

	require.False(t, outputContext.HasPendingCodeUpdate(address))

	outputContext.DeployCode(vmhost.CodeDeployInput{
		ContractAddress:      address,
		ContractCode:         []byte("code"),
		ContractCodeMetadata: []byte("metadata"),
		CodeDeployerAddress:  []byte("deployer"),
	})
	require.True(t, outputContext.HasPendingCodeUpdate(address))

	outputContext.DeleteOutputAccount(address)
	require.False(t, outputContext.HasPendingCodeUpdate(address))

	outputContext.DeployCode(vmhost.CodeDeployInput{
		ContractAddress:      address,
		ContractCode:         []byte("code"),
		ContractCodeMetadata: []byte("metadata"),
		CodeDeployerAddress:  []byte("deployer"),
	})
	require.True(t, outputContext.HasPendingCodeUpdate(address))

	outputContext.InitState()
	require.False(t, outputContext.HasPendingCodeUpdate(address))
}

func TestOutputContext_PendingCodeUpdateSurvivesNestedCall(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostStub{}
	host.RuntimeCalled = func() vmhost.RuntimeContext {
		return &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}
	}
	outputContext, _ := NewOutputContext(host)

	address := []byte("pending-deploy-address")

	outputContext.DeployCode(vmhost.CodeDeployInput{
		ContractAddress:      address,
		ContractCode:         []byte("code"),
		ContractCodeMetadata: []byte("metadata"),
		CodeDeployerAddress:  []byte("deployer"),
	})

	outputContext.PushState()

	require.True(t, outputContext.HasPendingCodeUpdate(address))

	outputContext.PopSetActiveState()

	require.True(t, outputContext.HasPendingCodeUpdate(address))
}

func TestOutputContext_PendingCodeUpdateFromNestedCallDiscardedOnRollback(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostStub{}
	host.RuntimeCalled = func() vmhost.RuntimeContext {
		return &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}
	}
	outputContext, _ := NewOutputContext(host)

	address := []byte("pending-deploy-address")

	outputContext.PushState()

	outputContext.DeployCode(vmhost.CodeDeployInput{
		ContractAddress:      address,
		ContractCode:         []byte("code"),
		ContractCodeMetadata: []byte("metadata"),
		CodeDeployerAddress:  []byte("deployer"),
	})
	require.True(t, outputContext.HasPendingCodeUpdate(address))

	outputContext.PopSetActiveState()

	require.False(t, outputContext.HasPendingCodeUpdate(address))
}

func TestOutputContext_PendingCodeUpdateFromNestedCallKeptOnMerge(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostStub{}
	host.RuntimeCalled = func() vmhost.RuntimeContext {
		return &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}
	}
	outputContext, _ := NewOutputContext(host)

	address := []byte("pending-deploy-address")

	outputContext.PushState()

	outputContext.DeployCode(vmhost.CodeDeployInput{
		ContractAddress:      address,
		ContractCode:         []byte("code"),
		ContractCodeMetadata: []byte("metadata"),
		CodeDeployerAddress:  []byte("deployer"),
	})

	outputContext.PopMergeActiveState()

	require.True(t, outputContext.HasPendingCodeUpdate(address))
	require.Equal(t, []byte("code"), outputContext.GetOutputAccounts()[string(address)].Code)
}

func TestOutputContext_PendingCodeUpdateFromNestedCallKeptOnDiscard(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostStub{}
	host.RuntimeCalled = func() vmhost.RuntimeContext {
		return &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}
	}
	outputContext, _ := NewOutputContext(host)

	address := []byte("pending-deploy-address")

	outputContext.PushState()

	outputContext.DeployCode(vmhost.CodeDeployInput{
		ContractAddress:      address,
		ContractCode:         []byte("code"),
		ContractCodeMetadata: []byte("metadata"),
		CodeDeployerAddress:  []byte("deployer"),
	})

	outputContext.PopDiscard()

	require.True(t, outputContext.HasPendingCodeUpdate(address))
	require.Equal(t, []byte("code"), outputContext.GetOutputAccounts()[string(address)].Code)
	require.Equal(t, 0, len(outputContext.codeUpdatesStack))
}

func newOutputContextWithFixAuditChangesV6(fixAuditChangesV6 bool) *outputContext {
	forkController := commonMock.NewForkControllerStub()
	forkController.FixAuditChangesV6Value = fixAuditChangesV6

	host := &contextmock.VMHostStub{}
	host.RuntimeCalled = func() vmhost.RuntimeContext {
		return &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}
	}
	host.ForkControllerCalled = func() core.ForkController {
		return forkController
	}
	outputContext, _ := NewOutputContext(host)

	return outputContext
}

func runSuccessfulNestedCalls(outputContext *outputContext, numCalls int) {
	for i := 0; i < numCalls; i++ {
		outputContext.PushState()
		outputContext.CensorVMOutput()
		outputContext.PopMergeActiveState()
	}
}

func TestOutputContext_DeletedAccountsStayLinearAcrossNestedCalls(t *testing.T) {
	t.Parallel()

	outputContext := newOutputContextWithFixAuditChangesV6(true)
	deletedAddress := []byte("deleted-address")

	outputContext.PushState()
	outputContext.CensorVMOutput()
	outputContext.outputState.DeletedAccounts = append(outputContext.outputState.DeletedAccounts, deletedAddress)
	outputContext.PopMergeActiveState()

	runSuccessfulNestedCalls(outputContext, 20)

	require.Equal(t, [][]byte{deletedAddress}, outputContext.outputState.DeletedAccounts)
}

func TestOutputContext_CensorVMOutputHidesParentDeletedAccounts(t *testing.T) {
	t.Parallel()

	outputContext := newOutputContextWithFixAuditChangesV6(true)
	parentDeleted := []byte("parent-deleted")
	childDeleted := []byte("child-deleted")
	outputContext.outputState.DeletedAccounts = [][]byte{parentDeleted}

	outputContext.PushState()
	outputContext.CensorVMOutput()
	require.Empty(t, outputContext.outputState.DeletedAccounts)

	outputContext.outputState.DeletedAccounts = append(outputContext.outputState.DeletedAccounts, childDeleted)
	outputContext.PopMergeActiveState()

	require.Equal(t, [][]byte{parentDeleted, childDeleted}, outputContext.outputState.DeletedAccounts)
}

func TestOutputContext_FailedNestedCallKeepsParentDeletedAccounts(t *testing.T) {
	t.Parallel()

	outputContext := newOutputContextWithFixAuditChangesV6(true)
	parentDeleted := []byte("parent-deleted")
	outputContext.outputState.DeletedAccounts = [][]byte{parentDeleted}

	outputContext.PushState()
	outputContext.CensorVMOutput()
	outputContext.outputState.DeletedAccounts = append(outputContext.outputState.DeletedAccounts, []byte("child-deleted"))
	outputContext.PopSetActiveState()

	require.Equal(t, [][]byte{parentDeleted}, outputContext.outputState.DeletedAccounts)
}

func TestOutputContext_DeletedAccountsInheritedBeforeFixAuditChangesV6(t *testing.T) {
	t.Parallel()

	outputContext := newOutputContextWithFixAuditChangesV6(false)
	deletedAddress := []byte("deleted-address")

	outputContext.PushState()
	outputContext.CensorVMOutput()
	outputContext.outputState.DeletedAccounts = append(outputContext.outputState.DeletedAccounts, deletedAddress)
	outputContext.PopMergeActiveState()

	runSuccessfulNestedCalls(outputContext, 3)

	require.Len(t, outputContext.outputState.DeletedAccounts, 8)
	for _, address := range outputContext.outputState.DeletedAccounts {
		require.Equal(t, deletedAddress, address)
	}
}

func newOutputContextForOwnershipTests() *outputContext {
	host := &contextmock.VMHostMock{ForkControllerContext: commonMock.NewForkControllerStub()}
	host.RuntimeContext = &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}
	outputContext, _ := NewOutputContext(host)
	host.OutputContext = outputContext

	return outputContext
}

func overwriteBytes(data []byte) {
	for i := range data {
		data[i] = 'X'
	}
}

func requireSameByteShape(t *testing.T, expected []byte, actual []byte) {
	t.Helper()

	if expected == nil {
		require.Nil(t, actual)
		return
	}

	require.NotNil(t, actual)
	require.Len(t, actual, len(expected))
	require.True(t, bytes.Equal(expected, actual))
}

func TestNewVMOutputAccount_KeepsItsOwnAddress(t *testing.T) {
	t.Parallel()

	address := []byte("address")
	account := NewVMOutputAccount(address)
	overwriteBytes(address)

	require.Equal(t, []byte("address"), account.Address)
}

func TestOutputContext_Finish_KeepsItsOwnData(t *testing.T) {
	t.Parallel()

	outputContext := newOutputContextForOwnershipTests()
	data := []byte("data")
	outputContext.Finish(data)
	overwriteBytes(data)

	require.Equal(t, [][]byte{[]byte("data")}, outputContext.ReturnData())
}

func TestOutputContext_PrependFinish_KeepsItsOwnData(t *testing.T) {
	t.Parallel()

	outputContext := newOutputContextForOwnershipTests()
	outputContext.Finish([]byte("existing"))
	data := []byte("data")
	outputContext.PrependFinish(data)
	overwriteBytes(data)

	require.Equal(t, [][]byte{[]byte("data"), []byte("existing")}, outputContext.ReturnData())
}

func TestOutputContext_WriteLogWithIdentifier_KeepsItsOwnCopies(t *testing.T) {
	t.Parallel()

	newInputs := func() ([]byte, []byte, [][]byte, [][]byte) {
		return []byte("address"),
			[]byte("identifier"),
			[][]byte{[]byte("topic1"), []byte("topic2")},
			[][]byte{[]byte("data1"), []byte("data2")}
	}

	t.Run("mutating the source bytes leaves the entry unchanged", func(t *testing.T) {
		t.Parallel()

		outputContext := newOutputContextForOwnershipTests()
		address, identifier, topics, data := newInputs()
		outputContext.WriteLogWithIdentifier(address, topics, data, identifier)

		overwriteBytes(address)
		overwriteBytes(identifier)
		overwriteBytes(topics[0])
		overwriteBytes(topics[1])
		overwriteBytes(data[0])
		overwriteBytes(data[1])

		require.Len(t, outputContext.outputState.Logs, 1)
		entry := outputContext.outputState.Logs[0]
		expectedAddress, expectedIdentifier, expectedTopics, expectedData := newInputs()
		require.Equal(t, expectedAddress, entry.Address)
		require.Equal(t, expectedIdentifier, entry.Identifier)
		require.Equal(t, expectedTopics, entry.Topics)
		require.Equal(t, expectedData, entry.Data)
	})

	t.Run("replacing outer elements leaves the entry unchanged", func(t *testing.T) {
		t.Parallel()

		outputContext := newOutputContextForOwnershipTests()
		address, identifier, topics, data := newInputs()
		outputContext.WriteLogWithIdentifier(address, topics, data, identifier)

		topics[0] = []byte("replaced")
		topics[1] = nil
		data[0] = []byte("replaced")
		data[1] = nil

		entry := outputContext.outputState.Logs[0]
		_, _, expectedTopics, expectedData := newInputs()
		require.Equal(t, expectedTopics, entry.Topics)
		require.Equal(t, expectedData, entry.Data)
	})
}

func TestOutputContext_TransferValueOnly_KeepsItsOwnCopies(t *testing.T) {
	t.Parallel()

	sender := []byte("sender")
	receiver := []byte("receiver")

	mockWorld := worldmock.NewMockWorld()
	mockWorld.PutAccount(&worldmock.Account{Address: []byte("sender"), Balance: big.NewInt(10000)})
	mockWorld.PutAccount(&worldmock.Account{Address: []byte("receiver"), Balance: big.NewInt(0)})

	host := &contextmock.VMHostMock{ForkControllerContext: commonMock.NewForkControllerStub()}
	host.RuntimeContext = &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}
	outputContext, _ := NewOutputContext(host)
	blockchainContext, _ := NewBlockchainContext(host, mockWorld)
	host.OutputContext = outputContext
	host.BlockchainContext = blockchainContext

	require.NoError(t, outputContext.TransferValueOnly(receiver, sender, big.NewInt(1000), false))

	overwriteBytes(sender)
	overwriteBytes(receiver)

	receiverAccount, _ := outputContext.GetOutputAccount([]byte("receiver"))
	require.Len(t, receiverAccount.OutputTransfers, 1)
	require.Equal(t, []byte("sender"), receiverAccount.OutputTransfers[0].SenderAddress)
	require.Equal(t, []byte("receiver"), receiverAccount.OutputTransfers[0].RcvAddr)
	tokenName := receiverAccount.OutputTransfers[0].KDATransfers.KDATokenName
	require.Equal(t, kdautils.KLVIdentifier, tokenName)
	require.NotSame(t, &kdautils.KLVIdentifier[0], &tokenName[0])
}

func TestOutputContext_TransferValueOnly_KeepsNilAndEmptyAddressesAsGiven(t *testing.T) {
	t.Parallel()

	newOutput := func() *outputContext {
		mockWorld := worldmock.NewMockWorld()
		mockWorld.PutAccount(&worldmock.Account{Address: []byte("other"), Balance: big.NewInt(10000)})
		mockWorld.PutAccount(&worldmock.Account{Address: []byte{}, Balance: big.NewInt(10000)})

		host := &contextmock.VMHostMock{ForkControllerContext: commonMock.NewForkControllerStub()}
		host.RuntimeContext = &contextmock.RuntimeContextMock{VMInput: &vmcommon.ContractCallInput{}}
		outputContext, _ := NewOutputContext(host)
		blockchainContext, _ := NewBlockchainContext(host, mockWorld)
		host.OutputContext = outputContext
		host.BlockchainContext = blockchainContext

		return outputContext
	}

	addressCases := []struct {
		name  string
		input func() []byte
	}{
		{name: "nil", input: func() []byte { return nil }},
		{name: "empty", input: func() []byte { return []byte{} }},
	}

	for _, tc := range addressCases {
		tc := tc
		t.Run("sender is "+tc.name, func(t *testing.T) {
			t.Parallel()

			outputContext := newOutput()
			require.NoError(t, outputContext.TransferValueOnly([]byte("other"), tc.input(), big.NewInt(0), false))

			account, _ := outputContext.GetOutputAccount([]byte("other"))
			require.Len(t, account.OutputTransfers, 1)
			requireSameByteShape(t, tc.input(), account.OutputTransfers[0].SenderAddress)
		})

		t.Run("destination is "+tc.name, func(t *testing.T) {
			t.Parallel()

			outputContext := newOutput()
			require.NoError(t, outputContext.TransferValueOnly(tc.input(), []byte("other"), big.NewInt(0), false))

			account, _ := outputContext.GetOutputAccount(tc.input())
			require.Len(t, account.OutputTransfers, 1)
			requireSameByteShape(t, tc.input(), account.OutputTransfers[0].RcvAddr)
		})
	}
}

func TestOutputContext_DeployCode_KeepsItsOwnCopies(t *testing.T) {
	t.Parallel()

	outputContext := newOutputContextForOwnershipTests()
	input := vmhost.CodeDeployInput{
		ContractCode:         []byte("code"),
		ContractCodeMetadata: []byte("metadata"),
		ContractAddress:      []byte("contract"),
		CodeDeployerAddress:  []byte("deployer"),
	}
	outputContext.DeployCode(input)

	overwriteBytes(input.ContractCode)
	overwriteBytes(input.ContractCodeMetadata)
	overwriteBytes(input.CodeDeployerAddress)

	account, _ := outputContext.GetOutputAccount([]byte("contract"))
	require.Equal(t, []byte("code"), account.Code)
	require.Equal(t, []byte("metadata"), account.CodeMetadata)
	require.Equal(t, []byte("deployer"), account.CodeDeployerAddress)
}

func TestOutputContext_MergeOutputAccounts_KeepsItsOwnCopies(t *testing.T) {
	t.Parallel()

	left := &vmcommon.OutputAccount{Address: []byte("left")}
	right := &vmcommon.OutputAccount{
		Address:             []byte("right"),
		Code:                []byte("code"),
		CodeMetadata:        []byte("metadata"),
		CodeDeployerAddress: []byte("deployer"),
	}

	mergeOutputAccounts(left, right, false)
	overwriteBytes(right.Address)
	overwriteBytes(right.Code)
	overwriteBytes(right.CodeMetadata)
	overwriteBytes(right.CodeDeployerAddress)

	require.Equal(t, []byte("right"), left.Address)
	require.Equal(t, []byte("code"), left.Code)
	require.Equal(t, []byte("metadata"), left.CodeMetadata)
	require.Equal(t, []byte("deployer"), left.CodeDeployerAddress)
}

func TestOutputContext_RecordedOutputKeepsNilAndEmptyAsGiven(t *testing.T) {
	t.Parallel()

	byteSliceCases := []struct {
		name  string
		input func() []byte
	}{
		{name: "nil", input: func() []byte { return nil }},
		{name: "empty", input: func() []byte { return []byte{} }},
		{name: "non-empty", input: func() []byte { return []byte("value") }},
	}

	byteSlicesCases := []struct {
		name  string
		input func() [][]byte
	}{
		{name: "nil", input: func() [][]byte { return nil }},
		{name: "empty", input: func() [][]byte { return [][]byte{} }},
		{name: "nil element", input: func() [][]byte { return [][]byte{nil} }},
		{name: "empty element", input: func() [][]byte { return [][]byte{{}} }},
		{name: "value element", input: func() [][]byte { return [][]byte{[]byte("value")} }},
	}

	for _, tc := range byteSliceCases {
		tc := tc
		t.Run("output account address is "+tc.name, func(t *testing.T) {
			t.Parallel()

			account := NewVMOutputAccount(tc.input())
			requireSameByteShape(t, tc.input(), account.Address)
		})

		t.Run("finished data is "+tc.name, func(t *testing.T) {
			t.Parallel()

			outputContext := newOutputContextForOwnershipTests()
			outputContext.Finish(tc.input())
			require.Len(t, outputContext.ReturnData(), 1)
			requireSameByteShape(t, tc.input(), outputContext.ReturnData()[0])
		})

		t.Run("prepended data is "+tc.name, func(t *testing.T) {
			t.Parallel()

			outputContext := newOutputContextForOwnershipTests()
			outputContext.PrependFinish(tc.input())
			require.Len(t, outputContext.ReturnData(), 1)
			requireSameByteShape(t, tc.input(), outputContext.ReturnData()[0])
		})

		t.Run("deployed code fields are "+tc.name, func(t *testing.T) {
			t.Parallel()

			outputContext := newOutputContextForOwnershipTests()
			outputContext.DeployCode(vmhost.CodeDeployInput{
				ContractCode:         tc.input(),
				ContractCodeMetadata: tc.input(),
				ContractAddress:      []byte("contract"),
				CodeDeployerAddress:  tc.input(),
			})

			account, _ := outputContext.GetOutputAccount([]byte("contract"))
			requireSameByteShape(t, tc.input(), account.Code)
			requireSameByteShape(t, tc.input(), account.CodeMetadata)
			requireSameByteShape(t, tc.input(), account.CodeDeployerAddress)
		})

		t.Run("log address and identifier are "+tc.name, func(t *testing.T) {
			t.Parallel()

			outputContext := newOutputContextForOwnershipTests()
			outputContext.WriteLogWithIdentifier(tc.input(), nil, nil, tc.input())

			require.Len(t, outputContext.outputState.Logs, 1)
			requireSameByteShape(t, tc.input(), outputContext.outputState.Logs[0].Address)
			requireSameByteShape(t, tc.input(), outputContext.outputState.Logs[0].Identifier)
		})
	}

	for _, tc := range byteSlicesCases {
		tc := tc
		t.Run("log data is "+tc.name, func(t *testing.T) {
			t.Parallel()

			outputContext := newOutputContextForOwnershipTests()
			outputContext.WriteLogWithIdentifier([]byte("address"), nil, tc.input(), []byte("identifier"))

			require.Equal(t, tc.input(), outputContext.outputState.Logs[0].Data)
		})

		t.Run("log topics are "+tc.name, func(t *testing.T) {
			t.Parallel()

			outputContext := newOutputContextForOwnershipTests()
			outputContext.WriteLogWithIdentifier([]byte("address"), tc.input(), nil, []byte("identifier"))

			recorded := outputContext.outputState.Logs[0].Topics
			if len(tc.input()) == 0 {
				require.Nil(t, recorded)
				return
			}
			require.Equal(t, tc.input(), recorded)
		})
	}

	t.Run("recorded values marshal like the raw inputs", func(t *testing.T) {
		t.Parallel()

		outputContext := newOutputContextForOwnershipTests()
		address := []byte("address")
		identifier := []byte{}
		topics := [][]byte{nil, {}, []byte("topic")}
		data := [][]byte{[]byte("data"), nil}
		outputContext.WriteLogWithIdentifier(address, topics, data, identifier)

		expectedEntry := &vmcommon.LogEntry{
			Address:    []byte("address"),
			Identifier: []byte{},
			Topics:     [][]byte{nil, {}, []byte("topic")},
			Data:       [][]byte{[]byte("data"), nil},
		}
		expectedEntryJSON, err := json.Marshal(expectedEntry)
		require.NoError(t, err)
		recordedEntryJSON, err := json.Marshal(outputContext.outputState.Logs[0])
		require.NoError(t, err)
		require.Equal(t, expectedEntryJSON, recordedEntryJSON)

		outputContext.DeployCode(vmhost.CodeDeployInput{
			ContractCode:         []byte{},
			ContractCodeMetadata: nil,
			ContractAddress:      []byte("contract"),
			CodeDeployerAddress:  []byte("deployer"),
		})
		recordedAccount, _ := outputContext.GetOutputAccount([]byte("contract"))
		expectedAccount := &vmcommon.OutputAccount{
			Address:             []byte("contract"),
			StorageUpdates:      make(map[string]*vmcommon.StorageUpdate),
			Code:                []byte{},
			CodeMetadata:        nil,
			CodeDeployerAddress: []byte("deployer"),
		}
		expectedAccountJSON, err := json.Marshal(expectedAccount)
		require.NoError(t, err)
		recordedAccountJSON, err := json.Marshal(recordedAccount)
		require.NoError(t, err)
		require.Equal(t, expectedAccountJSON, recordedAccountJSON)
	})
}
