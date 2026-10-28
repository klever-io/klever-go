package fallback

import (
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var log = logger.GetOrCreate("fallback")

type fallbackHeaderValidator struct {
	headersPool    retriever.HeadersPool
	marshalizer    marshal.Marshalizer
	storageService retriever.StorageService
	forkController core.ForkController
}

// NewFallbackHeaderValidator creates a new fallbackHeaderValidator object
func NewFallbackHeaderValidator(
	headersPool retriever.HeadersPool,
	marshalizer marshal.Marshalizer,
	storageService retriever.StorageService,
	forkController core.ForkController,
) (*fallbackHeaderValidator, error) {

	if check.IfNil(headersPool) {
		return nil, common.ErrNilHeadersDataPool
	}
	if check.IfNil(marshalizer) {
		return nil, process.ErrNilMarshalizer
	}
	if check.IfNil(storageService) {
		return nil, process.ErrNilStorage
	}
	if check.IfNil(forkController) {
		return nil, common.ErrNilForkController
	}

	hv := &fallbackHeaderValidator{
		headersPool:    headersPool,
		marshalizer:    marshalizer,
		storageService: storageService,
		forkController: forkController,
	}

	return hv, nil
}

// ShouldApplyFallbackValidation returns if for the given header could be applied fallback validation or not
func (fhv *fallbackHeaderValidator) ShouldApplyFallbackValidation(headerHandler data.HeaderHandler) bool {
	if check.IfNil(headerHandler) {
		return false
	}

	if !headerHandler.GetIsEpochStart() {
		return false
	}

	// From FixAuditChangesV7 epoch-start headers also need the normal PBFT quorum (KLR-53):
	// two n/2+1 certificates need not share an honest signer.
	if fhv.forkController.FixAuditChangesV7InEpoch(headerHandler.GetEpoch()) {
		return false
	}

	previousHeader, err := process.GetHeader(headerHandler.GetParentHash(), fhv.headersPool, fhv.marshalizer, fhv.storageService)
	if err != nil {
		log.Debug("ShouldApplyFallbackValidation", "GetMetaHeader", err.Error())
		return false
	}

	isRoundTooOld := tools.SafeU64ToI64(headerHandler.GetSlot())-tools.SafeU64ToI64(previousHeader.GetSlot()) >= core.MaxSlotsWithoutCommittedStartInEpochBlock
	return isRoundTooOld
}

// IsInterfaceNil returns true if there is no value under the interface
func (fhv *fallbackHeaderValidator) IsInterfaceNil() bool {
	return fhv == nil
}
