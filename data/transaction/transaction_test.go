package transaction_test

import (
	"bytes"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/assert"
)

func TestTransaction_GetDataSize(t *testing.T) {
	t.Parallel()

	tx := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{
			Data: make([][]byte, 0),
		},
	}

	bump := func() []byte {
		return make([]byte, 1000000000)
	}

	for i := 0; i < 100; i++ {
		tx.RawData.Data = append(tx.RawData.Data, bump())
	}

	size := tx.GetDataSize()
	assert.NotZero(t, size)
}

func TestTransaction_ValidatePermission(t *testing.T) {
	t.Parallel()

	validPermissions := []transaction.TXContract_ContractType{
		transaction.TXContract_TransferContractType,
		transaction.TXContract_UndelegateContractType,
		transaction.TXContract_SetAccountNameContractType,
	}

	tx := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{
			Contract: make([]*transaction.TXContract, 0),
		},
	}

	for _, ct := range validPermissions {
		tx.RawData.Contract = append(tx.RawData.Contract, &transaction.TXContract{Type: ct})
	}

	err := tx.ValidatePermission(bytes.Repeat([]byte{0xff}, 33))
	assert.Equal(t, err, common.ErrInvalidPermission)

	err = tx.ValidatePermission(nil)
	assert.Equal(t, err, common.ErrNoPermission)

	err = tx.ValidatePermission(bytes.Repeat([]byte{0xff}, 0))
	assert.Equal(t, err, common.ErrNoPermission)

	err = tx.ValidatePermission(bytes.Repeat([]byte{0xff}, 1))
	assert.Equal(t, err, common.ErrNoPermission)

	err = tx.ValidatePermission(bytes.Repeat([]byte{0xff}, 32))
	assert.Nil(t, err)
}
