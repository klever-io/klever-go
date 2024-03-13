package workItems_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/indexer/mock"
	"github.com/klever-io/klever-go/indexer/workItems"
	"github.com/stretchr/testify/require"
)

func TestItemTpsBenchmark_Save(t *testing.T) {
	called := false
	itemTPS := workItems.NewItemTpsBenchmark(
		&mock.ElasticProcessorStub{
			SaveNodeStatisticsCalled: func(tpsBenchmark statistics.TPSBenchmark) error {
				called = true
				return nil
			},
		},
		&statistics.TpsBenchmark{},
	)
	require.False(t, itemTPS.IsInterfaceNil())

	err := itemTPS.Save()
	require.NoError(t, err)
	require.True(t, called)
}

func TestItemTpsBenchmark_SaveTpsBenchmarkShouldErr(t *testing.T) {
	localErr := errors.New("local err")
	itemTPS := workItems.NewItemTpsBenchmark(
		&mock.ElasticProcessorStub{
			SaveNodeStatisticsCalled: func(tpsBenchmark statistics.TPSBenchmark) error {
				return localErr
			},
		},
		&statistics.TpsBenchmark{},
	)
	require.False(t, itemTPS.IsInterfaceNil())

	err := itemTPS.Save()
	require.Equal(t, localErr, err)
}
