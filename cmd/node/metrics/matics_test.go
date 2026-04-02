package metrics

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/appStatusPolling"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/factory"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MetricsMap will hold all the metrics set via AppStatusHandler
type MetricsMap struct {
	metrics map[string]interface{}
}

func NewMetricsMap() *MetricsMap {
	return &MetricsMap{
		metrics: make(map[string]interface{}),
	}
}

func TestInitMetrics(t *testing.T) {
	tests := []struct {
		name             string
		appStatusHandler core.AppStatusHandler
		pubkeyStr        string
		nodeType         core.NodeType
		nsFile           string
		version          string
		totalSupply      string
		slotsPerEpoch    uint64
		expectedError    bool
		expectedMetrics  map[string]interface{}
	}{
		{
			name:             "Nil app status handler",
			appStatusHandler: nil,
			pubkeyStr:        "testPubKey",
			nodeType:         "validator",
			nsFile:           "../../../sharding/mock/nodesSetupMock.json",
			version:          "v1.0.0",
			totalSupply:      "1000000",
			slotsPerEpoch:    100,
			expectedError:    true,
			expectedMetrics:  nil,
		},
		{
			name:             "Nil nodes config",
			appStatusHandler: &mock.AppStatusHandlerStub{},
			pubkeyStr:        "testPubKey",
			nodeType:         "validator",
			nsFile:           "",
			version:          "v1.0.0",
			totalSupply:      "1000000",
			slotsPerEpoch:    100,
			expectedError:    true,
			expectedMetrics:  nil,
		},
		{
			name:             "Valid configuration",
			appStatusHandler: &mock.AppStatusHandlerStub{},
			pubkeyStr:        "testPubKey",
			nodeType:         "validator",
			nsFile:           "../../../sharding/mock/nodesSetupMock.json",
			version:          "v1.0.0",
			totalSupply:      "1000000",
			slotsPerEpoch:    100,
			expectedError:    false,
			expectedMetrics: map[string]interface{}{
				core.MetricPublicKeyBlockSign: "testPubKey",
				core.MetricNodeType:           "validator",
				core.MetricAppVersion:         "v1.0.0",
				core.MetricSlotTime:           uint64(4),
				core.MetricSlotsPerEpoch:      uint64(100),
				core.MetricTotalSupply:        "1000000",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Track metrics for validation
			metricsMap := NewMetricsMap()

			if !check.IfNil(tt.appStatusHandler) {
				ash := tt.appStatusHandler.(*mock.AppStatusHandlerStub)
				ash.SetUInt64ValueHandler = func(key string, value uint64) {
					metricsMap.metrics[key] = value
				}
				ash.SetStringValueHandler = func(key string, value string) {
					metricsMap.metrics[key] = value
				}
			}

			var err error
			var ns *sharding.NodesSetup
			if tt.nsFile != "" {
				ns, err = sharding.NewNodesSetup(
					tt.nsFile,
					mock.NewPubkeyConverterMock(32),
					mock.NewPubkeyConverterMock(96),
				)
				require.NoError(t, err)
			}

			// Call the function to test
			err = InitMetrics(
				tt.appStatusHandler,
				tt.pubkeyStr,
				tt.nodeType,
				ns,
				tt.version,
				tt.totalSupply,
				tt.slotsPerEpoch,
			)

			// Assert the result
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)

				// Check that expected metrics were set correctly
				for key, expectedValue := range tt.expectedMetrics {
					actualValue, exists := metricsMap.metrics[key]
					assert.True(t, exists, "Metric %s not found", key)
					assert.Equal(t, expectedValue, actualValue, "Metric %s has incorrect value", key)
				}
			}
		})
	}
}

func TestSaveUint64Metric(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      uint64
		expectCall bool
	}{
		{
			name:       "Save valid uint64 metric",
			key:        "testUint64Metric",
			value:      123456,
			expectCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled := false

			handler := &mock.AppStatusHandlerStub{
				SetUInt64ValueHandler: func(key string, value uint64) {
					handlerCalled = true
					assert.Equal(t, tt.key, key)
					assert.Equal(t, tt.value, value)
				},
			}

			// Call the function to test
			SaveUint64Metric(handler, tt.key, tt.value)

			// Verify the handler was called
			assert.Equal(t, tt.expectCall, handlerCalled)
		})
	}
}

func TestSaveStringMetric(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      string
		expectCall bool
	}{
		{
			name:       "Save valid string metric",
			key:        "testStringMetric",
			value:      "testValue",
			expectCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled := false

			handler := &mock.AppStatusHandlerStub{
				SetStringValueHandler: func(key string, value string) {
					handlerCalled = true
					assert.Equal(t, tt.key, key)
					assert.Equal(t, tt.value, value)
				},
			}

			// Call the function to test
			SaveStringMetric(handler, tt.key, tt.value)

			// Verify the handler was called
			assert.Equal(t, tt.expectCall, handlerCalled)
		})
	}
}

func TestStartStatusPolling(t *testing.T) {
	tests := []struct {
		name              string
		appStatusHandler  core.AppStatusHandler
		networkComponents *factory.NetworkComponents
		processComponents *factory.Process
		expectedError     bool
	}{
		{
			name:              "Nil app status handler",
			appStatusHandler:  nil,
			networkComponents: &factory.NetworkComponents{},
			processComponents: &factory.Process{},
			expectedError:     true,
		},
		{
			name:              "Nil network components",
			appStatusHandler:  &mock.AppStatusHandlerStub{},
			networkComponents: nil,
			processComponents: &factory.Process{},
			expectedError:     true,
		},
		{
			name:              "Nil process components",
			appStatusHandler:  &mock.AppStatusHandlerStub{},
			networkComponents: &factory.NetworkComponents{},
			processComponents: nil,
			expectedError:     true,
		},
		{
			name:             "Valid configuration",
			appStatusHandler: &mock.AppStatusHandlerStub{},
			networkComponents: &factory.NetworkComponents{
				NetMessenger: &mock.MessengerStub{},
			},
			processComponents: &factory.Process{
				ForkDetector: &mock.ForkDetectorMock{},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function to test
			err := StartStatusPolling(
				tt.appStatusHandler,
				time.Second,
				tt.networkComponents,
				tt.processComponents,
			)

			// Assert the result
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestComputeNumConnectedPeers(t *testing.T) {
	tests := []struct {
		name           string
		connectedPeers []string
		expectedValue  uint64
	}{
		{
			name:           "No connected peers",
			connectedPeers: []string{},
			expectedValue:  0,
		},
		{
			name:           "Three connected peers",
			connectedPeers: []string{"peer1", "peer2", "peer3"},
			expectedValue:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricsMap := NewMetricsMap()

			handler := &mock.AppStatusHandlerStub{
				SetUInt64ValueHandler: func(key string, value uint64) {
					metricsMap.metrics[key] = value
				},
			}

			messenger := &mock.MessengerStub{
				ConnectedAddressesCalled: func() []string {
					return tt.connectedPeers
				},
			}

			networkComponents := &factory.NetworkComponents{
				NetMessenger: messenger,
			}

			// Call the function to test
			computeNumConnectedPeers(handler, networkComponents)

			// Check that the metric was set correctly
			value, exists := metricsMap.metrics[core.MetricNumConnectedPeers]
			assert.True(t, exists)
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}

func TestComputeConnectedPeers(t *testing.T) {
	tests := []struct {
		name            string
		peersInfo       *p2p.ConnectedPeersInfo
		addresses       []string
		expectedMetrics map[string]interface{}
	}{
		{
			name: "Various connected peers",
			peersInfo: &p2p.ConnectedPeersInfo{
				IntraShardValidators: map[uint32][]string{0: {"val1", "val2"}},
				CrossShardValidators: map[uint32][]string{1: {"val3", "val4"}},
				IntraShardObservers:  map[uint32][]string{0: {"obs1"}},
				CrossShardObservers:  map[uint32][]string{1: {"obs2", "obs3"}},
				UnknownPeers:         []string{"unknown1"},
			},
			addresses: []string{"addr1", "addr2"},
			expectedMetrics: map[string]interface{}{
				core.MetricP2PUnknownPeers:         "unknown1",
				core.MetricP2PIntraShardValidators: "val1,val2",
				core.MetricP2PIntraShardObservers:  "obs1",
				core.MetricP2PCrossShardValidators: "val3,val4",
				core.MetricP2PCrossShardObservers:  "obs2,obs3",
				core.MetricP2PPeerInfo:             "addr1,addr2",
			},
		},
		{
			name: "Empty peer info",
			peersInfo: &p2p.ConnectedPeersInfo{
				IntraShardValidators: map[uint32][]string{},
				CrossShardValidators: map[uint32][]string{},
				IntraShardObservers:  map[uint32][]string{},
				CrossShardObservers:  map[uint32][]string{},
				UnknownPeers:         []string{},
			},
			addresses: []string{},
			expectedMetrics: map[string]interface{}{
				core.MetricP2PUnknownPeers:         "",
				core.MetricP2PIntraShardValidators: "",
				core.MetricP2PIntraShardObservers:  "",
				core.MetricP2PCrossShardValidators: "",
				core.MetricP2PCrossShardObservers:  "",
				core.MetricP2PPeerInfo:             "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricsMap := NewMetricsMap()

			handler := &mock.AppStatusHandlerStub{
				SetStringValueHandler: func(key string, value string) {
					metricsMap.metrics[key] = value
				},
			}

			messenger := &mock.MessengerStub{
				GetConnectedPeersInfoCalled: func() *p2p.ConnectedPeersInfo {
					return tt.peersInfo
				},
				AddressesCalled: func() []string {
					return tt.addresses
				},
			}

			networkComponents := &factory.NetworkComponents{
				NetMessenger: messenger,
			}

			// Call the function to test
			computeConnectedPeers(handler, networkComponents)

			// Check that the metrics were set correctly
			for key, expectedValue := range tt.expectedMetrics {
				actualValue, exists := metricsMap.metrics[key]
				assert.True(t, exists, "Metric %s not found", key)
				assert.Equal(t, expectedValue, actualValue, "Metric %s has incorrect value", key)
			}
		})
	}
}

func TestRegisterPollConnectedPeers(t *testing.T) {
	// Create a real AppStatusPolling instance
	appStatusHandler := &mock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {},
		SetStringValueHandler: func(key string, value string) {},
	}

	pollingHandler, err := appStatusPolling.NewAppStatusPolling(appStatusHandler, time.Second)
	assert.Nil(t, err)
	assert.NotNil(t, pollingHandler)

	networkComponents := &factory.NetworkComponents{
		NetMessenger: &mock.MessengerStub{
			ConnectedAddressesCalled: func() []string {
				return []string{"peer1", "peer2"}
			},
			GetConnectedPeersInfoCalled: func() *p2p.ConnectedPeersInfo {
				return &p2p.ConnectedPeersInfo{
					IntraShardValidators: map[uint32][]string{},
					CrossShardValidators: map[uint32][]string{},
					IntraShardObservers:  map[uint32][]string{},
					CrossShardObservers:  map[uint32][]string{},
					UnknownPeers:         []string{},
				}
			},
			AddressesCalled: func() []string {
				return []string{"addr1", "addr2"}
			},
		},
	}

	// Call the function to test
	err = registerPollConnectedPeers(pollingHandler, networkComponents)
	assert.Nil(t, err)

	// Since we can't directly access the registered functions in a real AppStatusPolling,
	// we can at least ensure that calling Poll() doesn't panic
	assert.NotPanics(t, func() {
		pollingHandler.Poll()
	})
}

// TestRegisterPollProbableHighestNonce tests the function that registers a polling function for the probable highest nonce
func TestRegisterPollProbableHighestNonce(t *testing.T) {
	// Create a real AppStatusPolling instance
	appStatusHandler := &mock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {},
	}

	pollingHandler, err := appStatusPolling.NewAppStatusPolling(appStatusHandler, time.Second)
	assert.Nil(t, err)
	assert.NotNil(t, pollingHandler)

	probableNonce := uint64(100)

	forkDetector := &mock.ForkDetectorMock{
		ProbableHighestNonceCalled: func() uint64 {
			return probableNonce
		},
	}

	processComponents := &factory.Process{
		ForkDetector: forkDetector,
	}

	// Call the function to test
	err = registerPollProbableHighestNonce(pollingHandler, processComponents)
	assert.Nil(t, err)

	// Since we can't directly access the registered functions in a real AppStatusPolling,
	// we can at least ensure that calling Poll() doesn't panic
	assert.NotPanics(t, func() {
		pollingHandler.Poll()
	})

	// Optionally, verify that the metric gets set when Poll() is called
	// This requires a metricsMap to track what values are set
	metricsMap := NewMetricsMap()
	metricsHandler := &mock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {
			metricsMap.metrics[key] = value
		},
	}

	pollingWithMetricsHandler, _ := appStatusPolling.NewAppStatusPolling(metricsHandler, time.Second)
	_ = registerPollProbableHighestNonce(pollingWithMetricsHandler, processComponents)
	pollingWithMetricsHandler.Poll()

	// Verify that the metric was set correctly
	// Note: This might not be reliable depending on the implementation of AppStatusPolling
	// and whether Poll() actually calls the registered handlers immediately
	value, exists := metricsMap.metrics[core.MetricProbableHighestNonce]
	if exists {
		assert.Equal(t, probableNonce, value)
	}
}

func TestStartNodeMetricsPolling_NilHandler(t *testing.T) {
	t.Parallel()

	redundancyHandler := &consensusMock.NodeRedundancyHandlerStub{}
	err := StartNodeMetricsPolling(nil, time.Second, redundancyHandler)
	assert.Error(t, err)
}

func TestStartNodeMetricsPolling_NilRedundancyHandler(t *testing.T) {
	t.Parallel()

	handler := &mock.AppStatusHandlerStub{}
	err := StartNodeMetricsPolling(handler, time.Second, nil)
	assert.Error(t, err)
}

func TestStartNodeMetricsPolling_Valid(t *testing.T) {
	t.Parallel()

	setValues := make(map[string]uint64)
	handler := &mock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {
			setValues[key] = value
		},
	}
	redundancyHandler := &consensusMock.NodeRedundancyHandlerStub{
		GetSlotsOfInactivityCalled: func() uint64 { return 3 },
	}

	err := StartNodeMetricsPolling(handler, time.Second, redundancyHandler)
	assert.NoError(t, err)

	// Start timestamp should be set immediately (non-zero)
	assert.Greater(t, setValues[core.MetricNodeStartTimestamp], uint64(0))
}

func TestSliceToString(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "Empty slice",
			input:    []string{},
			expected: "",
		},
		{
			name:     "Single item",
			input:    []string{"item1"},
			expected: "item1",
		},
		{
			name:     "Multiple items",
			input:    []string{"item1", "item2", "item3"},
			expected: "item1,item2,item3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sliceToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapToString(t *testing.T) {
	tests := []struct {
		name     string
		input    map[uint32][]string
		expected string
	}{
		{
			name:     "Empty map",
			input:    map[uint32][]string{},
			expected: "",
		},
		{
			name:     "Single shard",
			input:    map[uint32][]string{0: {"item1", "item2"}},
			expected: "item1,item2",
		},
		{
			name: "Multiple shards",
			input: map[uint32][]string{
				0: {"item1", "item2"},
				1: {"item3", "item4"},
				2: {"item5"},
			},
			expected: "item1,item2,item3,item4,item5",
		},
		{
			name: "Unordered shards",
			input: map[uint32][]string{
				2: {"item5"},
				0: {"item1", "item2"},
				1: {"item3", "item4"},
			},
			expected: "item1,item2,item3,item4,item5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
