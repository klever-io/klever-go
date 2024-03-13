package genesis

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/marshal"
)

// TODO: create a structure or use this function also in process/peer/process.go
func getValidatorDataFromLeaves(
	leavesChannel chan data.KeyValueHolder,
	marshalizer marshal.Marshalizer,
) ([]*state.ValidatorInfo, error) {

	validators := make([]*state.ValidatorInfo, 0)

	return validators, nil
}

func unmarshalPeer(pa []byte, marshalizer marshal.Marshalizer) (state.PeerAccountHandler, error) {
	peerAccount := state.NewEmptyPeerAccount()
	err := marshalizer.Unmarshal(peerAccount, pa)
	if err != nil {
		return nil, err
	}
	return peerAccount, nil
}

func shouldExportValidator(validator *state.ValidatorInfo, allowedLists []core.PeerType) bool {
	validatorList := validator.GetList()

	for _, list := range allowedLists {
		if validatorList == string(list) {
			return true
		}
	}

	return false
}
