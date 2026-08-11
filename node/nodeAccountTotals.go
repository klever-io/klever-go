package node

import (
	"bytes"
	"context"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/tools/check"
)

// GetAccountTotals returns per-account aggregates (count + inline KLV Balance and Allowance), memoized
// per block. The `/network/account-totals` route is opt-in (disabled by default in api.yaml). See KLC-2506.
func (n *Node) GetAccountTotals() (*models.AccountTotalsResponse, error) {
	return n.accountTotals.get(n.blkc, n.computeAccountTotals)
}

// computeAccountTotals walks the user-accounts trie, summing each account's inline Balance and Allowance
// (frozen/unfrozen live in sub-tries, excluded). Code leaves share this trie and can decode cleanly into
// UserAccountData, so accounts are identified by Address == leaf key, not by decode success. A walk
// truncated by a read failure or a cancelled context is reported as an error rather than yielding
// silently undercounted totals. Use GetAccountTotals (cached).
func (n *Node) computeAccountTotals() (*models.AccountTotalsResponse, error) {
	if check.IfNil(n.accounts) {
		return nil, common.ErrNilAccountsAdapter
	}

	rootHash, err := n.accounts.RootHash()
	if err != nil {
		return nil, err
	}

	leavesChannels, err := n.accounts.GetAllLeaves(rootHash, context.Background())
	if err != nil {
		return nil, err
	}

	totals := &models.AccountTotalsResponse{}

	// A truncated walk undercounts every total, which must not be reported as a success.
	err = leavesChannels.ForEach(func(leaf data.KeyValueHolder) error {
		acc := &state.UserAccountData{}
		if errUnmarshal := n.internalMarshalizer.Unmarshal(acc, leaf.Value()); errUnmarshal != nil {
			return nil
		}
		if !bytes.Equal(acc.GetAddress(), leaf.Key()) {
			return nil // code leaf, not an account
		}
		totals.AccountCount++
		totals.BalanceTotal += acc.GetBalance()
		totals.AllowanceTotal += acc.GetAllowance()

		return nil
	})
	if err != nil {
		return nil, err
	}

	return totals, nil
}
