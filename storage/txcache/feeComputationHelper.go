package txcache

type feeHelper interface {
	// TODO:
	IsInterfaceNil() bool
}

type feeComputationHelper struct {
	// TODO:
}

func newFeeComputationHelper() *feeComputationHelper {
	feeComputeHelper := &feeComputationHelper{}
	return feeComputeHelper
}

// IsInterfaceNil returns nil if the underlying object is nil
func (fch *feeComputationHelper) IsInterfaceNil() bool {
	return fch == nil
}
