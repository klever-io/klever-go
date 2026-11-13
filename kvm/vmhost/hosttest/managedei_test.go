package hostCoretest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/elliptic"
	"encoding/hex"
	"math/big"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	blockchainConfig "github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/crypto/hashing"
	"github.com/klever-io/klever-go/kvm/crypto/signing/secp256k1"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/testcommon"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var baseTestConfig = &testcommon.TestConfig{
	GasProvided:     1000,
	GasUsedByParent: 400,
	GasUsedByChild:  200,

	ParentBalance: 1000,
	ChildBalance:  1000,
}

func Test_ManagedBufferToHex(t *testing.T) {
	testConfig := baseTestConfig

	asBytes := []byte{1, 2, 3}
	asString := "010203"

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						managedTypes := host.ManagedTypes()
						sourceHandle := managedTypes.NewManagedBufferFromBytes(asBytes)
						destHandle := managedTypes.NewManagedBuffer()

						vmhooks.ManagedBufferToHexWithHost(
							host,
							sourceHandle,
							destHandle)

						bytesResult, _ := managedTypes.GetBytes(destHandle)
						if string(bytesResult) != asString {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

// runManagedHookForkGateTest is a shared helper for the KLC-2566 [Certik KLR-43] fork-gate
// tests below: build a single mock contract that runs body against the real host, then hand
// the VM output to checkResult. The gas-scaling and size-cap math these hooks now enforce is
// tested directly against the hook functions in vmhost/vmhooks/managedei_test.go (mock
// ManagedTypes/Metering contexts, no need for a full host/instance) - what genuinely needs
// this real-host, real-instance setup is proving the fork gate itself behaves correctly
// end-to-end, which is all the tests below check.
func runManagedHookForkGateTest(
	t *testing.T,
	enableEpochs *blockchainConfig.EnableEpochs,
	body func(host vmhost.VMHost),
	checkResult func(*test.VMOutputVerifier),
) {
	t.Helper()
	testConfig := baseTestConfig

	builder := test.BuildMockInstanceCallTest(t)
	if enableEpochs != nil {
		builder = builder.WithEnableEpochs(*enableEpochs)
	}

	_, err := builder.
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						body(parentInstance.Host)
						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			checkResult(verify)
		})
	require.Nil(t, err)
}

// Test_ManagedBufferToHex_RejectsOversizedBuffer_PostFork and its _PreFork counterpart below
// prove the KLC-2566 [Certik KLR-43] mitigation is correctly fork-gated: post-fork, a source
// buffer over the protocol maximum must be rejected outright (unbounded allocation inside the
// uninterruptible Fast-hook window is what this fix closes); pre-fork, behavior must stay
// exactly as it was, so a rolling upgrade never causes upgraded and non-upgraded nodes to
// disagree on the same transaction.
func Test_ManagedBufferToHex_RejectsOversizedBuffer_PostFork(t *testing.T) {
	oversizedBuffer := bytes.Repeat([]byte{7}, 16000001)

	runManagedHookForkGateTest(t, nil,
		func(host vmhost.VMHost) {
			managedTypes := host.ManagedTypes()
			sourceHandle := managedTypes.NewManagedBufferFromBytes(oversizedBuffer)
			destHandle := managedTypes.NewManagedBuffer()
			vmhooks.ManagedBufferToHexWithHost(host, sourceHandle, destHandle)
		},
		func(verify *test.VMOutputVerifier) {
			verify.
				ExecutionFailed().
				ReturnMessageContains("managed buffer length exceeds maximum")
		},
	)
}

func Test_ManagedBufferToHex_AllowsOversizedBuffer_PreFork(t *testing.T) {
	if testcommon.RaceDetectorEnabled {
		t.Skip("the race detector slows execution past the smart contract execution timeout")
	}

	oversizedBuffer := bytes.Repeat([]byte{7}, 16000001)

	runManagedHookForkGateTest(t, &blockchainConfig.EnableEpochs{FixAuditChangesV5: 1},
		func(host vmhost.VMHost) {
			managedTypes := host.ManagedTypes()
			sourceHandle := managedTypes.NewManagedBufferFromBytes(oversizedBuffer)
			destHandle := managedTypes.NewManagedBuffer()
			vmhooks.ManagedBufferToHexWithHost(host, sourceHandle, destHandle)

			bytesResult, _ := managedTypes.GetBytes(destHandle)
			if string(bytesResult) != hex.EncodeToString(oversizedBuffer) {
				host.Runtime().SignalUserError("assert failed")
			}
		},
		func(verify *test.VMOutputVerifier) {
			verify.Ok()
		},
	)
}

// Test_ManagedIsBuiltinFunction_CapsOversizedName_PostFork and its _PreFork counterpart below
// are the ManagedIsBuiltinFunction equivalent of the ToHex fork-gate pair above. Post-fork
// this hook no longer reverts on an oversized name (see managedei.go: it's a pure predicate,
// so returning a truthful "not a builtin" is correct and avoids a new failure mode), so the
// post-fork proof here is that the call still succeeds and returns 0, not that it fails.
func Test_ManagedIsBuiltinFunction_CapsOversizedName_PostFork(t *testing.T) {
	oversizedName := bytes.Repeat([]byte("b"), 257)

	runManagedHookForkGateTest(t, nil,
		func(host vmhost.VMHost) {
			managedTypes := host.ManagedTypes()
			nameHandle := managedTypes.NewManagedBufferFromBytes(oversizedName)
			result := vmhooks.ManagedIsBuiltinFunctionWithHost(host, nameHandle)
			if result != 0 {
				host.Runtime().SignalUserError("assert failed")
			}
		},
		func(verify *test.VMOutputVerifier) {
			// Gas is the only pre/post-fork observable here: the hook returns 0 either
			// way (a 257-byte name is not a builtin under either path), so without this
			// assertion the pair would still pass with the whole fork-gated block removed.
			// 305 = 49 (pre-fork baseline below) + 256 bytes charged at the capped length.
			verify.Ok().
				GasUsed(test.ParentAddress, 305)
		},
	)
}

func Test_ManagedIsBuiltinFunction_AllowsOversizedName_PreFork(t *testing.T) {
	oversizedName := bytes.Repeat([]byte("b"), 257)

	runManagedHookForkGateTest(t, &blockchainConfig.EnableEpochs{FixAuditChangesV5: 1},
		func(host vmhost.VMHost) {
			managedTypes := host.ManagedTypes()
			nameHandle := managedTypes.NewManagedBufferFromBytes(oversizedName)
			vmhooks.ManagedIsBuiltinFunctionWithHost(host, nameHandle)
		},
		func(verify *test.VMOutputVerifier) {
			// Pre-fork the name is never charged per-byte, so the cost stays at the
			// baseline - this is what the post-fork 305 above is measured against.
			verify.Ok().
				GasUsed(test.ParentAddress, 49)
		},
	)
}

func Test_BigIntToString(t *testing.T) {
	testConfig := baseTestConfig

	asBigInt := big.NewInt(1234567890)
	asString := "1234567890"

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						managedTypes := host.ManagedTypes()
						sourceHandle := managedTypes.NewBigInt(asBigInt)
						destHandle := managedTypes.NewManagedBuffer()

						vmhooks.BigIntToStringWithHost(
							host,
							sourceHandle,
							destHandle)

						bytesResult, _ := managedTypes.GetBytes(destHandle)
						if string(bytesResult) != asString {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

func Test_ManagedRipemd160(t *testing.T) {
	testConfig := baseTestConfig

	asBytes := []byte{1, 2, 3}
	asRipemd160, _ := hashing.NewHasher().Ripemd160(asBytes)

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						managedTypes := host.ManagedTypes()
						sourceHandle := managedTypes.NewManagedBufferFromBytes(asBytes)
						destHandle := managedTypes.NewManagedBuffer()

						vmhooks.ManagedRipemd160WithHost(
							host,
							sourceHandle,
							destHandle)

						bytesResult, _ := managedTypes.GetBytes(destHandle)
						if !bytes.Equal(bytesResult, asRipemd160) {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

const blsCheckOK = "3e886a4c6e109a151f4105aee65a5192d150ef1fa68d3cd76964a0b086006dbe4324c989deb0e4416c6d6706db1b1910eb2732f08842fb4886067b9ed191109ac2188d76002d2e11da80a3f0ea89fee6b59c834cc478a6bd49cb8a193b1abb16@e96bd0f36b70c5ccc0c4396343bd7d8255b8a526c55fa1e218511fafe6539b8e@04725db195e37aa237cdbbda76270d4a229b6e7a3651104dc58c4349c0388e8546976fe54a04240530b99064e434c90f"

func blsSplitString(t testing.TB, str string) ([]byte, []byte, []byte) {
	split := strings.Split(str, "@")
	pkBuff, err := hex.DecodeString(split[0])
	require.Nil(t, err)

	msgBuff, err := hex.DecodeString(split[1])
	require.Nil(t, err)

	sigBuff, err := hex.DecodeString(split[2])
	require.Nil(t, err)

	return pkBuff, msgBuff, sigBuff
}

func Test_ManagedVerifyBLS(t *testing.T) {
	testConfig := baseTestConfig

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						key, message, sig := blsSplitString(t, blsCheckOK)
						managedTypes := host.ManagedTypes()
						keyHandle := managedTypes.NewManagedBufferFromBytes(key)
						messageHandle := managedTypes.NewManagedBufferFromBytes(message)
						sigHandle := managedTypes.NewManagedBufferFromBytes(sig)

						result := vmhooks.ManagedVerifyBLSWithHost(
							host,
							keyHandle,
							messageHandle,
							sigHandle)

						if result != 0 {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

func Test_ManagedVerifyEd25519(t *testing.T) {
	testConfig := baseTestConfig

	seed, _ := hex.DecodeString("1122334455667788990011223344556677889900112233445566778899001122")
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public()
	message := []byte("test message!")
	sig := ed25519.Sign(privateKey, message)

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						managedTypes := host.ManagedTypes()
						keyHandle := managedTypes.NewManagedBufferFromBytes(publicKey.(ed25519.PublicKey))
						messageHandle := managedTypes.NewManagedBufferFromBytes(message)
						sigHandle := managedTypes.NewManagedBufferFromBytes(sig)

						result := vmhooks.ManagedVerifyEd25519WithHost(
							host,
							keyHandle,
							messageHandle,
							sigHandle)

						if result != 0 {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

func Test_VerifySecp256k1(t *testing.T) {
	testConfig := baseTestConfig

	key, _ := hex.DecodeString("04d2e670a19c6d753d1a6d8b20bd045df8a08fb162cf508956c31268c6d81ffdabab65528eefbb8057aa85d597258a3fbd481a24633bc9b47a9aa045c91371de52")
	msg, _ := hex.DecodeString("01020304")
	r, _ := hex.DecodeString("fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c")
	s, _ := hex.DecodeString("2b8a9c0ad55394fb4aa21dc9483aea13279d6768ff2a9d6bcf589ac2613b3b02")

	verifier := secp256k1.NewSecp256k1()
	sig, err := verifier.EncodeSecp256k1DERSignature(r, s)
	assert.Nil(t, err)

	_, err = test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						managedTypes := host.ManagedTypes()
						keyHandle := managedTypes.NewManagedBufferFromBytes(key)
						messageHandle := managedTypes.NewManagedBufferFromBytes(msg)
						sigHandle := managedTypes.NewManagedBufferFromBytes(sig)

						result := vmhooks.ManagedVerifySecp256k1WithHost(
							host,
							keyHandle,
							messageHandle,
							sigHandle)

						if result != 0 {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

func Test_VerifyCustomSecp256k1(t *testing.T) {
	testConfig := baseTestConfig

	key, _ := hex.DecodeString("04d2e670a19c6d753d1a6d8b20bd045df8a08fb162cf508956c31268c6d81ffdabab65528eefbb8057aa85d597258a3fbd481a24633bc9b47a9aa045c91371de52")
	msg, _ := hex.DecodeString("01020304")
	r, _ := hex.DecodeString("fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c")
	s, _ := hex.DecodeString("2b8a9c0ad55394fb4aa21dc9483aea13279d6768ff2a9d6bcf589ac2613b3b02")

	verifier := secp256k1.NewSecp256k1()
	sig, err := verifier.EncodeSecp256k1DERSignature(r, s)
	assert.Nil(t, err)

	_, err = test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						managedTypes := host.ManagedTypes()
						keyHandle := managedTypes.NewManagedBufferFromBytes(key)
						messageHandle := managedTypes.NewManagedBufferFromBytes(msg)
						sigHandle := managedTypes.NewManagedBufferFromBytes(sig)

						result := vmhooks.ManagedVerifyCustomSecp256k1WithHost(
							host,
							keyHandle,
							messageHandle,
							sigHandle,
							int32(secp256k1.ECDSADoubleSha256))

						if result != 0 {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

func Test_ManagedEncodeSecp256k1DerSignature(t *testing.T) {
	testConfig := baseTestConfig

	// Define test cases
	testCases := []struct {
		name       string
		rHex       string
		sHex       string
		expectErr  bool
		errorMsg   string
		returnCode vmcommon.ReturnCode
		skipBuffer bool // Simulates an error when fetching bytes from managed type
	}{
		{
			name:       "Valid signature",
			rHex:       "fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c",
			sHex:       "2b8a9c0ad55394fb4aa21dc9483aea13279d6768ff2a9d6bcf589ac2613b3b02",
			expectErr:  false,
			returnCode: vmcommon.Ok,
		},
		{
			name:       "Invalid R (too short)",
			rHex:       "fef4",
			sHex:       "2b8a9c0ad55394fb4aa21dc9483aea13279d6768ff2a9d6bcf589ac2613b3b02",
			expectErr:  true,
			returnCode: vmcommon.VMUserError,
			errorMsg:   "assert failed",
		},
		{
			name:       "Invalid S (too short)",
			rHex:       "fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c",
			sHex:       "2b8a",
			expectErr:  true,
			returnCode: vmcommon.VMUserError,
			errorMsg:   "assert failed",
		},
		{
			name:       "ManagedType GetBytes error on R",
			rHex:       "fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c",
			sHex:       "2b8a9c0ad55394fb4aa21dc9483aea13279d6768ff2a9d6bcf589ac2613b3b02",
			expectErr:  true,
			returnCode: vmcommon.VMExecutionFailed,
			errorMsg:   "assert failed",
			skipBuffer: true, // Simulates GetBytes error for R
		},
		{
			name:       "Invalid signature S overflow order",
			rHex:       "fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c",
			sHex:       "d47563f52aac6b04b55de236b7c515eb9311757db01e02cff079c3ca6efb063f",
			expectErr:  true,
			returnCode: vmcommon.VMExecutionFailed,
			errorMsg:   "assert failed",
		},
		{
			name:       "Fails due to R greater than 32 bytes",
			rHex:       "fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c2b",
			sHex:       "2b8a9c0ad55394fb4aa21dc9483aea13279d6768ff2a9d6bcf589ac2613b3b02",
			expectErr:  true,
			returnCode: vmcommon.VMExecutionFailed,
		},
		{
			name:       "Fails due to S greater than 32 bytes",
			rHex:       "fef45d2892953aa5bbcdb057b5e98b208f1617a7498af7eb765574e29b5d9c2c",
			sHex:       "2b8a9c0ad55394fb4aa21dc9483aea13279d6768ff2a9d6bcf589ac2613b3b02fe",
			expectErr:  true,
			returnCode: vmcommon.VMExecutionFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := hex.DecodeString(tc.rHex)
			s, _ := hex.DecodeString(tc.sHex)

			verifier := secp256k1.NewSecp256k1()
			sig, _ := verifier.EncodeSecp256k1DERSignature(r, s)

			_, err := test.BuildMockInstanceCallTest(t).
				WithContracts(
					test.CreateMockContract(test.ParentAddress).
						WithBalance(testConfig.ParentBalance).
						WithConfig(testConfig).
						WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
							parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
								host := parentInstance.Host

								managedTypes := host.ManagedTypes()
								var rHandle, sHandle, sigHandle int32
								if tc.skipBuffer {
									rHandle = -1 // Simulate a failed buffer fetch
								} else {
									rHandle = managedTypes.NewManagedBufferFromBytes(r)
									sHandle = managedTypes.NewManagedBufferFromBytes(s)
								}
								sigHandle = managedTypes.NewManagedBuffer()

								retResult := vmhooks.ManagedEncodeSecp256k1DerSignatureWithHost(
									host,
									rHandle,
									sHandle,
									sigHandle)

								if tc.expectErr {
									if retResult == 0 {
										host.Runtime().SignalUserError(tc.errorMsg)
									}
									return parentInstance
								}

								result, _ := managedTypes.GetBytes(sigHandle)
								if retResult != 0 || !bytes.Equal(result, sig) {
									host.Runtime().SignalUserError("assert failed")
									return parentInstance
								}

								return parentInstance
							})
						}),
				).
				WithInput(test.CreateTestContractCallInputBuilder().
					WithRecipientAddr(test.ParentAddress).
					WithGasProvided(testConfig.GasProvided).
					WithFunction("testFunction").
					Build()).
				AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
					verify.ReturnCode(tc.returnCode)
				})
			assert.Nil(t, err)
		})
	}
}

func Test_ManagedScalarBaseMultEC(t *testing.T) {
	testConfig := baseTestConfig

	dataBytes, _ := hex.DecodeString("11839296a789a3bc0045c8a5fb42c7d1bd998f54449579b446817afbd17273e662c97ee72995ef42640c550b9013fad0761353c7086a272c24088be94769fd16650")

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						managedTypes := host.ManagedTypes()
						xResultHandle := managedTypes.NewBigIntFromInt64(0)
						yResultHandle := managedTypes.NewBigIntFromInt64(0)

						p521ec := elliptic.P521().Params()
						ecHandle := managedTypes.PutEllipticCurve(p521ec)
						dataHandle := managedTypes.NewManagedBufferFromBytes(dataBytes)

						retResult := vmhooks.ManagedScalarBaseMultECWithHost(
							host,
							xResultHandle,
							yResultHandle,
							ecHandle,
							dataHandle)

						if retResult != 0 {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

func Test_ManagedScalarMultEC(t *testing.T) {
	testConfig := baseTestConfig

	pointXBytes, _ := hex.DecodeString("010ba38127b62997b313aa2990a13fce55c46fc3ae751a7a7b91c41341719b57f13b9185edd96a0211acf922adb13aa9d7c64925664a9419ae6f5bc9cc4d25f91f50")
	pointYBytes, _ := hex.DecodeString("016967055bf964609b6fd853e0aa9b90d6e1e942066278a18e8604f9fcef5b64370412f20836767829ee7e0d3fc8e2e204e2a8ec4f9257a552d66647b2d1b9856223")
	dataBytes, _ := hex.DecodeString("f93e4ae433cc12cf2a43fc0ef26400c0e125508224cdb649380f25479148a4ad")

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						managedTypes := host.ManagedTypes()
						xResultHandle := managedTypes.NewBigIntFromInt64(0)
						yResultHandle := managedTypes.NewBigIntFromInt64(0)

						p521ec := elliptic.P521().Params()
						ecHandle := managedTypes.PutEllipticCurve(p521ec)
						pointXHandle := managedTypes.NewBigInt(big.NewInt(0).SetBytes(pointXBytes))
						pointYHandle := managedTypes.NewBigInt(big.NewInt(0).SetBytes(pointYBytes))
						dataHandle := managedTypes.NewManagedBufferFromBytes(dataBytes)

						retResult := vmhooks.ManagedScalarMultECWithHost(
							host,
							xResultHandle,
							yResultHandle,
							ecHandle,
							pointXHandle,
							pointYHandle,
							dataHandle)

						if retResult != 0 {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

func Test_ManagedMarshalEC(t *testing.T) {
	testConfig := baseTestConfig

	pointXBytes, _ := hex.DecodeString("010ba38127b62997b313aa2990a13fce55c46fc3ae751a7a7b91c41341719b57f13b9185edd96a0211acf922adb13aa9d7c64925664a9419ae6f5bc9cc4d25f91f50")
	pointYBytes, _ := hex.DecodeString("016967055bf964609b6fd853e0aa9b90d6e1e942066278a18e8604f9fcef5b64370412f20836767829ee7e0d3fc8e2e204e2a8ec4f9257a552d66647b2d1b9856223")
	marshalled, _ := hex.DecodeString("04010ba38127b62997b313aa2990a13fce55c46fc3ae751a7a7b91c41341719b57f13b9185edd96a0211acf922adb13aa9d7c64925664a9419ae6f5bc9cc4d25f91f50016967055bf964609b6fd853e0aa9b90d6e1e942066278a18e8604f9fcef5b64370412f20836767829ee7e0d3fc8e2e204e2a8ec4f9257a552d66647b2d1b9856223")

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						managedTypes := host.ManagedTypes()
						resultHandle := managedTypes.NewBigIntFromInt64(0)

						p521ec := elliptic.P521().Params()
						ecHandle := managedTypes.PutEllipticCurve(p521ec)
						xPairHandle := managedTypes.NewBigInt(big.NewInt(0).SetBytes(pointXBytes))
						yPairHandle := managedTypes.NewBigInt(big.NewInt(0).SetBytes(pointYBytes))

						retResult := vmhooks.ManagedMarshalECWithHost(
							host,
							xPairHandle,
							yPairHandle,
							ecHandle,
							resultHandle)

						resultBytes, _ := managedTypes.GetBytes(resultHandle)

						if retResult == -1 || !bytes.Equal(resultBytes, marshalled) {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

func Test_ManagedUnmarshalEC(t *testing.T) {
	testConfig := baseTestConfig

	pointXBytes, _ := hex.DecodeString("010ba38127b62997b313aa2990a13fce55c46fc3ae751a7a7b91c41341719b57f13b9185edd96a0211acf922adb13aa9d7c64925664a9419ae6f5bc9cc4d25f91f50")
	pointYBytes, _ := hex.DecodeString("016967055bf964609b6fd853e0aa9b90d6e1e942066278a18e8604f9fcef5b64370412f20836767829ee7e0d3fc8e2e204e2a8ec4f9257a552d66647b2d1b9856223")
	dataBytes, _ := hex.DecodeString("04010ba38127b62997b313aa2990a13fce55c46fc3ae751a7a7b91c41341719b57f13b9185edd96a0211acf922adb13aa9d7c64925664a9419ae6f5bc9cc4d25f91f50016967055bf964609b6fd853e0aa9b90d6e1e942066278a18e8604f9fcef5b64370412f20836767829ee7e0d3fc8e2e204e2a8ec4f9257a552d66647b2d1b9856223")

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						managedTypes := host.ManagedTypes()

						p521ec := elliptic.P521().Params()
						ecHandle := managedTypes.PutEllipticCurve(p521ec)
						xResultHandle := managedTypes.NewBigIntFromInt64(0)
						yResultHandle := managedTypes.NewBigIntFromInt64(0)
						dataHandle := managedTypes.NewManagedBufferFromBytes(dataBytes)

						retResult := vmhooks.ManagedUnmarshalECWithHost(
							host,
							xResultHandle,
							yResultHandle,
							ecHandle,
							dataHandle)

						xResult, _ := managedTypes.GetBigInt(xResultHandle)
						yResult, _ := managedTypes.GetBigInt(yResultHandle)

						if retResult == -1 ||
							!bytes.Equal(xResult.Bytes(), pointXBytes) ||
							!bytes.Equal(yResult.Bytes(), pointYBytes) {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

func Test_ManagedMarshalCompressedEC(t *testing.T) {
	testConfig := baseTestConfig

	pointXBytes, _ := hex.DecodeString("010ba38127b62997b313aa2990a13fce55c46fc3ae751a7a7b91c41341719b57f13b9185edd96a0211acf922adb13aa9d7c64925664a9419ae6f5bc9cc4d25f91f50")
	pointYBytes, _ := hex.DecodeString("016967055bf964609b6fd853e0aa9b90d6e1e942066278a18e8604f9fcef5b64370412f20836767829ee7e0d3fc8e2e204e2a8ec4f9257a552d66647b2d1b9856223")
	marshalled, _ := hex.DecodeString("03010ba38127b62997b313aa2990a13fce55c46fc3ae751a7a7b91c41341719b57f13b9185edd96a0211acf922adb13aa9d7c64925664a9419ae6f5bc9cc4d25f91f50")

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						managedTypes := host.ManagedTypes()
						resultHandle := managedTypes.NewBigIntFromInt64(0)

						p521ec := elliptic.P521().Params()
						ecHandle := managedTypes.PutEllipticCurve(p521ec)
						xPairHandle := managedTypes.NewBigInt(big.NewInt(0).SetBytes(pointXBytes))
						yPairHandle := managedTypes.NewBigInt(big.NewInt(0).SetBytes(pointYBytes))

						retResult := vmhooks.ManagedMarshalCompressedECWithHost(
							host,
							xPairHandle,
							yPairHandle,
							ecHandle,
							resultHandle)

						resultBytes, _ := managedTypes.GetBytes(resultHandle)

						if retResult == -1 || !bytes.Equal(resultBytes, marshalled) {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

func Test_ManagedUnmarshalCompressedEC(t *testing.T) {
	testConfig := baseTestConfig

	pointXBytes, _ := hex.DecodeString("010ba38127b62997b313aa2990a13fce55c46fc3ae751a7a7b91c41341719b57f13b9185edd96a0211acf922adb13aa9d7c64925664a9419ae6f5bc9cc4d25f91f50")
	pointYBytes, _ := hex.DecodeString("016967055bf964609b6fd853e0aa9b90d6e1e942066278a18e8604f9fcef5b64370412f20836767829ee7e0d3fc8e2e204e2a8ec4f9257a552d66647b2d1b9856223")
	dataBytes, _ := hex.DecodeString("03010ba38127b62997b313aa2990a13fce55c46fc3ae751a7a7b91c41341719b57f13b9185edd96a0211acf922adb13aa9d7c64925664a9419ae6f5bc9cc4d25f91f50")

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						managedTypes := host.ManagedTypes()

						p521ec := elliptic.P521().Params()
						ecHandle := managedTypes.PutEllipticCurve(p521ec)
						xResultHandle := managedTypes.NewBigIntFromInt64(0)
						yResultHandle := managedTypes.NewBigIntFromInt64(0)
						dataHandle := managedTypes.NewManagedBufferFromBytes(dataBytes)

						retResult := vmhooks.ManagedUnmarshalCompressedECWithHost(
							host,
							xResultHandle,
							yResultHandle,
							ecHandle,
							dataHandle)

						xResult, _ := managedTypes.GetBigInt(xResultHandle)
						yResult, _ := managedTypes.GetBigInt(yResultHandle)

						if retResult == -1 ||
							!bytes.Equal(xResult.Bytes(), pointXBytes) ||
							!bytes.Equal(yResult.Bytes(), pointYBytes) {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

func Test_ManagedGenerateKeyEC(t *testing.T) {
	testConfig := baseTestConfig

	pointXBytes, _ := hex.DecodeString("010ba38127b62997b313aa2990a13fce55c46fc3ae751a7a7b91c41341719b57f13b9185edd96a0211acf922adb13aa9d7c64925664a9419ae6f5bc9cc4d25f91f50")
	pointYBytes, _ := hex.DecodeString("016967055bf964609b6fd853e0aa9b90d6e1e942066278a18e8604f9fcef5b64370412f20836767829ee7e0d3fc8e2e204e2a8ec4f9257a552d66647b2d1b9856223")
	expectedResultBytes, _ := hex.DecodeString("00ddb81d205713945e203848e2f5c312067649f9a40727ca26b672b164cd1f9108f564958b20312146bb9750b74757d97cfbbba2aedebaba3a68fe3f2d669a992fab")

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						managedTypes := host.ManagedTypes()

						p521ec := elliptic.P521().Params()
						ecHandle := managedTypes.PutEllipticCurve(p521ec)
						xResultHandle := managedTypes.NewBigInt(big.NewInt(0).SetBytes(pointXBytes))
						yResultHandle := managedTypes.NewBigInt(big.NewInt(0).SetBytes(pointYBytes))
						resultHandle := managedTypes.NewManagedBuffer()

						retResult := vmhooks.ManagedGenerateKeyECWithHost(
							host,
							xResultHandle,
							yResultHandle,
							ecHandle,
							resultHandle)

						resultBytes, _ := managedTypes.GetBytes(resultHandle)
						if retResult != 0 ||
							!bytes.Equal(resultBytes, expectedResultBytes) {
							host.Runtime().SignalUserError("assert failed")
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

func Test_ManagedCreateEC(t *testing.T) {
	testConfig := baseTestConfig

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host

						p224ec := elliptic.P224().Params()
						if !checkCreateECSuccess(host, "p224", p224ec) {
							return parentInstance
						}

						p256ec := elliptic.P256().Params()
						if !checkCreateECSuccess(host, "p256", p256ec) {
							return parentInstance
						}

						p384ec := elliptic.P384().Params()
						if !checkCreateECSuccess(host, "p384", p384ec) {
							return parentInstance
						}

						p521ec := elliptic.P521().Params()
						if !checkCreateECSuccess(host, "p521", p521ec) {
							return parentInstance
						}

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok()
		})
	assert.Nil(t, err)
}

func checkCreateECSuccess(host vmhost.VMHost, name string, ecParams *elliptic.CurveParams) bool {
	managedTypes := host.ManagedTypes()
	dataHandle := managedTypes.NewManagedBufferFromBytes([]byte(name))

	retResult := vmhooks.ManagedCreateECWithHost(
		host,
		dataHandle)

	resultEC, _ := managedTypes.GetEllipticCurve(retResult)
	if resultEC.Params().Name != ecParams.Name {
		host.Runtime().SignalUserError("assert failed")
		return false
	}

	return true
}

func Test_ManagedDeleteContract(t *testing.T) {
	testConfig := baseTestConfig

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithCodeMetadata([]byte{vmcommon.MetadataUpgradeable, 0}).
				WithOwnerAddress(test.ParentAddress).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host
						managedTypes := host.ManagedTypes()

						argumentsHandle := managedTypes.NewManagedBuffer()
						require.NoError(t, managedTypes.WriteManagedVecOfManagedBuffers([][]byte{{1, 2}, {3, 4}}, argumentsHandle))

						destHandle := managedTypes.NewManagedBufferFromBytes(test.ParentAddress)

						vmhooks.ManagedDeleteContractWithHost(
							host,
							destHandle,
							100000,
							argumentsHandle)

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				Ok().
				DeletedAccounts(test.ParentAddress)
		})
	assert.Nil(t, err)
}

var (
	parentDeleteTarget       = test.MakeTestSCAddressWithDefaultVM("parentTarget")
	childDeleteTarget        = test.MakeTestSCAddressWithDefaultVM("childTarget")
	failingChildDeleteTarget = test.MakeTestSCAddressWithDefaultVM("failingTarget")
)

func deleteContractWithHost(t *testing.T, host vmhost.VMHost, target []byte) {
	managedTypes := host.ManagedTypes()
	argumentsHandle := managedTypes.NewManagedBuffer()
	require.NoError(t, managedTypes.WriteManagedVecOfManagedBuffers(nil, argumentsHandle))
	destHandle := managedTypes.NewManagedBufferFromBytes(target)

	vmhooks.ManagedDeleteContractWithHost(host, destHandle, 100000, argumentsHandle)
}

func callChildOnDestContext(host vmhost.VMHost, function string) int32 {
	return vmhooks.ExecuteOnDestContextWithTypedArgs(host, 100000, big.NewInt(0), []byte(function), test.ChildAddress, [][]byte{})
}

func callChildOnDestContextTimes(t *testing.T, host vmhost.VMHost, function string, numCalls int) {
	for i := 0; i < numCalls; i++ {
		require.Equal(t, int32(0), callChildOnDestContext(host, function))
	}
}

func runManagedDeleteContractScenario(t *testing.T, fixAuditChangesV6 uint32, parentFunction func(host vmhost.VMHost)) *vmcommon.VMOutput {
	testConfig := baseTestConfig

	deleteTarget := func(address []byte, owner []byte) test.MockTestSmartContract {
		return test.CreateMockContract(address).
			WithCodeMetadata([]byte{vmcommon.MetadataUpgradeable, 0}).
			WithOwnerAddress(owner).
			WithMethods()
	}

	vmOutput, err := test.BuildMockInstanceCallTest(t).
		WithEnableEpochs(blockchainConfig.EnableEpochs{FixAuditChangesV6: fixAuditChangesV6}).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						parentFunction(parentInstance.Host)
						return parentInstance
					})
				}),
			test.CreateMockContract(test.ChildAddress).
				WithMethods(func(childInstance *mock.InstanceMock, config interface{}) {
					childInstance.AddMockMethod("doNothing", func() *mock.InstanceMock {
						return childInstance
					})
					childInstance.AddMockMethod("deleteChildTarget", func() *mock.InstanceMock {
						deleteContractWithHost(t, childInstance.Host, childDeleteTarget)
						return childInstance
					})
					childInstance.AddMockMethod("deleteChildTargetAndFail", func() *mock.InstanceMock {
						deleteContractWithHost(t, childInstance.Host, failingChildDeleteTarget)
						childInstance.Host.Runtime().SignalUserError("child failed after delete")
						return childInstance
					})
					childInstance.AddMockMethod("deleteParentTargetOnSameContext", func() *mock.InstanceMock {
						deleteContractWithHost(t, childInstance.Host, parentDeleteTarget)
						return childInstance
					})
				}),
			deleteTarget(parentDeleteTarget, test.ParentAddress),
			deleteTarget(childDeleteTarget, test.ChildAddress),
			deleteTarget(failingChildDeleteTarget, test.ChildAddress),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			setZeroCodeCosts(host)
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok()
		})
	require.NoError(t, err)

	return vmOutput
}

func Test_ManagedDeleteContract_ListedOnceAfterNestedCalls(t *testing.T) {
	vmOutput := runManagedDeleteContractScenario(t, 0, func(host vmhost.VMHost) {
		deleteContractWithHost(t, host, parentDeleteTarget)
		callChildOnDestContextTimes(t, host, "doNothing", 10)
	})

	require.Equal(t, [][]byte{parentDeleteTarget}, vmOutput.DeletedAccounts)
}

func Test_ManagedDeleteContract_RepeatedAfterNestedCallsBeforeFixAuditChangesV6(t *testing.T) {
	vmOutput := runManagedDeleteContractScenario(t, 1_000_000, func(host vmhost.VMHost) {
		deleteContractWithHost(t, host, parentDeleteTarget)
		callChildOnDestContextTimes(t, host, "doNothing", 3)
	})

	require.Len(t, vmOutput.DeletedAccounts, 8)
	for _, address := range vmOutput.DeletedAccounts {
		require.Equal(t, parentDeleteTarget, address)
	}
}

func Test_ManagedDeleteContract_ParentAndChildDeletionsListedOnce(t *testing.T) {
	vmOutput := runManagedDeleteContractScenario(t, 0, func(host vmhost.VMHost) {
		deleteContractWithHost(t, host, parentDeleteTarget)
		callChildOnDestContextTimes(t, host, "deleteChildTarget", 1)
		callChildOnDestContextTimes(t, host, "doNothing", 5)
	})

	require.Equal(t, [][]byte{parentDeleteTarget, childDeleteTarget}, vmOutput.DeletedAccounts)
}

func Test_ManagedDeleteContract_RepeatedDeleteListedOncePerCall(t *testing.T) {
	vmOutput := runManagedDeleteContractScenario(t, 0, func(host vmhost.VMHost) {
		deleteContractWithHost(t, host, parentDeleteTarget)
		deleteContractWithHost(t, host, parentDeleteTarget)
		callChildOnDestContextTimes(t, host, "doNothing", 5)
	})

	require.Equal(t, [][]byte{parentDeleteTarget, parentDeleteTarget}, vmOutput.DeletedAccounts)
}

func Test_ManagedDeleteContract_OnSameContextListedOnceAfterNestedCalls(t *testing.T) {
	vmOutput := runManagedDeleteContractScenario(t, 0, func(host vmhost.VMHost) {
		result := vmhooks.ExecuteOnSameContextWithTypedArgs(host, 100000, big.NewInt(0), []byte("deleteParentTargetOnSameContext"), test.ChildAddress, [][]byte{})
		require.Equal(t, int32(0), result)
		callChildOnDestContextTimes(t, host, "doNothing", 5)
	})

	require.Equal(t, [][]byte{parentDeleteTarget}, vmOutput.DeletedAccounts)
}

func Test_ManagedDeleteContract_FailedNestedCallDropsItsDeletion(t *testing.T) {
	vmOutput := runManagedDeleteContractScenario(t, 0, func(host vmhost.VMHost) {
		callChildOnDestContextTimes(t, host, "deleteChildTarget", 1)

		failingInput := test.DefaultTestContractCallInput()
		failingInput.CallerAddr = test.ParentAddress
		failingInput.RecipientAddr = test.ChildAddress
		failingInput.Function = "deleteChildTargetAndFail"
		failingInput.GasProvided = host.Metering().GasLeft() / 2
		_, err := host.ExecuteOnDestContext(failingInput)
		require.Error(t, err)

		callChildOnDestContextTimes(t, host, "doNothing", 3)
	})

	require.Equal(t, [][]byte{childDeleteTarget}, vmOutput.DeletedAccounts)
}

func TestBaseOpsAPI_NFTNonceOverflow(t *testing.T) {
	testConfig := makeTestConfig()

	MaxUint := ^uint64(0)
	MaxInt := int64(MaxUint >> 1)

	OverflowedMaxInt := uint64(MaxInt) + 1

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(testConfig.ParentBalance).
				WithConfig(testConfig).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host
						managed := host.ManagedTypes()

						addressHandle := managed.NewManagedBufferFromBytes(test.ParentAddress)
						tickerHandle := managed.NewManagedBufferFromBytes(test.KDATestTokenName)

						nonce := int64(OverflowedMaxInt)

						precisionHandle := managed.NewBigIntFromInt64(0)
						idHandle := managed.NewManagedBuffer()
						logoHandle := managed.NewManagedBuffer()
						nameHandle := managed.NewManagedBuffer()
						initialSupplyHandle := managed.NewManagedBuffer()
						creatorHandle := managed.NewManagedBuffer()
						adminHandle := managed.NewManagedBuffer()
						royaltiesHandle := managed.NewManagedBuffer()
						urisHandle := managed.NewManagedBuffer()
						circulatingSupplyHandle := managed.NewManagedBuffer()
						maxSupplyHandle := managed.NewManagedBuffer()
						mintedHandle := managed.NewManagedBuffer()
						burnedHandle := managed.NewManagedBuffer()
						propertiesHandle := managed.NewManagedBuffer()
						attributesHandle := managed.NewManagedBuffer()
						rolesHandle := managed.NewManagedBuffer()
						issueDateHandle := managed.NewManagedBuffer()

						vmhooks.ManagedGetKDATokenDataWithHost(
							host,
							addressHandle,
							tickerHandle,
							nonce,
							precisionHandle,
							idHandle,
							nameHandle,
							creatorHandle,
							adminHandle,
							logoHandle,
							urisHandle,
							initialSupplyHandle,
							circulatingSupplyHandle,
							maxSupplyHandle,
							mintedHandle,
							burnedHandle,
							royaltiesHandle,
							propertiesHandle,
							attributesHandle,
							rolesHandle,
							issueDateHandle,
						)

						value, err := managed.GetBigInt(precisionHandle)
						if err != nil {
							host.Runtime().SignalUserError(err.Error())
							return parentInstance
						}
						host.Output().Finish(value.Bytes())

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(testConfig.GasProvided).
			WithFunction("testFunction").
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			createMockBuiltinFunctions(t, host, world)
			setZeroCodeCosts(host)
			world.MockAsset = &kapps.KDAData{
				AssetType:         1,
				ID:                test.KDATestTokenName,
				Name:              []byte("test"),
				Ticker:            []byte("TTT"),
				OwnerAddress:      test.ParentAddress,
				AdminAddress:      nil,
				Logo:              "",
				URIs:              nil,
				Precision:         0,
				InitialSupply:     1,
				CirculatingSupply: 1,
				MaxSupply:         10,
				MintedValue:       5,
				BurnedValue:       5,
				IssueDate:         time.Now().Unix(),
				Royalties:         nil,
				Properties: &kapps.PropertiesData{
					CanFreeze:      true,
					CanWipe:        true,
					CanPause:       true,
					CanMint:        true,
					CanBurn:        true,
					CanChangeOwner: true,
					CanAddRoles:    true,
				},
				Attributes: &kapps.AttributesData{
					IsPaused:                   false,
					IsNFTMintStopped:           false,
					IsRoyaltiesChangeStopped:   false,
					IsNFTMetadataChangeStopped: false,
				},
				Roles: nil,
			}
		}).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.
				WithTrace().
				Ok()
		})
	assert.Nil(t, err)
}

func opsValue(ops ...transaction.TXContract_ContractType) uint64 {
	value := uint64(0)
	for _, op := range ops {
		value |= 1 << uint(op)
	}

	return value
}

func opsBytes(ops ...transaction.TXContract_ContractType) []byte {
	return transaction.EncodeContractPermissions(ops...)
}

func TestManagedei_ManagedAccHasPerm_Helper_Func_ValidatePerm(t *testing.T) {

	testVectors := []struct {
		name          string
		targetAddress []byte
		perm          []*state.Permission
		contractType  []transaction.TXContract_ContractType
		expected      int32
	}{
		{
			name:          "no permission for target",
			targetAddress: []byte("targetaddress"),
			perm: []*state.Permission{
				{
					Type:       state.Permission_Owner,
					Threshold:  1,
					Operations: opsBytes(transaction.TXContract_TransferContractType),
					Signers:    []*state.Key{{Address: []byte("sourceaddress"), Weight: 1}},
				},
			},
			contractType: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType},
			expected:     0,
		},
		{
			name:          "no permission",
			targetAddress: []byte("targetaddress"),
			perm: []*state.Permission{
				{
					Type:       state.Permission_User,
					Threshold:  1,
					Operations: nil,
					Signers:    []*state.Key{{Address: []byte("targetaddress"), Weight: 1}},
				},
			},
			contractType: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType},
			expected:     0,
		},
		{
			name:          "no permission slice 8",
			targetAddress: []byte("targetaddress"),
			perm: []*state.Permission{
				{
					Type:       state.Permission_User,
					Threshold:  1,
					Operations: make([]byte, 8),
					Signers:    []*state.Key{{Address: []byte("targetaddress"), Weight: 1}},
				},
			},
			contractType: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType},
			expected:     0,
		},
		{
			name:          "wrong permission",
			targetAddress: []byte("targetaddress"),
			perm: []*state.Permission{
				{
					Type:       state.Permission_User,
					Threshold:  1,
					Operations: opsBytes(transaction.TXContract_AssetTriggerContractType),
					Signers:    []*state.Key{{Address: []byte("targetaddress"), Weight: 1}},
				},
			},
			contractType: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType},
			expected:     0,
		},
		{
			name:          "has permission",
			targetAddress: []byte("targetaddress"),
			perm: []*state.Permission{
				{
					Type:       state.Permission_User,
					Threshold:  1,
					Operations: opsBytes(transaction.TXContract_TransferContractType),
					Signers:    []*state.Key{{Address: []byte("targetaddress"), Weight: 1}},
				},
			},
			contractType: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType},
			expected:     1,
		},
		{
			name:          "has owner permission",
			targetAddress: []byte("targetaddress"),
			perm: []*state.Permission{
				{
					Type:       state.Permission_Owner,
					Threshold:  1,
					Operations: nil,
					Signers:    []*state.Key{{Address: []byte("targetaddress"), Weight: 1}},
				},
			},
			contractType: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType},
			expected:     1,
		},
		{
			name:          "low threshold permission",
			targetAddress: []byte("targetaddress"),
			perm: []*state.Permission{
				{
					Type:       state.Permission_Owner,
					Threshold:  2,
					Operations: opsBytes(transaction.TXContract_TransferContractType),
					Signers:    []*state.Key{{Address: []byte("targetaddress"), Weight: 1}},
				},
			},
			contractType: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType},
			expected:     0,
		},
		{
			name:          "missing permission",
			targetAddress: []byte("targetaddress"),
			perm: []*state.Permission{
				{
					Type:       state.Permission_User,
					Threshold:  1,
					Operations: opsBytes(transaction.TXContract_TransferContractType),
					Signers:    []*state.Key{{Address: []byte("targetaddress"), Weight: 1}},
				},
			},
			contractType: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType, transaction.TXContract_AssetTriggerContractType},
			expected:     0,
		},
		{
			name:          "double permission",
			targetAddress: []byte("targetaddress"),
			perm: []*state.Permission{
				{
					Type:       state.Permission_User,
					Threshold:  1,
					Operations: opsBytes(transaction.TXContract_TransferContractType, transaction.TXContract_AssetTriggerContractType),
					Signers:    []*state.Key{{Address: []byte("targetaddress"), Weight: 1}},
				},
			},
			contractType: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType, transaction.TXContract_AssetTriggerContractType},
			expected:     1,
		},
		{
			name:          "all permission slice 8",
			targetAddress: []byte("targetaddress"),
			perm: []*state.Permission{
				{
					Type:       state.Permission_User,
					Threshold:  1,
					Operations: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
					Signers:    []*state.Key{{Address: []byte("targetaddress"), Weight: 1}},
				},
			},
			contractType: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType, transaction.TXContract_AssetTriggerContractType},
			expected:     1,
		},
		{
			name:          "all permission slice > 8",
			targetAddress: []byte("targetaddress"),
			perm: []*state.Permission{
				{
					Type:       state.Permission_User,
					Threshold:  1,
					Operations: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
					Signers:    []*state.Key{{Address: []byte("targetaddress"), Weight: 1}},
				},
			},
			contractType: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType, transaction.TXContract_AssetTriggerContractType},
			expected:     1,
		},
		{
			name:          "doc example",
			targetAddress: []byte("targetaddress"),
			perm: []*state.Permission{
				{
					Type:       state.Permission_User,
					Threshold:  1,
					Operations: []byte{0xAF, 0x01},
					Signers:    []*state.Key{{Address: []byte("targetaddress"), Weight: 1}},
				},
			},
			contractType: []transaction.TXContract_ContractType{
				transaction.TXContract_TransferContractType,
				transaction.TXContract_CreateAssetContractType,
				transaction.TXContract_CreateValidatorContractType,
				transaction.TXContract_ValidatorConfigContractType,
				transaction.TXContract_UnfreezeContractType,
				transaction.TXContract_UndelegateContractType,
				transaction.TXContract_WithdrawContractType,
			},
			expected: 1,
		},
		{
			name:          "doc example, one invalid operation",
			targetAddress: []byte("targetaddress"),
			perm: []*state.Permission{
				{
					Type:       state.Permission_User,
					Threshold:  1,
					Operations: []byte{0xAF, 0x01},
					Signers:    []*state.Key{{Address: []byte("targetaddress"), Weight: 1}},
				},
			},
			contractType: []transaction.TXContract_ContractType{
				transaction.TXContract_TransferContractType,
				transaction.TXContract_CreateAssetContractType,
				transaction.TXContract_CreateValidatorContractType,
				transaction.TXContract_ValidatorConfigContractType,
				transaction.TXContract_UnfreezeContractType,
				transaction.TXContract_UndelegateContractType,
				transaction.TXContract_WithdrawContractType,
				transaction.TXContract_ClaimContractType,
			},
			expected: 0,
		},
	}

	for _, tv := range testVectors {
		sourceAccount := state.NewEmptyUserAccount()
		sourceAccount.SetPermissions(tv.perm)

		result := vmhooks.ValidatePerm(
			sourceAccount,
			tv.targetAddress,
			opsValue(tv.contractType...),
		)
		assert.Equal(t, tv.expected, result, tv.name)
	}
}

// A key or signature buffer longer than its scheme's size must never verify. Checking only a prefix
// of it would let a contract that keys on the whole buffer (a used-signature set, a per-key
// allowlist) accept the same signature or key again, padded differently. From FixAuditChangesV6 the
// size is checked before anything is copied.
func Test_ManagedVerify_RejectsPaddedKeyOrSignature_PostFork(t *testing.T) {
	seed, _ := hex.DecodeString("1122334455667788990011223344556677889900112233445566778899001122")
	edPrivateKey := ed25519.NewKeyFromSeed(seed)
	edKey := []byte(edPrivateKey.Public().(ed25519.PublicKey))
	edMessage := []byte("test message!")
	edSig := ed25519.Sign(edPrivateKey, edMessage)
	blsKey, blsMessage, blsSig := blsSplitString(t, blsCheckOK)
	pad := func(b []byte) []byte { return append(bytes.Clone(b), 0) }

	tests := []struct {
		name              string
		verify            func(host vmhost.VMHost, keyHandle, messageHandle, sigHandle int32) int32
		key, message, sig []byte
		wantErr           error
	}{
		{"BLS padded key", vmhooks.ManagedVerifyBLSWithHost, pad(blsKey), blsMessage, blsSig, vmhost.ErrInvalidPublicKeySize},
		{"BLS padded signature", vmhooks.ManagedVerifyBLSWithHost, blsKey, blsMessage, pad(blsSig), vmhost.ErrInvalidSignatureSize},
		{"Ed25519 padded key", vmhooks.ManagedVerifyEd25519WithHost, pad(edKey), edMessage, edSig, vmhost.ErrInvalidPublicKeySize},
		{"Ed25519 padded signature", vmhooks.ManagedVerifyEd25519WithHost, edKey, edMessage, pad(edSig), vmhost.ErrInvalidSignatureSize},
		{"secp256k1 key of neither size", vmhooks.ManagedVerifySecp256k1WithHost, make([]byte, 34), edMessage, edSig, vmhost.ErrInvalidPublicKeySize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runManagedHookForkGateTest(t, nil,
				func(host vmhost.VMHost) {
					managedTypes := host.ManagedTypes()
					tt.verify(host,
						managedTypes.NewManagedBufferFromBytes(tt.key),
						managedTypes.NewManagedBufferFromBytes(tt.message),
						managedTypes.NewManagedBufferFromBytes(tt.sig))
				},
				func(verify *test.VMOutputVerifier) {
					verify.ExecutionFailed().ReturnMessage(tt.wantErr.Error())
				},
			)
		})
	}
}

// Before the fork a padded Ed25519 signature was copied whole and failed in ed25519.Verify, so it is
// still rejected, only by the verifier rather than by the size check.
func Test_ManagedVerifyEd25519_RejectsPaddedSignature_PreFork(t *testing.T) {
	seed, _ := hex.DecodeString("1122334455667788990011223344556677889900112233445566778899001122")
	privateKey := ed25519.NewKeyFromSeed(seed)
	message := []byte("test message!")
	paddedSig := append(ed25519.Sign(privateKey, message), 0)

	runManagedHookForkGateTest(t, &blockchainConfig.EnableEpochs{FixAuditChangesV6: 1},
		func(host vmhost.VMHost) {
			managedTypes := host.ManagedTypes()
			vmhooks.ManagedVerifyEd25519WithHost(host,
				managedTypes.NewManagedBufferFromBytes(privateKey.Public().(ed25519.PublicKey)),
				managedTypes.NewManagedBufferFromBytes(message),
				managedTypes.NewManagedBufferFromBytes(paddedSig))
		},
		func(verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().ReturnMessage("invalid signature")
		},
	)
}

// Before FixAuditChangesV6 ManagedUnmarshalEC returned 1 on a missing data buffer without failing the
// execution. From it the data is paid for before it is loaded, so a load error has to stop the contract
// instead of letting it carry on, unpaid.
func Test_ManagedUnmarshalEC_MissingBufferFailsExecution_PostFork(t *testing.T) {
	runManagedHookForkGateTest(t, nil,
		func(host vmhost.VMHost) {
			callManagedUnmarshalECOnMissingBuffer(host)
		},
		func(verify *test.VMOutputVerifier) {
			verify.ExecutionFailed().ReturnMessage(vmhost.ErrNoManagedBufferUnderThisHandle.Error())
		},
	)
}

func Test_ManagedUnmarshalEC_MissingBufferReturnsWithoutFailing_PreFork(t *testing.T) {
	runManagedHookForkGateTest(t, &blockchainConfig.EnableEpochs{FixAuditChangesV6: 1},
		func(host vmhost.VMHost) {
			if callManagedUnmarshalECOnMissingBuffer(host) != 1 {
				host.Runtime().SignalUserError("assert failed")
			}
		},
		func(verify *test.VMOutputVerifier) {
			verify.Ok()
		},
	)
}

func callManagedUnmarshalECOnMissingBuffer(host vmhost.VMHost) int32 {
	const missingDataHandle = int32(12345)
	managedTypes := host.ManagedTypes()
	return vmhooks.ManagedUnmarshalECWithHost(host,
		managedTypes.NewBigIntFromInt64(0),
		managedTypes.NewBigIntFromInt64(0),
		managedTypes.PutEllipticCurve(elliptic.P256().Params()),
		missingDataHandle)
}

// From FixAuditChangesV6 a scalar longer than the curve order's byte size is reduced modulo the order
// before the multiplication. The curves have cofactor 1, so that has to be invisible: every long scalar
// here goes through both managed scalar hooks before and after the fork, and the outcome (point, return
// code, message, gas) must not change. Go's constant-time curves, which reduce the same way, give the
// expected point.
func Test_ManagedScalarMultEC_LongScalarKeepsHistoricalResult(t *testing.T) {
	// 32 bytes, used on P-224 by crypto_elliptic_curves_legacy.scen.json
	legacyScalar, _ := hex.DecodeString("f93e4ae433cc12cf2a43fc0ef26400c0e125508224cdb649380f25479148a4ad")
	rng := rand.New(rand.NewPCG(1, 2))
	randomBytes := func(n int) []byte {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(rng.Uint32())
		}
		return b
	}

	type outcome struct {
		x, y         string
		returnCode   vmcommon.ReturnCode
		message      string
		gasRemaining uint64
	}
	type scalarHook func(host vmhost.VMHost, xHandle, yHandle, ecHandle, dataHandle int32) int32

	run := func(t *testing.T, enableEpochs *blockchainConfig.EnableEpochs, hook scalarHook, curve *elliptic.CurveParams, scalar []byte) outcome {
		var got outcome
		runManagedHookForkGateTest(t, enableEpochs,
			func(host vmhost.VMHost) {
				managedTypes := host.ManagedTypes()
				xHandle := managedTypes.NewBigIntFromInt64(0)
				yHandle := managedTypes.NewBigIntFromInt64(0)
				hook(host, xHandle, yHandle, managedTypes.PutEllipticCurve(curve), managedTypes.NewManagedBufferFromBytes(scalar))
				x, _ := managedTypes.GetBigInt(xHandle)
				y, _ := managedTypes.GetBigInt(yHandle)
				got.x, got.y = x.Text(16), y.Text(16)
			},
			func(verify *test.VMOutputVerifier) {
				got.returnCode = verify.VmOutput.ReturnCode
				got.message = verify.VmOutput.ReturnMessage
				got.gasRemaining = verify.VmOutput.GasRemaining
			},
		)
		return got
	}

	for _, curve := range []elliptic.Curve{elliptic.P224(), elliptic.P256(), elliptic.P384(), elliptic.P521()} {
		params := curve.Params()
		byteLen := (params.N.BitLen() + 7) / 8
		pointX, pointY := curve.ScalarBaseMult([]byte{7})

		scalars := []struct {
			name   string
			scalar []byte
		}{
			{"a small scalar padded with leading zeros", append(make([]byte, byteLen+4), 7)},
			{"the order, padded", append([]byte{0}, params.N.Bytes()...)},
			{"the order plus one, padded", append([]byte{0}, new(big.Int).Add(params.N, big.NewInt(1)).Bytes()...)},
			{"a large multiple of the order minus one", new(big.Int).Sub(new(big.Int).Lsh(params.N, 40), big.NewInt(1)).Bytes()},
			{"a large multiple of the order plus five", new(big.Int).Add(new(big.Int).Lsh(params.N, 64), big.NewInt(5)).Bytes()},
			{"random, one byte too long", randomBytes(byteLen + 1)},
			{"random, twice as long", randomBytes(2*byteLen + 3)},
			{"the legacy scenario's 32 bytes", legacyScalar},
		}

		hooks := []struct {
			name     string
			hook     scalarHook
			expected func(scalar []byte) (*big.Int, *big.Int)
		}{
			{
				"ManagedScalarBaseMultEC",
				func(host vmhost.VMHost, xHandle, yHandle, ecHandle, dataHandle int32) int32 {
					return vmhooks.ManagedScalarBaseMultECWithHost(host, xHandle, yHandle, ecHandle, dataHandle)
				},
				curve.ScalarBaseMult,
			},
			{
				"ManagedScalarMultEC",
				func(host vmhost.VMHost, xHandle, yHandle, ecHandle, dataHandle int32) int32 {
					managedTypes := host.ManagedTypes()
					return vmhooks.ManagedScalarMultECWithHost(host, xHandle, yHandle, ecHandle,
						managedTypes.NewBigInt(new(big.Int).Set(pointX)), managedTypes.NewBigInt(new(big.Int).Set(pointY)), dataHandle)
				},
				func(scalar []byte) (*big.Int, *big.Int) { return curve.ScalarMult(pointX, pointY, scalar) },
			},
		}

		for _, sc := range scalars {
			for _, h := range hooks {
				t.Run(params.Name+"/"+h.name+"/"+sc.name, func(t *testing.T) {
					before := run(t, &blockchainConfig.EnableEpochs{FixAuditChangesV6: 1}, h.hook, params, sc.scalar)
					after := run(t, nil, h.hook, params, sc.scalar)

					require.Equal(t, before, after)

					if new(big.Int).Mod(new(big.Int).SetBytes(sc.scalar), params.N).Sign() == 0 {
						// k·P is the point at infinity, which the hooks have always refused
						require.Equal(t, vmcommon.VMExecutionFailed, after.returnCode)
						require.Equal(t, vmhost.ErrPointNotOnCurve.Error(), after.message)
						return
					}
					require.Equal(t, vmcommon.Ok, after.returnCode, after.message)
					expectedX, expectedY := h.expected(sc.scalar)
					require.Equal(t, expectedX.Text(16), after.x)
					require.Equal(t, expectedY.Text(16), after.y)
				})
			}
		}
	}
}
