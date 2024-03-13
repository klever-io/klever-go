package txcache

import (
	"encoding/binary"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools"
)

const delta = 0.00000001
const estimatedSizeOfBoundedTxFields = uint64(128)

func (cache *TxCache) areInternalMapsConsistent() bool {
	journal := cache.checkInternalConsistency()
	return journal.isFine()
}

func (cache *TxCache) getHashesForSender(sender string) []string {
	return cache.getListForSender(sender).getTxHashesAsStrings()
}

func (cache *TxCache) getListForSender(sender string) *txListForSender {
	return cache.txListBySender.testGetListForSender(sender)
}

func (txMap *txListBySenderMap) testGetListForSender(sender string) *txListForSender {
	list, ok := txMap.getListForSender(sender)
	if !ok {
		panic("sender not in cache")
	}

	return list
}

func (cache *TxCache) getScoreOfSender(sender string) uint32 {
	list := cache.getListForSender(sender)
	scoreParams := list.getScoreParams()
	computer := cache.txListBySender.scoreComputer
	return computer.computeScore(scoreParams)
}

func (listForSender *txListForSender) getTxHashesAsStrings() []string {
	hashes := listForSender.getTxHashes()
	return hashesAsStrings(hashes)
}

func hashesAsStrings(hashes [][]byte) []string {
	result := make([]string, len(hashes))
	for i := 0; i < len(hashes); i++ {
		result[i] = string(hashes[i])
	}

	return result
}

func addManyTransactionsWithUniformDistribution(cache *TxCache, nSenders int, nTransactionsPerSender int) {
	for senderTag := 0; senderTag < nSenders; senderTag++ {
		sender := createFakeSenderAddress(senderTag)

		for txNonce := nTransactionsPerSender; txNonce > 0; txNonce-- {
			txHash := createFakeTxHash(sender, txNonce)
			tx := createTx(txHash, string(sender), uint64(txNonce), int64(txNonce))
			cache.AddTx(tx)
		}
	}
}

func createTx(hash []byte, sender string, nonce uint64, fees int64) *WrappedTransaction {
	tx := transaction.NewBaseTransaction([]byte(sender), nonce, [][]byte{[]byte(fmt.Sprintf("%d", fees))}, fees, 1_000_000)

	return &WrappedTransaction{
		Tx:     tx,
		TxHash: hash,
		Size:   int64(estimatedSizeOfBoundedTxFields),
	}
}

func sleepFor(ms time.Duration) {
	now := time.Now()
	for !time.Now().After(now.Add(ms * time.Millisecond)) {
	}
}

func createTxWithDelay(hash []byte, sender string, nonce uint64, fees int64) *WrappedTransaction {
	sleepFor(1000)
	tx := transaction.NewBaseTransaction([]byte(sender), nonce, [][]byte{[]byte(fmt.Sprintf("%d", fees))}, fees, 1_000_000)

	return &WrappedTransaction{
		Tx:     tx,
		TxHash: hash,
		Size:   int64(estimatedSizeOfBoundedTxFields),
	}
}

func createTxWithParamsAndDelay(hash []byte, sender string, nonce uint64, size uint64, fees int64) *WrappedTransaction {
	sleepFor(1000)
	dataLength := int(size) - int(estimatedSizeOfBoundedTxFields)
	if dataLength < 0 {
		panic("createTxWithData(): invalid length for dummy tx")
	}
	tx := transaction.NewBaseTransaction([]byte(sender), nonce, make([][]byte, dataLength), fees, 1_000_000)

	return &WrappedTransaction{
		Tx:     tx,
		TxHash: hash,
		Size:   int64(size),
	}
}

func createTxWithParams(hash []byte, sender string, nonce uint64, size uint64, fees int64) *WrappedTransaction {
	dataLength := int(size) - int(estimatedSizeOfBoundedTxFields)
	if dataLength < 0 {
		panic("createTxWithData(): invalid length for dummy tx")
	}
	tx := transaction.NewBaseTransaction([]byte(sender), nonce, make([][]byte, dataLength), fees, 1_000_000)

	return &WrappedTransaction{
		Tx:     tx,
		TxHash: hash,
		Size:   int64(size),
	}
}

func createFakeSenderAddress(senderTag int) []byte {
	bytes := make([]byte, 32)
	binary.LittleEndian.PutUint64(bytes, uint64(senderTag))
	binary.LittleEndian.PutUint64(bytes[24:], uint64(senderTag))
	return bytes
}

func createFakeTxHash(fakeSenderAddress []byte, nonce int) []byte {
	bytes := make([]byte, 32)
	copy(bytes, fakeSenderAddress)
	binary.LittleEndian.PutUint64(bytes[8:], uint64(nonce))
	binary.LittleEndian.PutUint64(bytes[16:], uint64(nonce))
	return bytes
}

func measureWithStopWatch(b *testing.B, function func()) {
	sw := tools.NewStopWatch()
	sw.Start("time")
	function()
	sw.Stop("time")

	duration := sw.GetMeasurementsMap()["time"]
	b.ReportMetric(duration, "time@stopWatch")
}

// waitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
// Reference: https://stackoverflow.com/a/32843750/1475331
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

var _ scoreComputer = (*disabledScoreComputer)(nil)

type disabledScoreComputer struct {
}

func (computer *disabledScoreComputer) computeScore(_ senderScoreParams) uint32 {
	return 0
}
