package kda_test

import (
	"strings"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/require"
)

func TestValidateCreateAsset(t *testing.T) {
	t.Parallel()

	// Standard setup
	pubkeyConv := &mock.PubkeyConverterStub{
		LenCalled: func() int {
			return 32
		},
	}

	createValidContract := func() *transaction.CreateAssetContract {
		owner := make([]byte, 32)
		copy(owner, []byte("owner"))
		return &transaction.CreateAssetContract{
			Name:         []byte("ValidTokenName"),
			Ticker:       []byte("VTK"),
			OwnerAddress: owner,
			MaxSupply:    1000000,
		}
	}

	t.Run("ValidAsset_WithSmartContracts", func(t *testing.T) {
		tc := createValidContract()
		forkController := &mock.ForkControllerStub{
			EnableSmartContractsValue: true,
		}

		code, err := kda.ValidateCreateAsset(tc, forkController, pubkeyConv)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})

	t.Run("ValidAsset_WithoutSmartContracts", func(t *testing.T) {
		tc := createValidContract()
		forkController := &mock.ForkControllerStub{
			EnableSmartContractsValue: false,
		}

		code, err := kda.ValidateCreateAsset(tc, forkController, pubkeyConv)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})

	t.Run("InvalidAssetName_NotHumanReadable", func(t *testing.T) {
		tc := createValidContract()
		tc.Name = []byte{0xFF, 0xFE, 0xFD} // Invalid UTF-8
		forkController := &mock.ForkControllerStub{
			EnableSmartContractsValue: true,
		}

		code, err := kda.ValidateCreateAsset(tc, forkController, pubkeyConv)
		require.Error(t, err)
		require.Equal(t, process.ErrTokenNameNotHumanReadable, err)
		require.Equal(t, transaction.Transaction_ContractInvalid, code)
	})

	t.Run("InvalidTicker", func(t *testing.T) {
		tc := createValidContract()
		tc.Ticker = []byte("KLV") // Reserved ticker
		forkController := &mock.ForkControllerStub{
			EnableSmartContractsValue: true,
		}

		code, err := kda.ValidateCreateAsset(tc, forkController, pubkeyConv)
		require.Error(t, err)
		require.Equal(t, process.ErrTickerNameNotValid, err)
		require.Equal(t, transaction.Transaction_ContractInvalid, code)
	})

	t.Run("InvalidOwnerAddress_Length", func(t *testing.T) {
		tc := createValidContract()
		tc.OwnerAddress = []byte("short") // Too short
		forkController := &mock.ForkControllerStub{
			EnableSmartContractsValue: true,
		}

		code, err := kda.ValidateCreateAsset(tc, forkController, pubkeyConv)
		require.Error(t, err)
		require.Equal(t, process.ErrInvalidOwnerAddr, err)
		require.Equal(t, transaction.Transaction_AccountError, code)
	})

	t.Run("InvalidAdminAddress_WithSmartContracts", func(t *testing.T) {
		tc := createValidContract()
		tc.AdminAddress = []byte("short") // Too short
		forkController := &mock.ForkControllerStub{
			EnableSmartContractsValue: true,
		}

		code, err := kda.ValidateCreateAsset(tc, forkController, pubkeyConv)
		require.Error(t, err)
		require.Equal(t, process.ErrInvalidAdminAddr, err)
		require.Equal(t, transaction.Transaction_AccountError, code)
	})

	t.Run("ValidAdminAddress_Empty", func(t *testing.T) {
		tc := createValidContract()
		tc.AdminAddress = []byte{} // Empty is valid
		forkController := &mock.ForkControllerStub{
			EnableSmartContractsValue: true,
		}

		code, err := kda.ValidateCreateAsset(tc, forkController, pubkeyConv)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})

	t.Run("InvalidAdminAddress_WithoutSmartContracts", func(t *testing.T) {
		tc := createValidContract()
		tc.AdminAddress = []byte("short") // Too short but SC disabled
		forkController := &mock.ForkControllerStub{
			EnableSmartContractsValue: false,
		}

		// Should not check admin address when SC is disabled
		code, err := kda.ValidateCreateAsset(tc, forkController, pubkeyConv)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})

	t.Run("NegativeMaxSupply", func(t *testing.T) {
		tc := createValidContract()
		tc.MaxSupply = -100
		forkController := &mock.ForkControllerStub{
			EnableSmartContractsValue: true,
		}

		code, err := kda.ValidateCreateAsset(tc, forkController, pubkeyConv)
		require.Error(t, err)
		require.Equal(t, process.ErrSupplyNotValid, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
	})

	t.Run("ZeroMaxSupply", func(t *testing.T) {
		tc := createValidContract()
		tc.MaxSupply = 0 // Zero is valid
		forkController := &mock.ForkControllerStub{
			EnableSmartContractsValue: true,
		}

		code, err := kda.ValidateCreateAsset(tc, forkController, pubkeyConv)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})
}

func TestValidateLogoAndUri(t *testing.T) {
	t.Parallel()

	t.Run("ValidLogoAndUri", func(t *testing.T) {
		tc := &transaction.CreateAssetContract{
			Logo: "https://example.com/logo.png",
			URIs: map[string]string{
				"website": "https://example.com",
				"docs":    "https://docs.example.com",
			},
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})

	t.Run("EmptyLogoAndUri", func(t *testing.T) {
		tc := &transaction.CreateAssetContract{
			Logo: "",
			URIs: map[string]string{},
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})

	t.Run("InvalidLogo_NotUTF8", func(t *testing.T) {
		tc := &transaction.CreateAssetContract{
			Logo: string([]byte{0xFF, 0xFE, 0xFD}), // Invalid UTF-8
			URIs: map[string]string{},
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.Error(t, err)
		require.Equal(t, common.ErrInvalidValue, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
	})

	t.Run("InvalidLogo_TooLarge", func(t *testing.T) {
		tc := &transaction.CreateAssetContract{
			Logo: strings.Repeat("a", core.MaxLogoURISize+1), // Exceeds max size
			URIs: map[string]string{},
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.Error(t, err)
		require.Equal(t, common.ErrInvalidValue, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
	})

	t.Run("ValidLogo_MaxSize", func(t *testing.T) {
		tc := &transaction.CreateAssetContract{
			Logo: strings.Repeat("a", core.MaxLogoURISize), // Exactly max size
			URIs: map[string]string{},
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})

	t.Run("TooManyURIs", func(t *testing.T) {
		uris := make(map[string]string)
		for i := 0; i <= core.MaxURIMapSize; i++ {
			uris[string(rune('a'+i))] = "value"
		}

		tc := &transaction.CreateAssetContract{
			Logo: "https://example.com/logo.png",
			URIs: uris,
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.Error(t, err)
		require.Equal(t, common.ErrInvalidValue, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
	})

	t.Run("MaxURIs_Valid", func(t *testing.T) {
		uris := make(map[string]string)
		for i := 0; i < core.MaxURIMapSize; i++ {
			uris[string(rune('a'+i))] = "value"
		}

		tc := &transaction.CreateAssetContract{
			Logo: "https://example.com/logo.png",
			URIs: uris,
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})

	t.Run("InvalidURIKey_NotUTF8", func(t *testing.T) {
		tc := &transaction.CreateAssetContract{
			Logo: "https://example.com/logo.png",
			URIs: map[string]string{
				string([]byte{0xFF, 0xFE, 0xFD}): "value", // Invalid UTF-8 key
			},
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.Error(t, err)
		require.Equal(t, common.ErrInvalidValue, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
	})

	t.Run("InvalidURIValue_NotUTF8", func(t *testing.T) {
		tc := &transaction.CreateAssetContract{
			Logo: "https://example.com/logo.png",
			URIs: map[string]string{
				"key": string([]byte{0xFF, 0xFE, 0xFD}), // Invalid UTF-8 value
			},
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.Error(t, err)
		require.Equal(t, common.ErrInvalidValue, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
	})

	t.Run("InvalidURIKey_TooLong", func(t *testing.T) {
		tc := &transaction.CreateAssetContract{
			Logo: "https://example.com/logo.png",
			URIs: map[string]string{
				strings.Repeat("k", core.MaxURIKeySize+1): "value", // Key too long
			},
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.Error(t, err)
		require.Equal(t, common.ErrInvalidValue, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
	})

	t.Run("ValidURIKey_MaxSize", func(t *testing.T) {
		tc := &transaction.CreateAssetContract{
			Logo: "https://example.com/logo.png",
			URIs: map[string]string{
				strings.Repeat("k", core.MaxURIKeySize): "value", // Exactly max size
			},
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})

	t.Run("InvalidURIValue_TooLong", func(t *testing.T) {
		tc := &transaction.CreateAssetContract{
			Logo: "https://example.com/logo.png",
			URIs: map[string]string{
				"key": strings.Repeat("v", core.MaxURIValueSize+1), // Value too long
			},
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.Error(t, err)
		require.Equal(t, common.ErrInvalidValue, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
	})

	t.Run("ValidURIValue_MaxSize", func(t *testing.T) {
		tc := &transaction.CreateAssetContract{
			Logo: "https://example.com/logo.png",
			URIs: map[string]string{
				"key": strings.Repeat("v", core.MaxURIValueSize), // Exactly max size
			},
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})

	t.Run("MultipleValidURIs", func(t *testing.T) {
		tc := &transaction.CreateAssetContract{
			Logo: "https://example.com/logo.png",
			URIs: map[string]string{
				"website":  "https://example.com",
				"docs":     "https://docs.example.com",
				"github":   "https://github.com/example",
				"twitter":  "https://twitter.com/example",
				"telegram": "https://t.me/example",
			},
		}

		code, err := kda.ValidateLogoAndUri(tc)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
	})
}

func TestCreateNewAssetIdentifier(t *testing.T) {
	t.Parallel()

	hasher := &sha256.Sha256{}

	t.Run("BasicCreation", func(t *testing.T) {
		randSeed := []byte("random-seed")
		caller := make([]byte, 32)
		copy(caller, []byte("caller"))
		nonce := uint64(100)
		ticker := []byte("TEST")
		contractID := 0

		identifier := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, nonce, ticker, contractID)

		// Verify identifier format: TICKER-XXXX
		require.NotNil(t, identifier)
		identifierStr := string(identifier)
		require.True(t, strings.HasPrefix(identifierStr, "TEST-"))
		require.Len(t, identifier, 9) // TEST (4) + - (1) + XXXX (4)
	})

	t.Run("WithContractID", func(t *testing.T) {
		randSeed := []byte("random-seed")
		caller := make([]byte, 32)
		copy(caller, []byte("caller"))
		nonce := uint64(100)
		ticker := []byte("TEST")
		contractID := 5

		identifier := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, nonce, ticker, contractID)

		require.NotNil(t, identifier)
		identifierStr := string(identifier)
		require.True(t, strings.HasPrefix(identifierStr, "TEST-"))
		require.Len(t, identifier, 9)
	})

	t.Run("DifferentNonces_ProduceDifferentIdentifiers", func(t *testing.T) {
		randSeed := []byte("random-seed")
		caller := make([]byte, 32)
		copy(caller, []byte("caller"))
		ticker := []byte("TEST")
		contractID := 0

		id1 := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, 1, ticker, contractID)
		id2 := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, 2, ticker, contractID)

		require.NotEqual(t, id1, id2)
	})

	t.Run("DifferentCallers_ProduceDifferentIdentifiers", func(t *testing.T) {
		randSeed := []byte("random-seed")
		nonce := uint64(100)
		ticker := []byte("TEST")
		contractID := 0

		caller1 := make([]byte, 32)
		copy(caller1, []byte("caller1"))
		caller2 := make([]byte, 32)
		copy(caller2, []byte("caller2"))

		id1 := kda.CreateNewAssetIdentifier(hasher, randSeed, caller1, nonce, ticker, contractID)
		id2 := kda.CreateNewAssetIdentifier(hasher, randSeed, caller2, nonce, ticker, contractID)

		require.NotEqual(t, id1, id2)
	})

	t.Run("DifferentTickers_ProduceDifferentPrefixes", func(t *testing.T) {
		randSeed := []byte("random-seed")
		caller := make([]byte, 32)
		copy(caller, []byte("caller"))
		nonce := uint64(100)
		contractID := 0

		id1 := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, nonce, []byte("COIN"), contractID)
		id2 := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, nonce, []byte("TOKEN"), contractID)

		require.True(t, strings.HasPrefix(string(id1), "COIN-"))
		require.True(t, strings.HasPrefix(string(id2), "TOKEN-"))
	})

	t.Run("DifferentRandSeeds_ProduceDifferentIdentifiers", func(t *testing.T) {
		caller := make([]byte, 32)
		copy(caller, []byte("caller"))
		nonce := uint64(100)
		ticker := []byte("TEST")
		contractID := 0

		id1 := kda.CreateNewAssetIdentifier(hasher, []byte("seed1"), caller, nonce, ticker, contractID)
		id2 := kda.CreateNewAssetIdentifier(hasher, []byte("seed2"), caller, nonce, ticker, contractID)

		require.NotEqual(t, id1, id2)
	})

	t.Run("DifferentContractIDs_ProduceDifferentIdentifiers", func(t *testing.T) {
		randSeed := []byte("random-seed")
		caller := make([]byte, 32)
		copy(caller, []byte("caller"))
		nonce := uint64(100)
		ticker := []byte("TEST")

		id1 := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, nonce, ticker, 1)
		id2 := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, nonce, ticker, 2)

		require.NotEqual(t, id1, id2)
	})

	t.Run("ZeroContractID_VsPositive", func(t *testing.T) {
		randSeed := []byte("random-seed")
		caller := make([]byte, 32)
		copy(caller, []byte("caller"))
		nonce := uint64(100)
		ticker := []byte("TEST")

		id1 := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, nonce, ticker, 0)
		id2 := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, nonce, ticker, 1)

		require.NotEqual(t, id1, id2)
	})

	t.Run("ShortTicker", func(t *testing.T) {
		randSeed := []byte("random-seed")
		caller := make([]byte, 32)
		copy(caller, []byte("caller"))
		nonce := uint64(100)
		ticker := []byte("AB")
		contractID := 0

		identifier := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, nonce, ticker, contractID)

		require.NotNil(t, identifier)
		identifierStr := string(identifier)
		require.True(t, strings.HasPrefix(identifierStr, "AB-"))
		require.Len(t, identifier, 7) // AB (2) + - (1) + XXXX (4)
	})

	t.Run("LongTicker", func(t *testing.T) {
		randSeed := []byte("random-seed")
		caller := make([]byte, 32)
		copy(caller, []byte("caller"))
		nonce := uint64(100)
		ticker := []byte("VERYLONGTICKERNAM")
		contractID := 0

		identifier := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, nonce, ticker, contractID)

		require.NotNil(t, identifier)
		identifierStr := string(identifier)
		require.True(t, strings.HasPrefix(identifierStr, "VERYLONGTICKERNAM-"))
		require.Len(t, identifier, 22) // VERYLONGTICKERNAM (17) + - (1) + XXXX (4)
	})

	t.Run("SameInputs_ProduceSameIdentifier", func(t *testing.T) {
		randSeed := []byte("random-seed")
		caller := make([]byte, 32)
		copy(caller, []byte("caller"))
		nonce := uint64(100)
		ticker := []byte("TEST")
		contractID := 5

		id1 := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, nonce, ticker, contractID)
		id2 := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, nonce, ticker, contractID)

		require.Equal(t, id1, id2)
	})

	t.Run("IdentifierContainsBase36Characters", func(t *testing.T) {
		randSeed := []byte("random-seed")
		caller := make([]byte, 32)
		copy(caller, []byte("caller"))
		nonce := uint64(100)
		ticker := []byte("TEST")
		contractID := 0

		identifier := kda.CreateNewAssetIdentifier(hasher, randSeed, caller, nonce, ticker, contractID)

		// Extract the random part (after the dash)
		parts := strings.Split(string(identifier), "-")
		require.Len(t, parts, 2)
		randomPart := parts[1]

		// Verify it's 4 characters (TickerRandomSequenceLength)
		require.Len(t, randomPart, 4)

		// Verify all characters are base36 (0-9, a-z, A-Z)
		for _, char := range randomPart {
			require.True(t,
				(char >= '0' && char <= '9') || (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z'),
				"character %c is not a valid base36 character", char)
		}
	})
}
