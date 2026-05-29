package versioning

import (
	"testing"

	"github.com/klever-io/klever-go/core/process"
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

// Regression test for GHSA-rm5c-5x2p-48wr: nil RawData must error, not panic.
func TestTxVersionChecker_CheckTxVersionNilRawDataShouldErrAndNotPanic(t *testing.T) {
	tvc := NewTxVersionChecker(1)

	require.NotPanics(t, func() {
		err := tvc.CheckTxVersion(&transaction.Transaction{})
		require.Equal(t, process.ErrInvalidTransactionVersion, err)
	})
}

func TestTxVersionChecker_CheckTxVersionNilTxShouldErrAndNotPanic(t *testing.T) {
	tvc := NewTxVersionChecker(1)

	require.NotPanics(t, func() {
		err := tvc.CheckTxVersion(nil)
		require.Equal(t, process.ErrInvalidTransactionVersion, err)
	})
}

func TestTxVersionChecker_CheckTxVersionBelowMinShouldErr(t *testing.T) {
	minTxVersion := uint32(2)
	tx := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{
			Version: minTxVersion - 1,
		},
	}
	tvc := NewTxVersionChecker(minTxVersion)
	err := tvc.CheckTxVersion(tx)
	require.Equal(t, process.ErrInvalidTransactionVersion, err)
}
