package hostCoretest

import (
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/vm"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"

	"github.com/stretchr/testify/require"
)

func setupMEXPair(t *testing.T, owner Address, user Address) (*worldmock.MockWorld, vmhost.VMHost, *MEXSetup) {
	world, ownerAccount, host, err := prepare(t, owner)
	require.Nil(t, err)

	userAccount := world.CreateAccount(user, world)
	userAccount.Balance = big.NewInt(100)
	mex := NewMEXSetup(t, host, world, ownerAccount, userAccount)
	mex.Deploy()

	mex.ApplyInitialSetup()

	return world, host, mex
}

type MEXSetup struct {
	WKLVToken                []byte
	MEXToken                 []byte
	LPToken                  []byte
	OwnerAccount             *worldmock.Account
	OwnerAddress             Address
	RouterAddress            Address
	PairAddress              Address
	TotalFeePercent          uint64
	SpecialFeePercent        uint64
	MaxObservationsPerRecord int
	Code                     []byte
	UserAccount              *worldmock.Account
	UserWKLVBalance          uint64
	UserMEXBalance           uint64

	T     *testing.T
	Host  vmhost.VMHost
	World *worldmock.MockWorld
}

func NewMEXSetup(
	t *testing.T,
	host vmhost.VMHost,
	world *worldmock.MockWorld,
	ownerAccount *worldmock.Account,
	userAccount *worldmock.Account,
) *MEXSetup {
	return &MEXSetup{
		WKLVToken:                []byte("WKLV-abcdef"),
		MEXToken:                 []byte("MEX-abcdef"),
		LPToken:                  []byte("LPTOK-abcdef"),
		OwnerAccount:             ownerAccount,
		OwnerAddress:             ownerAccount.Address,
		RouterAddress:            ownerAccount.Address,
		PairAddress:              test.MakeTestSCAddress("pairSC"),
		TotalFeePercent:          300,
		SpecialFeePercent:        50,
		MaxObservationsPerRecord: 10,
		Code:                     test.GetTestSCCode("pair", "../../"),
		UserAccount:              userAccount,
		UserWKLVBalance:          5_000_000_000,
		UserMEXBalance:           5_000_000_000,

		T:     t,
		Host:  host,
		World: world,
	}
}

func (mex *MEXSetup) Deploy() {
	t := mex.T
	host := mex.Host
	world := mex.World

	vmInput := test.CreateTestContractCreateInputBuilder().
		WithCallerAddr(mex.OwnerAddress).
		WithContractCode(mex.Code).
		WithArguments(
			mex.WKLVToken,
			mex.MEXToken,
			mex.OwnerAddress,
			mex.RouterAddress,
			big.NewInt(int64(mex.TotalFeePercent)).Bytes(),
			big.NewInt(int64(mex.SpecialFeePercent)).Bytes(),
		).
		WithGasProvided(0xFFFFFFFFFFFFFFFF).
		Build()

	world.NewAddressMocks = append(world.NewAddressMocks, &worldmock.NewAddressMock{
		CreatorAddress: mex.OwnerAddress,
		CreatorNonce:   mex.OwnerAccount.Nonce,
		NewAddress:     mex.PairAddress,
	})

	mex.OwnerAccount.Nonce++ // nonce increases before deploy
	vmOutput, err := host.RunSmartContractCreate(vmInput)
	require.Nil(t, err)
	require.NotNil(t, vmOutput)
	require.Equal(t, "", vmOutput.ReturnMessage)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)
	_ = world.UpdateAccounts(vmOutput.OutputAccounts, nil)
}

func (mex *MEXSetup) ApplyInitialSetup() {
	mex.setLPToken()
	mex.setActiveState()
	mex.setMaxObservationsPerRecord()
	mex.setKDABalances()
}

func (mex *MEXSetup) setLPToken() {
	t := mex.T
	host := mex.Host
	world := mex.World

	vmInput := test.CreateTestContractCallInputBuilder().
		WithCallerAddr(mex.OwnerAddress).
		WithRecipientAddr(mex.PairAddress).
		WithFunction("setLpTokenIdentifier").
		WithArguments(mex.LPToken).
		WithGasProvided(0xFFFFFFFFFFFFFFFF).
		Build()

	vmOutput, err := host.RunSmartContractCall(vmInput)
	require.Nil(t, err)
	require.NotNil(t, vmOutput)
	require.Equal(t, "", vmOutput.ReturnMessage)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)
	_ = world.UpdateAccounts(vmOutput.OutputAccounts, nil)
}

func (mex *MEXSetup) setActiveState() {
	t := mex.T
	host := mex.Host
	world := mex.World

	vmInput := test.CreateTestContractCallInputBuilder().
		WithCallerAddr(mex.OwnerAddress).
		WithRecipientAddr(mex.PairAddress).
		WithFunction("resume").
		WithGasProvided(0xFFFFFFFFFFFFFFFF).
		Build()

	vmOutput, err := host.RunSmartContractCall(vmInput)
	require.Nil(t, err)
	require.NotNil(t, vmOutput)
	require.Equal(t, "", vmOutput.ReturnMessage)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)
	_ = world.UpdateAccounts(vmOutput.OutputAccounts, nil)
}

func (mex *MEXSetup) setMaxObservationsPerRecord() {
	t := mex.T
	host := mex.Host
	world := mex.World

	vmInput := test.CreateTestContractCallInputBuilder().
		WithCallerAddr(mex.OwnerAddress).
		WithRecipientAddr(mex.PairAddress).
		WithFunction("setMaxObservationsPerRecord").
		WithArguments(big.NewInt(int64(mex.MaxObservationsPerRecord)).Bytes()).
		WithGasProvided(0xFFFFFFFFFFFFFFFF).
		Build()

	vmOutput, err := host.RunSmartContractCall(vmInput)
	require.Nil(t, err)
	require.NotNil(t, vmOutput)
	require.Equal(t, "", vmOutput.ReturnMessage)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)
	_ = world.UpdateAccounts(vmOutput.OutputAccounts, nil)
}

func (mex *MEXSetup) setKDABalances() {
	_ = mex.UserAccount.SetTokenBalanceUint64(mex.WKLVToken, 0, mex.UserWKLVBalance)
	_ = mex.UserAccount.SetTokenBalanceUint64(mex.MEXToken, 0, mex.UserMEXBalance)
}

func (mex *MEXSetup) AddLiquidity(
	userAddress Address,
	WKLVAmount uint64,
	minWKLVAmount uint64,
	MEXAmount uint64,
	minMEXAmount uint64,
) {
	t := mex.T
	host := mex.Host
	world := mex.World

	vmInputBuiler := test.CreateTestContractCallInputBuilder().
		WithCallerAddr(mex.UserAccount.Address).
		WithRecipientAddr(mex.PairAddress).
		WithFunction("addLiquidity").
		WithArguments(
			big.NewInt(int64(minWKLVAmount)).Bytes(),
			big.NewInt(int64(minMEXAmount)).Bytes(),
		).
		WithGasProvided(0xFFFFFFFFFFFFFFFF)

	vmInputBuiler.
		WithKDATokenName(mex.WKLVToken).
		WithKDAValue(big.NewInt(int64(WKLVAmount))).
		NextKDATransfer().
		WithKDATokenName(mex.MEXToken).
		WithKDAValue(big.NewInt(int64(MEXAmount)))

	vmInput := vmInputBuiler.Build()

	addLiquidityKDA := mex.createMultiKDATransferVMInput(
		vmInput.CallerAddr,
		vmInput.RecipientAddr,
		vmInput.KDATransfers,
	)

	mex.performMultiKDATransfer(addLiquidityKDA)

	vmOutput, err := host.RunSmartContractCall(vmInput)
	require.Nil(t, err)
	require.NotNil(t, vmOutput)
	require.Equal(t, "", vmOutput.ReturnMessage)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)
	_ = world.UpdateAccounts(vmOutput.OutputAccounts, nil)
}

func (mex *MEXSetup) CreateSwapVMInputs(
	leftToken []byte,
	leftAmount uint64,
	rightToken []byte,
	rightAmount uint64,
) (*vmcommon.ContractCallInput, *vmcommon.ContractCallInput) {
	vmInputBuiler := test.CreateTestContractCallInputBuilder().
		WithCallerAddr(mex.UserAccount.Address).
		WithRecipientAddr(mex.PairAddress)

	vmInputBuiler.
		WithKDATokenName(leftToken).
		WithKDAValue(big.NewInt(int64(leftAmount))).
		WithFunction("swapTokensFixedInput").
		WithArguments(
			rightToken,
			big.NewInt(int64(rightAmount)).Bytes(),
		).
		WithGasProvided(0xFFFFFFFFFFF)

	vmInput := vmInputBuiler.Build()

	multiTransferInput := mex.createMultiKDATransferVMInput(
		vmInput.CallerAddr,
		vmInput.RecipientAddr,
		vmInput.KDATransfers,
	)

	return multiTransferInput, vmInput

}

func (mex *MEXSetup) ExecuteSwap(
	multiTransferInput *vmcommon.ContractCallInput,
	vmInput *vmcommon.ContractCallInput,
) {
	t := mex.T
	host := mex.Host
	world := mex.World

	mex.performMultiKDATransfer(multiTransferInput)

	vmOutput, err := host.RunSmartContractCall(vmInput)
	require.Nil(t, err)
	require.NotNil(t, vmOutput)
	require.Equal(t, "", vmOutput.ReturnMessage)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)
	_ = world.UpdateAccounts(vmOutput.OutputAccounts, nil)

	err = world.CommitChanges()
	require.Nil(t, err)
}

func (mex *MEXSetup) createMultiKDATransferVMInput(
	sender Address,
	receiver Address,
	kdaTransfers []*vmcommon.KDATransfer,
) *vmcommon.ContractCallInput {
	nrTransfers := len(kdaTransfers)
	nrTransfersAsBytes := big.NewInt(0).SetUint64(uint64(nrTransfers)).Bytes()

	multiTransferInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr: sender,
			Arguments:  make([][]byte, 0),
			KDATransfers: []*vmcommon.KDATransfer{
				{
					KDAValue: big.NewInt(0),
				},
			},
			CallType:    vm.DirectCall,
			GasProvided: 0xFFFFFFFF,
		},
		RecipientAddr:     sender,
		Function:          core.BuiltInFunctionTransfer,
		AllowInitFunction: false,
	}
	multiTransferInput.Arguments = append(multiTransferInput.Arguments, receiver, nrTransfersAsBytes)

	for i := 0; i < nrTransfers; i++ {
		token := kdaTransfers[i].KDATokenName
		nonceAsBytes := big.NewInt(0).SetUint64(kdaTransfers[i].KDATokenNonce).Bytes()
		value := kdaTransfers[i].KDAValue.Bytes()

		multiTransferInput.Arguments = append(multiTransferInput.Arguments, token, nonceAsBytes, value)
	}

	return multiTransferInput
}

func (mex *MEXSetup) performMultiKDATransfer(
	multiTransferInput *vmcommon.ContractCallInput,
) {
	t := mex.T
	world := mex.World

	vmOutput, err := world.BuiltinFuncs.ProcessBuiltInFunction(multiTransferInput)
	require.Nil(t, err)
	require.NotNil(t, vmOutput)
	require.Equal(t, "", vmOutput.ReturnMessage)
	require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)
	_ = world.UpdateAccounts(vmOutput.OutputAccounts, nil)
}
