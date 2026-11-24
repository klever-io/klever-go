package node

import (
	"context"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/kapp"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/stretchr/testify/require"
)

// accountsTripwire and kappControllerTripwire stand in for block processing's objects. They embed
// nil interfaces, so any method a REST read calls on them panics.
type accountsTripwire struct{ state.AccountsAdapter }
type kappControllerTripwire struct{ kapp.KAppController }

// Every REST read uses the node's REST state only: block processing's accounts, KApps and KApp
// controller (the processing* fields) are reserved for the bootstrapper, transaction admission,
// pending nonces, Merkle proofs and the proposal controller at startup (KLC-2877).
func TestNode_RESTReadsNeverReachProcessingState(t *testing.T) {
	t.Parallel()

	address := make([]byte, 32)
	address[0] = 1
	conv, err := pubkeyConverter.NewBech32PubkeyConverter(len(address))
	require.NoError(t, err)
	encoded := conv.Encode(address)

	n := &Node{
		addressPubkeyConverter: conv,
		encodedAddressLength:   len(encoded),
		forkController:         mock.NewForkControllerStub(),
		internalMarshalizer:    &mock.ProtobufMarshalizerMock{},
		accounts: &mock.AccountsStub{
			GetExistingAccountCalled: func(addr []byte) (state.AccountHandler, error) {
				return state.NewUserAccount(addr)
			},
			RootHashCalled: func() ([]byte, error) { return []byte("root"), nil },
			GetAllLeavesCalled: func([]byte) (*data.TrieIteratorChannels, error) {
				return data.NewCompletedTrieIteratorChannels(), nil
			},
		},
		kapps: &mock.AccountsStub{
			LoadAccountCalled: func(addr []byte) (state.AccountHandler, error) {
				return seededKApp(t, addr)
			},
		},
		kappController: &stub.KAppControllerStub{
			GetValidatorsKAppCalled:  func() kapp.ValidatorsKapp { return &mock.ValidatorsKAppStub{} },
			GetMarketKAppCalled:      func() kapp.MarketKapp { return &marketKappEscrowStub{} },
			GetKDAFeesPoolKAppCalled: func() kapp.KDAFeesPoolKapp { return &stub.KDAFeesPoolKappStub{} },
		},
		feeHandler: &mock.FeeHandlerStub{
			ComputeTransactionCostCalled: func(_ process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
				return &transaction.CostResponse{}, nil
			},
		},
		processingAccounts:       accountsTripwire{},
		processingKApps:          accountsTripwire{},
		processingKAppController: kappControllerTripwire{},
	}

	reads := map[string]func(){
		"GetAccount": func() { _, _ = n.GetAccount(encoded) },
		"getKLVAllowanceWithPending": func() {
			user, errUser := state.NewUserAccount(address)
			require.NoError(t, errUser)
			_ = n.getKLVAllowanceWithPending(user)
		},
		"GetBalance":            func() { _, _ = n.GetBalance(encoded, "KLV") },
		"GetUserKDA":            func() { _, _ = n.GetUserKDA(encoded, "KFI") },
		"GetAvailableClaim":     func() { _, _, _, _ = n.GetAvailableClaim(encoded, "KLV") },
		"GetAvailableClaimList": func() { _, _, _ = n.GetAvailableClaimList(context.Background(), encoded, []string{"KLV"}) },
		"GetAsset":              func() { _, _ = n.GetAsset("KFI") },
		"GetNFT":                func() { _, _, _ = n.GetNFT(encoded, "NFT/1") },
		"GetKDAFeePool":         func() { _, _ = n.GetKDAFeePool("KFI") },
		"GetMarketplace":        func() { _, _ = n.GetMarketplace("id") },
		"computeAccountTotals":  func() { _, _ = n.computeAccountTotals() },
		"loadKAppHeldTotals":    func() { _, _, _ = n.loadKAppHeldTotals() },
		"loadKAppKLVPools":      func() { _, _, _, _ = n.loadKAppKLVPools() },
		"loadTotalStakedKLV":    func() { _, _ = n.loadTotalStakedKLV() },
		"computeTransactionFees": func() {
			_, _ = n.computeTransactionFees(&transaction.Transaction{}, &transaction.TXBaseInfo{KDAFee: "KFI"})
		},
	}
	for name, read := range reads {
		require.NotPanics(t, read, "%s reached block processing's state", name)
	}
}

// seededKApp returns the KApp at address holding the records the REST reads above look up, so each
// read gets past its lookups to the code that reads the KApp controller or the accounts.
func seededKApp(t *testing.T, address []byte) (state.AccountHandler, error) {
	marshalizer := &mock.ProtobufMarshalizerMock{}
	app, err := state.NewKAppAccount(address)
	require.NoError(t, err)

	save := func(key []byte, record interface{}) {
		value, errMarshal := marshalizer.Marshal(record)
		require.NoError(t, errMarshal)
		require.NoError(t, app.DataTrieTracker().SaveKeyValue(key, value))
	}
	switch string(address) {
	case string(kapps.KDAKAppAddress):
		save(kdautils.ToKDAKey([]byte("KFI"), nil), &kapps.KDAData{ID: []byte("KFI")})
		save(kdautils.ToKDAKey([]byte("NFT"), nil), &kapps.KDAData{ID: []byte("NFT"), AssetType: kapps.KDAData_NonFungible})
	case string(kapps.KDAFeesPoolKAppAddress):
		save([]byte("KFI"), &kdafeespool.KDAFeesPoolData{})
	}

	return app, nil
}
