package provider

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
)

// signedMetricKeys lists metrics whose JSON value may be negative. Routing them
// through SetInt64Value preserves the sign — `uint64(float64(-1.0))` is
// implementation-defined per the Go spec and yields 0 on gc.
//
// NOTE: keys here must be co-listed in core/metrics.go as conceptually-signed.
// Today the only signed metric is MetricRedundancyLevel (domain: -1/0/N). Adding
// a future signed metric without listing it here will silently truncate negatives
// to 0 on the connector path. Consider co-locating with core/metrics.go via a
// `signed: true` flag or a registry walked at init.
var signedMetricKeys = map[string]struct{}{
	core.MetricRedundancyLevel: {},
}

var log = logger.GetOrCreate("connector/provider")

const statusMetricsUrlSuffix = "/node/status"

type statusMetricsResponseData struct {
	Response map[string]interface{} `json:"metrics"`
}

type responseFromApi struct {
	Data  statusMetricsResponseData `json:"data"`
	Error string                    `json:"error"`
	Code  string                    `json:"code"`
}

// StatusMetricsProvider is the struct that will handle initializing the presenter and fetching updated metrics from the node
type StatusMetricsProvider struct {
	presenter     PresenterHandler
	nodeAddress   string
	fetchInterval int
}

// NewStatusMetricsProvider will return a new instance of a StatusMetricsProvider
func NewStatusMetricsProvider(presenter PresenterHandler, nodeAddress string, fetchInterval int) (*StatusMetricsProvider, error) {
	if len(nodeAddress) == 0 {
		return nil, ErrInvalidAddressLength
	}
	if fetchInterval < 1 {
		return nil, ErrInvalidFetchInterval
	}
	if presenter == nil {
		return nil, ErrNilConnectorPresenter
	}

	return &StatusMetricsProvider{
		presenter:     presenter,
		nodeAddress:   formatUrlAddress(nodeAddress),
		fetchInterval: fetchInterval,
	}, nil
}

// StartUpdatingData will update data from the API at a given interval
func (smp *StatusMetricsProvider) StartUpdatingData() {
	go func() {
		for {
			metricsMap, err := smp.loadMetricsFromApi()
			if err != nil {
				log.Debug("fetch from API",
					"error", err.Error())
			} else {
				smp.applyMetricsToPresenter(metricsMap)
			}

			time.Sleep(time.Duration(smp.fetchInterval) * time.Millisecond)
		}
	}()
}

func (smp *StatusMetricsProvider) loadMetricsFromApi() (map[string]interface{}, error) {
	client := http.Client{}

	statusMetricsUrl := smp.nodeAddress + statusMetricsUrlSuffix
	resp, err := client.Get(statusMetricsUrl)
	if err != nil {
		return nil, err
	}

	responseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = resp.Body.Close()
		if err != nil {
			log.Error("close response body", "error", err.Error())
		}
	}()

	var metricsResponse responseFromApi
	err = json.Unmarshal(responseBytes, &metricsResponse)
	if err != nil {
		return nil, err
	}

	return metricsResponse.Data.Response, nil
}

func (smp *StatusMetricsProvider) applyMetricsToPresenter(metricsMap map[string]interface{}) {
	var err error
	for key, value := range metricsMap {
		err = smp.setPresenterValue(key, value)
		if err != nil {
			log.Debug("connector metric set",
				"error", err.Error())
		}
	}
}

func (smp *StatusMetricsProvider) setPresenterValue(key string, value interface{}) error {
	switch v := value.(type) {
	case float64:
		// JSON numbers decode as float64; route signed metrics through SetInt64Value
		// to preserve the sign (see signedMetricKeys). Bound-check both casts —
		// `int64(NaN/±Inf/out-of-range float64)` is implementation-defined per the
		// Go spec, same trap this allowlist exists to avoid for uint64. Use `>=`
		// because float64 cannot represent MaxInt64 / MaxUint64 exactly — both
		// round up to 2^63 / 2^64, and `int64(2^63)` / `uint64(2^64)` are UB.
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return ErrTypeAssertionFailed
		}
		if _, signed := signedMetricKeys[key]; signed {
			if v < math.MinInt64 || v >= math.MaxInt64 {
				return ErrTypeAssertionFailed
			}
			smp.presenter.SetInt64Value(key, int64(v))
			return nil
		}
		if v < 0 || v >= math.MaxUint64 {
			return ErrTypeAssertionFailed
		}
		smp.presenter.SetUInt64Value(key, uint64(v))
	case string:
		smp.presenter.SetStringValue(key, v)
	default:
		return ErrTypeAssertionFailed
	}

	return nil
}

func formatUrlAddress(address string) string {
	httpPrefix := "http://"
	if !strings.HasPrefix(address, httpPrefix) {
		address = httpPrefix + address
	}

	suffix := "/"
	if strings.HasSuffix(address, suffix) {
		address = address[:len(address)-1]
	}

	return address
}
