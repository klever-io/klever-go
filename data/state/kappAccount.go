//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. kappAccountData.proto
package state

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ KAppAccountHandler = (*kappAccount)(nil)

// Account is the struct used in serialization/deserialization
type kappAccount struct {
	*baseAccount
	KAppAccountData
}

// NewEmptyKAppAccount creates new simple account wrapper for an AccountContainer (that has just been initialized)
func NewEmptyKAppAccount() *kappAccount {
	return &kappAccount{
		baseAccount:     &baseAccount{},
		KAppAccountData: KAppAccountData{},
	}
}

// NewKAppAccount creates new simple account wrapper for an AccountContainer (that has just been initialized)
func NewKAppAccount(address []byte) (*kappAccount, error) {
	if len(address) == 0 {
		return nil, common.ErrNilAddress
	}

	return &kappAccount{
		baseAccount: &baseAccount{
			address:         address,
			dataTrieTracker: NewTrackableDataTrie(address, nil),
		},
		KAppAccountData: KAppAccountData{
			Address: address,
		},
	}, nil
}

// SetName sets the users name
func (a *kappAccount) SetName(name []byte) {
	a.Name = make([]byte, 0, len(name))
	a.Name = append(a.Name, name...)
}

// SetRootHash sets the root hash associated with the account
func (a *kappAccount) SetRootHash(roothash []byte) {
	a.RootHash = roothash
}

// GetNonce -
func (a *kappAccount) GetNonce() uint64 {
	return 0
}

// IncreaseNonce -
func (a *kappAccount) IncreaseNonce(_ uint64) {
}

func (a *kappAccount) GetStorage(key []byte) []byte {
	data, err := a.dataTrieTracker.RetrieveValue(key)
	if err != nil {
		return nil
	}
	return data
}

// SetStorage saves the key value storage under the address
func (a *kappAccount) SetStorage(key []byte, value []byte) error {
	return a.dataTrieTracker.SaveKeyValue(key, value)
}

// StartProposalsKApp initializes or loads the proposal controller for the KApp account
func (a *kappAccount) StartProposalsKApp(forks core.ForkController) (kapps.ActiveProposalController, error) {
	marshalizer := marshal.NewProtoMarshalizer()

	initialProposal, err := kapps.NewProposalController(forks)
	if err != nil {
		return nil, err
	}

	controllerBytes, err := a.dataTrieTracker.RetrieveValue(kdautils.ProposalControllerKey)
	if err != nil || len(controllerBytes) == 0 {
		proposalData, errMarshal := marshalizer.Marshal(initialProposal.ProposalController)
		if errMarshal != nil {
			return nil, errMarshal
		}

		errSave := a.dataTrieTracker.SaveKeyValue(kdautils.ProposalControllerKey, proposalData)
		if errSave != nil {
			return nil, errSave
		}

		return initialProposal, nil
	}

	current := &kapps.ProposalController{}
	if err = marshalizer.Unmarshal(current, controllerBytes); err != nil {
		return nil, err
	}

	if len(initialProposal.ActiveParameters) > len(current.ActiveParameters) {
		if err = a.mergeAndSaveParameters(marshalizer, initialProposal.ActiveParameters, current); err != nil {
			return nil, err
		}
	}

	initialProposal.ProposalController = current
	return initialProposal, nil
}

func (a *kappAccount) mergeAndSaveParameters(marshalizer marshal.Marshalizer, initialParams map[int32]*kapps.Parameter, current *kapps.ProposalController) error {
	for key, v := range current.ActiveParameters {
		initialParams[key] = v
	}
	current.ActiveParameters = initialParams

	proposalData, err := marshalizer.Marshal(current)
	if err != nil {
		return err
	}
	return a.dataTrieTracker.SaveKeyValue(kdautils.ProposalControllerKey, proposalData)
}

// AddInternalKDA adds new internal KDA to kapp
func (a *kappAccount) AddInternalKDA(assetID []byte, internalID []byte, data []byte) error {
	if len(data) == 0 {
		return common.ErrAssetNotFound
	}

	key := kdautils.ToKDAKey(assetID, internalID)

	return a.dataTrieTracker.SaveKeyValue(key, data)
}

// SubInternalKDA subtracts new internal KDA from kapp
func (a *kappAccount) SubInternalKDA(assetID []byte, internalID []byte) ([]byte, error) {
	key := kdautils.ToKDAKey(assetID, internalID)

	data, err := a.dataTrieTracker.RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, common.ErrAssetNotFound
	}

	err = a.dataTrieTracker.SaveKeyValue(key, nil)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// GetUserKDA returns the unmarshalled kda data from kapp for the given key
func (a *kappAccount) GetUserKDA(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
	kdaData := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{},
		Buckets:   make(map[string]*kapps.UserBucket),
	}

	if check.IfNil(a.dataTrieTracker.DataTrie()) &&
		(!checkDirtData || len(a.dataTrieTracker.DirtyData()) == 0) {
		return kdaData, nil
	}

	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	// check assetID for NFT
	key := kdautils.ToKDAKey(assetID, nonce)

	value, err := a.dataTrieTracker.RetrieveValue(key)
	if err != nil && err != common.ErrNilTrie {
		return nil, err
	}
	if len(value) == 0 {
		return kdaData, nil
	}

	marshalizer := marshal.NewProtoMarshalizer()

	err = marshalizer.Unmarshal(kdaData, value)
	if err != nil {
		return nil, err
	}

	return kdaData, nil
}
