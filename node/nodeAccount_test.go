package node_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/node"
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

	t.Run("returns new account when ErrAccNotFound (also via wrapped error)", func(t *testing.T) {
		cases := []struct {
			name string
			err  error
		}{
			{"direct sentinel", common.ErrAccNotFound},
			{"wrapped sentinel", fmt.Errorf("lookup: %w", common.ErrAccNotFound)},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				accDB := &mock.AccountsStub{
					GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
						return nil, c.err
					},
				}
				n, err := createNodeWithKAppController(t, accDB, &stub.KAppControllerStub{})
				require.NoError(t, err)

				account, err := n.GetAccount("AABB")
				require.NoError(t, err)
				require.NotNil(t, account)
			})
		}
	})

	t.Run("wraps non-sentinel errors with %w", func(t *testing.T) {
		underlying := errors.New("disk failure")
		accDB := &mock.AccountsStub{
			GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
				return nil, underlying
			},
		}
		n, err := createNodeWithKAppController(t, accDB, &stub.KAppControllerStub{})
		require.NoError(t, err)

		account, err := n.GetAccount("AABB")
		require.Nil(t, account)
		require.Error(t, err)
		require.True(t, errors.Is(err, underlying), "underlying error must remain in the chain")
	})
}

func TestGetNextNonce_AccountNotFound(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"direct sentinel", common.ErrAccNotFound},
		{"wrapped sentinel", fmt.Errorf("lookup: %w", common.ErrAccNotFound)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			accDB := &mock.AccountsStub{
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					return nil, c.err
				},
			}
			n, err := createNodeWithKAppController(t, accDB, &stub.KAppControllerStub{})
			require.NoError(t, err)

			nonce, firstPending, numPending, err := n.GetNextNonce("AABB")
			require.NoError(t, err)
			require.Zero(t, nonce)
			require.Zero(t, firstPending)
			require.Zero(t, numPending)
		})
	}
}

func TestGetNextNonce_WrapsNonSentinelError(t *testing.T) {
	t.Parallel()

	underlying := errors.New("disk failure")
	accDB := &mock.AccountsStub{
		GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
			return nil, underlying
		},
	}
	n, err := createNodeWithKAppController(t, accDB, &stub.KAppControllerStub{})
	require.NoError(t, err)

	_, _, _, err = n.GetNextNonce("AABB")
	require.Error(t, err)
	require.True(t, errors.Is(err, underlying), "underlying error must remain in the chain")
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

	t.Run("returns error when there is no current block header", func(t *testing.T) {
		t.Parallel()

		userAcc := &mock.UserAccountHandlerStub{
			AddressBytesCalled: func() []byte { return []byte{0xAA, 0xBB} },
			ComputeAvailableClaimCalled: func([]byte, uint32, int64, *kapps.UserKDA, *kapps.StakingData, core.ForkController) (map[string]int64, error) {
				require.Fail(t, "ComputeAvailableClaim must not be called without a current block header")
				return nil, nil
			},
		}

		kdaBytes, _ := marshalizer.Marshal(&kapps.KDAData{Name: []byte("KLV")})

		n, err := createNodeWithOptions(t, nodeTestOptions{
			accAdapter:   createAccDBWithUser(userAcc),
			kappsAdapter: createKappsDBWithStakingAndKDA(stakingBytes, kdaBytes),
			blockchain: &mock.BlockChainMock{
				GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return nil },
			},
		})
		require.NoError(t, err)

		rewards, rewardsMap, allowance, err := n.GetAvailableClaim("AABB", "KLV")
		require.ErrorIs(t, err, common.ErrNilHeader)
		require.Zero(t, rewards)
		require.Nil(t, rewardsMap)
		require.Zero(t, allowance)
	})

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

	aprSupplyTests := []struct {
		name            string
		computed        map[string]int64
		computeErr      error
		kda             *kapps.KDAData
		expectedRewards int64
	}{
		{"gain overflowing int64 reports nothing", nil, common.ErrInt64Overflow, &kapps.KDAData{Name: []byte("MYTOKEN")}, 0},
		{"negative supply reports nothing", map[string]int64{"MYTOKEN": 200}, nil, &kapps.KDAData{CirculatingSupply: -1}, 0},
		{"uncapped supply overflow reports nothing", map[string]int64{"MYTOKEN": 200}, nil, &kapps.KDAData{CirculatingSupply: 1<<63 - 100}, 0},
		{"gain above the cap reports nothing", map[string]int64{"MYTOKEN": 200}, nil, &kapps.KDAData{MaxSupply: 1000, CirculatingSupply: 1000}, 0},
		{"gain within the cap is reported", map[string]int64{"MYTOKEN": 200}, nil, &kapps.KDAData{MaxSupply: 1000, CirculatingSupply: 800}, 200},
	}

	for _, tt := range aprSupplyTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			computed, computeErr := tt.computed, tt.computeErr
			userAcc := &mock.UserAccountHandlerStub{
				AddressBytesCalled: func() []byte { return []byte{0xAA, 0xBB} },
				GetUserKDACalled:   func([]byte, []byte, bool) (*kapps.UserKDA, error) { return &kapps.UserKDA{}, nil },
				ComputeAvailableClaimCalled: func([]byte, uint32, int64, *kapps.UserKDA, *kapps.StakingData, core.ForkController) (map[string]int64, error) {
					return computed, computeErr
				},
			}

			kdaBytes, _ := marshalizer.Marshal(tt.kda)
			n, err := createNodeWithOptions(t, nodeTestOptions{
				accAdapter:   createAccDBWithUser(userAcc),
				kappsAdapter: createKappsDBWithStakingAndKDA(stakingBytes, kdaBytes),
				blockchain:   createBlockchainWithHeader(10, 1234567890),
			})
			require.NoError(t, err)

			rewards, _, _, err := n.GetAvailableClaim("AABB", "MYTOKEN")
			require.NoError(t, err)
			require.Equal(t, tt.expectedRewards, rewards)
		})
	}
}

func TestGetAvailableClaimList(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.ProtoMarshalizerMock{}
	stakingBytes, _ := marshalizer.Marshal(&kapps.StakingData{TotalStaked: 1000})
	kdaBytes, _ := marshalizer.Marshal(&kapps.KDAData{Name: []byte("KDA")})

	claimingUserAcc := func() *mock.UserAccountHandlerStub {
		return &mock.UserAccountHandlerStub{
			AddressBytesCalled: func() []byte { return []byte{0xAA, 0xBB} },
			GetAllowanceCalled: func() int64 { return 1000 },
			GetUserKDACalled:   func([]byte, []byte, bool) (*kapps.UserKDA, error) { return &kapps.UserKDA{}, nil },
			ComputeAvailableClaimCalled: func(assetID []byte, _ uint32, _ int64, _ *kapps.UserKDA, _ *kapps.StakingData, _ core.ForkController) (map[string]int64, error) {
				return map[string]int64{string(assetID): 100}, nil
			},
		}
	}

	t.Run("loads the shared account state once per request", func(t *testing.T) {
		t.Parallel()

		var userLoads, kappLoads, pendingRewardsReads int
		accDB := createAccDBWithUser(claimingUserAcc())
		getExisting := accDB.GetExistingAccountCalled
		accDB.GetExistingAccountCalled = func(address []byte) (state.AccountHandler, error) {
			userLoads++
			return getExisting(address)
		}
		kappsDB := createKappsDBWithStakingAndKDA(stakingBytes, kdaBytes)
		loadKapp := kappsDB.LoadAccountCalled
		kappsDB.LoadAccountCalled = func(address []byte) (state.AccountHandler, error) {
			kappLoads++
			return loadKapp(address)
		}
		// GetPendingRewards loads the Validators kapp from the kapps adapter in production, so it
		// is the fourth exclusive acquisition of a request that includes KLV.
		kappCtrl := &stub.KAppControllerStub{
			GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
				return &mock.ValidatorsKAppStub{
					GetPendingRewardsCalled: func([]byte) (int64, error) {
						pendingRewardsReads++
						return 500, nil
					},
				}
			},
		}

		n, err := createNodeWithOptions(t, nodeTestOptions{
			accAdapter:     accDB,
			kappsAdapter:   kappsDB,
			kappController: kappCtrl,
			blockchain:     createBlockchainWithHeader(10, 1234567890),
		})
		require.NoError(t, err)

		assetIds := []string{"KLV", "KFI", "AAA-1234", "BBB-1234", "CCC-1234", "DDD-1234"}
		claims, claimErrors, err := n.GetAvailableClaimList(context.Background(), "AABB", assetIds)
		require.NoError(t, err)
		require.Empty(t, claimErrors)
		require.Len(t, claims, len(assetIds))

		require.Equal(t, 1, userLoads, "user account must be loaded once per request")
		require.Equal(t, 2, kappLoads, "staking and KDA kapps must each be loaded once per request")
		require.Equal(t, 1, pendingRewardsReads, "KLV pending rewards must be read once per request")

		require.Equal(t, int64(1500), claims["KLV"].Allowance)
		require.Equal(t, int64(100), claims["KLV"].StakingRewards)
		require.Equal(t, int64(0), claims["AAA-1234"].Allowance)
		require.Equal(t, map[string]int64{"AAA-1234": 100}, claims["AAA-1234"].AllStakingRewards)
	})

	t.Run("reads the header after each asset's user data", func(t *testing.T) {
		t.Parallel()

		userKDAReads, headerReads := 0, 0
		userAcc := claimingUserAcc()
		userAcc.GetUserKDACalled = func([]byte, []byte, bool) (*kapps.UserKDA, error) {
			userKDAReads++
			return &kapps.UserKDA{}, nil
		}
		blockchain := createBlockchainWithHeader(10, 1234567890)
		getHeader := blockchain.GetCurrentBlockHeaderCalled
		blockchain.GetCurrentBlockHeaderCalled = func() data.HeaderHandler {
			headerReads++
			require.Equal(t, headerReads, userKDAReads, "header must be read after the asset's user data")
			return getHeader()
		}

		n, err := createNodeWithOptions(t, nodeTestOptions{
			accAdapter:   createAccDBWithUser(userAcc),
			kappsAdapter: createKappsDBWithStakingAndKDA(stakingBytes, kdaBytes),
			blockchain:   blockchain,
		})
		require.NoError(t, err)

		assetIds := []string{"AAA-1234", "BBB-1234", "CCC-1234"}
		_, claimErrors, err := n.GetAvailableClaimList(context.Background(), "AABB", assetIds)
		require.NoError(t, err)
		require.Empty(t, claimErrors)
		require.Equal(t, len(assetIds), headerReads)
	})

	const headerEpoch, headerTimestamp = uint32(10), int64(1234567890)
	claimInFlightTests := []struct {
		name         string
		lastClaim    *kapps.LastClaim
		wantInFlight bool
	}{
		{"claim newer than header by timestamp", &kapps.LastClaim{Timestamp: headerTimestamp + 1, Epoch: headerEpoch}, true},
		{"claim newer than header by epoch only", &kapps.LastClaim{Timestamp: headerTimestamp, Epoch: headerEpoch + 1}, true},
		{"claim in the current header", &kapps.LastClaim{Timestamp: headerTimestamp, Epoch: headerEpoch}, false},
	}

	for _, tt := range claimInFlightTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			computeCalls := 0
			userAcc := claimingUserAcc()
			userAcc.GetUserKDACalled = func([]byte, []byte, bool) (*kapps.UserKDA, error) {
				return &kapps.UserKDA{LastClaim: tt.lastClaim}, nil
			}
			userAcc.ComputeAvailableClaimCalled = func([]byte, uint32, int64, *kapps.UserKDA, *kapps.StakingData, core.ForkController) (map[string]int64, error) {
				computeCalls++
				return map[string]int64{"KLV": 100}, nil
			}

			n, err := createNodeWithOptions(t, nodeTestOptions{
				accAdapter:   createAccDBWithUser(userAcc),
				kappsAdapter: createKappsDBWithStakingAndKDA(stakingBytes, kdaBytes),
				blockchain:   createBlockchainWithHeader(headerEpoch, headerTimestamp),
			})
			require.NoError(t, err)

			claims, claimErrors, err := n.GetAvailableClaimList(context.Background(), "AABB", []string{"KLV"})
			require.NoError(t, err)
			require.Empty(t, claimErrors)
			require.Equal(t, int64(1000), claims["KLV"].Allowance, "allowance does not depend on the claim maths")

			if tt.wantInFlight {
				require.Zero(t, computeCalls, "claim maths must not run while a claim is in flight")
				require.Zero(t, claims["KLV"].StakingRewards)
				require.Nil(t, claims["KLV"].AllStakingRewards)
				return
			}
			require.Equal(t, 1, computeCalls)
			require.Equal(t, int64(100), claims["KLV"].StakingRewards)
		})
	}

	t.Run("reports per-asset failures keyed by asset", func(t *testing.T) {
		t.Parallel()

		missingKey := string(kdautils.ToKDAKey([]byte("MISSING-1234"), nil))
		loads := 0
		kappsDB := &mock.AccountsStub{
			LoadAccountCalled: func([]byte) (state.AccountHandler, error) {
				loads++
				isStaking := loads == 1
				payload := stakingBytes
				if !isStaking {
					payload = kdaBytes
				}
				return &mock.KAppAccountHandlerStub{
					DataTrieTrackerCalled: func() state.DataTrieTracker {
						return &mock.DataTrieTrackerStub{
							RetrieveValueCalled: func(key []byte) ([]byte, error) {
								if isStaking && string(key) == missingKey {
									return nil, nil
								}
								return payload, nil
							},
						}
					},
				}, nil
			},
		}

		n, err := createNodeWithOptions(t, nodeTestOptions{
			accAdapter:   createAccDBWithUser(claimingUserAcc()),
			kappsAdapter: kappsDB,
			blockchain:   createBlockchainWithHeader(10, 1234567890),
		})
		require.NoError(t, err)

		claims, claimErrors, err := n.GetAvailableClaimList(context.Background(), "AABB", []string{"KLV", "MISSING-1234"})
		require.NoError(t, err)
		require.Len(t, claimErrors, 1)
		require.ErrorIs(t, claimErrors["MISSING-1234"], common.ErrStakingNotFound)
		require.Contains(t, claims, "KLV")
		require.NotContains(t, claims, "MISSING-1234")
	})

	t.Run("returns request-level error when a shared load fails", func(t *testing.T) {
		t.Parallel()

		n, err := createNodeWithOptions(t, nodeTestOptions{
			accAdapter:   createAccDBWithUser(claimingUserAcc()),
			kappsAdapter: createKappsDBWithLoadError(common.ErrAccountNotFound),
			blockchain:   createBlockchainWithHeader(10, 1234567890),
		})
		require.NoError(t, err)

		claims, claimErrors, err := n.GetAvailableClaimList(context.Background(), "AABB", []string{"KLV", "KFI"})
		require.ErrorIs(t, err, common.ErrAccountNotFound)
		require.Nil(t, claims)
		require.Nil(t, claimErrors)
	})

	t.Run("returns zero values for non-user account", func(t *testing.T) {
		t.Parallel()

		n, err := createNodeWithOptions(t, nodeTestOptions{
			accAdapter:   createAccDBWithUser(&mock.KAppAccountHandlerStub{}),
			kappsAdapter: &mock.AccountsStub{},
			blockchain:   &mock.BlockChainMock{},
		})
		require.NoError(t, err)

		claims, claimErrors, err := n.GetAvailableClaimList(context.Background(), "AABB", []string{"KLV", "KFI"})
		require.NoError(t, err)
		require.Empty(t, claimErrors)
		require.Len(t, claims, 2)
		require.Equal(t, &models.AvailableClaimResponse{}, claims["KLV"])
		require.Equal(t, &models.AvailableClaimResponse{}, claims["KFI"])
	})

	t.Run("does not touch the state when the context is already done", func(t *testing.T) {
		t.Parallel()

		var userLoads, kappLoads int
		accDB := createAccDBWithUser(claimingUserAcc())
		accDB.GetExistingAccountCalled = func([]byte) (state.AccountHandler, error) {
			userLoads++
			return nil, errors.New("must not be called")
		}
		kappsDB := createKappsDBWithStakingAndKDA(stakingBytes, kdaBytes)
		kappsDB.LoadAccountCalled = func([]byte) (state.AccountHandler, error) {
			kappLoads++
			return nil, errors.New("must not be called")
		}

		n, err := createNodeWithOptions(t, nodeTestOptions{
			accAdapter:   accDB,
			kappsAdapter: kappsDB,
			blockchain:   createBlockchainWithHeader(10, 1234567890),
		})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		claims, claimErrors, err := n.GetAvailableClaimList(ctx, "AABB", []string{"KLV", "KFI"})
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, claims)
		require.Nil(t, claimErrors)
		require.Zero(t, userLoads, "a cancelled request must not load the user account")
		require.Zero(t, kappLoads, "a cancelled request must not load the kapps")
	})

	t.Run("stops at the next asset once the context is done", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		const assetsBeforeCancel = 2
		userKDAReads := 0
		userAcc := claimingUserAcc()
		userAcc.GetUserKDACalled = func([]byte, []byte, bool) (*kapps.UserKDA, error) {
			userKDAReads++
			if userKDAReads == assetsBeforeCancel {
				cancel()
			}
			return &kapps.UserKDA{}, nil
		}

		n, err := createNodeWithOptions(t, nodeTestOptions{
			accAdapter:   createAccDBWithUser(userAcc),
			kappsAdapter: createKappsDBWithStakingAndKDA(stakingBytes, kdaBytes),
			blockchain:   createBlockchainWithHeader(10, 1234567890),
		})
		require.NoError(t, err)

		assetIds := []string{"AAA-1234", "BBB-1234", "CCC-1234", "DDD-1234"}
		claims, claimErrors, err := n.GetAvailableClaimList(ctx, "AABB", assetIds)
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, claims, "a request that did not finish must not return partial results")
		require.Nil(t, claimErrors)
		require.Equal(t, assetsBeforeCancel, userKDAReads, "no asset may be read after the context is done")
	})

	t.Run("stops loading the state once the context is done", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		accDB := createAccDBWithUser(claimingUserAcc())
		getExisting := accDB.GetExistingAccountCalled
		accDB.GetExistingAccountCalled = func(address []byte) (state.AccountHandler, error) {
			cancel()
			return getExisting(address)
		}
		kappLoads := 0
		kappsDB := createKappsDBWithStakingAndKDA(stakingBytes, kdaBytes)
		loadKapp := kappsDB.LoadAccountCalled
		kappsDB.LoadAccountCalled = func(address []byte) (state.AccountHandler, error) {
			kappLoads++
			return loadKapp(address)
		}

		n, err := createNodeWithOptions(t, nodeTestOptions{
			accAdapter:   accDB,
			kappsAdapter: kappsDB,
			blockchain:   createBlockchainWithHeader(10, 1234567890),
		})
		require.NoError(t, err)

		claims, claimErrors, err := n.GetAvailableClaimList(ctx, "AABB", []string{"KLV", "KFI"})
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, claims)
		require.Nil(t, claimErrors)
		require.Zero(t, kappLoads, "no kapp may be loaded after the context is done")
	})

	t.Run("does not load the KDA kapp once the context is done", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		kappLoads := 0
		kappsDB := createKappsDBWithStakingAndKDA(stakingBytes, kdaBytes)
		loadKapp := kappsDB.LoadAccountCalled
		kappsDB.LoadAccountCalled = func(address []byte) (state.AccountHandler, error) {
			kappLoads++
			cancel()
			return loadKapp(address)
		}

		n, err := createNodeWithOptions(t, nodeTestOptions{
			accAdapter:   createAccDBWithUser(claimingUserAcc()),
			kappsAdapter: kappsDB,
			blockchain:   createBlockchainWithHeader(10, 1234567890),
		})
		require.NoError(t, err)

		claims, claimErrors, err := n.GetAvailableClaimList(ctx, "AABB", []string{"KLV", "KFI"})
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, claims)
		require.Nil(t, claimErrors)
		require.Equal(t, 1, kappLoads, "the KDA kapp must not be loaded after the context is done")
	})

	t.Run("does not read the KLV pending rewards once the context is done", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		userAcc := claimingUserAcc()
		userAcc.GetUserKDACalled = func([]byte, []byte, bool) (*kapps.UserKDA, error) {
			cancel()
			return &kapps.UserKDA{}, nil
		}
		pendingRewardsReads := 0
		kappCtrl := &stub.KAppControllerStub{
			GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
				return &mock.ValidatorsKAppStub{
					GetPendingRewardsCalled: func([]byte) (int64, error) {
						pendingRewardsReads++
						return 500, nil
					},
				}
			},
		}

		n, err := createNodeWithOptions(t, nodeTestOptions{
			accAdapter:     createAccDBWithUser(userAcc),
			kappsAdapter:   createKappsDBWithStakingAndKDA(stakingBytes, kdaBytes),
			kappController: kappCtrl,
			blockchain:     createBlockchainWithHeader(10, 1234567890),
		})
		require.NoError(t, err)

		// KLV is the last asset, so no later per-asset check can end the request
		claims, claimErrors, err := n.GetAvailableClaimList(ctx, "AABB", []string{"KLV"})
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, claims, "a request that did not finish must not return partial results")
		require.Nil(t, claimErrors)
		require.Zero(t, pendingRewardsReads, "the Validators kapp must not be loaded after the context is done")
	})
}

func TestNode_LookupsReportMissingStateAsNotFound(t *testing.T) {
	t.Parallel()

	assetBytes, _ := getMarshalizer().Marshal(&kapps.KDAData{ID: []byte("KDA-1Q2W")})
	assetKey := string(kdautils.ToKDAKey([]byte("KDA-1Q2W"), nil))
	// A real trie returns (nil, nil) for a missing key.
	onlyAsset := func(key []byte) ([]byte, error) {
		if string(key) == assetKey {
			return assetBytes, nil
		}
		return nil, nil
	}
	trieErr := errors.New("trie node 0xdeadbeef missing")
	failing := func([]byte) ([]byte, error) { return nil, trieErr }

	tests := []struct {
		name      string
		retriever func([]byte) ([]byte, error)
		lookup    func(n *node.Node) (any, error)
		wantErr   error
	}{
		{"unknown asset", onlyAsset, func(n *node.Node) (any, error) { return n.GetAsset("NOPE-1Q2W") }, common.ErrAssetNotFound},
		{"asset trie error is not leaked", failing, func(n *node.Node) (any, error) { return n.GetAsset("KDA-1Q2W") }, common.ErrAssetNotFound},
		{"pool of an unknown asset", onlyAsset, func(n *node.Node) (any, error) { return n.GetKDAFeePool("NOPE-1Q2W") }, common.ErrAssetNotFound},
		{"asset without a pool", onlyAsset, func(n *node.Node) (any, error) { return n.GetKDAFeePool("KDA-1Q2W") }, common.ErrAssetPoolNotFound},
		{"unknown marketplace", onlyAsset, func(n *node.Node) (any, error) { return n.GetMarketplace("deadbeefdeadbeef") }, common.ErrMarketplaceNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			n, err := createNodeWithOptions(t, nodeTestOptions{kappsAdapter: createKappsDBWithTrieValue(tt.retriever)})
			require.NoError(t, err)

			got, err := tt.lookup(n)
			require.ErrorIs(t, err, tt.wantErr, "got %+v", got)
			require.NotContains(t, err.Error(), "deadbeef")
		})
	}
}
