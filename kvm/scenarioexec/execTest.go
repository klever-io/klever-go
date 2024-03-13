package scenarioexec

import (
	"fmt"

	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
)

// ExecuteTest executes an individual test.
func (ae *VMTestExecutor) ExecuteTest(test *scenjsonmodel.Test) error {
	// reset world
	ae.World.Clear()
	ae.World.Blockhashes = test.BlockHashes.ToValues()

	for _, acct := range test.Pre {
		account, err := convertAccount(acct, ae.World)
		if err != nil {
			return err
		}

		err = ae.World.AccountsAdapter.SaveAccount(account)
		if err != nil {
			return err
		}
	}

	for _, block := range test.Blocks {
		for txIndex, tx := range block.Transactions {
			txName := fmt.Sprintf("%d", txIndex)

			// execute
			output, err := ae.executeTx(txName, tx)
			if err != nil {
				return err
			}

			blResult := block.Results[txIndex]

			// check results
			err = ae.checkTxResults(txName, blResult, test.CheckGas, output)
			if err != nil {
				return err
			}
		}
	}

	baseErrMsg := "Legacy test check: "
	err := ae.checkAccounts(baseErrMsg, test.PostState)
	ae.Close()

	return err
}
