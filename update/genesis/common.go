package genesis

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/marshal"
)

func getValidatorDataFromLeaves(
	leavesChannel chan data.KeyValueHolder,
	marshalizer marshal.Marshalizer,
) ([]*state.ValidatorInfo, error) {

	validators := make([]*state.ValidatorInfo, 0)

	for pa := range leavesChannel {
		peer, err := unmarshalPeer(pa.Value(), marshalizer)
		if err != nil {
			return nil, err
		}

		if peer.GetRevoked() {
			continue
		}

		validatorInfoData := &state.ValidatorInfo{
			OwnerAddress:                    peer.GetOwnerAddress(),
			PublicKey:                       peer.GetBLSPublicKey(),
			List:                            peer.GetListString(),
			TempRating:                      peer.GetTempRating(),
			Rating:                          peer.GetRating(),
			RatingModifier:                  0,
			LeaderSuccess:                   peer.GetLeaderSuccessRateSuccess(),
			LeaderFailure:                   peer.GetLeaderSuccessRateFailure(),
			ValidatorSuccess:                peer.GetValidatorSuccessRateSuccess(),
			ValidatorFailure:                peer.GetValidatorSuccessRateFailure(),
			ValidatorIgnoredSignatures:      peer.GetValidatorIgnoredSignaturesRate(),
			TotalLeaderSuccess:              peer.GetTotalLeaderSuccessRateSuccess(),
			TotalLeaderFailure:              peer.GetTotalLeaderSuccessRateFailure(),
			TotalValidatorSuccess:           peer.GetTotalValidatorSuccessRateSuccess(),
			TotalValidatorFailure:           peer.GetTotalValidatorSuccessRateFailure(),
			TotalValidatorIgnoredSignatures: peer.GetTotalValidatorIgnoredSignaturesRate(),
			NumSelectedInSuccessBlocks:      peer.GetNumSelectedInSuccessBlocks(),
			IsPubKeyRevoked:                 peer.GetRevoked(),
		}

		validators = append(validators, validatorInfoData)
	}

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
