package transaction

import (
	"encoding/json"
	"testing"

	"errors"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	"github.com/stretchr/testify/require"
)

const ConfigPath = "../../../integrationTest/config/config.yaml"
const EnableEpochsPath = "../../../integrationTest/config/enableEpochs.yaml"

var ErrStatusNotSuccess = errors.New("tx status is not success")
var ErrNotInBlock = errors.New("tx is not in block")

var log = logger.GetOrCreate("transactions/common")

func LoadDefaultConfigs(t *testing.T) config.Config {
	config, err := integrationTest.LoadConfig(ConfigPath)
	require.Nil(t, err)
	enableEpochs, err := integrationTest.LoadEnableEpochsConfig(EnableEpochsPath)
	require.Nil(t, err)
	config.EnableEpochs = enableEpochs.EnableEpochs
	config.GasScheduleConfig = enableEpochs.GasSchedule

	return config
}

func CreateStandardSetupForTxTests(numWallets int) ([]*processorNode.ProcessorNode, []*processorNode.NodeAccount, error) {

	mainConfig, err := integrationTest.LoadConfig(ConfigPath)
	if err != nil {
		return nil, nil, err
	}
	enableEpochs, err := integrationTest.LoadEnableEpochsConfig(EnableEpochsPath)
	if err != nil {
		return nil, nil, err
	}

	mainConfig.EnableEpochs = enableEpochs.EnableEpochs
	mainConfig.GasScheduleConfig = enableEpochs.GasSchedule

	initialBalance := int64(1_000_000_000_000)
	numOfNodes := 2
	numConsensusSize := 2

	nodes, err := processorNode.CreateNodesWithNodesCoordinatorAndHeaderSigVerifier(numOfNodes, numConsensusSize, mainConfig)

	wallets := make([]*processorNode.NodeAccount, numWallets)
	for i := 0; i < numWallets; i++ {
		wallets[i] = processorNode.CreateNodeAccount()
	}
	integrationTest.MintAllWallets(nodes, wallets, initialBalance)

	return nodes, wallets, err
}

func CreateSetupForTxTests(initialBalance int64, numOfNodes int, numConsensusSize int, numWallets int, mainConfig config.Config) ([]*processorNode.ProcessorNode, []*processorNode.NodeAccount, error) {

	nodes, err := processorNode.CreateNodesWithNodesCoordinatorAndHeaderSigVerifier(numOfNodes, numConsensusSize, mainConfig)

	wallets := make([]*processorNode.NodeAccount, numWallets)
	for i := 0; i < numWallets; i++ {
		wallets[i] = processorNode.CreateNodeAccount()
	}
	integrationTest.MintAllWallets(nodes, wallets, initialBalance)

	return nodes, wallets, err
}

func CreateAndMintAccount(initialBalance int64, nodes []*processorNode.ProcessorNode) *processorNode.NodeAccount {
	wallets := []*processorNode.NodeAccount{
		processorNode.CreateNodeAccount(),
	}

	integrationTest.MintAllWallets(nodes, wallets, initialBalance)

	return wallets[0]
}

func createTransaction(
	sender *processorNode.ProcessorNode,
	wallet *processorNode.NodeAccount,
	txType transaction.TXContract_ContractType,
	contract any,
) (*transaction.Transaction, []byte, error) {
	parsedContract, err := json.Marshal(contract)
	if err != nil {
		return nil, nil, err
	}

	return sender.CreateTransaction(
		uint32(txType.Number()), // #nosec G115
		processorNode.TestAddressPubkeyConverter.Encode(wallet.Address),
		wallet.Nonce,
		[]byte("chainID"),
		[][]byte{[]byte("data")},
		0,
		[]json.RawMessage{parsedContract},
	)
}

func CreateAndSendTransaction(
	sender *processorNode.ProcessorNode,
	wallet *processorNode.NodeAccount,
	txType transaction.TXContract_ContractType,
	contract any,
) (*transaction.Transaction, []byte, error) {
	tx, hash, err := createTransaction(sender, wallet, txType, contract)
	if err != nil {
		return nil, nil, err
	}

	tx.Signature[0], err = wallet.SingleSigner.Sign(wallet.SkTxSign, hash)
	if err != nil {
		return nil, nil, err
	}

	wallet.Nonce++

	_, err = sender.SendTransaction(tx)
	if err != nil {
		return nil, nil, err
	}

	return tx, hash, nil
}

func GetTransaction(
	node *processorNode.ProcessorNode,
	txHash []byte,
) *transaction.Transaction {
	tx, err := node.GetTransaction(txHash, true)
	if err != nil {
		return nil
	}

	return tx.Transaction
}

func GetAndCheckTransaction(
	node *processorNode.ProcessorNode,
	txHash []byte,
) (*transaction.Transaction, error) {
	tx, err := node.GetTransaction(txHash, true)
	if err != nil {
		return nil, err
	}

	if tx.Result != transaction.Transaction_SUCCESS {
		log.Error("transaction not success", "status", tx.Result, "code", tx.ResultCode)
		return nil, ErrStatusNotSuccess
	}

	// // check if TX is in the block
	if tx.Status != api.TRANSACTION_STATUS_ON_CHAIN {
		log.Error("transaction not in block", "status", tx.Status)
		return nil, ErrNotInBlock
	}

	return tx.Transaction, nil
}
