package block

import (
	"fmt"
	"sync"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/counting"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools/display"
)

type transactionCounter struct {
	mutex           sync.RWMutex
	currentBlockTxs uint64
	totalTxs        uint64
}

// NewTransactionCounter returns a new object that keeps track of how many transactions
// were executed in total, and in the current block
func NewTransactionCounter() *transactionCounter {
	return &transactionCounter{
		mutex:           sync.RWMutex{},
		currentBlockTxs: 0,
		totalTxs:        0,
	}
}

func (txc *transactionCounter) getPoolCounts(poolsHolder retriever.PoolsHolder) (txCounts counting.Counts) {
	txCounts = poolsHolder.Transactions().GetCounts()
	return
}

// subtractRestoredTxs updated the total processed txs in case of restore
func (txc *transactionCounter) subtractRestoredTxs(txsNr int) {
	txc.mutex.Lock()
	defer txc.mutex.Unlock()
	if txc.totalTxs < uint64(txsNr) {
		txc.totalTxs = 0
		return
	}

	txc.totalTxs -= uint64(txsNr)
}

// displayLogInfo writes to the output information about the block and transactions
func (txc *transactionCounter) displayLogInfo(
	blck *block.Block,
	headerHash []byte,
	dataPool retriever.PoolsHolder,
	appStatusHandler core.AppStatusHandler,
) {
	dispHeader, dispLines := txc.createDisplayableHeaderAndBlockBody(blck)

	txc.mutex.RLock()
	appStatusHandler.SetUInt64Value(core.MetricNumProcessedTxs, txc.totalTxs)
	txc.mutex.RUnlock()

	tblString, err := display.CreateTableString(dispHeader, dispLines)
	if err != nil {
		log.Debug("CreateTableString", "error", err.Error())
		return
	}

	txc.mutex.RLock()
	message := fmt.Sprintf("header hash: %s\n%s", logger.DisplayByteSlice(headerHash), tblString)
	arguments := []interface{}{
		"total txs processed", txc.totalTxs,
		"block txs processed", txc.currentBlockTxs,
	}
	txc.mutex.RUnlock()
	log.Debug(message, arguments...)
}

func (txc *transactionCounter) createDisplayableHeaderAndBlockBody(
	blck *block.Block,
) ([]string, []*display.LineData) {

	tableHeader := []string{"Part", "Parameter", "Value"}

	headerLines := []*display.LineData{
		display.NewLineData(false, []string{
			"Header",
			"Block type",
			"TxBlock"}),
	}

	lines := displayHeader(blck)

	shardLines := make([]*display.LineData, 0, len(lines)+len(headerLines))
	shardLines = append(shardLines, headerLines...)
	shardLines = append(shardLines, lines...)

	shardLines = txc.displayTxBlockBody(shardLines, blck)

	return tableHeader, shardLines

}

func (txc *transactionCounter) displayTxBlockBody(lines []*display.LineData, blck *block.Block) []*display.LineData {
	currentBlockTxs := len(blck.TxHashes)

	part := "Block"

	for j := 0; j < len(blck.TxHashes); j++ {
		if j == 0 || j >= len(blck.TxHashes)-1 {
			lines = append(lines, display.NewLineData(false, []string{
				part,
				fmt.Sprintf("TxHash_%d", j+1),
				logger.DisplayByteSlice(blck.TxHashes[j])}))

			part = ""
		} else if j == 1 {
			lines = append(lines, display.NewLineData(false, []string{
				part,
				"...",
				"...",
			}))

			part = ""
		}
	}

	lines[len(lines)-1].HorizontalRuleAfter = true

	txc.mutex.Lock()
	txc.currentBlockTxs = uint64(currentBlockTxs)
	txc.totalTxs += uint64(currentBlockTxs)
	txc.mutex.Unlock()

	return lines
}
