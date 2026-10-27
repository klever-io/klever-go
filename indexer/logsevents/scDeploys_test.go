package logsevents_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	coreData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/logsevents"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAddressLen = 32

func extractSCDeploys(t *testing.T, event *transaction.Event) map[string]*data.ScDeployInfo {
	t.Helper()

	processor, err := logsevents.NewLogsAndEventsProcessor(logsevents.ArgsLogsAndEventsProcessor{
		PubKeyConverter: mock.NewPubkeyConverterMock(testAddressLen),
		Marshalizer:     &mock.MarshalizerMock{},
		Hasher:          &mock.HasherMock{},
	})
	require.NoError(t, err)

	pool := &indexer.Pool{
		Logs: []*coreData.LogData{
			{
				LogHandler: &transaction.Log{
					Address: event.Address,
					Events:  []*transaction.Event{event},
				},
				TxHash: "txHash",
			},
		},
	}

	return processor.ExtractDataFromLogs(pool, nil, 1234).ScDeploys
}

func TestSCDeploysProcessor_SystemLogGate(t *testing.T) {
	scAddress := bytes.Repeat([]byte{1}, testAddressLen)
	creator := bytes.Repeat([]byte{2}, testAddressLen)

	for _, identifier := range []string{core.SCDeployIdentifier, core.SCUpgradeIdentifier} {
		t.Run(identifier+"_SystemLog_IsIndexed", func(t *testing.T) {
			scDeploys := extractSCDeploys(t, &transaction.Event{
				Address:     scAddress,
				Identifier:  []byte(identifier),
				Topics:      [][]byte{scAddress, creator},
				IsSystemLog: true,
			})

			require.Len(t, scDeploys, 1)
			info := scDeploys[hex.EncodeToString(scAddress)]
			require.NotNil(t, info)
			assert.Equal(t, hex.EncodeToString(creator), info.Creator)
			assert.Equal(t, hex.EncodeToString([]byte("txHash")), info.TxHash)
			assert.Equal(t, uint64(1234), info.Timestamp)
		})

		t.Run(identifier+"_ContractLog_IsIgnored", func(t *testing.T) {
			scDeploys := extractSCDeploys(t, &transaction.Event{
				Address:     scAddress,
				Identifier:  []byte(identifier),
				Topics:      [][]byte{scAddress, creator},
				IsSystemLog: false,
			})

			assert.Empty(t, scDeploys)
		})
	}
}

func TestSCDeploysProcessor_WrongSizeTopics(t *testing.T) {
	scAddress := bytes.Repeat([]byte{1}, testAddressLen)
	creator := bytes.Repeat([]byte{2}, testAddressLen)
	short := []byte("short")

	tests := map[string][][]byte{
		"MissingCreator":     {scAddress},
		"ShortSCAddress":     {short, creator},
		"ShortCreator":       {scAddress, short},
		"EmptySCAddress":     {{}, creator},
		"OversizedSCAddress": {append(bytes.Clone(scAddress), 0), creator},
	}

	for name, topics := range tests {
		t.Run(name, func(t *testing.T) {
			scDeploys := extractSCDeploys(t, &transaction.Event{
				Address:     scAddress,
				Identifier:  []byte(core.SCDeployIdentifier),
				Topics:      topics,
				IsSystemLog: true,
			})

			assert.Empty(t, scDeploys)
		})
	}
}
