package versioning

import (
	"testing"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/require"
)

func TestTxVersionChecker_CheckTxVersionShouldWork(t *testing.T) {
	minTxVersion := uint32(1)
	tx := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{
			Version: minTxVersion + 1,
		},
	}
	tvc := NewTxVersionChecker(minTxVersion)
	err := tvc.CheckTxVersion(tx)
	require.Nil(t, err)
}
