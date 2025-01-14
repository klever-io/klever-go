package factory

import (
	"fmt"
	"io"
	"math/big"
	"os"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/data/metrics"
	"github.com/klever-io/klever-go/statusHandler"
	factoryViews "github.com/klever-io/klever-go/statusHandler/factory"
	"github.com/klever-io/klever-go/statusHandler/persister"
	"github.com/klever-io/klever-go/statusHandler/view"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
	"github.com/urfave/cli"
)

var log = logger.GetOrCreate("factory")

const defaultConnectorRefreshTimeInMilliseconds = 500

// ArgStatusHandlers is a struct that stores arguments needed to create status handlers
type ArgStatusHandlers struct {
	LogViewName                  string
	ServersConfigurationFileName string
	Ctx                          *cli.Context
	Marshalizer                  marshal.Marshalizer
	Uint64ByteSliceConverter     typeConverters.Uint64ByteSliceConverter
	ChanStartViews               chan struct{}
	ChanLogRewrite               chan struct{}
}

// StatusHandlersInfo is struct that stores all components that are returned when status handlers are created
type statusHandlersInfo struct {
	UseConnector             bool
	StatusHandler            core.AppStatusHandler
	StatusMetrics            core.StatusMetricsHandler
	PersistentHandler        *persister.PersistentStatusHandler
	Uint64ByteSliceConverter typeConverters.Uint64ByteSliceConverter
}

// NewStatusHandlersFactoryArgs will return arguments for status handlers
func NewStatusHandlersFactoryArgs(
	logViewName string,
	ctx *cli.Context,
	marshalizer marshal.Marshalizer,
	uint64ByteSliceConverter typeConverters.Uint64ByteSliceConverter,
	chanStartViews chan struct{},
	chanLogRewrite chan struct{},
) (*ArgStatusHandlers, error) {
	baseErrMessage := "error creating status handler factory arguments"
	if ctx == nil {
		return nil, fmt.Errorf("%s: nil context", baseErrMessage)
	}
	if check.IfNil(marshalizer) {
		return nil, fmt.Errorf("%s: nil marshalizer", baseErrMessage)
	}
	if check.IfNil(uint64ByteSliceConverter) {
		return nil, fmt.Errorf("%s: nil uint64 byte slice converter", baseErrMessage)
	}
	if chanLogRewrite == nil {
		return nil, fmt.Errorf("%s: nil log rewrite channel", baseErrMessage)
	}
	if chanStartViews == nil {
		return nil, fmt.Errorf("%s: nil views start channel", baseErrMessage)
	}

	return &ArgStatusHandlers{
		LogViewName:              logViewName,
		Ctx:                      ctx,
		Marshalizer:              marshalizer,
		Uint64ByteSliceConverter: uint64ByteSliceConverter,
		ChanStartViews:           chanStartViews,
		ChanLogRewrite:           chanLogRewrite,
	}, nil
}

// CreateStatusHandlers will return a slice of status handlers
func CreateStatusHandlers(arguments *ArgStatusHandlers) (*statusHandlersInfo, error) {
	var appStatusHandlers []core.AppStatusHandler
	var views []factoryViews.Viewer
	var err error
	var handler core.AppStatusHandler

	presenterStatusHandler := createStatusHandlerPresenter()

	useConnector := !arguments.Ctx.GlobalBool(arguments.LogViewName)
	if useConnector {
		views, err = createViews(presenterStatusHandler, arguments.ChanStartViews)
		if err != nil {
			return nil, err
		}

		go func() {
			<-arguments.ChanLogRewrite
			writer, ok := presenterStatusHandler.(io.Writer)
			if !ok {
				return
			}
			err = logger.RemoveLogObserver(os.Stdout)
			if err != nil {
				log.Warn("cannot remove the log observer for std out", "error", err)
			}

			err = logger.AddLogObserver(writer, &logger.PlainFormatter{})
			if err != nil {
				log.Warn("cannot add log observer for Connector", "error", err)
			}
		}()

		appStatusHandler, ok := presenterStatusHandler.(core.AppStatusHandler)
		if ok {
			appStatusHandlers = append(appStatusHandlers, appStatusHandler)
		}
	}

	if len(views) == 0 {
		log.Info("current mode is log-view")
	}

	statusMetrics := statusHandler.NewStatusMetrics()
	appStatusHandlers = append(appStatusHandlers, statusMetrics)

	persistentHandler, err := persister.NewPersistentStatusHandler(arguments.Marshalizer, arguments.Uint64ByteSliceConverter)
	if err != nil {
		return nil, err
	}
	appStatusHandlers = append(appStatusHandlers, persistentHandler)

	if len(appStatusHandlers) > 0 {
		handler, err = statusHandler.NewAppStatusFacadeWithHandlers(appStatusHandlers...)
		if err != nil {
			log.Warn("cannot init AppStatusFacade", "error", err)
		}
	} else {
		handler = statusHandler.NewNilStatusHandler()
		log.Info("no AppStatusHandler used: started with NilStatusHandler")
	}

	statusHandlersInfoObject := new(statusHandlersInfo)
	statusHandlersInfoObject.StatusHandler = handler
	statusHandlersInfoObject.UseConnector = useConnector
	statusHandlersInfoObject.StatusMetrics = statusMetrics
	statusHandlersInfoObject.PersistentHandler = persistentHandler
	return statusHandlersInfoObject, nil
}

// UpdateStorerAndMetricsForPersistentHandler will set storer for persistent status handler
func (shi *statusHandlersInfo) UpdateStorerAndMetricsForPersistentHandler(store storage.Storer) error {
	if store == nil {
		return common.ErrNilStorage
	}

	err := shi.PersistentHandler.SetStorage(store)
	if err != nil {
		return err
	}

	return nil
}

// LoadTpsBenchmarkFromStorage will try to load tps benchmark from storage or zero values otherwise
func (shi *statusHandlersInfo) LoadTpsBenchmarkFromStorage(
	store storage.Storer,
	marshalizer marshal.Marshalizer,
) *statistics.TpsPersistentData {
	emptyTpsBenchmarks := &statistics.TpsPersistentData{
		BlockNumber:           0,
		SlotNumber:            0,
		PeakTPS:               0,
		AverageTPS:            big.NewInt(0),
		AverageBlockTxCount:   big.NewInt(0),
		TotalProcessedTxCount: big.NewInt(0),
		CurrentBlockTxCount:   0,
	}
	lastNonceBytes, err := store.Get([]byte(core.LastNonceKeyMetricsStorage))
	if err != nil {
		log.Debug("cannot load last nonce from metrics storage", "error", err, "key", []byte(core.LastNonceKeyMetricsStorage))
		return emptyTpsBenchmarks
	}

	lastDataList, err := store.Get(lastNonceBytes)
	if err != nil {
		log.Debug("cannot load metrics from storage", "error", err, "key", lastNonceBytes)
		return emptyTpsBenchmarks
	}

	metricsList := &metrics.MetricsList{}
	err = marshalizer.Unmarshal(metricsList, lastDataList)
	if err != nil {
		log.Debug("cannot unmarshal persistent metrics", "error", err)
		return emptyTpsBenchmarks
	}

	metricsMap := metrics.MapFromList(metricsList)

	tpsBenchmarks := &statistics.TpsPersistentData{}

	tpsBenchmarks.BlockNumber = persister.GetUint64(metricsMap[core.MetricNonceForTPS])
	tpsBenchmarks.SlotNumber = persister.GetUint64(metricsMap[core.MetricCurrentSlot])
	tpsBenchmarks.AverageTPS = persister.GetBigIntFromString(metricsMap[core.MetricAverageTPS])
	// #nosec G115 - max tx per block is uint32 value
	tpsBenchmarks.CurrentBlockTxCount = uint32(persister.GetUint64(metricsMap[core.MetricCurrentBlockTxCount]))
	tpsBenchmarks.PeakTPS = float64(persister.GetUint64(metricsMap[core.MetricPeakTPS]))
	tpsBenchmarks.TotalProcessedTxCount = big.NewInt(int64(persister.GetUint64(metricsMap[core.MetricNumProcessedTxsTPSBenchmark])))
	// #nosec G115 - total processed tx count is uint32

	tpsBenchmarks.AverageBlockTxCount = persister.GetBigIntFromString(metricsMap[core.MetricAverageBlockTxCount])

	shi.updateTpsMetrics(metricsMap)

	log.Debug("loaded tps benchmark from storage",
		"block number", tpsBenchmarks.BlockNumber,
		"slot number", tpsBenchmarks.SlotNumber,
		"peak tps", tpsBenchmarks.PeakTPS,
		"average tps", tpsBenchmarks.AverageTPS,
		"current block tx count", tpsBenchmarks.CurrentBlockTxCount,
		"average block tx count", tpsBenchmarks.AverageBlockTxCount,
		"total txs processed", tpsBenchmarks.TotalProcessedTxCount.String())

	return tpsBenchmarks
}

func (shi *statusHandlersInfo) updateTpsMetrics(metricsMap map[string]interface{}) {
	for key, value := range metricsMap {
		stringValue, isString := value.(string)
		if isString {
			log.Trace("setting metric value", "key", key, "value string", stringValue)
			shi.StatusHandler.SetStringValue(key, stringValue)
			continue
		}

		uint64Value, isUint64 := value.(uint64)
		if isUint64 {
			log.Trace("setting metric value", "key", key, "value uint64", uint64Value)
			shi.StatusHandler.SetUInt64Value(key, uint64Value)
		}
	}
}

// CreateStatusHandlerPresenter will return an instance of PresenterStatusHandler
func createStatusHandlerPresenter() view.Presenter {
	presenterStatusHandlerFactory := factoryViews.NewPresenterFactory()

	return presenterStatusHandlerFactory.Create()
}

// CreateViews will start an connector console  and will return an object if cannot create and start connectorConsole
func createViews(presenter view.Presenter, chanStart chan struct{}) ([]factoryViews.Viewer, error) {
	viewsFactory, err := factoryViews.NewViewsFactory(presenter, defaultConnectorRefreshTimeInMilliseconds)
	if err != nil {
		return nil, err
	}

	views, err := viewsFactory.Create()
	if err != nil {
		return nil, err
	}

	for _, v := range views {
		err = v.Start(chanStart)
		if err != nil {
			return nil, err
		}
	}

	return views, nil
}
