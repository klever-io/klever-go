package facade_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/facade"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/stretchr/testify/require"
)

// createMockArgNodeFacade creates a minimal valid ArgNodeFacade for testing
func createMockArgNodeFacade() facade.ArgNodeFacade {
	return facade.ArgNodeFacade{
		Node:        &mock.NodeHandlerStub{},
		APIResolver: &mock.APIResolverStub{},
		WsAntifloodConfig: config.WebServerAntifloodConfig{
			SimultaneousRequests:         100,
			SameSourceRequests:           10,
			SameSourceResetIntervalInSec: 1,
			EndpointsThrottlers:          []config.EndpointsThrottlersConfig{},
		},
		FacadeConfig: config.FacadeConfig{
			RestAPIInterface: "localhost:8080",
		},
		APIRoutesConfig: config.APIRoutesConfig{
			APIPackages: map[string]config.APIPackageConfig{
				"node": {
					Routes: []config.RouteConfig{
						{Name: "status", Open: true},
					},
				},
			},
		},
		AccountsState: &mock.AccountsStub{},
		KAppsState:    &mock.AccountsStub{},
		PeerState:     &mock.AccountsStub{},
	}
}

func TestNodeFacade_WSLimitGetters(t *testing.T) {
	t.Parallel()

	args := createMockArgNodeFacade()
	args.WsAntifloodConfig.WebSocketConnections = 1234
	args.WsAntifloodConfig.WebSocketConnectionsPerIP = 56
	args.WsAntifloodConfig.WebSocketMaxAddressesPerSubscribe = 78
	args.WsAntifloodConfig.WebSocketMaxAddressesPerClient = 9012

	nf, err := facade.NewNodeFacade(args)
	require.NoError(t, err)

	require.Equal(t, uint32(1234), nf.WSMaxConnections())
	require.Equal(t, uint32(56), nf.WSMaxConnectionsPerIP())
	require.Equal(t, uint32(78), nf.WSMaxAddressesPerSubscribe())
	require.Equal(t, uint32(9012), nf.WSMaxAddressesPerClient())
}

func TestNewNodeFacade(t *testing.T) {
	t.Parallel()

	t.Run("NilNode", func(t *testing.T) {
		args := createMockArgNodeFacade()
		args.Node = nil

		nf, err := facade.NewNodeFacade(args)
		require.Equal(t, common.ErrNilNode, err)
		require.Nil(t, nf)
	})

	t.Run("NilAPIResolver", func(t *testing.T) {
		args := createMockArgNodeFacade()
		args.APIResolver = nil

		nf, err := facade.NewNodeFacade(args)
		require.Equal(t, common.ErrNilAPIResolver, err)
		require.Nil(t, nf)
	})

	t.Run("NoAPIRoutesConfig", func(t *testing.T) {
		args := createMockArgNodeFacade()
		args.APIRoutesConfig.APIPackages = map[string]config.APIPackageConfig{}

		nf, err := facade.NewNodeFacade(args)
		require.Equal(t, common.ErrNoAPIRoutesConfig, err)
		require.Nil(t, nf)
	})

	t.Run("ZeroSimultaneousRequests", func(t *testing.T) {
		args := createMockArgNodeFacade()
		args.WsAntifloodConfig.SimultaneousRequests = 0

		nf, err := facade.NewNodeFacade(args)
		require.Error(t, err)
		require.Contains(t, err.Error(), "SimultaneousRequests should not be 0")
		require.Nil(t, nf)
	})

	t.Run("ZeroSameSourceRequests", func(t *testing.T) {
		args := createMockArgNodeFacade()
		args.WsAntifloodConfig.SameSourceRequests = 0

		nf, err := facade.NewNodeFacade(args)
		require.Error(t, err)
		require.Contains(t, err.Error(), "SameSourceRequests should not be 0")
		require.Nil(t, nf)
	})

	t.Run("ZeroSameSourceResetIntervalInSec", func(t *testing.T) {
		args := createMockArgNodeFacade()
		args.WsAntifloodConfig.SameSourceResetIntervalInSec = 0

		nf, err := facade.NewNodeFacade(args)
		require.Error(t, err)
		require.Contains(t, err.Error(), "SameSourceResetIntervalInSec should not be 0")
		require.Nil(t, nf)
	})

	t.Run("NilAccountsState", func(t *testing.T) {
		args := createMockArgNodeFacade()
		args.AccountsState = nil

		nf, err := facade.NewNodeFacade(args)
		require.Equal(t, common.ErrNilAccountState, err)
		require.Nil(t, nf)
	})

	t.Run("NilKAppsState", func(t *testing.T) {
		args := createMockArgNodeFacade()
		args.KAppsState = nil

		nf, err := facade.NewNodeFacade(args)
		require.Equal(t, common.ErrNilKappState, err)
		require.Nil(t, nf)
	})

	t.Run("NilPeerState", func(t *testing.T) {
		args := createMockArgNodeFacade()
		args.PeerState = nil

		nf, err := facade.NewNodeFacade(args)
		require.Equal(t, common.ErrNilPeerState, err)
		require.Nil(t, nf)
	})

	t.Run("Success", func(t *testing.T) {
		args := createMockArgNodeFacade()

		nf, err := facade.NewNodeFacade(args)
		require.NoError(t, err)
		require.NotNil(t, nf)
		require.False(t, nf.IsInterfaceNil())
	})

	t.Run("Success_WithThrottlers", func(t *testing.T) {
		args := createMockArgNodeFacade()
		args.WsAntifloodConfig.EndpointsThrottlers = []config.EndpointsThrottlersConfig{
			{
				Endpoint:         "/transaction/send",
				MaxNumGoRoutines: 10,
			},
		}

		nf, err := facade.NewNodeFacade(args)
		require.NoError(t, err)
		require.NotNil(t, nf)
		throttler, found := nf.GetThrottlerForEndpoint("/transaction/send")
		require.True(t, found)
		require.NotNil(t, throttler)
	})
}

func TestGetNodeOverview(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		// Create real StatusMetrics instance and populate it with correct metric keys
		statusMetrics := statusHandler.NewStatusMetrics()
		statusMetrics.SetStringValue(core.MetricChainID, "mainnet")
		statusMetrics.SetInt64Value(core.MetricSlotAtEpochStart, 200)
		statusMetrics.SetInt64Value(core.MetricSlotsPerEpoch, 150)
		statusMetrics.SetInt64Value(core.MetricCurrentSlot, 250)
		statusMetrics.SetInt64Value(core.MetricSlotInterval, 6)
		statusMetrics.SetInt64Value(core.MetricCurrentSlotTimestamp, 1234567890)
		statusMetrics.SetInt64Value(core.MetricStartTime, 1234567800)
		statusMetrics.SetInt64Value(core.MetricEpochNumber, 10)
		statusMetrics.SetInt64Value(core.MetricNonceAtEpochStart, 950)
		statusMetrics.SetInt64Value(core.MetricNonce, 1000)

		args := createMockArgNodeFacade()
		args.APIResolver = &mock.APIResolverStub{
			StatusMetricsCalled: func() core.StatusMetricsHandler {
				return statusMetrics
			},
		}

		nf, err := facade.NewNodeFacade(args)
		require.NoError(t, err)

		overview, err := nf.GetNodeOverview()
		require.NoError(t, err)

		// Verify all fields are correctly populated
		require.Equal(t, "mainnet", overview.ChainID)
		require.Equal(t, int64(250), overview.BaseTxSize) // core.BaseTxSize constant value
		require.Equal(t, int64(200), overview.SlotAtEpochStart)
		require.Equal(t, int64(150), overview.SlotsPerEpoch)
		require.Equal(t, int64(250), overview.CurrentSlot)
		require.Equal(t, int64(6), overview.SlotDuration)
		require.Equal(t, int64(1234567890), overview.SlotCurrentTimestamp)
		require.Equal(t, int64(1234567800), overview.StartTime)
		require.Equal(t, int64(10), overview.EpochNumber)
		require.Equal(t, int64(950), overview.NonceAtEpochStart)
		require.Equal(t, int64(1000), overview.Nonce)
	})

	t.Run("MixedTypes", func(t *testing.T) {
		// Test with mixed types to verify type conversion using real StatusMetrics
		statusMetrics := statusHandler.NewStatusMetrics()
		statusMetrics.SetStringValue(core.MetricChainID, "testnet")
		// Set values with different numeric types to test conversion
		statusMetrics.SetUInt64Value(core.MetricCurrentSlot, 250)
		statusMetrics.SetInt64Value(core.MetricEpochNumber, 10)
		statusMetrics.SetInt64Value(core.MetricNonce, 1000)
		statusMetrics.SetInt64Value(core.MetricSlotInterval, 6)

		args := createMockArgNodeFacade()
		args.APIResolver = &mock.APIResolverStub{
			StatusMetricsCalled: func() core.StatusMetricsHandler {
				return statusMetrics
			},
		}

		nf, err := facade.NewNodeFacade(args)
		require.NoError(t, err)

		overview, err := nf.GetNodeOverview()
		require.NoError(t, err)

		require.Equal(t, "testnet", overview.ChainID)
		require.Equal(t, int64(core.BaseTxSize), overview.BaseTxSize) // core.BaseTxSize constant
		require.Equal(t, int64(10), overview.EpochNumber)
		require.Equal(t, int64(1000), overview.Nonce)
		require.Equal(t, int64(250), overview.CurrentSlot)
		require.Equal(t, int64(6), overview.SlotDuration)
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Parallel()

	t.Run("GetStringValue", func(t *testing.T) {
		require.Equal(t, "test-string", facade.GetStringValue("test-string"))
		require.Equal(t, "", facade.GetStringValue(123))
		require.Equal(t, "", facade.GetStringValue(true))
		require.Equal(t, "", facade.GetStringValue(nil))
		require.Equal(t, "", facade.GetStringValue([]byte("bytes")))
	})

	t.Run("GetInt64Value", func(t *testing.T) {
		require.Equal(t, int64(12345), facade.GetInt64Value(int64(12345)))
		require.Equal(t, int64(54321), facade.GetInt64Value(uint64(54321)))
		require.Equal(t, int64(999), facade.GetInt64Value(int(999)))
		require.Equal(t, int64(888), facade.GetInt64Value(uint(888)))
		require.Equal(t, int64(0), facade.GetInt64Value("string"))
		require.Equal(t, int64(0), facade.GetInt64Value(true))
		require.Equal(t, int64(0), facade.GetInt64Value(nil))
		require.Equal(t, int64(0), facade.GetInt64Value(12.34))
	})

	t.Run("ComputeEndpointsNumGoRoutinesThrottlers_EmptyConfig", func(t *testing.T) {
		config := config.WebServerAntifloodConfig{
			EndpointsThrottlers: []config.EndpointsThrottlersConfig{},
		}

		throttlers := facade.ComputeEndpointsNumGoRoutinesThrottlers(config)
		require.NotNil(t, throttlers)
		require.Empty(t, throttlers)
	})

	t.Run("ComputeEndpointsNumGoRoutinesThrottlers_ValidThrottlers", func(t *testing.T) {
		config := config.WebServerAntifloodConfig{
			EndpointsThrottlers: []config.EndpointsThrottlersConfig{
				{
					Endpoint:         "/transaction/send",
					MaxNumGoRoutines: 10,
				},
				{
					Endpoint:         "/account/:address",
					MaxNumGoRoutines: 20,
				},
			},
		}

		throttlers := facade.ComputeEndpointsNumGoRoutinesThrottlers(config)
		require.NotNil(t, throttlers)
		require.Len(t, throttlers, 2)
		require.Contains(t, throttlers, "/transaction/send")
		require.Contains(t, throttlers, "/account/:address")
	})

	t.Run("ComputeEndpointsNumGoRoutinesThrottlers_InvalidThrottler", func(t *testing.T) {
		config := config.WebServerAntifloodConfig{
			EndpointsThrottlers: []config.EndpointsThrottlersConfig{
				{
					Endpoint:         "/invalid",
					MaxNumGoRoutines: 0, // Invalid - should be skipped
				},
			},
		}

		throttlers := facade.ComputeEndpointsNumGoRoutinesThrottlers(config)
		require.NotNil(t, throttlers)
		require.Empty(t, throttlers) // Invalid throttler should be skipped
	})
}
