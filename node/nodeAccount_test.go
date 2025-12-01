package node_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// headerHandlerWithTimestamp extends HeaderHandlerStub to support GetTimestamp
type headerHandlerWithTimestamp struct {
	mock.HeaderHandlerStub
	timestamp int64
}

func (h *headerHandlerWithTimestamp) GetTimestamp() int64 {
	return h.timestamp
}

// Test helpers for reducing boilerplate
func createBasicUserAcc() *mock.UserAccountHandlerStub {
	return &mock.UserAccountHandlerStub{
		AddressBytesCalled: func() []byte { return []byte{0xAA, 0xBB} },
	}
}

func createAccDBWithUser(userAcc state.AccountHandler) *mock.AccountsStub {
	return &mock.AccountsStub{
		GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
			return userAcc, nil
		},
	}
}

func createKappsDBWithStakingAndKDA(stakingBytes, kdaBytes []byte) *mock.AccountsStub {
	callCount := 0
	return &mock.AccountsStub{
		LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
			callCount++
			returnBytes := stakingBytes
			if callCount > 1 {
				returnBytes = kdaBytes
			}
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return returnBytes, nil
						},
					}
				},
			}, nil
		},
	}
}

func createBlockchainWithHeader(epoch uint32, timestamp int64) *mock.BlockChainMock {
	return &mock.BlockChainMock{
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
			return &headerHandlerWithTimestamp{
				HeaderHandlerStub: mock.HeaderHandlerStub{EpochField: epoch},
				timestamp:         timestamp,
			}
		},
	}
}

func createKappsDBWithLoadError(err error) *mock.AccountsStub {
	return &mock.AccountsStub{
		LoadAccountCalled: func([]byte) (state.AccountHandler, error) { return nil, err },
	}
}

func createKappsDBWithTrieValue(retriever func([]byte) ([]byte, error)) *mock.AccountsStub {
	return &mock.AccountsStub{
		LoadAccountCalled: func([]byte) (state.AccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{RetrieveValueCalled: retriever}
				},
			}, nil
		},
	}
}

// createKappsDBSequential returns different results for sequential LoadAccount calls
func createKappsDBSequential(first, second func() (state.AccountHandler, error)) *mock.AccountsStub {
	callCount := 0
	return &mock.AccountsStub{
		LoadAccountCalled: func([]byte) (state.AccountHandler, error) {
			callCount++
			if callCount == 1 {
				return first()
			}
			return second()
		},
	}
}

func kappWithTrieData(data []byte) func() (state.AccountHandler, error) {
	return func() (state.AccountHandler, error) {
		return &mock.KAppAccountHandlerStub{
			DataTrieTrackerCalled: func() state.DataTrieTracker {
				return &mock.DataTrieTrackerStub{
					RetrieveValueCalled: func([]byte) ([]byte, error) { return data, nil },
				}
			},
		}, nil
	}
}

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
				return &mock.AccountsStub{
					GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
						return nil, common.ErrAccountNotFound
					},
				}
			},
			expectedKDA: nil,
			expectedErr: common.ErrAccountNotFound,
		},
		{
			name:    "invalid account address",
			address: "AAB",
			assetID: "KLV",
			accSetup: func() *mock.AccountsStub {
				return &mock.AccountsStub{
					GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
						return nil, common.ErrAccountNotFound
					},
				}
			},
			expectedKDA: nil,
			expectedErr: errors.New("invalid address, could not decode from: encoding/hex: odd length hex string"),
		},
		{
			name:    "successful KDA retrieval",
			address: "AABB",
			assetID: "KLV",
			accSetup: func() *mock.AccountsStub {
				return &mock.AccountsStub{
					GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
						return &mock.UserAccountHandlerStub{
							GetUserKDACalled: func(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
								return &kapps.UserKDA{Balance: 1000, FrozenBalance: 0}, nil
							},
						}, nil
					},
				}
			},
			expectedKDA: &kapps.UserKDA{Balance: 1000, FrozenBalance: 0},
			expectedErr: nil,
		},
		{
			name:    "non-user account type",
			address: "AABBCC",
			assetID: "KLV",
			accSetup: func() *mock.AccountsStub {
				return &mock.AccountsStub{
					GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
						return &mock.KAppAccountHandlerStub{}, nil
					},
				}
			},
			expectedKDA: nil,
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := createNodeWithAccountsAdapter(t, tt.accSetup())
			require.Nil(t, err)

			gotKDA, err := n.GetUserKDA(tt.address, tt.assetID)

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

func TestGetAccount(t *testing.T) {
	t.Parallel()

	t.Run("adds pending rewards to allowance when available", func(t *testing.T) {
		allowanceAfterAdd := int64(1000)
		userAcc := &mock.UserAccountHandlerStub{
			AddressBytesCalled:   func() []byte { return []byte{0xAA, 0xBB} },
			GetAllowanceCalled:   func() int64 { return allowanceAfterAdd },
			AddToAllowanceCalled: func(value int64) error { allowanceAfterAdd += value; return nil },
		}

		kappController := &stub.KAppControllerStub{
			GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
				return &mock.ValidatorsKAppStub{
					GetPendingRewardsCalled: func(address []byte) (int64, error) { return 500, nil },
				}
			},
		}

		n, err := createNodeWithKAppController(t, createAccDBWithUser(userAcc), kappController)
		require.NoError(t, err)

		account, err := n.GetAccount("AABB")
		require.NoError(t, err)
		require.Equal(t, int64(1500), account.GetAllowance())
	})

	t.Run("returns account even when GetPendingRewards fails", func(t *testing.T) {
		userAcc := &mock.UserAccountHandlerStub{
			AddressBytesCalled: func() []byte { return []byte{0xAA, 0xBB} },
			GetAllowanceCalled: func() int64 { return 1000 },
		}

		kappController := &stub.KAppControllerStub{
			GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
				return &mock.ValidatorsKAppStub{
					GetPendingRewardsCalled: func(address []byte) (int64, error) { return 0, errors.New("some error") },
				}
			},
		}

		n, err := createNodeWithKAppController(t, createAccDBWithUser(userAcc), kappController)
		require.NoError(t, err)

		account, err := n.GetAccount("AABB")
		require.NoError(t, err)
		require.Equal(t, int64(1000), account.GetAllowance())
	})

	t.Run("does not add allowance when pending rewards is zero", func(t *testing.T) {
		addToAllowanceCalled := false
		userAcc := &mock.UserAccountHandlerStub{
			AddressBytesCalled:   func() []byte { return []byte{0xAA, 0xBB} },
			GetAllowanceCalled:   func() int64 { return 1000 },
			AddToAllowanceCalled: func(value int64) error { addToAllowanceCalled = true; return nil },
		}

		kappController := &stub.KAppControllerStub{
			GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
				return &mock.ValidatorsKAppStub{
					GetPendingRewardsCalled: func(address []byte) (int64, error) { return 0, nil },
				}
			},
		}

		n, err := createNodeWithKAppController(t, createAccDBWithUser(userAcc), kappController)
		require.NoError(t, err)

		_, err = n.GetAccount("AABB")
		require.NoError(t, err)
		require.False(t, addToAllowanceCalled, "AddToAllowance should not be called when pending rewards is 0")
	})
}

func TestGetAvailableClaim(t *testing.T) {
	t.Parallel()

	// Table-driven tests for loadStakingData error cases
	loadStakingDataErrorTests := []struct {
		name        string
		kappsDB     *mock.AccountsStub
		expectedErr string
	}{
		{"staking account not found", createKappsDBWithLoadError(common.ErrAccountNotFound), common.ErrAccountNotFound.Error()},
		{"wrong type assertion", &mock.AccountsStub{LoadAccountCalled: func([]byte) (state.AccountHandler, error) { return &mock.UserAccountHandlerStub{}, nil }}, common.ErrWrongTypeAssertion.Error()},
		{"staking not found (empty data)", createKappsDBWithTrieValue(func([]byte) ([]byte, error) { return []byte{}, nil }), common.ErrStakingNotFound.Error()},
		{"retrieve value error", createKappsDBWithTrieValue(func([]byte) ([]byte, error) { return nil, errors.New("trie error") }), "trie error"},
	}

	for _, tt := range loadStakingDataErrorTests {
		t.Run("loadStakingData fails - "+tt.name, func(t *testing.T) {
			t.Parallel()

			n, err := createNodeWithOptions(t, nodeTestOptions{
				accAdapter:   createAccDBWithUser(createBasicUserAcc()),
				kappsAdapter: tt.kappsDB,
				blockchain:   &mock.BlockChainMock{},
			})
			require.NoError(t, err)

			_, _, _, err = n.GetAvailableClaim("AABB", "KLV")
			require.EqualError(t, err, tt.expectedErr)
		})
	}

	t.Run("loadKDAData fails", func(t *testing.T) {
		t.Parallel()

		stakingBytes, _ := (&mock.ProtoMarshalizerMock{}).Marshal(&kapps.StakingData{TotalStaked: 1000})
		kappsDB := createKappsDBSequential(
			kappWithTrieData(stakingBytes),
			func() (state.AccountHandler, error) { return nil, common.ErrAccountNotFound },
		)

		n, err := createNodeWithOptions(t, nodeTestOptions{
			accAdapter:   createAccDBWithUser(createBasicUserAcc()),
			kappsAdapter: kappsDB,
			blockchain:   &mock.BlockChainMock{},
		})
		require.NoError(t, err)

		_, _, _, err = n.GetAvailableClaim("AABB", "KLV")
		require.ErrorIs(t, err, common.ErrAccountNotFound)
	})

	t.Run("returns zero values for non-user account", func(t *testing.T) {
		t.Parallel()

		n, err := createNodeWithOptions(t, nodeTestOptions{
			accAdapter:   createAccDBWithUser(&mock.KAppAccountHandlerStub{}), // non-user account type
			kappsAdapter: &mock.AccountsStub{},
			blockchain:   &mock.BlockChainMock{},
		})
		require.NoError(t, err)

		rewards, rewardsMap, allowance, err := n.GetAvailableClaim("AABB", "KLV")
		require.NoError(t, err)
		require.Zero(t, rewards)
		require.Nil(t, rewardsMap)
		require.Zero(t, allowance)
	})

	// Success tests - shared marshaler for data setup
	marshalizer := &mock.ProtoMarshalizerMock{}
	stakingBytes, _ := marshalizer.Marshal(&kapps.StakingData{TotalStaked: 1000})

	successTests := []struct {
		name            string
		assetID         string
		rewardsMap      map[string]int64
		expectedRewards int64
		pendingRewards  int64
		wantAllowance   int64
	}{
		{"KLV with pending rewards", "KLV", map[string]int64{"KLV": 100}, 100, 500, 1500},
		{"non-KLV asset zero allowance", "MYTOKEN", map[string]int64{"MYTOKEN": 200}, 200, 0, 0},
	}

	for _, tt := range successTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rewardsMap := tt.rewardsMap // capture for closure
			userAcc := &mock.UserAccountHandlerStub{
				AddressBytesCalled: func() []byte { return []byte{0xAA, 0xBB} },
				GetAllowanceCalled: func() int64 { return 1000 },
				GetUserKDACalled:   func([]byte, []byte, bool) (*kapps.UserKDA, error) { return &kapps.UserKDA{}, nil },
				ComputeAvailableClaimCalled: func([]byte, uint32, int64, *kapps.UserKDA, *kapps.StakingData, core.ForkController) (map[string]int64, error) {
					return rewardsMap, nil
				},
			}

			kdaBytes, _ := marshalizer.Marshal(&kapps.KDAData{Name: []byte(tt.assetID)})
			var kappCtrl kapp.KAppController
			if tt.pendingRewards > 0 {
				pending := tt.pendingRewards // capture
				kappCtrl = &stub.KAppControllerStub{
					GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
						return &mock.ValidatorsKAppStub{
							GetPendingRewardsCalled: func([]byte) (int64, error) { return pending, nil },
						}
					},
				}
			}

			n, err := createNodeWithOptions(t, nodeTestOptions{
				accAdapter:     createAccDBWithUser(userAcc),
				kappsAdapter:   createKappsDBWithStakingAndKDA(stakingBytes, kdaBytes),
				kappController: kappCtrl,
				blockchain:     createBlockchainWithHeader(10, 1234567890),
			})
			require.NoError(t, err)

			rewards, rewardsMap, allowance, err := n.GetAvailableClaim("AABB", tt.assetID)
			require.NoError(t, err)
			require.Equal(t, tt.expectedRewards, rewards)
			require.Equal(t, tt.rewardsMap, rewardsMap)
			require.Equal(t, tt.wantAllowance, allowance)
		})
	}
}
