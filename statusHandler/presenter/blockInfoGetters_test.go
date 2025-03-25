package presenter

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/stretchr/testify/assert"
)

func TestPresenterStatusHandler_GetNumTxInBlock(t *testing.T) {
	t.Parallel()

	numTxInBlock := uint64(1000)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricNumTxInBlock, numTxInBlock)
	result := presenterStatusHandler.GetNumTxInBlock()

	assert.Equal(t, numTxInBlock, result)
}

func TestPresenterStatusHandler_GetNumTxInBlockShouldBeZero(t *testing.T) {
	t.Parallel()

	numTxInBlock := "1000"
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetStringValue(core.MetricNumTxInBlock, numTxInBlock)
	result := presenterStatusHandler.GetNumTxInBlock()

	assert.Equal(t, uint64(0), result)
}

func TestPresenterStatusHandler_GetNumTxShouldZeroIfIsNotSet(t *testing.T) {
	t.Parallel()

	presenterStatusHandler := NewPresenterStatusHandler()
	result := presenterStatusHandler.GetNumTxInBlock()

	assert.Equal(t, uint64(0), result)
}

func TestPresenterStatusHandler_GetConsensusState(t *testing.T) {
	t.Parallel()

	consensusState := "not in consensus group"
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetStringValue(core.MetricConsensusState, consensusState)
	result := presenterStatusHandler.GetConsensusState()

	assert.Equal(t, consensusState, result)
}

func TestPresenterStatusHandler_GetConsensusStateShouldReturnErrorMessageInvalidType(t *testing.T) {
	t.Parallel()

	consensusState := uint64(1)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricConsensusState, consensusState)
	result := presenterStatusHandler.GetConsensusState()

	assert.Equal(t, metricNotAvailable, result)
}

func TestPresenterStatusHandler_GetConsensusStateShouldReturnErrorMessageInvalidKey(t *testing.T) {
	t.Parallel()

	presenterStatusHandler := NewPresenterStatusHandler()
	result := presenterStatusHandler.GetConsensusState()

	assert.Equal(t, metricNotAvailable, result)
}

func TestPresenterStatusHandler_GetConsensusSlotStateState(t *testing.T) {
	t.Parallel()

	consensusSlotState := "participant"
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetStringValue(core.MetricConsensusSlotState, consensusSlotState)
	result := presenterStatusHandler.GetConsensusSlotState()

	assert.Equal(t, consensusSlotState, result)
}

func TestPresenterStatusHandler_GetCurrentBlockHash(t *testing.T) {
	t.Parallel()

	currentBlockHash := "hash"
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetStringValue(core.MetricCurrentBlockHash, currentBlockHash)
	result := presenterStatusHandler.GetCurrentBlockHash()

	assert.Equal(t, currentBlockHash, result)
}

func TestPresenterStatusHandler_GetCurrentSlotTimestamp(t *testing.T) {
	t.Parallel()

	currentSlotTimestamp := uint64(time.Now().Unix())
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricCurrentSlotTimestamp, currentSlotTimestamp)
	result := presenterStatusHandler.GetCurrentSlotTimestamp()

	assert.Equal(t, currentSlotTimestamp, result)
}

func TestPresenterStatusHandler_GetBlockSize(t *testing.T) {
	t.Parallel()

	bodyBlocksSize := uint64(100)
	headerSize := uint64(50)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricBodyBlocksSize, bodyBlocksSize)
	presenterStatusHandler.SetUInt64Value(core.MetricHeaderSize, headerSize)
	result := presenterStatusHandler.GetBlockSize()

	blockExpectedSize := bodyBlocksSize + headerSize
	assert.Equal(t, blockExpectedSize, result)
}

func TestPresenterStatusHandler_GetTxsSize(t *testing.T) {
	t.Parallel()

	txsSize := uint64(100)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricTXsBlocksSize, txsSize)
	result := presenterStatusHandler.GetTxsSize()

	assert.Equal(t, txsSize, result)
}

func TestPresenterStatusHandler_GetHighestFinalBlock(t *testing.T) {
	t.Parallel()

	highestFinalBlockNonce := uint64(100)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricHighestFinalBlock, highestFinalBlockNonce)
	result := presenterStatusHandler.GetHighestFinalBlock()

	assert.Equal(t, highestFinalBlockNonce, result)
}
