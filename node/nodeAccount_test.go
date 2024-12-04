package node_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserKDA(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		address     string
		assetID     string
		accSetup    func() *mock.AccountsStub
		expectedKDA *kapps.UserKDA
		expectedErr error
	}{
		{
			name:    "account not found",
			address: "AABC",
			assetID: "KLV",
			accSetup: func() *mock.AccountsStub {
				accDB := &mock.AccountsStub{}
				accDB.GetExistingAccountCalled = func(address []byte) (state.AccountHandler, error) {
					return nil, common.ErrAccountNotFound
				}
				return accDB
			},
			expectedKDA: nil,
			expectedErr: common.ErrAccountNotFound,
		},
		{
			name:    "invalid account address",
			address: "AAB",
			assetID: "KLV",
			accSetup: func() *mock.AccountsStub {
				accDB := &mock.AccountsStub{}
				accDB.GetExistingAccountCalled = func(address []byte) (state.AccountHandler, error) {
					return nil, common.ErrAccountNotFound
				}
				return accDB
			},
			expectedKDA: nil,
			expectedErr: errors.New("invalid address, could not decode from: encoding/hex: odd length hex string"),
		},
		{
			name:    "successful KDA retrieval",
			address: "AABB",
			assetID: "KLV",
			accSetup: func() *mock.AccountsStub {
				accDB := &mock.AccountsStub{}
				accDB.GetExistingAccountCalled = func(address []byte) (state.AccountHandler, error) {
					acc := &mock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{
								Balance:       1000,
								FrozenBalance: 0,
							}, nil
						},
					}

					return acc, nil
				}
				return accDB
			},
			expectedKDA: &kapps.UserKDA{
				Balance:       1000,
				FrozenBalance: 0,
			},
			expectedErr: nil,
		},
		{
			name:    "non-user account type",
			address: "AABBCC",
			assetID: "KLV",
			accSetup: func() *mock.AccountsStub {
				accDB := &mock.AccountsStub{}
				accDB.GetExistingAccountCalled = func(address []byte) (state.AccountHandler, error) {
					return &mock.KAppAccountHandlerStub{}, nil
				}
				return accDB
			},
			expectedKDA: nil,
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			// t.Parallel()

			// Create node with mocks and custom accounts adapter
			n, err := createNodeWithAccountsAdapter(t, tt.accSetup())
			require.Nil(t, err)

			// Execute test
			gotKDA, err := n.GetUserKDA(tt.address, tt.assetID)

			// Verify results
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedKDA, gotKDA)
		})
	}
}
