package fork

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/stretchr/testify/require"
)

func TestNewForkController(t *testing.T) {
	t.Run("nil epoch notifier returns error", func(t *testing.T) {
		fc, err := NewForkController(config.EnableEpochs{}, nil)
		require.Nil(t, fc)
		require.Equal(t, common.ErrNilEpochNotifier, err)
	})

	t.Run("valid notifier registers the handler", func(t *testing.T) {
		registered := false
		notifier := &mock.EpochNotifierStub{
			RegisterNotifyHandlerCalled: func(handler core.EpochSubscriberHandler) {
				registered = true
			},
		}

		fc, err := NewForkController(config.EnableEpochs{}, notifier)
		require.NoError(t, err)
		require.NotNil(t, fc)
		require.True(t, registered)
		require.False(t, fc.IsInterfaceNil())
	})
}

func TestForkController_FlagsToggleAtConfiguredEpoch(t *testing.T) {
	cfg := config.EnableEpochs{
		ClaimKFI:                1,
		ProcessorFlowITOPrice:   2,
		FixStakingBuckets:       3,
		KdaFpr:                  4,
		BigBucketsCompute:       5,
		FPRComputeAndKdaFeeFlow: 6,
		FixDelegationSameEpoch:  7,
		SmartContracts:          8,
		FixAuditChanges:         9,
		EpochRewardsV2:          10,
		FixAuditChangesV2:       11,
		FixAuditChangesV3:       12,
	}
	fc := &forkController{enableEpochs: cfg}

	flags := []struct {
		name  string
		epoch uint32
		get   func() bool
	}{
		{"ClaimKFI", cfg.ClaimKFI, fc.ClaimKFI},
		{"ProcessorFlowITOPrice", cfg.ProcessorFlowITOPrice, fc.ProcessorFlowITOPrice},
		{"FixStakingBuckets", cfg.FixStakingBuckets, fc.FixStakingBuckets},
		{"KdaFpr", cfg.KdaFpr, fc.KdaFpr},
		{"BigBucketsCompute", cfg.BigBucketsCompute, fc.BigBucketsCompute},
		{"FPRComputeAndKdaFeeFlow", cfg.FPRComputeAndKdaFeeFlow, fc.FPRComputeAndKdaFeeFlow},
		{"FixDelegationSameEpoch", cfg.FixDelegationSameEpoch, fc.FixDelegationSameEpoch},
		{"EnableSmartContracts", cfg.SmartContracts, fc.EnableSmartContracts},
		{"FixAuditChanges", cfg.FixAuditChanges, fc.FixAuditChanges},
		{"EpochRewardsV2", cfg.EpochRewardsV2, fc.EpochRewardsV2},
		{"FixAuditChangesV2", cfg.FixAuditChangesV2, fc.FixAuditChangesV2},
		{"FixAuditChangesV3", cfg.FixAuditChangesV3, fc.FixAuditChangesV3},
	}

	maxEpoch := uint32(0)
	for _, f := range flags {
		if f.epoch > maxEpoch {
			maxEpoch = f.epoch
		}
	}

	for epoch := uint32(0); epoch <= maxEpoch; epoch++ {
		fc.EpochConfirmed(epoch)
		for _, f := range flags {
			require.Equal(t, epoch >= f.epoch, f.get(),
				"flag %s (activation epoch %d) is wired wrong: unexpected state at confirmed epoch %d", f.name, f.epoch, epoch)
		}
	}
}
