package logsevents_test

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/logsevents"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSCInvocationsProcessor(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		pubKeyConverter := &mock.PubkeyConverterStub{}

		processor := logsevents.NewSCInvocationsProcessorForTest(pubKeyConverter)

		require.NotNil(t, processor)
		// Test behavior rather than internal state - processor should handle completedTxEvent
		event := &transaction.Event{
			Identifier: []byte(core.CompletedTxEventIdentifier),
		}
		args := logsevents.NewArgsProcessEventForTest(
			event,
			"test",
			[]byte("test"),
			nil,
		)
		result := processor.ProcessEventForTest(args)
		assert.True(t, result.IsProcessedForTest(), "Should process CompletedTxEventIdentifier")
	})
}

func TestSCInvocationsProcessor_ProcessEvent(t *testing.T) {
	t.Run("NonMatchingEventIdentifier_ReturnsNotProcessed", func(t *testing.T) {
		pubKeyConverter := &mock.PubkeyConverterStub{
			EncodeCalled: func(pkBytes []byte) string {
				return "klv1qqqqqqqqqqqqqpgqtest"
			},
		}
		processor := logsevents.NewSCInvocationsProcessorForTest(pubKeyConverter)

		event := &transaction.Event{
			Identifier: []byte("someOtherEvent"),
		}

		args := logsevents.NewArgsProcessEventForTest(
			event,
			"txhash123",
			[]byte("address"),
			nil,
		)

		result := processor.ProcessEventForTest(args)

		assert.False(t, result.IsProcessedForTest())
	})

	t.Run("CompletedTxEvent_NonSmartContractAddress_ReturnsProcessed", func(t *testing.T) {
		// Regular address (not starting with klv1qqqq...)
		regularAddress := make([]byte, 32)
		copy(regularAddress, []byte("regular-address"))

		pubKeyConverter := &mock.PubkeyConverterStub{
			EncodeCalled: func(pkBytes []byte) string {
				return "klv1regularaddress"
			},
		}
		processor := logsevents.NewSCInvocationsProcessorForTest(pubKeyConverter)

		event := &transaction.Event{
			Identifier: []byte(core.CompletedTxEventIdentifier),
		}

		args := logsevents.NewArgsProcessEventForTest(
			event,
			"txhash123",
			regularAddress,
			nil, // No alteredSC handler
		)

		result := processor.ProcessEventForTest(args)

		assert.True(t, result.IsProcessedForTest(), "Should be marked as processed even for non-SC addresses")
	})

	t.Run("CompletedTxEvent_SmartContractAddress_NilAlteredSC_ReturnsProcessed", func(t *testing.T) {
		// Smart contract address (first 8 bytes must be zero for SC address)
		// NumInitCharactersForScAddress = 10, VMTypeLen = 2, so first 8 bytes = 0
		scAddress := make([]byte, 32)
		// Bytes 0-7 are zero (smart contract prefix)
		// Bytes 8-9 are VM type (can be anything)
		scAddress[8] = 0x05
		scAddress[9] = 0x00

		pubKeyConverter := &mock.PubkeyConverterStub{
			EncodeCalled: func(pkBytes []byte) string {
				return "klv1qqqqqqqqqqqqqpgqtest"
			},
		}
		processor := logsevents.NewSCInvocationsProcessorForTest(pubKeyConverter)

		event := &transaction.Event{
			Identifier: []byte(core.CompletedTxEventIdentifier),
		}

		args := logsevents.NewArgsProcessEventForTest(
			event,
			"txhash123",
			scAddress,
			nil, // No alteredSC handler
		)

		result := processor.ProcessEventForTest(args)

		assert.True(t, result.IsProcessedForTest())
	})

	t.Run("CompletedTxEvent_SmartContractAddress_AddsToAlteredSC", func(t *testing.T) {
		// Smart contract address (first 8 bytes zero, then VM type)
		scAddress := make([]byte, 32)
		// Bytes 0-7 are zero (smart contract prefix)
		// Bytes 8-9 are VM type
		scAddress[8] = 0x05
		scAddress[9] = 0x00

		encodedSCAddress := "klv1qqqqqqqqqqqqqpgqtest"

		pubKeyConverter := &mock.PubkeyConverterStub{
			EncodeCalled: func(pkBytes []byte) string {
				return encodedSCAddress
			},
		}
		processor := logsevents.NewSCInvocationsProcessorForTest(pubKeyConverter)

		event := &transaction.Event{
			Identifier: []byte(core.CompletedTxEventIdentifier),
		}

		alteredSC := data.NewAlteredSmartContracts()

		args := logsevents.NewArgsProcessEventForTest(
			event,
			"txhash123",
			scAddress,
			alteredSC,
		)

		result := processor.ProcessEventForTest(args)

		assert.True(t, result.IsProcessedForTest())

		// Verify the smart contract was added to alteredSC
		contracts := alteredSC.GetAll()
		require.Len(t, contracts, 1)

		scContracts, exists := contracts[encodedSCAddress]
		assert.True(t, exists, "Smart contract address should exist in alteredSC")
		require.Len(t, scContracts, 1)
		assert.False(t, scContracts[0].IsNew, "IsNew should be false for invocations")
	})

	t.Run("CompletedTxEvent_MultipleInvocations_SameContract", func(t *testing.T) {
		// Smart contract address (first 8 bytes zero, then VM type)
		scAddress := make([]byte, 32)
		scAddress[8] = 0x05
		scAddress[9] = 0x00

		encodedSCAddress := "klv1qqqqqqqqqqqqqpgqtest"

		pubKeyConverter := &mock.PubkeyConverterStub{
			EncodeCalled: func(pkBytes []byte) string {
				return encodedSCAddress
			},
		}
		processor := logsevents.NewSCInvocationsProcessorForTest(pubKeyConverter)

		event := &transaction.Event{
			Identifier: []byte(core.CompletedTxEventIdentifier),
		}

		alteredSC := data.NewAlteredSmartContracts()

		// Process first invocation
		args1 := logsevents.NewArgsProcessEventForTest(
			event,
			"txhash123",
			scAddress,
			alteredSC,
		)
		result1 := processor.ProcessEventForTest(args1)
		assert.True(t, result1.IsProcessedForTest())

		// Process second invocation of same contract
		args2 := logsevents.NewArgsProcessEventForTest(
			event,
			"txhash456",
			scAddress,
			alteredSC,
		)
		result2 := processor.ProcessEventForTest(args2)
		assert.True(t, result2.IsProcessedForTest())

		// Verify the contract was tracked with both invocations
		// Each transaction interaction should be counted separately for accurate totalTransactions
		contracts := alteredSC.GetAll()
		require.Len(t, contracts, 1)

		scContracts, exists := contracts[encodedSCAddress]
		assert.True(t, exists)
		assert.Len(t, scContracts, 2, "Should have two entries (one for each transaction)")
		assert.False(t, scContracts[0].IsNew, "IsNew should be false")
		assert.False(t, scContracts[1].IsNew, "IsNew should be false")
	})

	t.Run("CompletedTxEvent_DifferentSmartContracts", func(t *testing.T) {
		// First smart contract address (first 8 bytes zero, then VM type)
		scAddress1 := make([]byte, 32)
		scAddress1[8] = 0x05
		scAddress1[9] = 0x00
		scAddress1[31] = 1 // Make it unique

		// Second smart contract address (first 8 bytes zero, then VM type)
		scAddress2 := make([]byte, 32)
		scAddress2[8] = 0x05
		scAddress2[9] = 0x00
		scAddress2[31] = 2 // Make it unique

		encodedSC1 := "klv1qqqqqqqqqqqqqpgqtest1"
		encodedSC2 := "klv1qqqqqqqqqqqqqpgqtest2"

		pubKeyConverter := &mock.PubkeyConverterStub{
			EncodeCalled: func(pkBytes []byte) string {
				if pkBytes[31] == 1 {
					return encodedSC1
				}
				return encodedSC2
			},
		}
		processor := logsevents.NewSCInvocationsProcessorForTest(pubKeyConverter)

		event := &transaction.Event{
			Identifier: []byte(core.CompletedTxEventIdentifier),
		}

		alteredSC := data.NewAlteredSmartContracts()

		// Process first contract
		args1 := logsevents.NewArgsProcessEventForTest(
			event,
			"txhash123",
			scAddress1,
			alteredSC,
		)
		processor.ProcessEventForTest(args1)

		// Process second contract
		args2 := logsevents.NewArgsProcessEventForTest(
			event,
			"txhash456",
			scAddress2,
			alteredSC,
		)
		processor.ProcessEventForTest(args2)

		// Verify both contracts were tracked
		contracts := alteredSC.GetAll()
		assert.Len(t, contracts, 2, "Should have two different smart contracts")

		_, exists1 := contracts[encodedSC1]
		assert.True(t, exists1, "First contract should exist")

		_, exists2 := contracts[encodedSC2]
		assert.True(t, exists2, "Second contract should exist")
	})
}
