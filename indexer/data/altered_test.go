package data

import (
	"testing"

	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/require"
)

func TestAlteredAccounts_Add(t *testing.T) {
	t.Parallel()

	altAccounts := NewAlteredAccounts()

	addr := "my-addr"
	acct1 := &AlteredAccount{
		BalanceChange: true,
	}
	altAccounts.Add(addr, acct1)

	acct2 := &AlteredAccount{
		IsSender: true,
	}
	altAccounts.Add(addr, acct2)

	res, ok := altAccounts.Get(addr)
	require.True(t, ok)
	require.Equal(t, 1, len(res))
	require.True(t, res[0].IsSender)
	require.True(t, res[0].BalanceChange)
}

func TestAlteredAccounts_AddTokenAddFunds(t *testing.T) {
	t.Parallel()

	altAccounts := NewAlteredAccounts()

	addr := "my-addr"
	acct1 := &AlteredAccount{
		BalanceChange: true,
	}
	altAccounts.Add(addr, acct1)

	acct2 := &AlteredAccount{
		IsKDAOperation:  true,
		TokenIdentifier: "my-token",
	}
	altAccounts.Add(addr, acct2)

	res, ok := altAccounts.Get(addr)
	require.True(t, ok)
	require.Equal(t, 1, len(res))
	require.True(t, res[0].BalanceChange)
}

func TestAlteredAccounts_AddKDA(t *testing.T) {
	t.Parallel()

	altAccounts := NewAlteredAccounts()

	acct1 := &AlteredAccount{
		BalanceChange: true,
	}
	addr := "my-addr"
	altAccounts.Add(addr, acct1)

	acct2 := &AlteredAccount{
		TokenIdentifier: "my-token",
		IsKDAOperation:  true,
		NFTNonce:        "0",
	}
	altAccounts.Add(addr, acct2)

	acct3 := &AlteredAccount{
		IsSender:        true,
		TokenIdentifier: "my-token",
		IsKDAOperation:  true,
		NFTNonce:        "0",
	}
	altAccounts.Add(addr, acct3)

	acct4 := &AlteredAccount{
		IsSender:        true,
		TokenIdentifier: "my-nft-token",
		IsNFTOperation:  true,
		NFTNonce:        "1",
		Type:            kapps.KDAData_NonFungible.String(),
	}
	altAccounts.Add(addr, acct4)

	acct5 := &AlteredAccount{
		IsSender:        true,
		TokenIdentifier: "my-nft-token",
		IsNFTOperation:  true,
		NFTNonce:        "1",
		Type:            kapps.KDAData_NonFungible.String(),
	}
	altAccounts.Add(addr, acct5)

	acct6 := &AlteredAccount{
		IsSender:        true,
		TokenIdentifier: "my-nft-token",
		IsNFTOperation:  true,
		NFTNonce:        "2",
		Type:            kapps.KDAData_NonFungible.String(),
	}
	altAccounts.Add(addr, acct6)

	require.Equal(t, 1, altAccounts.Len())
	res, ok := altAccounts.Get(addr)
	require.True(t, ok)
	require.Equal(t, 3, len(res))
	require.Equal(t, &AlteredAccount{
		BalanceChange:   true,
		IsSender:        true,
		IsKDAOperation:  true,
		TokenIdentifier: "my-token",
		NFTNonce:        "0",
	}, res[0])

	require.Equal(t, &AlteredAccount{
		IsNFTOperation:  true,
		TokenIdentifier: "my-nft-token",
		NFTNonce:        "1",
		Type:            kapps.KDAData_NonFungible.String(),
	}, res[1])

	require.Equal(t, &AlteredAccount{
		IsNFTOperation:  true,
		TokenIdentifier: "my-nft-token",
		NFTNonce:        "2",
		Type:            kapps.KDAData_NonFungible.String(),
	}, res[2])
}

func TestAlteredAccounts_GetAll(t *testing.T) {
	t.Parallel()

	altAccounts := &alteredAccounts{}

	res := altAccounts.GetAll()
	require.NotNil(t, res)

	altAccounts = NewAlteredAccounts()
	res = altAccounts.GetAll()
	require.NotNil(t, res)
}
