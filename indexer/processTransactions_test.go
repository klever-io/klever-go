package indexer

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	nodeData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestPrepareTransactionsForDatabase(t *testing.T) {
	t.Parallel()

	contract := transaction.TransferContract{
		ToAddress: []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
		Amount:    45,
	}

	txHash1 := []byte("txHash1")
	tx1, _ := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"))
	txHash2 := []byte("txHash2")
	tx2, _ := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"))
	txHash3 := []byte("txHash3")
	tx3, _ := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"))

	sc_contract := transaction.SmartContract{
		Type:    transaction.SmartContract_SCInvoke,
		Address: []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
		CallValue: map[string]*transaction.CallValue{
			"KLV": {Amount: 100},
			"KFI": {Amount: 200},
		},
	}
	txHash4 := []byte("txHash4")
	tx4, _ := createTransactionHandlerMock(&sc_contract, transaction.TXContract_SmartContractType, []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"))

	header := &block.Block{
		Header:   &block.BlockHeader{},
		TxHashes: [][]byte{txHash1, txHash2, txHash3, txHash4},
	}
	txPool := &indexer.Pool{
		Txs: map[string]nodeData.TransactionHandler{
			string(txHash1): tx1,
			string(txHash2): tx2,
			string(txHash3): tx3,
			string(txHash4): tx4,
		},
	}

	txDbProc := newTxDatabaseProcessor(
		&mock.HasherMock{},
		&mock.MarshalizerMock{},
		&mock.PubkeyConverterMock{},
		&mock.PubkeyConverterMock{},
		false,
	)

	transactions, _, _, _ := txDbProc.prepareTransactionsForDatabase(header, txPool)
	assert.Equal(t, 4, len(transactions))
}

func createTransactionHandlerMock(contract proto.Message, txType transaction.TXContract_ContractType, sender []byte) (nodeData.TransactionHandler, error) {

	serialized, err := proto.Marshal(contract)
	if err != nil {
		return nil, errors.New("could not serialize constract")
	}

	var txSignature [][]byte
	txSignature = append(txSignature, []byte("signature"))

	tx := &transaction.Transaction{
		Signature: txSignature,
		RawData: &transaction.Transaction_Raw{
			Sender: sender,
			Contract: []*transaction.TXContract{
				{
					Type: txType,
					Parameter: &anypb.Any{
						TypeUrl: "github.com/klever-io/klever-go/" + string(proto.MessageName(contract)),
						Value:   serialized,
					},
				},
			},
		},
		Result: transaction.Transaction_SUCCESS,
	}

	return tx, nil
}
