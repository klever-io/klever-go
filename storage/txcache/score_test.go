package txcache

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultScoreComputer_computeRawScore(t *testing.T) {
	computer := newDefaultScoreComputer()

	// 50k movefees, 100Bil minPrice -> normalizedFee 8940
	score := computer.computeRawScore(senderScoreParams{count: 1, size: 128, fees: 1_500_000})
	assert.InDelta(t, float64(16.77879391994379), score, delta)

	score = computer.computeRawScore(senderScoreParams{count: 1, size: 512, fees: 10_000_000})
	assert.InDelta(t, float64(39.189929887198005), score, delta)

	score = computer.computeRawScore(senderScoreParams{count: 1, size: 2048, fees: 30_000_000})
	assert.InDelta(t, float64(80.57142853536077), score, delta)

	score = computer.computeRawScore(senderScoreParams{count: 2, size: 256, fees: 5_500_000})
	assert.InDelta(t, float64(18.53459837699225), score, delta)

	score = computer.computeRawScore(senderScoreParams{count: 1000, size: 2048, fees: 100_000_000})
	assert.InDelta(t, float64(17.94953975864093), score, delta)

	score = computer.computeRawScore(senderScoreParams{count: 10000, size: 4096, fees: 1000_000_000})
	assert.InDelta(t, float64(31.71505820780851), score, delta)
}

func BenchmarkScoreComputer_computeRawScore(b *testing.B) {
	computer := newDefaultScoreComputer()

	for i := 0; i < b.N; i++ {
		for j := uint64(0); j < 10000000; j++ {
			computer.computeRawScore(senderScoreParams{count: j, size: uint64(float64(8000) * float64(j)), fees: 100000 * j})
		}
	}
}

func TestDefaultScoreComputer_computeRawScoreOfTxListForSender(t *testing.T) {
	computer := newDefaultScoreComputer()
	list := newUnconstrainedListToTest()

	list.AddTx(createTxWithParams([]byte("a"), ".", 1, 512, 1_500_000))
	list.AddTx(createTxWithParams([]byte("b"), ".", 1, 256, 1_500_000))
	list.AddTx(createTxWithParams([]byte("c"), ".", 1, 256, 1_500_000))

	require.Equal(t, uint64(3), list.countTx())
	require.Equal(t, int64(1024), list.totalBytes.Get())
	require.Equal(t, int64(7500000), list.totalFees.Get())

	scoreParams := list.getScoreParams()
	rawScore := computer.computeRawScore(scoreParams)
	require.InDelta(t, float64(23.927395424614417), rawScore, delta)
}

func TestDefaultScoreComputer_scoreFluctuatesDeterministicallyWhileTxListForSenderMutates(t *testing.T) {
	computer := newDefaultScoreComputer()
	list := newUnconstrainedListToTest()

	A := createTxWithParams([]byte("A"), ".", 1, 1024, 1_500_000)
	B := createTxWithParams([]byte("b"), ".", 1, 512, 1_500_000)
	C := createTxWithParams([]byte("c"), ".", 1, 512, 1_500_000)
	D := createTxWithParams([]byte("d"), ".", 1, 128, 1_500_000)

	scoreNone := int(computer.computeScore(list.getScoreParams()))
	list.AddTx(A)
	scoreA := int(computer.computeScore(list.getScoreParams()))
	list.AddTx(B)
	scoreAB := int(computer.computeScore(list.getScoreParams()))
	list.AddTx(C)
	scoreABC := int(computer.computeScore(list.getScoreParams()))
	list.AddTx(D)
	scoreABCD := int(computer.computeScore(list.getScoreParams()))

	require.Equal(t, 0, scoreNone)
	require.Equal(t, 35, scoreA)
	require.Equal(t, 36, scoreAB)
	require.Equal(t, 35, scoreABC)
	require.Equal(t, 38, scoreABCD)

	list.RemoveTx(D)
	scoreABC = int(computer.computeScore(list.getScoreParams()))
	list.RemoveTx(C)
	scoreAB = int(computer.computeScore(list.getScoreParams()))
	list.RemoveTx(B)
	scoreA = int(computer.computeScore(list.getScoreParams()))
	list.RemoveTx(A)
	scoreNone = int(computer.computeScore(list.getScoreParams()))

	require.Equal(t, 0, scoreNone)
	require.Equal(t, 35, scoreA)
	require.Equal(t, 36, scoreAB)
	require.Equal(t, 35, scoreABC)
}

func TestDefaultScoreComputer_DifferentSenders(t *testing.T) {
	computer := newDefaultScoreComputer()

	A := createTxWithParams([]byte("a"), "a", 1, 128, 1_500_000)     // min value normal tx
	B := createTxWithParams([]byte("b"), "b", 1, 128, 3_000_000_000) // 50% higher value normal tx
	C := createTxWithParams([]byte("c"), "c", 1, 128, 1_500_000)     // min value SC call
	D := createTxWithParams([]byte("d"), "d", 1, 128, 3_000_000_000) // 50% higher value SC call

	listA := newUnconstrainedListToTest()
	listA.AddTx(A)
	scoreA := int(computer.computeScore(listA.getScoreParams()))

	listB := newUnconstrainedListToTest()
	listB.AddTx(B)
	scoreB := int(computer.computeScore(listB.getScoreParams()))

	listC := newUnconstrainedListToTest()
	listC.AddTx(C)
	scoreC := int(computer.computeScore(listC.getScoreParams()))

	listD := newUnconstrainedListToTest()
	listD.AddTx(D)
	scoreD := int(computer.computeScore(listD.getScoreParams()))

	require.Equal(t, 21, scoreA)
	require.Equal(t, 85, scoreB)
	require.Equal(t, 21, scoreC)
	require.Equal(t, 85, scoreD)

	// adding same type of transactions for each sender decreases the score
	for i := 2; i < 1000; i++ {
		A = createTxWithParams([]byte("a"+strconv.Itoa(i)), "a", uint64(i), 128, 1_500_000) // min value normal tx
		listA.AddTx(A)
		B = createTxWithParams([]byte("b"+strconv.Itoa(i)), "b", uint64(i), 128, 3_000_000_000) // 50% higher value normal tx
		listB.AddTx(B)
		C = createTxWithParams([]byte("c"+strconv.Itoa(i)), "c", uint64(i), 128, 1_500_000) // min value SC call
		listC.AddTx(C)
		D = createTxWithParams([]byte("d"+strconv.Itoa(i)), "d", uint64(i), 128, 3_000_000_000) // 50% higher value SC call
		listD.AddTx(D)
	}

	scoreA = int(computer.computeScore(listA.getScoreParams()))
	scoreB = int(computer.computeScore(listB.getScoreParams()))
	scoreC = int(computer.computeScore(listC.getScoreParams()))
	scoreD = int(computer.computeScore(listD.getScoreParams()))

	require.Equal(t, 91, scoreA)
	require.Equal(t, 99, scoreB)
	require.Equal(t, 91, scoreC)
	require.Equal(t, 99, scoreD)
}
