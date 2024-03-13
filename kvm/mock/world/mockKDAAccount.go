package worldmock

// // GetTokenBalance returns the KDA balance of the account, specified by the
// // token key.
// func (a *Account) GetTokenBalance(tokenIdentifier []byte, nonce uint64) (*big.Int, error) {
// 	return kdaconvert.GetTokenBalance(tokenIdentifier, nonce, a.Storage)
// }

// // GetTokenBalanceUint64 returns the KDA balance of the account, specified by the
// // token key.
// func (a *Account) GetTokenBalanceUint64(tokenIdentifier []byte, nonce uint64) (uint64, error) {
// 	balance, err := a.GetTokenBalance(tokenIdentifier, nonce)
// 	if err != nil {
// 		return 0, err
// 	}
// 	return balance.Uint64(), nil
// }

// // SetTokenBalance sets the KDA balance of the account, specified by the token
// // key.
// func (a *Account) SetTokenBalance(tokenIdentifier []byte, nonce uint64, balance *big.Int) error {
// 	return kdaconvert.SetTokenBalance(tokenIdentifier, nonce, balance, a.Storage)
// }

// // SetTokenBalanceUint64 sets the KDA balance of the account, specified by the
// // token key.
// func (a *Account) SetTokenBalanceUint64(tokenIdentifier []byte, nonce uint64, balance uint64) error {
// 	return kdaconvert.SetTokenBalance(tokenIdentifier, nonce, big.NewInt(0).SetUint64(balance), a.Storage)
// }

// // GetTokenData gets the KDA information related to a token from the storage of the account.
// func (a *Account) GetTokenData(tokenIdentifier []byte, nonce uint64, systemAccStorage map[string][]byte) (*kda.KDigitalToken, error) {
// 	return kdaconvert.GetTokenData(tokenIdentifier, nonce, a.Storage, systemAccStorage)
// }

// // SetTokenData sets the KDA information related to a token into the storage of the account.
// func (a *Account) SetTokenData(tokenIdentifier []byte, nonce uint64, tokenData *kda.KDigitalToken) error {
// 	return kdaconvert.SetTokenData(tokenIdentifier, nonce, tokenData, a.Storage)
// }

// // SetTokenRolesAsStrings sets the specified roles to the account, corresponding to the given tokenName.
// func (a *Account) SetTokenRolesAsStrings(tokenIdentifier []byte, rolesAsStrings []string) error {
// 	return kdaconvert.SetTokenRolesAsStrings(tokenIdentifier, rolesAsStrings, a.Storage)
// }
