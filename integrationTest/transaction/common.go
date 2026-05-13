package transaction

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

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

func LoadDefaultConfigs(t *testing.T, configPath string) config.Config {
	config, err := integrationTest.LoadConfig(configPath)
	require.Nil(t, err)
	// replace config.yaml -> enableEpochs.yaml
	enableEpochsPath := strings.Replace(configPath, "config.yaml", "enableEpochs.yaml", 1)
	enableEpochs, err := integrationTest.LoadEnableEpochsConfig(enableEpochsPath)
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
	tx, hash, err := CreateTransactionOnly(sender, wallet, txType, contract)
	if err != nil {
		return nil, nil, err
	}

	_, err = sender.SendTransaction(tx)
	if err != nil {
		return nil, nil, err
	}

	return tx, hash, nil
}

func CreateTransactionOnly(
	sender *processorNode.ProcessorNode,
	wallet *processorNode.NodeAccount,
	txType transaction.TXContract_ContractType,
	contract any,
) (*transaction.Transaction, []byte, error) {
	tx, hash, err := createTransaction(sender, wallet, txType, contract)
	if err != nil {
		return nil, nil, err
	}

	tx.Signature[0], err = wallet.TxSingleSigner.Sign(wallet.SkTxSign, hash)
	if err != nil {
		return nil, nil, err
	}

	wallet.Nonce++

	return tx, hash, nil
}

func SendTx(t *testing.T, nodes []*processorNode.ProcessorNode, wallets []*processorNode.NodeAccount, sendAmount int64) (*transaction.Transaction, []byte) {
	tx, txHash, err := CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_TransferContractType,
		struct {
			Receiver string
			Amount   int64
			Asset    string
		}{
			Receiver: processorNode.TestAddressPubkeyConverter.Encode(wallets[1].Address),
			Amount:   sendAmount,
			Asset:    "KLV",
		},
	)
	require.Nil(t, err)

	// wait for tx to be propagated
	time.Sleep(500 * time.Millisecond)

	return tx, txHash
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
		log.Error("transaction not success", "status", tx.Result, "code", tx.ResultCode, "txHash", txHash)
		return nil, ErrStatusNotSuccess
	}

	// // check if TX is in the block
	if tx.Status != api.TRANSACTION_STATUS_ON_CHAIN {
		log.Error("transaction not in block", "status", tx.Status, "txHash", txHash)
		return nil, ErrNotInBlock
	}

	return tx.Transaction, nil
}

func WrapError(err error, newErr error) error {
	if err == nil {
		return newErr
	}
	return fmt.Errorf("%w, %v", err, newErr)
}

func CheckTXInBlock(nodes []*processorNode.ProcessorNode, txHash []byte, blockNonce uint64) error {
	var finalErr error
	for i, n := range nodes {
		txResult, err := GetAndCheckTransaction(n, txHash)
		if err != nil {
			finalErr = WrapError(finalErr, err)
			log.Warn("TX not found", "nodeIdx", i, "slot", n.SlotManager.SlotIndex.Load(), "nonce", n.Blkc.GetCurrentBlockHeader().GetNonce())
			continue

		}

		// check if TX is in the block or pending
		if txResult.Block != blockNonce {
			finalErr = WrapError(finalErr, ErrNotInBlock)
			log.Warn("TX block does not match", "nodeIdx", i, "slot", n.SlotManager.SlotIndex.Load(), "nodeHight", n.Blkc.GetCurrentBlockHeader().GetNonce(), "foundBlock", txResult.Block, "expectedBlock", blockNonce)
			continue
		}

		// check current block
		b, err := n.GetBlockByNonce(txResult.Block)
		if err != nil {
			finalErr = WrapError(finalErr, err)
			log.Warn("Block not found", "nodeIdx", i, "slot", n.SlotManager.SlotIndex.Load(), "nonce", txResult.Block, "nodeBlock", n.Blkc.GetCurrentBlockHeader().GetBlockHeader())
			continue
		}

		log.Info("TX found in node", "nodeIdx", i, "slot", b.GetSlot(), "nonce", txResult.Block)
		log.Info("Block info", "nodeIdx", i, "slot", b.GetSlot(), "hash", b.Hash, "txLen", len(b.TxHashes))
	}

	if finalErr != nil {
		log.Error("Some errors occurred", "err", finalErr)
	}

	return finalErr
}

func CheckTXIsPending(nodes []*processorNode.ProcessorNode, txHash []byte) error {
	var finalErr error
	for i, n := range nodes {
		_, err := GetAndCheckTransaction(n, txHash)
		if err != ErrNotInBlock {
			finalErr = WrapError(finalErr, err)
			log.Warn("TX not pending", "nodeIdx", i, "slot", n.SlotManager.SlotIndex.Load(), "nonce", n.Blkc.GetCurrentBlockHeader().GetNonce(), "error", err)
			continue
		}
	}

	return finalErr
}
