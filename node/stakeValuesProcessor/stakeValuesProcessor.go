package stakeValuesProcessor

import (
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

type stakedValuesProc struct {
	marshalizer marshal.Marshalizer
	accounts    state.AccountsAdapter
}

// NewTotalStakedValueProcessor will create a new instance of stakedValuesProc
func NewTotalStakedValueProcessor(
	nodePrice string,
	marshalizer marshal.Marshalizer,
	accounts state.AccountsAdapter,
) (*stakedValuesProc, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(accounts) {
		return nil, ErrNilAccountsAdapter
	}

	return &stakedValuesProc{
		marshalizer: marshalizer,
		accounts:    accounts,
	}, nil
}

// GetTotalStakedValue will calculate total staked value if needed and return calculated value
func (svp *stakedValuesProc) GetTotalStakedValue() (int64, error) {
	totalStaked, err := svp.computeStakedValueAndTopUp()
	return totalStaked, err
}

func (svp *stakedValuesProc) computeStakedValueAndTopUp() (int64, error) {
	return 0, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (svp *stakedValuesProc) IsInterfaceNil() bool {
	return svp == nil
}
