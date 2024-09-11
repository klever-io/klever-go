package memorytests

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"runtime/pprof"
	"testing"

	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/txpool"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools"
	"github.com/stretchr/testify/require"
)

// useMemPprof dictates whether to save heap profiles when running the test.
// Enable this manually, locally.
const useMemPprof = false

// We run all scenarios within a single test so that we minimize memory interferences (of tests running in parallel)
func TestShardedTxPool_MemoryFootprint(t *testing.T) {
	if testing.Short() {
		t.Skip("this is not a short test")
	}

	journals := make([]*memoryFootprintJournal, 0)

	journals = append(journals, runScenario(t, newScenario(200, 1, tools.MegabyteSize, "0"), memoryAssertion{200, 200}, memoryAssertion{0, 1}))
	journals = append(journals, runScenario(t, newScenario(10, 1000, 20480, "0"), memoryAssertion{190, 205}, memoryAssertion{1, 4}))
	journals = append(journals, runScenario(t, newScenario(10000, 1, 1024, "0"), memoryAssertion{10, 16}, memoryAssertion{4, 10}))
	journals = append(journals, runScenario(t, newScenario(1, 60000, 256, "0"), memoryAssertion{38, 40}, memoryAssertion{10, 16}))
	journals = append(journals, runScenario(t, newScenario(10, 10000, 100, "0"), memoryAssertion{50, 54}, memoryAssertion{16, 30}))
	journals = append(journals, runScenario(t, newScenario(100000, 1, 1024, "0"), memoryAssertion{135, 141}, memoryAssertion{51, 79}))

	// With larger memory footprint

	journals = append(journals, runScenario(t, newScenario(100000, 3, 650, "0"), memoryAssertion{330, 343}, memoryAssertion{95, 115}))
	journals = append(journals, runScenario(t, newScenario(150000, 2, 650, "0"), memoryAssertion{330, 343}, memoryAssertion{110, 120}))
	journals = append(journals, runScenario(t, newScenario(300000, 1, 650, "0"), memoryAssertion{330, 343}, memoryAssertion{160, 170}))
	journals = append(journals, runScenario(t, newScenario(30, 10000, 640, "0"), memoryAssertion{310, 325}, memoryAssertion{60, 70}))
	journals = append(journals, runScenario(t, newScenario(300, 1000, 640, "0"), memoryAssertion{310, 325}, memoryAssertion{60, 70}))

	for _, journal := range journals {
		journal.displayFootprintsSummary()
	}
}

func runScenario(t *testing.T, scenario *scenario, payloadFootprint memoryAssertion, structuralFootprint memoryAssertion) *memoryFootprintJournal {
	pool := newPool()
	journal := analyzeMemoryFootprint(t, pool, scenario)
	require.True(t, journal.payloadFootprintBetween(payloadFootprint.lower, payloadFootprint.upper))
	require.True(t, journal.structuralFootprintBetween(structuralFootprint.lower, structuralFootprint.upper))
	keepPoolInMemoryUpToThisPoint(pool)

	return journal
}

type scenario struct {
	name               string
	numSenders         int
	numTxsPerSender    int
	payloadLengthPerTx int
	cacheID            string
}

func newScenario(numSenders int, numTxsPerSender int, payloadLengthPerTx int, cacheID string) *scenario {
	name := fmt.Sprintf("%dx%dx%d_%s", numSenders, numTxsPerSender, payloadLengthPerTx, cacheID)

	return &scenario{
		name:               name,
		numSenders:         numSenders,
		numTxsPerSender:    numTxsPerSender,
		payloadLengthPerTx: payloadLengthPerTx,
		cacheID:            cacheID,
	}
}

func (scenario *scenario) numTxs() int {
	return scenario.numSenders * scenario.numTxsPerSender
}

type memoryAssertion struct {
	lower int
	upper int
}

func newPool() retriever.ShardedDataCacherNotifier {
	config := storageUnit.CacheConfig{
		Capacity:             600000,
		SizePerSender:        60000,
		SizeInBytes:          400 * tools.MegabyteSize,
		SizeInBytesPerSender: 32 * tools.MegabyteSize,
		Shards:               1,
	}

	args := txpool.ArgShardedTxPool{
		Config: config,
	}
	pool, err := txpool.NewShardedTxPool(args)
	if err != nil {
		panic(fmt.Sprintf("newPool: %s", err))
	}

	return pool
}

// keepPoolInMemoryUpToThisPoint makes sure that GC doesn't recall the pool instance.
// Displays the state of the pool in the most intrusive way - getting all keys.
func keepPoolInMemoryUpToThisPoint(pool retriever.ShardedDataCacherNotifier) {
	fmt.Println("[0]:", len(pool.ShardDataStore("0").Keys()))
	fmt.Println("[1 -> 0]:", len(pool.ShardDataStore("1_0").Keys()))
	fmt.Println("[4294967295 -> 0]:", len(pool.ShardDataStore("4294967295_0").Keys()))
}

// analyzeMemoryFootprint gets memory footprints:
// - at the start of the scenario
// - after generating the test transactions
// - after adding the test transactions in the pool
func analyzeMemoryFootprint(t *testing.T, pool retriever.ShardedDataCacherNotifier, scenario *scenario) *memoryFootprintJournal {
	fmt.Println("analyzeMemoryFootprint", scenario.name)

	journal := &memoryFootprintJournal{scenario: scenario}

	journal.beforeGenerate = getMemStats()
	txs := generateTxs(scenario.numSenders, scenario.numTxsPerSender, scenario.payloadLengthPerTx)
	journal.afterGenerate = getMemStats()

	for _, tx := range txs {
		pool.AddData(tx.hash, tx, tx.GetSize(), scenario.cacheID)
	}

	require.Equal(t, scenario.numTxs(), len(pool.ShardDataStore(scenario.cacheID).Keys()))

	if useMemPprof {
		pprofHeap(scenario, "afterAddition")
	}

	journal.afterAddition = getMemStats()
	journal.display()
	return journal
}

// generateTxs generates a number of (numSenders * numTxsPerSender) dummy transactions
func generateTxs(numSenders int, numTxsPerSender int, payloadLengthPerTx int) []*dummyTx {
	txs := make([]*dummyTx, 0, numSenders*numTxsPerSender)
	for senderTag := 0; senderTag < numSenders; senderTag++ {
		for nonce := 0; nonce < numTxsPerSender; nonce++ {
			tx := createTxWithPayload(senderTag, nonce, payloadLengthPerTx)
			txs = append(txs, tx)
		}
	}

	return txs
}

type dummyTx struct {
	transaction.Transaction
	hash []byte
}

func createTxWithPayload(senderTag int, nonce int, payloadLength int) *dummyTx {
	sender := createFakeSenderAddress(senderTag)
	hash := createFakeTxHash(sender, nonce)

	return &dummyTx{
		Transaction: *transaction.NewBaseTransaction(sender, uint64(nonce), [][]byte{make([]byte, payloadLength)}, int64(nonce), int64(nonce)),
		hash:        hash,
	}
}

func createFakeSenderAddress(senderTag int) []byte {
	bytes := make([]byte, 32)
	binary.LittleEndian.PutUint32(bytes, uint32(senderTag))
	binary.LittleEndian.PutUint32(bytes[28:], uint32(senderTag))
	return bytes
}

func createFakeTxHash(fakeSenderAddress []byte, nonce int) []byte {
	bytes := make([]byte, 32)
	copy(bytes, fakeSenderAddress)
	binary.LittleEndian.PutUint32(bytes[4:], uint32(nonce))
	binary.LittleEndian.PutUint32(bytes[24:], uint32(nonce))
	return bytes
}

func getMemStats() runtime.MemStats {
	runtime.GC()

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats
}

func pprofHeap(scenario *scenario, step string) {
	runtime.GC()

	filename := path.Join(".", "pprofoutput", fmt.Sprintf("%s_%s.pprof", scenario.name, step))
	file, err := os.Create(filename)
	if err != nil {
		panic(fmt.Sprintf("pprofHeap: %s", err))
	}

	defer func() {
		errClose := file.Close()
		panic(fmt.Sprintf("cannot close file: %s", errClose.Error()))
	}()

	err = pprof.WriteHeapProfile(file)
	if err != nil {
		panic(fmt.Sprintf("pprofHeap: %s", err))
	}

	convertPprofToHumanReadable(filename)
}

// convertPprofToHumanReadable converts the ",pprof" file to ".png" and ".txt"
func convertPprofToHumanReadable(filename string) {
	// ensure arguments
	filename, err := tools.SanitizePath(filename)
	if err != nil {
		panic(fmt.Sprintf("convertPprofToHumanReadable: %v", err))
	}
	cmd := exec.Command("go", "tool", "pprof", "-png", "-output", fmt.Sprintf("%s.png", filename), filename) // #nosec G204 - sanitized
	err = cmd.Run()
	if err != nil {
		panic(fmt.Sprintf("convertPprofToHumanReadable: %v", err))
	}

	cmd = exec.Command("go", "tool", "pprof", "-text", "-output", fmt.Sprintf("%s.txt", filename), filename) // #nosec G204 - sanitized
	err = cmd.Run()
	if err != nil {
		panic(fmt.Sprintf("convertPprofToHumanReadable: %v", err))
	}
}

type memoryFootprintJournal struct {
	scenario       *scenario
	beforeGenerate runtime.MemStats
	afterGenerate  runtime.MemStats
	afterAddition  runtime.MemStats
}

func (journal *memoryFootprintJournal) txsFootprint() uint64 {
	return uint64(tools.MaxInt(0, int(journal.afterGenerate.HeapInuse)-int(journal.beforeGenerate.HeapInuse)))
}

func (journal *memoryFootprintJournal) payloadFootprintBetween(lower int, upper int) bool {
	fmt.Println("payloadFootprintBetween", lower, upper, bToMb(journal.txsFootprint()))
	return lower <= bToMb(journal.txsFootprint()) && bToMb(journal.txsFootprint()) <= upper
}

func (journal *memoryFootprintJournal) poolStructuresFootprint() uint64 {
	return uint64(tools.MaxInt(0, int(journal.afterAddition.HeapInuse)-int(journal.afterGenerate.HeapInuse)))
}

func (journal *memoryFootprintJournal) structuralFootprintBetween(lower int, upper int) bool {
	fmt.Println("structuralFootprintBetween", lower, upper, bToMb(journal.poolStructuresFootprint()))
	return lower <= bToMb(journal.poolStructuresFootprint()) && bToMb(journal.poolStructuresFootprint()) <= upper
}

func (journal *memoryFootprintJournal) display() {
	// See: https://golang.org/pkg/runtime/#MemStats

	fmt.Printf("beforeGenerate:\tHeapAlloc = %v MiB\tHeapInUse = %v MiB\n",
		bToMb(journal.beforeGenerate.HeapAlloc), bToMb(journal.beforeGenerate.HeapInuse))

	fmt.Printf("afterGenerate:\tHeapAlloc = %v MiB\tHeapInUse = %v MiB\n",
		bToMb(journal.afterGenerate.HeapAlloc), bToMb(journal.afterGenerate.HeapInuse))

	fmt.Printf("afterAddition:\tHeapAlloc = %v MiB\tHeapInUse = %v MiB\n",
		bToMb(journal.afterAddition.HeapAlloc), bToMb(journal.afterAddition.HeapInuse))

	fmt.Println("Txs footprint:", bToMb(journal.txsFootprint()))
	fmt.Println("Pool structures footprint:", bToMb(journal.poolStructuresFootprint()))
}

func (journal *memoryFootprintJournal) displayFootprintsSummary() {
	fmt.Printf("%d %d %d %dMB %dMB\n", journal.scenario.numSenders, journal.scenario.numTxsPerSender, journal.scenario.payloadLengthPerTx, bToMb(journal.poolStructuresFootprint()), bToMb(journal.txsFootprint()))
}

func bToMb(b uint64) int {
	return int(b / 1024 / 1024)
}
