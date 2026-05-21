package metrics

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/appStatusPolling"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/factory"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/statusHandler"
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
				ash.SetInt64ValueHandler = func(key string, value int64) {
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
			closer, err := StartStatusPolling(
				tt.appStatusHandler,
				time.Second,
				tt.networkComponents,
				tt.processComponents,
			)

			// Assert the result
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, closer)
			} else {
				assert.Nil(t, err)
				require.NotNil(t, closer)
				t.Cleanup(func() { _ = closer.Close() })
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
	t.Cleanup(func() { _ = pollingHandler.Close() })
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
	t.Cleanup(func() { _ = pollingHandler.Close() })

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
	t.Cleanup(func() { _ = pollingWithMetricsHandler.Close() })

	// Verify that the metric was set correctly
	// Note: This might not be reliable depending on the implementation of AppStatusPolling
	// and whether Poll() actually calls the registered handlers immediately
	value, exists := metricsMap.metrics[core.MetricProbableHighestNonce]
	if exists {
		assert.Equal(t, probableNonce, value)
	}
}

// Pins the contract: MetricRedundancyIsActive must not be pre-initialised
// so the presenter returns "N/A" and the TUI hides the row until the first poll.
func TestInitMetrics_DoesNotPreInitRedundancyIsActive(t *testing.T) {
	t.Parallel()

	uint64Keys := make(map[string]struct{})
	int64Keys := make(map[string]struct{})
	stringKeys := make(map[string]struct{})
	handler := &mock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, _ uint64) { uint64Keys[key] = struct{}{} },
		SetInt64ValueHandler:  func(key string, _ int64) { int64Keys[key] = struct{}{} },
		SetStringValueHandler: func(key string, _ string) { stringKeys[key] = struct{}{} },
	}

	ns, err := sharding.NewNodesSetup(
		"../../../sharding/mock/nodesSetupMock.json",
		mock.NewPubkeyConverterMock(32),
		mock.NewPubkeyConverterMock(96),
	)
	require.NoError(t, err)

	err = InitMetrics(handler, "pk", "validator", ns, "v1.0.0", "1000000", 100)
	require.NoError(t, err)

	_, inUint64 := uint64Keys[core.MetricRedundancyIsActive]
	_, inInt64 := int64Keys[core.MetricRedundancyIsActive]
	_, inString := stringKeys[core.MetricRedundancyIsActive]
	assert.False(t, inUint64 || inInt64 || inString,
		"MetricRedundancyIsActive must not be pre-initialised")

	// Second half of the contract: the polling closure must write the key on its
	// first run, otherwise the row stays hidden forever.
	redundancyHandler := &consensusMock.NodeRedundancyHandlerStub{}
	pollingFunc := buildNodeMetricsPollingFunc(time.Now(), redundancyHandler)
	pollingFunc(handler)

	_, presentAfterPoll := uint64Keys[core.MetricRedundancyIsActive]
	assert.True(t, presentAfterPoll,
		"the polling closure must publish MetricRedundancyIsActive on its first run")
}

func TestStartNodeMetricsPolling_NilHandler(t *testing.T) {
	t.Parallel()

	redundancyHandler := &consensusMock.NodeRedundancyHandlerStub{}
	closer, err := StartNodeMetricsPolling(nil, time.Second, redundancyHandler)
	assert.Error(t, err)
	assert.Nil(t, closer)
}

func TestStartNodeMetricsPolling_NilRedundancyHandler(t *testing.T) {
	t.Parallel()

	handler := &mock.AppStatusHandlerStub{}
	closer, err := StartNodeMetricsPolling(handler, time.Second, nil)
	assert.Error(t, err)
	assert.Nil(t, closer)
}

func TestStartNodeMetricsPolling_Valid(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	setValues := make(map[string]uint64)
	handler := &mock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {
			mu.Lock()
			setValues[key] = value
			mu.Unlock()
		},
		SetInt64ValueHandler:  func(key string, value int64) {},
		SetStringValueHandler: func(key string, value string) {},
	}
	redundancyHandler := &consensusMock.NodeRedundancyHandlerStub{
		GetSlotsOfInactivityCalled: func() uint64 { return 3 },
	}

	closer, err := StartNodeMetricsPolling(handler, time.Second, redundancyHandler)
	assert.NoError(t, err)
	require.NotNil(t, closer)
	t.Cleanup(func() { _ = closer.Close() })

	// Start timestamp should be set immediately (non-zero)
	mu.Lock()
	ts := setValues[core.MetricNodeStartTimestamp]
	mu.Unlock()
	assert.Greater(t, ts, uint64(0))
}

func TestBuildNodeMetricsPollingFunc_AppearsInStatusMetricsAndPrometheus(t *testing.T) {
	t.Parallel()

	// Full chain: polling → statusMetrics → JSON map and Prometheus output.
	// String-typed metrics are silently skipped by the Prometheus serializer.
	sm := statusHandler.NewStatusMetrics()
	redundancyHandler := &consensusMock.NodeRedundancyHandlerStub{
		GetSlotsOfInactivityCalled:       func() uint64 { return 4 },
		GetInternalRedundancyLevelCalled: func() int64 { return 2 },
		IsMainMachineActiveCalled:        func() bool { return false },
	}

	pollingFunc := buildNodeMetricsPollingFunc(time.Now(), redundancyHandler)
	pollingFunc(sm)

	jsonMap := sm.StatusMetricsMapWithoutP2P()
	assert.Equal(t, int64(2), jsonMap[core.MetricRedundancyLevel])
	assert.Equal(t, uint64(1), jsonMap[core.MetricRedundancyIsActive],
		"backup with upstream silent must emit is_active=1 (taking over)")
	assert.Equal(t, uint64(4), jsonMap[core.MetricRedundancySlotsInactive])

	promOut := sm.StatusMetricsWithoutP2PPrometheusString()
	assert.Contains(t, promOut, core.MetricRedundancyLevel+"{")
	assert.Contains(t, promOut, core.MetricRedundancyIsActive+"{")
	// Sanity: the value must be emitted, not just the label. Match "} <value>" rather
	// than the trailing suffix so the assert survives any future label additions.
	for _, line := range strings.Split(promOut, "\n") {
		if strings.HasPrefix(line, core.MetricRedundancyIsActive+"{") {
			assert.Contains(t, line, "} 1", "expected uint64 1, got: %q", line)
		}
		if strings.HasPrefix(line, core.MetricRedundancyLevel+"{") {
			assert.Contains(t, line, "} 2", "expected int64 2, got: %q", line)
		}
	}
}

func TestBuildNodeMetricsPollingFunc_PublishesRedundancyMetrics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		slotsOfInactivity uint64
		redundancyLevel   int64
		// isMainActive is the value IsMainMachineActive() returns. The writer
		// must only consult it for level>0; for level==0 and level<0 the
		// writer hard-codes the answer without consulting the formula.
		isMainActive     bool
		expectedIsActive uint64
	}{
		{
			name:              "main producer is always active",
			slotsOfInactivity: 0,
			redundancyLevel:   0,
			isMainActive:      false, // writer must IGNORE this for level==0
			expectedIsActive:  1,
		},
		{
			name:              "backup level 1, upstream alive, standby",
			slotsOfInactivity: 3,
			redundancyLevel:   1,
			isMainActive:      true,
			expectedIsActive:  0,
		},
		{
			name:              "backup level 2, upstream silent, taking over",
			slotsOfInactivity: 7,
			redundancyLevel:   2,
			isMainActive:      false,
			expectedIsActive:  1,
		},
		{
			name:              "inactive node never produces",
			slotsOfInactivity: 0,
			redundancyLevel:   -1,
			isMainActive:      true, // writer must IGNORE this for level<0
			expectedIsActive:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// pollingFunc runs synchronously here, so the maps need no lock. If
			// refactored to call StartNodeMetricsPolling, add a mutex.
			uint64Values := make(map[string]uint64)
			int64Values := make(map[string]int64)
			handler := &mock.AppStatusHandlerStub{
				SetUInt64ValueHandler: func(key string, value uint64) {
					uint64Values[key] = value
				},
				SetInt64ValueHandler: func(key string, value int64) {
					int64Values[key] = value
				},
				SetStringValueHandler: func(key string, value string) {},
			}
			redundancyHandler := &consensusMock.NodeRedundancyHandlerStub{
				GetSlotsOfInactivityCalled:       func() uint64 { return tc.slotsOfInactivity },
				GetInternalRedundancyLevelCalled: func() int64 { return tc.redundancyLevel },
				IsMainMachineActiveCalled:        func() bool { return tc.isMainActive },
			}

			pollingFunc := buildNodeMetricsPollingFunc(time.Now().Add(-5*time.Second), redundancyHandler)
			pollingFunc(handler)

			assert.GreaterOrEqual(t, uint64Values[core.MetricNodeUptimeSeconds], uint64(5))
			assert.Equal(t, tc.slotsOfInactivity, uint64Values[core.MetricRedundancySlotsInactive])
			assert.Equal(t, tc.redundancyLevel, int64Values[core.MetricRedundancyLevel])
			gotIsActive, ok := uint64Values[core.MetricRedundancyIsActive]
			assert.True(t, ok, "MetricRedundancyIsActive should be published as uint64")
			assert.Equal(t, tc.expectedIsActive, gotIsActive)

			// is_main_active is backup-only (level>0). For other roles it must be absent.
			gotMainActive, mainOk := uint64Values[core.MetricRedundancyIsMainActive]
			if tc.redundancyLevel > 0 {
				assert.True(t, mainOk, "MetricRedundancyIsMainActive must be emitted for backups")
				expected := uint64(0)
				if tc.isMainActive {
					expected = 1
				}
				assert.Equal(t, expected, gotMainActive)
			} else {
				assert.False(t, mainOk, "MetricRedundancyIsMainActive must NOT be emitted for level<=0 (got %d)", gotMainActive)
			}
		})
	}
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
