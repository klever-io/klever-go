package fork

import (
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/tools/atomic"
	"github.com/klever-io/klever-go/tools/check"
)

var log = logger.GetOrCreate("core/fork")

type forkController struct {
	enableEpochs                     config.EnableEpochs
	flagClaimKFIEnabled              atomic.Flag
	flagProcessorFlowITOPriceEnabled atomic.Flag
	flagFixStakingBuckets            atomic.Flag
	flagKdaFpr                       atomic.Flag
	flagBigBucketsCompute            atomic.Flag
	flagFPRComputeAndKdaFeeFlow      atomic.Flag
	flagFixDelegationSameEpoch       atomic.Flag
	flagEnableSmartContracts         atomic.Flag
}

func NewForkController(cfg config.EnableEpochs, epochNotifier process.EpochNotifier) (*forkController, error) {

	if check.IfNil(epochNotifier) {
		return nil, common.ErrNilEpochNotifier
	}

	fc := &forkController{enableEpochs: cfg}

	epochNotifier.RegisterNotifyHandler(fc)

	return fc, nil
}

func (f *forkController) ProcessorFlowITOPrice() bool {
	return f.flagProcessorFlowITOPriceEnabled.IsSet()
}

func (f *forkController) ClaimKFI() bool {
	return f.flagClaimKFIEnabled.IsSet()
}

func (f *forkController) FixStakingBuckets() bool {
	return f.flagFixStakingBuckets.IsSet()
}

func (f *forkController) KdaFpr() bool {
	return f.flagKdaFpr.IsSet()
}

func (f *forkController) BigBucketsCompute() bool {
	return f.flagBigBucketsCompute.IsSet()
}

func (f *forkController) FPRComputeAndKdaFeeFlow() bool {
	return f.flagFPRComputeAndKdaFeeFlow.IsSet()
}

func (f *forkController) FixDelegationSameEpoch() bool {
	return f.flagFixDelegationSameEpoch.IsSet()
}

func (f *forkController) EnableSmartContracts() bool {
	return f.flagEnableSmartContracts.IsSet()
}

// EpochConfirmed is called whenever a new epoch is confirmed
func (f *forkController) EpochConfirmed(epoch uint32) {
	f.flagClaimKFIEnabled.Toggle(epoch >= f.enableEpochs.ClaimKFI)
	log.Debug("forkController: ClaimKFI", "enabled", f.flagClaimKFIEnabled.IsSet())

	f.flagProcessorFlowITOPriceEnabled.Toggle(epoch >= f.enableEpochs.ProcessorFlowITOPrice)
	log.Debug("forkController: ProcessorFlowITOPrice", "enabled", f.flagProcessorFlowITOPriceEnabled.IsSet())

	f.flagFixStakingBuckets.Toggle(epoch >= f.enableEpochs.FixStakingBuckets)
	log.Debug("forkController: FixStakingBuckets", "enabled", f.flagFixStakingBuckets.IsSet())

	f.flagKdaFpr.Toggle(epoch >= f.enableEpochs.KdaFpr)
	log.Debug("forkController: KDAFPR", "enabled", f.flagKdaFpr.IsSet())

	f.flagBigBucketsCompute.Toggle(epoch >= f.enableEpochs.BigBucketsCompute)
	log.Debug("forkController: BigBucketsCompute", "enabled", f.flagBigBucketsCompute.IsSet())

	f.flagFPRComputeAndKdaFeeFlow.Toggle(epoch >= f.enableEpochs.FPRComputeAndKdaFeeFlow)
	log.Debug("forkController: FPRComputeAndKdaFeeFlow", "enabled", f.flagFPRComputeAndKdaFeeFlow.IsSet())

	f.flagFixDelegationSameEpoch.Toggle(epoch >= f.enableEpochs.FixDelegationSameEpoch)
	log.Debug("forkController: FixDelegationSameEpoch", "enabled", f.flagFixDelegationSameEpoch.IsSet())

	f.flagEnableSmartContracts.Toggle(epoch >= f.enableEpochs.SmartContracts)
	log.Debug("forkController: EnableSmartContracts", "enabled", f.flagEnableSmartContracts.IsSet())
}

// IsInterfaceNil returns true if there is no value under the interface
func (f *forkController) IsInterfaceNil() bool {
	return f == nil
}
