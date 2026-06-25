package node

import (
	"bytes"
	"context"

	"github.com/klever-io/klever-go/common"
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
// UserAccountData, so accounts are identified by Address == leaf key, not by decode success. Mid-walk trie
// errors are swallowed upstream (KLC-2509). Use GetAccountTotals (cached).
func (n *Node) computeAccountTotals() (*models.AccountTotalsResponse, error) {
	if check.IfNil(n.accounts) {
		return nil, common.ErrNilAccountsAdapter
	}

	rootHash, err := n.accounts.RootHash()
	if err != nil {
		return nil, err
	}

	leavesChannel, err := n.accounts.GetAllLeaves(rootHash, context.Background())
	if err != nil {
		return nil, err
	}

	totals := &models.AccountTotalsResponse{}
	for leaf := range leavesChannel {
		acc := &state.UserAccountData{}
		if errUnmarshal := n.internalMarshalizer.Unmarshal(acc, leaf.Value()); errUnmarshal != nil {
			continue
		}
		if !bytes.Equal(acc.GetAddress(), leaf.Key()) {
			continue // code leaf, not an account
		}
		totals.AccountCount++
		totals.BalanceTotal += acc.GetBalance()
		totals.AllowanceTotal += acc.GetAllowance()
	}

	return totals, nil
}
