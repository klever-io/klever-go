package vmhooks_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	btcecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/crypto/factory"
	"github.com/klever-io/klever-go/kvm/crypto/signing"
	"github.com/klever-io/klever-go/kvm/crypto/signing/secp256k1"
	"github.com/klever-io/klever-go/kvm/executor"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/vmhost"
	hostmock "github.com/klever-io/klever-go/kvm/vmhost/mock"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/stretchr/testify/require"
)

func TestEncodeSecp256k1DerSignature(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string // Test case name
		rHex        string // Hex-encoded R value
		sHex        string // Hex-encoded S value
		expectErr   bool   // Whether an error is expected
		expectedSig string // Hex-encoded expected DER signature (if no error)
	}

	// Define test cases
	testCases := []testCase{
		{
			name:        "Valid R and S",
			rHex:        "fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c",
			sHex:        "2b8a9c0ad55394fb4aa21dc9483aea13279d6768ff2a9d6bcf589ac2613b3b02",
			expectErr:   false,
			expectedSig: "3045022100fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c02202b8a9c0ad55394fb4aa21dc9483aea13279d6768ff2a9d6bcf589ac2613b3b02",
		},
		{
			name:      "Valid R and S but fail due overflow S",
			rHex:      "fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c",
			sHex:      "d47563f52aac6b04b55de236b7c515eb9311757db01e02cff079c3ca6efb063f",
			expectErr: true,
		},
		{
			name:        "Invalid R (too short)",
			rHex:        "fef4",
			sHex:        "2b8a9c0ad55394fb4aa21dc9483aea13279d6768ff2a9d6bcf589ac2613b3b02",
			expectErr:   false,
			expectedSig: "3027020300fef402202b8a9c0ad55394fb4aa21dc9483aea13279d6768ff2a9d6bcf589ac2613b3b02",
		},
		{
			name:        "Invalid S (too short)",
			rHex:        "fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c",
			sHex:        "2b8a",
			expectErr:   false,
			expectedSig: "3027022100fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c02022b8a",
		},
		{
			name:        "Empty R and S",
			rHex:        "",
			sHex:        "",
			expectErr:   false,
			expectedSig: "3006020100020100",
		},
		{
			name:      "Fail due to R too long",
			rHex:      "fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2cd4",
			sHex:      "2b8a9c0ad55394fb4aa21dc9483aea13279d6768ff2a9d6bcf589ac2613b3b02",
			expectErr: true,
		},
		{
			name:      "Fail due to S too long",
			rHex:      "fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c",
			sHex:      "2b8a9c0ad55394fb4aa21dc9483aea13279d6768ff2a9d6bcf589ac2613b3b02fe",
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			host := newMockVMHost()
			host.CryptoHook = factory.NewVMCrypto()
			hooks := vmhooks.NewVMHooksImpl(host)

			// Decode hex strings to bytes
			r, err := hex.DecodeString(tc.rHex)
			require.Nil(t, err, "failed to decode rHex: %v", err)

			s, err := hex.DecodeString(tc.sHex)
			require.Nil(t, err, "failed to decode sHex: %v", err)

			// Set up memory for inputs
			rPtr := executor.MemPtr(100)
			err = host.MemStoreToMock(rPtr, r)
			require.Nil(t, err, "failed to store R in memory")

			sPtr := executor.MemPtr(200)
			err = host.MemStoreToMock(sPtr, s)
			require.Nil(t, err, "failed to store S in memory")

			// Allocate memory for the signature
			sigPtr := executor.MemPtr(300)

			// Call the method
			result := hooks.EncodeSecp256k1DerSignature(rPtr, executor.MemLength(len(r)), sPtr, executor.MemLength(len(s)), sigPtr)

			if tc.expectErr {
				require.Equal(t, int32(1), result, "expected error but got success")
			} else {
				require.Equal(t, int32(0), result, "expected success but got error")

				// Retrieve the signature from memory
				sig, err := host.MemLoadFromMock(sigPtr, len(tc.expectedSig)/2)
				require.Nil(t, err, "failed to load signature from memory")
				require.Equal(t, tc.expectedSig, hex.EncodeToString(sig), "signature does not match expected")
			}
		})
	}
}

// newMockVMHostForSecp256k1 also returns the ordered unbounded gas charges the hook made, as
// "gas:<n>". Before FixAuditChangesV6 every charge of these hooks is unbounded; from it they are
// all bounded and only show up in GasLeft.
func newMockVMHostForSecp256k1(fixAuditChangesV6 bool) (*contextmock.VMHostMock, *[]string) {
	host := newMockVMHost()
	host.CryptoHook = factory.NewVMCrypto()
	host.ForkControllerContext = &commonMock.ForkControllerStub{FixAuditChangesV6Value: fixAuditChangesV6}
	host.RuntimeContext.(*contextmock.RuntimeContextMock).FailCryptoAPI = true

	charges := &[]string{}
	host.MeteringContext.(*contextmock.MeteringContextMock).UseAndTraceGasCalled = func(gas uint64) {
		*charges = append(*charges, fmt.Sprintf("gas:%d", gas))
	}

	return host, charges
}

func verifyCustomSecp256k1Raw(t *testing.T, host *contextmock.VMHostMock, key, msg, sig []byte, hashType int32) int32 {
	t.Helper()

	keyPtr, msgPtr, sigPtr := executor.MemPtr(100), executor.MemPtr(200), executor.MemPtr(400)
	require.Nil(t, host.MemStoreToMock(keyPtr, key))
	require.Nil(t, host.MemStoreToMock(msgPtr, msg))
	require.Nil(t, host.MemStoreToMock(sigPtr, sig))

	hooks := vmhooks.NewVMHooksImpl(host)
	return hooks.VerifyCustomSecp256k1(
		keyPtr, executor.MemLength(len(key)),
		msgPtr, executor.MemLength(len(msg)),
		sigPtr, hashType)
}

func verifyCustomSecp256k1Managed(t *testing.T, host *contextmock.VMHostMock, key, msg, sig []byte, hashType int32) int32 {
	t.Helper()

	mockSecp256k1Buffers(host, key, msg, sig)
	return vmhooks.ManagedVerifyCustomSecp256k1WithHost(host, 1, 2, 3, hashType)
}

// mockSecp256k1Buffers serves key, msg and sig as managed buffers 1, 2 and 3
func mockSecp256k1Buffers(host *contextmock.VMHostMock, key, msg, sig []byte) {
	buffers := map[int32][]byte{1: key, 2: msg, 3: sig}
	managedTypes := host.ManagedTypesContext.(*hostmock.ManagedTypesContextMock)
	managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
		return buffers[handle], nil
	}
	managedTypes.GetLengthCalled = func(handle int32) int32 {
		buffer, ok := buffers[handle]
		if !ok {
			return -1
		}
		return int32(len(buffer)) // #nosec G115
	}
}

func TestVerifyCustomSecp256k1_PlainMessageLength(t *testing.T) {
	t.Parallel()

	// A plain-mode signature over a 32-byte digest, the way a contract holding an
	// already-hashed message would verify it.
	privKey, _ := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x42}, 32))
	key := privKey.PubKey().SerializeCompressed()
	digestArray := sha256.Sum256([]byte("klever"))
	digest := digestArray[:]
	sig := btcecdsa.Sign(privKey, digest).Serialize()

	appended := append(append([]byte{}, digest...), []byte("unsigned tail")...)
	plain := int32(secp256k1.ECDSAPlainMsg)
	// 256 narrows to 0 on the way into VerifySecp256k1, so it selects plain mode too.
	plainWrapped := int32(256)

	messageHashed := []byte("a message longer than thirty-two bytes, hashed by the curve path")
	messageDigest := sha256.Sum256(messageHashed)
	hashedSig := btcecdsa.Sign(privKey, messageDigest[:]).Serialize()

	type testCase struct {
		name        string
		fixV6       bool
		msg         []byte
		sig         []byte
		hashType    int32
		expected    int32
		expectedErr error
	}

	testCases := []testCase{
		{name: "plain 32-byte digest verifies with fix", fixV6: true, msg: digest, sig: sig, hashType: plain, expected: 0},
		{name: "plain 32-byte digest verifies without fix", fixV6: false, msg: digest, sig: sig, hashType: plain, expected: 0},
		{name: "plain digest with appended bytes rejected with fix", fixV6: true, msg: appended, sig: sig, hashType: plain, expected: 1, expectedErr: vmhost.ErrInvalidSecp256k1PlainMessageLength},
		{name: "plain digest with appended bytes still verifies without fix", fixV6: false, msg: appended, sig: sig, hashType: plain, expected: 0},
		{name: "plain 31-byte message rejected with fix", fixV6: true, msg: digest[1:], sig: sig, hashType: plain, expected: 1, expectedErr: vmhost.ErrInvalidSecp256k1PlainMessageLength},
		{name: "plain 31-byte message fails signature without fix", fixV6: false, msg: digest[1:], sig: sig, hashType: plain, expected: -1, expectedErr: signing.ErrInvalidSignature},
		{name: "empty plain message rejected with fix", fixV6: true, msg: []byte{}, sig: sig, hashType: plain, expected: 1, expectedErr: vmhost.ErrInvalidSecp256k1PlainMessageLength},
		{name: "hash type 256 narrows to plain and is rejected with fix", fixV6: true, msg: appended, sig: sig, hashType: plainWrapped, expected: 1, expectedErr: vmhost.ErrInvalidSecp256k1PlainMessageLength},
		{name: "hash type 256 narrows to plain and verifies without fix", fixV6: false, msg: appended, sig: sig, hashType: plainWrapped, expected: 0},
		{name: "hash type 256 with a 32-byte digest verifies with fix", fixV6: true, msg: digest, sig: sig, hashType: plainWrapped, expected: 0},
		{name: "hash type -256 with a 32-byte digest verifies with fix", fixV6: true, msg: digest, sig: sig, hashType: -plainWrapped, expected: 0},
		{name: "unsupported hash type keeps failing on the hasher with fix", fixV6: true, msg: appended, sig: sig, hashType: 5, expected: -1, expectedErr: signing.ErrHasherNotSupported},
		{name: "hashed mode accepts any message length with fix", fixV6: true, msg: messageHashed, sig: hashedSig, hashType: int32(secp256k1.ECDSASha256), expected: 0},
	}

	gasCost := config.MakeGasMapForTests()
	verifyCost := gasCost["CryptoAPICost"]["VerifySecp256k1"]
	dataCopyPerByte := gasCost["BaseOperationCost"]["DataCopyPerByte"]

	// expectedCharges is the charge sequence of a call that gets past the guard: the raw hook
	// charges the flat cost then the message copy, the managed one the flat cost then each buffer
	// it reads. The guard is a no-op before the fork, so the flag-off rows pin the historical,
	// unbounded sequence; from the fork the same amounts are charged bounded. A call that faults,
	// whether the guard rejects it or verification fails, burns the remaining gas, so from the
	// fork its charges are not compared.
	expectedCharges := map[string]func(tc testCase) []uint64{
		"raw": func(tc testCase) []uint64 {
			return []uint64{verifyCost, dataCopyPerByte * uint64(len(tc.msg))}
		},
		"managed": func(tc testCase) []uint64 {
			return []uint64{
				verifyCost,
				dataCopyPerByte * uint64(len(key)),
				dataCopyPerByte * uint64(len(tc.msg)),
				dataCopyPerByte * uint64(len(tc.sig)),
			}
		},
	}

	hooks := map[string]func(*testing.T, *contextmock.VMHostMock, []byte, []byte, []byte, int32) int32{
		"raw":     verifyCustomSecp256k1Raw,
		"managed": verifyCustomSecp256k1Managed,
	}

	for hookName, verify := range hooks {
		for _, tc := range testCases {
			t.Run(hookName+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				host, charges := newMockVMHostForSecp256k1(tc.fixV6)
				gasBefore := host.Metering().GasLeft()
				result := verify(t, host, key, tc.msg, tc.sig, tc.hashType)

				require.Equal(t, tc.expected, result)
				runtimeErr := host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr
				require.Equal(t, tc.expectedErr, runtimeErr)

				expected := expectedCharges[hookName](tc)
				if !tc.fixV6 {
					expectedTraced := make([]string, 0, len(expected))
					for _, charge := range expected {
						expectedTraced = append(expectedTraced, fmt.Sprintf("gas:%d", charge))
					}
					require.Equal(t, expectedTraced, *charges)
					return
				}

				require.Empty(t, *charges)
				if tc.expected != 0 {
					return
				}
				expectedTotal := uint64(0)
				for _, charge := range expected {
					expectedTotal += charge
				}
				require.Equal(t, expectedTotal, gasBefore-host.Metering().GasLeft())
			})
		}
	}
}

// The guard reads only the message length, like the other FixAuditChangesV6 length checks, so a
// rejected call copies no buffer at all - not the message, and not the key checked before it. A
// missing message buffer is not the guard's to report and reaches the loader.
func TestManagedVerifyCustomSecp256k1_PlainMessageRejectedBeforeCopy(t *testing.T) {
	t.Parallel()

	key := bytes.Repeat([]byte{0x02}, 33)
	sig := bytes.Repeat([]byte{0x30}, 70)
	plain := int32(secp256k1.ECDSAPlainMsg)

	t.Run("wrong-length message", func(t *testing.T) {
		t.Parallel()

		host, _ := newMockVMHostForSecp256k1(true)
		mockSecp256k1Buffers(host, key, bytes.Repeat([]byte{0x01}, 33), sig)
		managedTypes := host.ManagedTypesContext.(*hostmock.ManagedTypesContextMock)
		copied := []int32{}
		getBytes := managedTypes.GetBytesCalled
		managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
			copied = append(copied, handle)
			return getBytes(handle)
		}

		require.Equal(t, int32(1), vmhooks.ManagedVerifyCustomSecp256k1WithHost(host, 1, 2, 3, plain))
		require.Equal(t, vmhost.ErrInvalidSecp256k1PlainMessageLength, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
		require.Empty(t, copied)
	})

	t.Run("missing message buffer", func(t *testing.T) {
		t.Parallel()

		host, _ := newMockVMHostForSecp256k1(true)
		host.RuntimeContext.(*contextmock.RuntimeContextMock).FailManagedBuffersAPI = true
		mockSecp256k1Buffers(host, key, nil, sig)
		managedTypes := host.ManagedTypesContext.(*hostmock.ManagedTypesContextMock)
		managedTypes.GetLengthCalled = func(handle int32) int32 {
			if handle == 2 {
				return -1
			}
			return int32(len(key)) // #nosec G115
		}
		getBytes := managedTypes.GetBytesCalled
		managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
			if handle == 2 {
				return nil, vmhost.ErrNoManagedBufferUnderThisHandle
			}
			return getBytes(handle)
		}

		require.Equal(t, int32(1), vmhooks.ManagedVerifyCustomSecp256k1WithHost(host, 1, 2, 3, plain))
		require.Equal(t, vmhost.ErrNoManagedBufferUnderThisHandle, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
	})
}

// A rejected plain message is refused before it is copied, so with a nonfatal fault the call is charged
// the flat VerifySecp256k1 cost and nothing for the message.
func TestVerifyCustomSecp256k1_PlainMessageRejectionChargesOnlyFlatCost(t *testing.T) {
	t.Parallel()

	key := bytes.Repeat([]byte{0x02}, 33)
	msg := bytes.Repeat([]byte{0x01}, 33)
	sig := bytes.Repeat([]byte{0x30}, 70)
	verifyCost := config.MakeGasMapForTests()["CryptoAPICost"]["VerifySecp256k1"]

	hooks := map[string]func(*testing.T, *contextmock.VMHostMock, []byte, []byte, []byte, int32) int32{
		"raw":     verifyCustomSecp256k1Raw,
		"managed": verifyCustomSecp256k1Managed,
	}
	for hookName, verify := range hooks {
		t.Run(hookName, func(t *testing.T) {
			t.Parallel()

			host, charges := newMockVMHostForSecp256k1(true)
			host.RuntimeContext.(*contextmock.RuntimeContextMock).FailCryptoAPI = false
			gasBefore := host.Metering().GasLeft()

			require.Equal(t, int32(1), verify(t, host, key, msg, sig, int32(secp256k1.ECDSAPlainMsg)))
			require.Empty(t, *charges)
			require.Equal(t, verifyCost, gasBefore-host.Metering().GasLeft())
		})
	}
}

// The raw hook checks the declared message length before the load, but MemLoad truncates a range that
// runs past the end of contract memory instead of failing. A 32-byte plain message placed 20 bytes from
// the end comes back as 20 bytes, which the curve zero-extends, so a signature over 12 zero bytes
// followed by those 20 would verify. From FixAuditChangesV6 the truncated message is out of bounds.
func TestVerifyCustomSecp256k1_PlainMessageTruncatedByMemoryEnd(t *testing.T) {
	t.Parallel()

	privKey, _ := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x42}, 32))
	key := privKey.PubKey().SerializeCompressed()
	tail := bytes.Repeat([]byte{0xab}, 20)
	padded := append(make([]byte, 32-len(tail)), tail...)
	sig := btcecdsa.Sign(privKey, padded).Serialize()

	testCases := []struct {
		name        string
		fixV6       bool
		expected    int32
		expectedErr error
	}{
		{name: "rejected with fix", fixV6: true, expected: 1, expectedErr: vmhost.ErrBadBounds},
		{name: "zero-extended and verifies without fix", fixV6: false, expected: 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			host, _ := newMockVMHostForSecp256k1(tc.fixV6)
			keyPtr, sigPtr := executor.MemPtr(100), executor.MemPtr(400)
			msgPtr := executor.MemPtr(host.RuntimeContext.GetInstance().MemLength() - uint32(len(tail))) // #nosec G115
			require.Nil(t, host.MemStoreToMock(keyPtr, key))
			require.Nil(t, host.MemStoreToMock(msgPtr, tail))
			require.Nil(t, host.MemStoreToMock(sigPtr, sig))

			result := vmhooks.NewVMHooksImpl(host).VerifyCustomSecp256k1(
				keyPtr, executor.MemLength(len(key)),
				msgPtr, 32,
				sigPtr, int32(secp256k1.ECDSAPlainMsg))

			require.Equal(t, tc.expected, result)
			require.Equal(t, tc.expectedErr, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
		})
	}
}

// A negative raw message length in plain mode is not a 32-byte digest either, so from FixAuditChangesV6
// the length check reports it rather than leaving it for MemLoad.
func TestVerifyCustomSecp256k1_PlainMessageNegativeLength(t *testing.T) {
	t.Parallel()

	host, _ := newMockVMHostForSecp256k1(true)
	key := bytes.Repeat([]byte{0x02}, 33)
	require.Nil(t, host.MemStoreToMock(executor.MemPtr(100), key))

	result := vmhooks.NewVMHooksImpl(host).VerifyCustomSecp256k1(
		executor.MemPtr(100), executor.MemLength(len(key)),
		executor.MemPtr(200), -1,
		executor.MemPtr(400), int32(secp256k1.ECDSAPlainMsg))

	require.Equal(t, int32(1), result)
	require.Equal(t, vmhost.ErrInvalidSecp256k1PlainMessageLength, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
}

// The default hooks hardcode double SHA-256, so the plain-mode guard must never reach them:
// a message of any length still verifies with the fix active.
func TestVerifySecp256k1_DefaultHooksUnaffectedByPlainMessageGuard(t *testing.T) {
	t.Parallel()

	privKey, _ := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x42}, 32))
	key := privKey.PubKey().SerializeCompressed()
	msg := []byte("forty-two bytes long, not a 32-byte digest")
	sig := btcecdsa.Sign(privKey, chainhash.DoubleHashB(msg)).Serialize()

	t.Run("raw", func(t *testing.T) {
		t.Parallel()

		host, _ := newMockVMHostForSecp256k1(true)
		keyPtr, msgPtr, sigPtr := executor.MemPtr(100), executor.MemPtr(200), executor.MemPtr(400)
		require.Nil(t, host.MemStoreToMock(keyPtr, key))
		require.Nil(t, host.MemStoreToMock(msgPtr, msg))
		require.Nil(t, host.MemStoreToMock(sigPtr, sig))

		result := vmhooks.NewVMHooksImpl(host).VerifySecp256k1(
			keyPtr, executor.MemLength(len(key)),
			msgPtr, executor.MemLength(len(msg)),
			sigPtr)
		require.Equal(t, int32(0), result)
	})

	t.Run("managed", func(t *testing.T) {
		t.Parallel()

		host, _ := newMockVMHostForSecp256k1(true)
		mockSecp256k1Buffers(host, key, msg, sig)

		require.Equal(t, int32(0), vmhooks.ManagedVerifySecp256k1WithHost(host, 1, 2, 3))
	})
}

// The curve reduces a plain message mod N, so m and m+N share a signature; from FixAuditChangesV7
// a plain message at or above N is rejected.
func TestVerifyCustomSecp256k1_PlainMessageAboveCurveOrder(t *testing.T) {
	t.Parallel()

	privKey, _ := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x42}, 32))
	key := privKey.PubKey().SerializeCompressed()
	order := btcec.S256().N

	// a uint256 amount signed directly in plain mode; small enough that m+N fits in 32 bytes
	m := make([]byte, 32)
	m[31] = 0x2a
	sig := btcecdsa.Sign(privKey, m).Serialize()
	mPlusN := new(big.Int).Add(new(big.Int).SetBytes(m), order).FillBytes(make([]byte, 32))

	nMinusOne := new(big.Int).Sub(order, big.NewInt(1)).FillBytes(make([]byte, 32))
	nMinusOneSig := btcecdsa.Sign(privKey, nMinusOne).Serialize()
	n := order.FillBytes(make([]byte, 32))
	allOnes := bytes.Repeat([]byte{0xff}, 32)

	digestArray := sha256.Sum256([]byte("klever"))
	digest := digestArray[:]
	digestSig := btcecdsa.Sign(privKey, digest).Serialize()

	// hashed modes reduce their own output mod N, so raw bytes above N are just input
	allOnesDigest := sha256.Sum256(allOnes)
	hashedSig := btcecdsa.Sign(privKey, allOnesDigest[:]).Serialize()

	plain := int32(secp256k1.ECDSAPlainMsg)
	errNonCanonical := vmhost.ErrNonCanonicalSecp256k1PlainMessage

	type testCase struct {
		name        string
		fixV6       bool
		fixV7       bool
		msg         []byte
		sig         []byte
		hashType    int32
		expected    int32
		expectedErr error
	}

	testCases := []testCase{
		{name: "m verifies before any fix", msg: m, sig: sig, hashType: plain, expected: 0},
		{name: "m verifies with V6", fixV6: true, msg: m, sig: sig, hashType: plain, expected: 0},
		{name: "m verifies with V7", fixV6: true, fixV7: true, msg: m, sig: sig, hashType: plain, expected: 0},
		{name: "m+N verifies before any fix", msg: mPlusN, sig: sig, hashType: plain, expected: 0},
		{name: "m+N verifies with V6 only", fixV6: true, msg: mPlusN, sig: sig, hashType: plain, expected: 0},
		{name: "m+N rejected with V7", fixV6: true, fixV7: true, msg: mPlusN, sig: sig, hashType: plain, expected: 1, expectedErr: errNonCanonical},
		{name: "m+N rejected with V7 and V6 unset", fixV7: true, msg: mPlusN, sig: sig, hashType: plain, expected: 1, expectedErr: errNonCanonical},
		{name: "hash type 256 narrows to plain and m+N is rejected with V7", fixV6: true, fixV7: true, msg: mPlusN, sig: sig, hashType: 256, expected: 1, expectedErr: errNonCanonical},
		{name: "N-1 is the largest message that verifies with V7", fixV6: true, fixV7: true, msg: nMinusOne, sig: nMinusOneSig, hashType: plain, expected: 0},
		{name: "N is rejected with V7 before the signature is checked", fixV6: true, fixV7: true, msg: n, sig: sig, hashType: plain, expected: 1, expectedErr: errNonCanonical},
		{name: "N fails signature with V6 only", fixV6: true, msg: n, sig: sig, hashType: plain, expected: -1, expectedErr: signing.ErrInvalidSignature},
		{name: "all-ones message rejected with V7", fixV6: true, fixV7: true, msg: allOnes, sig: sig, hashType: plain, expected: 1, expectedErr: errNonCanonical},
		{name: "honest digest verifies with V7", fixV6: true, fixV7: true, msg: digest, sig: digestSig, hashType: plain, expected: 0},
		{name: "hashed mode accepts a message above N with V7", fixV6: true, fixV7: true, msg: allOnes, sig: hashedSig, hashType: int32(secp256k1.ECDSASha256), expected: 0},
	}

	gasCost := config.MakeGasMapForTests()
	verifyCost := gasCost["CryptoAPICost"]["VerifySecp256k1"]
	dataCopyPerByte := gasCost["BaseOperationCost"]["DataCopyPerByte"]

	// the charges of a call that reaches the curve, as in TestVerifyCustomSecp256k1_PlainMessageLength
	expectedCharges := map[string]func(tc testCase) []uint64{
		"raw": func(tc testCase) []uint64 {
			return []uint64{verifyCost, dataCopyPerByte * uint64(len(tc.msg))}
		},
		"managed": func(tc testCase) []uint64 {
			return []uint64{
				verifyCost,
				dataCopyPerByte * uint64(len(key)),
				dataCopyPerByte * uint64(len(tc.msg)),
				dataCopyPerByte * uint64(len(tc.sig)),
			}
		},
	}

	hooks := map[string]func(*testing.T, *contextmock.VMHostMock, []byte, []byte, []byte, int32) int32{
		"raw":     verifyCustomSecp256k1Raw,
		"managed": verifyCustomSecp256k1Managed,
	}

	for hookName, verify := range hooks {
		for _, tc := range testCases {
			t.Run(hookName+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				host, charges := newMockVMHostForSecp256k1(tc.fixV6)
				host.ForkControllerContext.(*commonMock.ForkControllerStub).FixAuditChangesV7Value = tc.fixV7
				gasBefore := host.Metering().GasLeft()
				result := verify(t, host, key, tc.msg, tc.sig, tc.hashType)

				require.Equal(t, tc.expected, result)
				require.Equal(t, tc.expectedErr, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)

				expected := expectedCharges[hookName](tc)
				if !tc.fixV6 {
					// a rejection comes after the message load, so the managed hook never copies the signature
					if tc.expected == 1 && hookName == "managed" {
						expected = expected[:len(expected)-1]
					}
					expectedTraced := make([]string, 0, len(expected))
					for _, charge := range expected {
						expectedTraced = append(expectedTraced, fmt.Sprintf("gas:%d", charge))
					}
					require.Equal(t, expectedTraced, *charges)
					return
				}

				require.Empty(t, *charges)
				if tc.expected != 0 {
					return
				}
				expectedTotal := uint64(0)
				for _, charge := range expected {
					expectedTotal += charge
				}
				require.Equal(t, expectedTotal, gasBefore-host.Metering().GasLeft())
			})
		}
	}
}

// The scalar check charges nothing: without V7, m+N costs what a verified call always did, and with it
// a rejection costs what the hook had charged up to the message load.
func TestVerifyCustomSecp256k1_PlainMessageAboveCurveOrderGas(t *testing.T) {
	t.Parallel()

	privKey, _ := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x42}, 32))
	key := privKey.PubKey().SerializeCompressed()
	m := make([]byte, 32)
	m[31] = 0x2a
	sig := btcecdsa.Sign(privKey, m).Serialize()
	mPlusN := new(big.Int).Add(new(big.Int).SetBytes(m), btcec.S256().N).FillBytes(make([]byte, 32))
	plain := int32(secp256k1.ECDSAPlainMsg)

	gasCost := config.MakeGasMapForTests()
	verifyCost := gasCost["CryptoAPICost"]["VerifySecp256k1"]
	dataCopyPerByte := gasCost["BaseOperationCost"]["DataCopyPerByte"]
	acceptedCharges := map[string][]uint64{
		"raw":     {verifyCost, dataCopyPerByte * uint64(len(m))},
		"managed": {verifyCost, dataCopyPerByte * uint64(len(key)), dataCopyPerByte * uint64(len(m)), dataCopyPerByte * uint64(len(sig))},
	}
	// the raw hook loads the message last of what it charges for; the managed one has loaded the key too
	rejectedCost := map[string]uint64{
		"raw":     verifyCost + dataCopyPerByte*uint64(len(m)),
		"managed": verifyCost + dataCopyPerByte*uint64(len(key)+len(m)),
	}

	hooks := map[string]func(*testing.T, *contextmock.VMHostMock, []byte, []byte, []byte, int32) int32{
		"raw":     verifyCustomSecp256k1Raw,
		"managed": verifyCustomSecp256k1Managed,
	}

	consumed := func(t *testing.T, verify func(*testing.T, *contextmock.VMHostMock, []byte, []byte, []byte, int32) int32, fixV6, fixV7 bool, msg []byte) (int32, uint64, []string) {
		host, charges := newMockVMHostForSecp256k1(fixV6)
		host.ForkControllerContext.(*commonMock.ForkControllerStub).FixAuditChangesV7Value = fixV7
		host.RuntimeContext.(*contextmock.RuntimeContextMock).FailCryptoAPI = false
		gasBefore := host.Metering().GasLeft()
		result := verify(t, host, key, msg, sig, plain)
		return result, gasBefore - host.Metering().GasLeft(), *charges
	}

	for hookName, verify := range hooks {
		t.Run(hookName+"/without V7 m+N costs what a verified call always did", func(t *testing.T) {
			t.Parallel()

			expectedTraced := make([]string, 0, len(acceptedCharges[hookName]))
			expectedTotal := uint64(0)
			for _, charge := range acceptedCharges[hookName] {
				expectedTraced = append(expectedTraced, fmt.Sprintf("gas:%d", charge))
				expectedTotal += charge
			}

			// before V6 every charge is traced unbounded; from it they are bounded and only show in GasLeft
			result, _, charges := consumed(t, verify, false, false, mPlusN)
			require.Equal(t, int32(0), result)
			require.Equal(t, expectedTraced, charges)

			result, gas, charges := consumed(t, verify, true, false, mPlusN)
			require.Equal(t, int32(0), result)
			require.Empty(t, charges)
			require.Equal(t, expectedTotal, gas)
		})

		t.Run(hookName+"/with V7 the rejection adds no charge", func(t *testing.T) {
			t.Parallel()

			result, gas, charges := consumed(t, verify, true, true, mPlusN)
			require.Equal(t, int32(1), result)
			require.Empty(t, charges)
			require.Equal(t, rejectedCost[hookName], gas)
		})
	}
}
