package factory_test

import (
	"errors"
	"flag"
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"

	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/data/metrics"

	"github.com/klever-io/klever-go/factory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func createMockCliContext() *cli.Context {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = set.Bool("log-view", false, "")
	context := cli.NewContext(app, set, nil)
	return context
}

func createMockStatusHandlerArgs() *factory.ArgStatusHandlers {
	return &factory.ArgStatusHandlers{
		LogViewName:              "log-view",
		Ctx:                      createMockCliContext(),
		Marshalizer:              &commonMock.MarshalizerMock{},
		Uint64ByteSliceConverter: &commonMock.Uint64ByteSliceConverterMock{},
		ChanStartViews:           make(chan struct{}, 1),
		ChanLogRewrite:           make(chan struct{}, 1),
	}
}

func TestNewStatusHandlersFactory(t *testing.T) {
	t.Parallel()
	t.Run("nil marshalizer should err", func(t *testing.T) {
		args := createMockStatusHandlerArgs()
		args.Marshalizer = nil

		statusHandlersInfo, err := factory.CreateStatusHandlers(args)

		assert.Nil(t, statusHandlersInfo)
		assert.Equal(t, common.ErrNilMarshalizer, err)
	})

	t.Run("nil converter should err", func(t *testing.T) {
		args := createMockStatusHandlerArgs()
		args.Uint64ByteSliceConverter = nil

		statusHandlersInfo, err := factory.CreateStatusHandlers(args)

		assert.Nil(t, statusHandlersInfo)
		assert.Equal(t, process.ErrNilUint64Converter, err)
	})

	t.Run("should work", func(t *testing.T) {
		args := createMockStatusHandlerArgs()

		statusHandlersInfo, err := factory.CreateStatusHandlers(args)

		require.NoError(t, err)
		require.NotNil(t, statusHandlersInfo)
		assert.NotNil(t, statusHandlersInfo.StatusHandler)
		assert.NotNil(t, statusHandlersInfo.PersistentHandler)
	})
}

func TestStatusHandlersInfo_UpdateStorerAndMetricsForPersistentHandler(t *testing.T) {
	t.Parallel()

	t.Run("should work when setting storage succeeds", func(t *testing.T) {
		args := createMockStatusHandlerArgs()
		statusHandlersInfo, _ := factory.CreateStatusHandlers(args)

		storerMock := mock.NewStorerMock("test", 1)

		err := statusHandlersInfo.UpdateStorerAndMetricsForPersistentHandler(storerMock)
		assert.Nil(t, err)
	})

	t.Run("nil storer should return error", func(t *testing.T) {
		args := createMockStatusHandlerArgs()
		statusHandlersInfo, _ := factory.CreateStatusHandlers(args)

		err := statusHandlersInfo.UpdateStorerAndMetricsForPersistentHandler(nil)
		assert.Equal(t, common.ErrNilStorage, err)
	})
}

func TestStatusHandlersInfo_LoadTpsBenchmarkFromStorage(t *testing.T) {
	t.Parallel()

	t.Run("should return empty when Get last nonce fails", func(t *testing.T) {
		storerMock := commonMock.NewStorerMock("test", 1)

		args := createMockStatusHandlerArgs()
		statusHandlersInfo, _ := factory.CreateStatusHandlers(args)

		result := statusHandlersInfo.LoadTpsBenchmarkFromStorage(storerMock, args.Marshalizer)

		expectedBenchmarks := &statistics.TpsPersistentData{
			BlockNumber:           0,
			SlotNumber:            0,
			PeakTPS:               0,
			AverageTPS:            big.NewInt(0),
			AverageBlockTxCount:   big.NewInt(0),
			TotalProcessedTxCount: big.NewInt(0),
			CurrentBlockTxCount:   0,
		}
		assert.Equal(t, expectedBenchmarks, result)
	})

	t.Run("should return empty when Get metrics data fails", func(t *testing.T) {
		storerMock := commonMock.NewStorerMock("test", 1)
		lastNonceBytes := []byte("nonce")
		storerMock.GetCurrentEpochData().Set(string([]byte(core.LastNonceKeyMetricsStorage)), lastNonceBytes)

		args := createMockStatusHandlerArgs()
		statusHandlersInfo, _ := factory.CreateStatusHandlers(args)

		result := statusHandlersInfo.LoadTpsBenchmarkFromStorage(storerMock, args.Marshalizer)

		expectedBenchmarks := &statistics.TpsPersistentData{
			BlockNumber:           0,
			SlotNumber:            0,
			AverageTPS:            big.NewInt(0),
			PeakTPS:               0,
			AverageBlockTxCount:   big.NewInt(0),
			TotalProcessedTxCount: big.NewInt(0),
			CurrentBlockTxCount:   0,
		}
		assert.Equal(t, expectedBenchmarks, result)
	})

	t.Run("should return empty when unmarshal fails", func(t *testing.T) {
		storerMock := commonMock.NewStorerMock("test", 1)
		lastNonceBytes := []byte("nonce")
		storerMock.GetCurrentEpochData().Set(string([]byte(core.LastNonceKeyMetricsStorage)), lastNonceBytes)
		storerMock.GetCurrentEpochData().Set(string(lastNonceBytes), []byte("invalid data"))

		args := createMockStatusHandlerArgs()
		args.Marshalizer = &commonMock.MarshalizerStub{
			UnmarshalCalled: func(obj interface{}, buff []byte) error {
				return errors.New("unmarshal error")
			},
		}
		statusHandlersInfo, _ := factory.CreateStatusHandlers(args)

		result := statusHandlersInfo.LoadTpsBenchmarkFromStorage(storerMock, args.Marshalizer)

		expectedBenchmarks := &statistics.TpsPersistentData{
			BlockNumber:           0,
			SlotNumber:            0,
			AverageTPS:            big.NewInt(0),
			PeakTPS:               0,
			AverageBlockTxCount:   big.NewInt(0),
			TotalProcessedTxCount: big.NewInt(0),
			CurrentBlockTxCount:   0,
		}
		assert.Equal(t, expectedBenchmarks, result)
	})

	t.Run("should work with valid data", func(t *testing.T) {
		storerMock := commonMock.NewStorerMock("test", 1)

		metricsList := &metrics.MetricsList{
			Metrics: []*metrics.Metric{
				{
					Key:   core.MetricNonceForTPS,
					Value: &metrics.Metric_ValUint64{ValUint64: 100},
				},
				{
					Key:   core.MetricCurrentSlot,
					Value: &metrics.Metric_ValUint64{ValUint64: 200},
				},
				{
					Key:   core.MetricAverageTPS,
					Value: &metrics.Metric_ValString{ValString: "100"},
				},
				{
					Key:   core.MetricCurrentBlockTxCount,
					Value: &metrics.Metric_ValUint64{ValUint64: 50},
				},
				{
					Key:   core.MetricPeakTPS,
					Value: &metrics.Metric_ValUint64{ValUint64: 1000},
				},
				{
					Key:   core.MetricNumProcessedTxsTPSBenchmark,
					Value: &metrics.Metric_ValUint64{ValUint64: 5000},
				},
				{
					Key:   core.MetricAverageBlockTxCount,
					Value: &metrics.Metric_ValString{ValString: "100"},
				},
			},
		}

		lastNonceBytes := []byte("nonce")
		args := createMockStatusHandlerArgs()
		marshalizedData, _ := args.Marshalizer.Marshal(metricsList)
		storerMock.GetCurrentEpochData().Set(string([]byte(core.LastNonceKeyMetricsStorage)), lastNonceBytes)
		storerMock.GetCurrentEpochData().Set(string(lastNonceBytes), marshalizedData)

		args.Marshalizer = &commonMock.MarshalizerStub{
			UnmarshalCalled: func(obj interface{}, buff []byte) error {
				if ml, ok := obj.(*metrics.MetricsList); ok {
					ml.Metrics = make([]*metrics.Metric, len(metricsList.Metrics))
					for i, metric := range metricsList.Metrics {
						newMetric := &metrics.Metric{
							Key: metric.Key,
						}
						switch v := metric.Value.(type) {
						case *metrics.Metric_ValUint64:
							newMetric.Value = &metrics.Metric_ValUint64{ValUint64: v.ValUint64}
						case *metrics.Metric_ValString:
							newMetric.Value = &metrics.Metric_ValString{ValString: v.ValString}
						}
						ml.Metrics[i] = newMetric
					}
					return nil
				}
				return errors.New("invalid type")
			},
			MarshalCalled: func(obj interface{}) ([]byte, error) {
				return []byte("valid marshalized data"), nil
			},
		}
		statusHandlersInfo, _ := factory.CreateStatusHandlers(args)

		result := statusHandlersInfo.LoadTpsBenchmarkFromStorage(storerMock, args.Marshalizer)

		expectedBenchmarks := &statistics.TpsPersistentData{
			BlockNumber:           100,
			SlotNumber:            200,
			CurrentBlockTxCount:   50,
			PeakTPS:               1000,
			AverageTPS:            big.NewInt(100),
			TotalProcessedTxCount: big.NewInt(5000),
			AverageBlockTxCount:   big.NewInt(100),
		}

		assert.Equal(t, expectedBenchmarks.BlockNumber, result.BlockNumber)
		assert.Equal(t, expectedBenchmarks.SlotNumber, result.SlotNumber)
		assert.Equal(t, expectedBenchmarks.CurrentBlockTxCount, result.CurrentBlockTxCount)
		assert.Equal(t, expectedBenchmarks.PeakTPS, result.PeakTPS)
		assert.Equal(t, expectedBenchmarks.AverageTPS, result.AverageTPS)
		assert.Equal(t, expectedBenchmarks.TotalProcessedTxCount, result.TotalProcessedTxCount)
		assert.Equal(t, expectedBenchmarks.AverageBlockTxCount, result.AverageBlockTxCount)
	})
}
