package vmhooks_test

import (
	"encoding/hex"
	"testing"

	"github.com/klever-io/klever-go/kvm/crypto/factory"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/stretchr/testify/require"
)

func TestEncodeSecp256k1DerSignature(t *testing.T) {
	t.Parallel()

	const sigSize = 71

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
