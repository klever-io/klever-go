package vmhooks_test

import (
	"encoding/hex"
	"math/big"
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/stretchr/testify/require"
)

func TestMBufferToBigFloat(t *testing.T) {
	t.Run("Canonical encoding succeeds", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		canonical, err := big.NewFloat(1.5).GobEncode()
		require.NoError(t, err)
		mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(canonical)
		bigFloatHandle := int32(100)

		ret := hooks.MBufferToBigFloat(mBufferHandle, bigFloatHandle)

		require.Equal(t, int32(0), ret)
		result, err := hooks.GetManagedTypesContext().GetBigFloat(bigFloatHandle)
		require.NoError(t, err)
		require.Equal(t, 0, big.NewFloat(1.5).Cmp(result))
	})

	// A canonical prec-53 encoding of 1.5 packs its mantissa into a single
	// 64-bit word, of which only the top 53 bits are significant. Appending
	// an extra mantissa word still has a valid prefix and still GobDecodes
	// successfully, but MinPrec() on the decoded value exceeds 53, which is
	// exactly the non-canonical encoding the check rejects.
	t.Run("Non-canonical encoding faults", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		canonical, err := big.NewFloat(1.5).GobEncode()
		require.NoError(t, err)
		nonCanonical := append(append([]byte{}, canonical...), 0, 0, 0, 0, 0, 0, 0, 1)
		mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(nonCanonical)
		bigFloatHandle := int32(101)

		ret := hooks.MBufferToBigFloat(mBufferHandle, bigFloatHandle)

		require.Equal(t, int32(1), ret)
		result, err := hooks.GetManagedTypesContext().GetBigFloat(bigFloatHandle)
		require.NoError(t, err)
		require.Equal(t, 0, big.NewFloat(0).Cmp(result), "destination must be left untouched (still the zero value GetBigFloatOrCreate seeded)")
	})

	// Stuffing non-zero bits into the low, supposedly-unused bits of that same
	// single mantissa word (rather than appending a whole extra word) changes
	// the represented value without changing byte length or declared
	// precision: GobDecode copies the word verbatim and GobEncode reproduces
	// it unchanged, so a raw byte round-trip can't see it. MinPrec() can: it
	// reports the true bits needed to represent the value exactly (64 here),
	// which exceeds 53 even though Prec() still reports 53.
	t.Run("Excess mantissa bits within a single word fault", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		nonCanonical, err := hex.DecodeString("010a0000003500000001c00000000000ffff")
		require.NoError(t, err)
		mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(nonCanonical)
		bigFloatHandle := int32(103)

		ret := hooks.MBufferToBigFloat(mBufferHandle, bigFloatHandle)

		require.Equal(t, int32(1), ret)
		result, err := hooks.GetManagedTypesContext().GetBigFloat(bigFloatHandle)
		require.NoError(t, err)
		require.Equal(t, 0, big.NewFloat(0).Cmp(result), "destination must be left untouched (still the zero value GetBigFloatOrCreate seeded)")
	})

	t.Run("Pre-fork: excess mantissa bits within a single word succeeds", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		fc := commonMock.NewForkControllerStub()
		fc.FixAuditChangesV4Value = false
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParametersWithFork(fc))
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		nonCanonical, err := hex.DecodeString("010a0000003500000001c00000000000ffff")
		require.NoError(t, err)
		mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(nonCanonical)
		bigFloatHandle := int32(104)

		ret := hooks.MBufferToBigFloat(mBufferHandle, bigFloatHandle)

		require.Equal(t, int32(0), ret)
	})

	t.Run("Exponent too big faults", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		outOfRange := new(big.Float).SetPrec(53)
		outOfRange.SetMantExp(big.NewFloat(0.5), 65026)
		encoded, err := outOfRange.GobEncode()
		require.NoError(t, err)
		mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(encoded)
		bigFloatHandle := int32(102)

		ret := hooks.MBufferToBigFloat(mBufferHandle, bigFloatHandle)

		require.Equal(t, int32(1), ret)
		result, err := hooks.GetManagedTypesContext().GetBigFloat(bigFloatHandle)
		require.NoError(t, err)
		require.Equal(t, 0, big.NewFloat(0).Cmp(result), "destination must be left untouched (still the zero value GetBigFloatOrCreate seeded)")
	})
}
