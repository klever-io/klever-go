package parsing_test

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/genesis"
	"github.com/klever-io/klever-go/genesis/data"
	"github.com/klever-io/klever-go/genesis/parsing"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createMockInitialAccount() *data.InitialAccount {
	return &data.InitialAccount{
		Address: "0001",
		Balance: 1,
		Delegation: &data.DelegationData{
			Address: "0002",
			Value:   2,
		},
	}
}

func createMockHexPubkeyConverter() *mock.PubkeyConverterStub {
	return &mock.PubkeyConverterStub{
		DecodeCalled: func(humanReadable string) ([]byte, error) {
			return hex.DecodeString(humanReadable)
		},
	}
}

func createDelegatedInitialAccount(address string, delegatedBytes []byte, delegatedBalance int64) *data.InitialAccount {
	ia := &data.InitialAccount{
		Address: address,
		Balance: 0,
		Delegation: &data.DelegationData{
			Address: hex.EncodeToString(delegatedBytes),
			Value:   delegatedBalance,
		},
	}
	ia.SetAddressBytes(delegatedBytes)

	return ia
}

func TestNewAccountsParser_BadFilenameShouldErr(t *testing.T) {
	t.Parallel()

	ap, err := parsing.NewAccountsParser(
		"inexistent file",
		createMockHexPubkeyConverter(),
		&mock.KeyGeneratorStub{},
	)

	assert.True(t, check.IfNil(ap))
	assert.NotNil(t, err)
}

func TestNewAccountsParser_NilPubkeyConverterShouldErr(t *testing.T) {
	t.Parallel()

	ap, err := parsing.NewAccountsParser(
		"inexistent file",
		nil,
		&mock.KeyGeneratorStub{},
	)

	assert.True(t, check.IfNil(ap))
	assert.Equal(t, genesis.ErrNilPubkeyConverter, err)
}

func TestNewAccountsParser_NilKeyGeneratorShouldErr(t *testing.T) {
	t.Parallel()

	ap, err := parsing.NewAccountsParser(
		"inexistent file",
		createMockHexPubkeyConverter(),
		nil,
	)

	assert.True(t, check.IfNil(ap))
	assert.Equal(t, genesis.ErrNilKeyGenerator, err)
}

func TestNewAccountsParser_BadJsonShouldErr(t *testing.T) {
	t.Parallel()

	ap, err := parsing.NewAccountsParser(
		"testdata/genesis_bad.json",
		createMockHexPubkeyConverter(),
		&mock.KeyGeneratorStub{},
	)

	assert.True(t, check.IfNil(ap))
	assert.True(t, errors.Is(err, genesis.ErrInvalidAddress))
}

func TestNewAccountsParser_ShouldWork(t *testing.T) {
	t.Parallel()

	ap, err := parsing.NewAccountsParser(
		"testdata/genesis_ok.json",
		createMockHexPubkeyConverter(),
		&mock.KeyGeneratorStub{},
	)

	assert.False(t, check.IfNil(ap))
	assert.Nil(t, err)
	assert.Equal(t, 6, len(ap.InitialAccounts()))
}

//------- process

func TestAccountsParser_ProcessEmptyAddressShouldErr(t *testing.T) {
	t.Parallel()

	ap := parsing.NewTestAccountsParser(createMockHexPubkeyConverter())
	ib := createMockInitialAccount()
	ib.Address = ""
	ap.SetInitialAccounts([]*data.InitialAccount{ib})

	err := ap.Process()

	assert.True(t, errors.Is(err, genesis.ErrEmptyAddress))
}

func TestAccountsParser_ProcessInvalidAddressShouldErr(t *testing.T) {
	t.Parallel()

	ap := parsing.NewTestAccountsParser(createMockHexPubkeyConverter())
	ib := createMockInitialAccount()
	ib.Address = "invalid address"
	ap.SetInitialAccounts([]*data.InitialAccount{ib})

	err := ap.Process()

	assert.True(t, errors.Is(err, genesis.ErrInvalidAddress))
}

func TestAccountsParser_ProcessInvalidPublicKeyShouldErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("expected error")
	ap := parsing.NewTestAccountsParser(createMockHexPubkeyConverter())
	ap.SetKeyGenerator(&mock.KeyGeneratorStub{
		CheckPublicKeyValidCalled: func(b []byte) error {
			return expectedErr
		},
	})
	ib := createMockInitialAccount()
	ib.Address = "00"
	ap.SetInitialAccounts([]*data.InitialAccount{ib})

	err := ap.Process()

	assert.True(t, errors.Is(err, genesis.ErrInvalidPubKey))
}

func TestAccountsParser_ProcessEmptyDelegationAddressButWithBalanceShouldErr(t *testing.T) {
	t.Parallel()

	ap := parsing.NewTestAccountsParser(createMockHexPubkeyConverter())
	ib := createMockInitialAccount()
	ib.Delegation.Address = ""
	ap.SetInitialAccounts([]*data.InitialAccount{ib})

	err := ap.Process()

	assert.True(t, errors.Is(err, genesis.ErrEmptyDelegationAddress))
}

func TestAccountsParser_ProcessInvalidDelegationAddressShouldErr(t *testing.T) {
	t.Parallel()

	ap := parsing.NewTestAccountsParser(createMockHexPubkeyConverter())
	ib := createMockInitialAccount()
	ib.Delegation.Address = "invalid address"
	ap.SetInitialAccounts([]*data.InitialAccount{ib})

	err := ap.Process()

	assert.True(t, errors.Is(err, genesis.ErrInvalidDelegationAddress))
}

func TestAccountsParser_ProcessInvalidBalanceShouldErr(t *testing.T) {
	t.Parallel()

	ap := parsing.NewTestAccountsParser(createMockHexPubkeyConverter())
	ib := createMockInitialAccount()
	ib.Balance = -1
	ap.SetInitialAccounts([]*data.InitialAccount{ib})

	err := ap.Process()
	assert.True(t, errors.Is(err, genesis.ErrInvalidBalance))
}

func TestAccountsParser_ProcessInvalidDelegationValueShouldErr(t *testing.T) {
	t.Parallel()

	ap := parsing.NewTestAccountsParser(createMockHexPubkeyConverter())
	ib := createMockInitialAccount()
	ib.Delegation.Value = -1
	ap.SetInitialAccounts([]*data.InitialAccount{ib})

	err := ap.Process()
	assert.True(t, errors.Is(err, genesis.ErrInvalidDelegationValue))
}

func TestAccountsParser_ProcessDuplicatesShouldErr(t *testing.T) {
	t.Parallel()

	ap := parsing.NewTestAccountsParser(createMockHexPubkeyConverter())
	ib1 := createMockInitialAccount()
	ib2 := createMockInitialAccount()
	ap.SetInitialAccounts([]*data.InitialAccount{ib1, ib2})

	err := ap.Process()
	assert.True(t, errors.Is(err, genesis.ErrDuplicateAddress))
}

func TestAccountsParser_ProcessShouldWork(t *testing.T) {
	t.Parallel()

	ap := parsing.NewTestAccountsParser(createMockHexPubkeyConverter())
	ib := createMockInitialAccount()
	ap.SetInitialAccounts([]*data.InitialAccount{ib})

	err := ap.Process()
	assert.Nil(t, err)
}
func TestAccountsParser_GetInitialAccountsForDelegated(t *testing.T) {
	t.Parallel()

	addr1 := "1000"
	addr2 := "2000"
	delegatedUpon := int64(78)

	ap := parsing.NewTestAccountsParser(createMockHexPubkeyConverter())
	ib1 := createDelegatedInitialAccount("0001", []byte(addr1), delegatedUpon)
	ib2 := createDelegatedInitialAccount("0002", []byte(addr1), delegatedUpon)
	ib3 := createDelegatedInitialAccount("0003", []byte(addr2), delegatedUpon)

	ap.SetInitialAccounts([]*data.InitialAccount{ib1, ib2, ib3})

	err := ap.Process()
	require.Nil(t, err)

	list := ap.GetInitialAccountsForDelegated([]byte(addr1))
	require.Equal(t, 2, len(list))
	//order is important
	assert.Equal(t, ib1, list[0])
	assert.Equal(t, ib2, list[1])
	delegated := ap.GetTotalStakedForDelegationAddress(hex.EncodeToString([]byte(addr1)))
	assert.Equal(t, delegatedUpon*2, delegated)

	list = ap.GetInitialAccountsForDelegated([]byte(addr2))
	require.Equal(t, 1, len(list))
	assert.Equal(t, ib3, list[0])
	delegated = ap.GetTotalStakedForDelegationAddress(hex.EncodeToString([]byte(addr2)))
	assert.Equal(t, delegatedUpon, delegated)

	list = ap.GetInitialAccountsForDelegated([]byte("not delegated"))
	require.Equal(t, 0, len(list))
	delegated = ap.GetTotalStakedForDelegationAddress(hex.EncodeToString([]byte("not delegated")))
	assert.Equal(t, int64(0), delegated)
}
