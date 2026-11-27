package vmhooks_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/common"
	blockchainConfig "github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// Reading the destination of the managed calling hooks.
//
// readDestinationArguments and managedDeleteContract clone the destination buffer before anything
// checks it. Every destination they read is an address, which the memory path always loads as
// exactly AddressLen bytes. Post-fork (FixAuditChangesV6) a destination longer than an address
// fails before it is cloned.

func TestManagedCallingHooks_OversizedDestination_PreForkVersusPostFork(t *testing.T) {
	if test.RaceDetectorEnabled {
		t.Skip("the race detector slows execution past the smart contract execution timeout")
	}

	const gasLeft = uint64(10_000_000)
	const destLen = 4 * 1024 * 1024

	hooks := []struct {
		name string
		call func(host vmhost.VMHost, dstHandle int32) func()
	}{
		{name: "managedExecuteOnDestContext", call: func(host vmhost.VMHost, dstHandle int32) func() {
			managedType := host.ManagedTypes()
			value := managedType.NewBigIntFromInt64(0)
			function := managedType.NewManagedBufferFromBytes([]byte(managedHookChildFunction))
			arguments, result := managedType.NewManagedBufferFromBytes(nil), managedType.NewManagedBuffer()
			return func() {
				_ = vmhooks.NewVMHooksImpl(host).ManagedExecuteOnDestContext(100_000, dstHandle, value, function, arguments, result)
			}
		}},
		{name: "managedDeleteContract", call: func(host vmhost.VMHost, dstHandle int32) func() {
			arguments := host.ManagedTypes().NewManagedBufferFromBytes(nil)
			return func() { vmhooks.NewVMHooksImpl(host).ManagedDeleteContract(dstHandle, 100_000, arguments) }
		}},
	}

	for _, hook := range hooks {
		prepare := func(host vmhost.VMHost) func() {
			// an address followed by padding, so only the length is wrong
			destination := append(append([]byte(nil), test.ChildAddress...), make([]byte, destLen-len(test.ChildAddress))...)
			return hook.call(host, host.ManagedTypes().NewManagedBufferFromBytes(destination))
		}

		pre := runManagedHook(t, preForkV6EnableEpochs(), gasLeft, prepare)
		post := runManagedHook(t, postForkV6EnableEpochs(), gasLeft, prepare)

		t.Logf("%s: pre-fork allocated=%d B, post-fork allocated=%d B", hook.name, pre.allocated, post.allocated)

		// the legacy path clones the destination before anything rejects it
		require.GreaterOrEqual(t, pre.allocated, uint64(destLen), hook.name)
		// TotalAlloc is process-wide, so this is a loose bound, still far below one copy of it
		require.Less(t, post.allocated, uint64(destLen/8), hook.name)

		// post-fork the oversized destination fails the execution
		require.Equal(t, vmcommon.VMExecutionFailed, post.returnCode, hook.name)
		require.Equal(t, vmhost.ErrInvalidArgument.Error(), post.returnMessage, hook.name)
	}
}

// managedDeleteContract is where the check changes consensus: pre-fork a destination longer than an
// address still ends the call Ok, post-fork it fails the execution. An address itself is Ok on both
// sides, so the boundary sits between AddressLen and AddressLen+1.
func TestManagedDeleteContract_DestinationLengthBoundary_PreForkVersusPostFork(t *testing.T) {
	const gasLeft = uint64(10_000_000)

	deleteWithDestinationLen := func(destLen int) func(vmhost.VMHost) func() {
		return func(host vmhost.VMHost) func() {
			managedType := host.ManagedTypes()
			// the child address followed by padding, so only the length is wrong
			destination := append(append([]byte(nil), test.ChildAddress...), make([]byte, destLen-len(test.ChildAddress))...)
			dstHandle := managedType.NewManagedBufferFromBytes(destination)
			arguments := managedType.NewManagedBufferFromBytes(nil)
			return func() { vmhooks.NewVMHooksImpl(host).ManagedDeleteContract(dstHandle, 100_000, arguments) }
		}
	}

	for _, destLen := range []int{vmhost.AddressLen, vmhost.AddressLen + 1} {
		pre := runManagedHook(t, preForkV6EnableEpochs(), gasLeft, deleteWithDestinationLen(destLen))
		require.Equal(t, vmcommon.Ok, pre.returnCode, "pre-fork, %d bytes: %s", destLen, pre.returnMessage)
	}

	atAddressLen := runManagedHook(t, postForkV6EnableEpochs(), gasLeft, deleteWithDestinationLen(vmhost.AddressLen))
	require.Equal(t, vmcommon.Ok, atAddressLen.returnCode, atAddressLen.returnMessage)

	overAddressLen := runManagedHook(t, postForkV6EnableEpochs(), gasLeft, deleteWithDestinationLen(vmhost.AddressLen+1))
	require.Equal(t, vmcommon.VMExecutionFailed, overAddressLen.returnCode)
	require.Equal(t, vmhost.ErrInvalidArgument.Error(), overAddressLen.returnMessage)
}

// A short destination needs no length check: these hooks already fail the same way on both sides
// of the fork. managedDeleteContract ends Ok on both sides only because executeDelete discards the
// output of the failed owner check: nothing is deleted and the node never sees the short address.
func TestManagedCallingHooks_ShortDestination_FailsWithoutLengthCheck(t *testing.T) {
	const gasLeft = uint64(10_000_000)
	shortAddress := test.ChildAddress[:vmhost.AddressLen-1]
	upgradeableMetadata := []byte{vmcommon.MetadataUpgradeable, 0}

	failing := []struct {
		name    string
		prepare func(host vmhost.VMHost) func()
	}{
		{name: "managedExecuteOnDestContext", prepare: func(host vmhost.VMHost) func() {
			dst, value, fn, args, result := managedBuffer(host, shortAddress), managedZero(host), managedBuffer(host, []byte(managedHookChildFunction)), managedBuffer(host, nil), managedBuffer(host, nil)
			return func() { _ = newHooks(host).ManagedExecuteOnDestContext(100_000, dst, value, fn, args, result) }
		}},
		{name: "managedExecuteOnSameContext", prepare: func(host vmhost.VMHost) func() {
			dst, value, fn, args, result := managedBuffer(host, shortAddress), managedZero(host), managedBuffer(host, []byte(managedHookChildFunction)), managedBuffer(host, nil), managedBuffer(host, nil)
			return func() { _ = newHooks(host).ManagedExecuteOnSameContext(100_000, dst, value, fn, args, result) }
		}},
		{name: "managedExecuteReadOnly", prepare: func(host vmhost.VMHost) func() {
			dst, fn, args, result := managedBuffer(host, shortAddress), managedBuffer(host, []byte(managedHookChildFunction)), managedBuffer(host, nil), managedBuffer(host, nil)
			return func() { _ = newHooks(host).ManagedExecuteReadOnly(100_000, dst, fn, args, result) }
		}},
		{name: "managedUpgradeContract", prepare: func(host vmhost.VMHost) func() {
			dst, value, code, meta := managedBuffer(host, shortAddress), managedZero(host), managedBuffer(host, []byte("code")), managedBuffer(host, upgradeableMetadata)
			args, result := managedBuffer(host, nil), managedBuffer(host, nil)
			return func() { newHooks(host).ManagedUpgradeContract(dst, 100_000, value, code, meta, args, result) }
		}},
		{name: "managedUpgradeFromSourceContract source", prepare: func(host vmhost.VMHost) func() {
			dst, value, source, meta := managedBuffer(host, test.ChildAddress), managedZero(host), managedBuffer(host, shortAddress), managedBuffer(host, upgradeableMetadata)
			args, result := managedBuffer(host, nil), managedBuffer(host, nil)
			return func() {
				newHooks(host).ManagedUpgradeFromSourceContract(dst, 100_000, value, source, meta, args, result)
			}
		}},
		{name: "managedDeployFromSourceContract source", prepare: func(host vmhost.VMHost) func() {
			value, source, meta, args := managedZero(host), managedBuffer(host, shortAddress), managedBuffer(host, upgradeableMetadata), managedBuffer(host, nil)
			address, result := managedBuffer(host, nil), managedBuffer(host, nil)
			return func() {
				_ = newHooks(host).ManagedDeployFromSourceContract(100_000, value, source, meta, args, address, result)
			}
		}},
	}

	for _, hook := range failing {
		pre := runManagedHook(t, preForkV6EnableEpochs(), gasLeft, hook.prepare)
		post := runManagedHook(t, postForkV6EnableEpochs(), gasLeft, hook.prepare)

		// no account under the short address, not some unrelated failure
		require.Equal(t, vmcommon.VMExecutionFailed, pre.returnCode, hook.name)
		require.Equal(t, common.ErrAccNotFound.Error(), pre.returnMessage, hook.name)
		require.Equal(t, pre.returnCode, post.returnCode, hook.name)
		require.Equal(t, pre.returnMessage, post.returnMessage, hook.name)
	}

	// not SC-shaped, so the mock and production both reach the accounts kapp's receiver check
	transferShort := func(host vmhost.VMHost) func() {
		token, amount := managedBuffer(host, test.KDATestTokenName), host.ManagedTypes().NewBigInt(big.NewInt(1))
		dst := managedBuffer(host, bytes.Repeat([]byte{0xAB}, vmhost.AddressLen-1))
		transfers, fn, args := managedBuffer(host, kdaTransferVector(1, token, amount)), managedBuffer(host, nil), managedBuffer(host, nil)
		return func() { _ = newHooks(host).ManagedMultiTransferKDANFTExecute(dst, transfers, 0, fn, args) }
	}
	for _, epochs := range []blockchainConfig.EnableEpochs{preForkV6EnableEpochs(), postForkV6EnableEpochs()} {
		run := runManagedHook(t, epochs, gasLeft, transferShort)
		require.Equal(t, vmcommon.VMExecutionFailed, run.returnCode, run.returnMessage)
		require.Equal(t, process.ErrInvalidRcvAddr.Error(), run.returnMessage)
	}

	var deleteHost vmhost.VMHost
	deleteShort := func(host vmhost.VMHost) func() {
		dst, args := managedBuffer(host, shortAddress), managedBuffer(host, nil)
		return func() {
			newHooks(host).ManagedDeleteContract(dst, 100_000, args)
			deleteHost = host
		}
	}
	for _, epochs := range []blockchainConfig.EnableEpochs{preForkV6EnableEpochs(), postForkV6EnableEpochs()} {
		run := runManagedHook(t, epochs, gasLeft, deleteShort)
		require.Equal(t, vmcommon.Ok, run.returnCode, run.returnMessage)
		// executeDelete swallowed the failed owner check: nothing deleted, the error only in the runtime
		require.Empty(t, deleteHost.Output().GetVMOutput().DeletedAccounts)
		require.ErrorIs(t, deleteHost.Runtime().GetAllErrors(), common.ErrAccNotFound)
	}
}
