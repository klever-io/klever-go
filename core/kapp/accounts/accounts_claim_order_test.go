package accounts

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/transaction"
	integrationMock "github.com/klever-io/klever-go/integrationTest/mock"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/require"
)

// TestClaimBalance_CreditsGainsInAssetOrder pins the order ClaimBalance credits a claim's gains in: from
// FixAuditChangesV7 it follows the sorted asset IDs on every run, so when two credits fail the returned
// error is the same on every node instead of following Go's random map order (KLC-2770).
func TestClaimBalance_CreditsGainsInAssetOrder(t *testing.T) {
	t.Parallel()

	const numAssets = 8
	gains := make(map[string]int64, numAssets)
	expected := make([]string, 0, numAssets)
	for i := numAssets; i > 0; i-- {
		assetID := fmt.Sprintf("ASSET%d-1234", i)
		gains[assetID] = int64(i * 100)
		expected = append(expected, assetID)
	}
	slices.Sort(expected)

	newKApp := func(v7 bool) *accountsKapp {
		return &accountsKapp{
			forkController: &integrationMock.ForkControllerStub{
				EnableSmartContractsCalled: func() bool { return true },
				FixAuditChangesV7Called:    func() bool { return v7 },
			},
		}
	}
	newAccount := func(credited *[]string, failing map[string]error) *commonMock.UserAccountHandlerStub {
		return &commonMock.UserAccountHandlerStub{
			ClaimCalled: func(_ transaction.ClaimContract_EnumClaimType, _ []byte, _ uint32, _ int64, _ *kapps.StakingData, _ *kapps.KDAData, _ *kapps.UserKDA, _ core.ForkController) (map[string]int64, error) {
				return gains, nil
			},
			AddToBalanceCalled: func(_ int64, assetID []byte, _ bool, _ ...*kapps.UserKDA) error {
				*credited = append(*credited, string(assetID))
				return failing[string(assetID)]
			},
		}
	}
	claim := func(a *accountsKapp, credited *[]string, failing map[string]error) error {
		_, err := a.ClaimBalance(transaction.ClaimContract_AllowanceClaim, []byte(expected[0]), &block.Block{}, newAccount(credited, failing), nil, nil, &kapps.UserKDA{})
		return err
	}

	t.Run("gains are credited in asset order", func(t *testing.T) {
		a := newKApp(true)
		for i := 0; i < 20; i++ {
			credited := make([]string, 0, numAssets)
			require.NoError(t, claim(a, &credited, nil))
			require.Equal(t, expected, credited, "run %d", i)
		}
	})

	t.Run("first failing credit in order is returned", func(t *testing.T) {
		errFirst := errors.New("first failing credit")
		failing := map[string]error{
			expected[1]:           errFirst,
			expected[numAssets-1]: errors.New("last failing credit"),
		}

		a := newKApp(true)
		for i := 0; i < 20; i++ {
			credited := make([]string, 0, numAssets)
			require.Equal(t, errFirst, claim(a, &credited, failing), "run %d", i)
			require.Equal(t, expected[:2], credited, "run %d", i)
		}
	})

	t.Run("gains keep map order before FixAuditChangesV7", func(t *testing.T) {
		a := newKApp(false)
		unsorted := false
		for i := 0; i < 20; i++ {
			credited := make([]string, 0, numAssets)
			require.NoError(t, claim(a, &credited, nil))
			require.ElementsMatch(t, expected, credited)
			unsorted = unsorted || !slices.Equal(expected, credited)
		}
		require.True(t, unsorted, "pre-fork order must stay Go's map order")
	})
}
