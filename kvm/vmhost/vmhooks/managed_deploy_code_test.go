package vmhooks_test

import (
	"testing"

	mock "github.com/klever-io/klever-go/kvm/mock/context"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// Pre-fork the managed deploy and upgrade hooks clone the code and code metadata before charging
// for them. From FixAuditChangesV6 they pay first, bounded, and fail before the clone.

const oversizedDeployBufferLen = 4 * 1024 * 1024

// pays every hook with honest buffers, not one oversized buffer
const gasLeftForOversizedBuffer = uint64(1_000_000)

// managedDeployHook calls one hook with the given code and code metadata.
type managedDeployHook struct {
	name    string
	hasCode bool // false for the from-source hooks
	// gas the legacy path uses when it accepts oversized code metadata, measured on
	// MakeGasMapForTests (every cost 1) with DataCopyPerByte 50; a change here means the legacy
	// gas path changed, which must not happen without a gate
	preForkMetadataGas uint64
	call               func(host vmhost.VMHost, code []byte, codeMetadata []byte) func()
}

func managedDeployHooks() []managedDeployHook {
	const childGas = 1000

	return []managedDeployHook{
		{name: "managedCreateContract", hasCode: true, preForkMetadataGas: 296, call: func(host vmhost.VMHost, code []byte, codeMetadata []byte) func() {
			value, codeHandle, meta, args := managedZero(host), managedBuffer(host, code), managedBuffer(host, codeMetadata), managedBuffer(host, nil)
			address, result := managedBuffer(host, nil), managedBuffer(host, nil)
			return func() {
				_ = newHooks(host).ManagedCreateContract(childGas, value, codeHandle, meta, args, address, result)
			}
		}},
		{name: "managedUpgradeContract", hasCode: true, preForkMetadataGas: 798, call: func(host vmhost.VMHost, code []byte, codeMetadata []byte) func() {
			dst, value, codeHandle, meta := managedBuffer(host, test.ChildAddress), managedZero(host), managedBuffer(host, code), managedBuffer(host, codeMetadata)
			args, result := managedBuffer(host, nil), managedBuffer(host, nil)
			return func() { newHooks(host).ManagedUpgradeContract(dst, childGas, value, codeHandle, meta, args, result) }
		}},
		{name: "managedDeployFromSourceContract", preForkMetadataGas: 296, call: func(host vmhost.VMHost, _ []byte, codeMetadata []byte) func() {
			value, source, meta, args := managedZero(host), managedBuffer(host, test.ChildAddress), managedBuffer(host, codeMetadata), managedBuffer(host, nil)
			address, result := managedBuffer(host, nil), managedBuffer(host, nil)
			return func() {
				_ = newHooks(host).ManagedDeployFromSourceContract(childGas, value, source, meta, args, address, result)
			}
		}},
		{name: "managedUpgradeFromSourceContract", preForkMetadataGas: 798, call: func(host vmhost.VMHost, _ []byte, codeMetadata []byte) func() {
			dst, value, source, meta := managedBuffer(host, test.ChildAddress), managedZero(host), managedBuffer(host, test.ChildAddress), managedBuffer(host, codeMetadata)
			args, result := managedBuffer(host, nil), managedBuffer(host, nil)
			return func() {
				newHooks(host).ManagedUpgradeFromSourceContract(dst, childGas, value, source, meta, args, result)
			}
		}},
	}
}

func TestManagedDeployHooks_OversizedCodeAndMetadata_PreForkVersusPostFork(t *testing.T) {
	honestCode := mock.MockContractCode(test.ChildAddress)
	honestMetadata := []byte{vmcommon.MetadataUpgradeable, 0}
	oversized := make([]byte, oversizedDeployBufferLen)

	type oversizedBuffer struct {
		name         string
		code         []byte
		codeMetadata []byte
		codeOnly     bool
		// legacy outcome: oversized code runs out of gas, oversized code metadata goes through
		preReturnCode    vmcommon.ReturnCode
		preReturnMessage string
	}
	buffers := []oversizedBuffer{
		{name: "code", code: oversized, codeMetadata: honestMetadata, codeOnly: true,
			preReturnCode: vmcommon.VMOutOfGas, preReturnMessage: vmhost.ErrNotEnoughGas.Error()},
		{name: "code metadata", code: honestCode, codeMetadata: oversized, preReturnCode: vmcommon.Ok},
	}

	for _, hook := range managedDeployHooks() {
		for _, buf := range buffers {
			if buf.codeOnly && !hook.hasCode {
				continue
			}
			name := hook.name + " " + buf.name
			prepare := func(host vmhost.VMHost) func() { return hook.call(host, buf.code, buf.codeMetadata) }

			pre := runManagedHook(t, preForkV6EnableEpochs(), gasLeftForOversizedBuffer, prepare)
			post := runManagedHook(t, postForkV6EnableEpochs(), gasLeftForOversizedBuffer, prepare)

			t.Logf("%s: pre-fork %s allocated=%d B (%s), post-fork %s allocated=%d B (%s)",
				name, pre.returnCode, pre.allocated, pre.returnMessage, post.returnCode, post.allocated, post.returnMessage)

			// legacy clone, pinned for block replay
			require.GreaterOrEqual(t, pre.allocated, uint64(oversizedDeployBufferLen), name)
			require.Equal(t, buf.preReturnCode, pre.returnCode, name)
			require.Equal(t, buf.preReturnMessage, pre.returnMessage, name)
			preGasUsed := hook.preForkMetadataGas
			if buf.preReturnCode == vmcommon.VMOutOfGas {
				preGasUsed = gasLeftForOversizedBuffer
			}
			require.Equal(t, preGasUsed, pre.gasUsed, name)

			require.Equal(t, vmcommon.VMOutOfGas, post.returnCode, name)
			require.Equal(t, vmhost.ErrNotEnoughGas.Error(), post.returnMessage, name)
			require.LessOrEqual(t, post.gasUsed, gasLeftForOversizedBuffer, name)
			// TotalAlloc is process-wide, so the bound is loose
			require.Less(t, post.allocated, uint64(oversizedDeployBufferLen/8), name)
		}
	}
}

// Same gas, honest buffers: the oversized calls above fail on size, not budget, and the legacy
// path accepts honest buffers with it too.
func TestManagedDeployHooks_HonestCodeAndMetadata_SucceedOnBothSidesOfFork(t *testing.T) {
	honestCode := mock.MockContractCode(test.ChildAddress)
	honestMetadata := []byte{vmcommon.MetadataUpgradeable, 0}

	for _, hook := range managedDeployHooks() {
		prepare := func(host vmhost.VMHost) func() { return hook.call(host, honestCode, honestMetadata) }

		pre := runManagedHook(t, preForkV6EnableEpochs(), gasLeftForOversizedBuffer, prepare)
		post := runManagedHook(t, postForkV6EnableEpochs(), gasLeftForOversizedBuffer, prepare)

		require.Equal(t, vmcommon.Ok, pre.returnCode, "%s pre-fork: %s", hook.name, pre.returnMessage)
		require.Equal(t, vmcommon.Ok, post.returnCode, "%s post-fork: %s", hook.name, post.returnMessage)
	}
}
