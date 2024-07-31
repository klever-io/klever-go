package checking_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/genesis"
	"github.com/klever-io/klever-go/genesis/checking"
	"github.com/klever-io/klever-go/genesis/data"
	"github.com/klever-io/klever-go/genesis/mock"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func createEmptyInitialAccount() *data.InitialAccount {
	return &data.InitialAccount{
		Address: "",
		Balance: int64(0),
		Delegation: &data.DelegationData{
			Address: "",
			Value:   int64(0),
		},
	}
}

//------- NewNodesSetupChecker

func TestNewNodesSetupChecker_NilGenesisParserShouldErr(t *testing.T) {
	t.Parallel()

	nsc, err := checking.NewNodesSetupChecker(
		nil,
		int64(0),
		mock.NewPubkeyConverterMock(32),
		&mock.KeyGeneratorStub{},
	)

	assert.True(t, check.IfNil(nsc))
	assert.Equal(t, genesis.ErrNilAccountsParser, err)
}

func TestNewNodesSetupChecker_InvalidInitialNodePriceShouldErr(t *testing.T) {
	t.Parallel()

	nsc, err := checking.NewNodesSetupChecker(
		&mock.AccountsParserStub{},
		int64(-1),
		mock.NewPubkeyConverterMock(32),
		&mock.KeyGeneratorStub{},
	)

	assert.True(t, check.IfNil(nsc))
	assert.True(t, errors.Is(err, genesis.ErrInvalidInitialNodePrice))
}

func TestNewNodesSetupChecker_NilValidatorPubkeyConverterShouldErr(t *testing.T) {
	t.Parallel()

	nsc, err := checking.NewNodesSetupChecker(
		&mock.AccountsParserStub{},
		int64(0),
		nil,
		&mock.KeyGeneratorStub{},
	)

	assert.True(t, check.IfNil(nsc))
	assert.Equal(t, genesis.ErrNilPubkeyConverter, err)
}

func TestNewNodesSetupChecker_NilKeyGeneratorShouldErr(t *testing.T) {
	t.Parallel()

	nsc, err := checking.NewNodesSetupChecker(
		&mock.AccountsParserStub{},
		int64(0),
		mock.NewPubkeyConverterMock(32),
		nil,
	)

	assert.True(t, check.IfNil(nsc))
	assert.Equal(t, genesis.ErrNilKeyGenerator, err)
}

func TestNewNodesSetupChecker_ShouldWork(t *testing.T) {
	t.Parallel()

	nsc, err := checking.NewNodesSetupChecker(
		&mock.AccountsParserStub{},
		int64(0),
		mock.NewPubkeyConverterMock(32),
		&mock.KeyGeneratorStub{},
	)

	assert.False(t, check.IfNil(nsc))
	assert.Nil(t, err)
}

//------- Check

func TestNewNodesSetupChecker_CheckNotAValidPubkeyShouldErr(t *testing.T) {
	t.Parallel()

	ia := createEmptyInitialAccount()
	ia.SetAddressBytes([]byte("staked address"))

	expectedErr := errors.New("expected error")
	nsc, _ := checking.NewNodesSetupChecker(
		&mock.AccountsParserStub{
			InitialAccountsCalled: func() []genesis.InitialAccountHandler {
				return []genesis.InitialAccountHandler{ia}
			},
		},
		int64(0),
		mock.NewPubkeyConverterMock(32),
		&mock.KeyGeneratorStub{
			CheckPublicKeyValidCalled: func(b []byte) error {
				return expectedErr
			},
		},
	)

	err := nsc.Check(
		[]sharding.GenesisNodeInfoHandler{
			&mock.GenesisNodeInfoHandlerMock{
				AssignedShardValue: 0,
				AddressBytesValue:  []byte("staked address"),
				PubKeyBytesValue:   []byte("pubkey"),
			},
		},
	)

	assert.True(t, errors.Is(err, genesis.ErrInvalidPubKey))
}
