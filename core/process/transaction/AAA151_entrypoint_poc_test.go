package transaction_test

import (
	"bytes"
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAAA151EntrypointSCInvokeRejectsSpoofedRecipientAddrForPermissionUpdate(t *testing.T) {
	victim := testOwnerAddress
	attackerContract := testToAddress
	attackerNewOwner := testReferralAddress

	_, _, _, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	victimAcc := loadUserAccount(accCacher, victim)
	victimAcc.SetPermissions([]*state.Permission{
		{
			ID:        0,
			Type:      state.Permission_Owner,
			Threshold: 1,
			Signers: []*state.Key{
				{Address: victim, Weight: 1},
			},
		},
	})
	require.NoError(t, accCacher.UpdateUser(victimAcc))
	require.NoError(t, accCacher.SaveAll())

	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, err := fork.NewForkController(config.EnableEpochs{
		KdaFpr:         0,
		SmartContracts: 0,
	}, epochNotifier)
	require.NoError(t, err)

	argsKapp := kappcontroller.ArgsNewKApp{
		Hasher:         &commonMock.HasherMock{},
		Marshalizer:    marshalizer,
		PubkeyConv:     addressConverter,
		AccountsCacher: accCacher,
		ForkController: forkController,
		RatingsData:    &commonMock.RatingsInfoMock{},
	}

	kAppController, err := kappcontroller.NewKappController(argsKapp)
	require.NoError(t, err)
	require.NoError(t, kAppController.SetProposalController(createProposalController()))
	require.NoError(t, kAppController.InitKApps(accCacher))

	block := createBlockHeader()
	kAppController.SetCurrentKAppContext(kapp.NewKappContext(kapp.ArgsNewKAppContext{
		ContractID:   0,
		ContractType: transaction.TXContract_UpdateAccountPermissionContractType,
		Block:        block,
	}))

	updatePermissionFunc, err := builtInFunctions.NewKleverUpdateAccountPermissionFunc(
		100,
		marshalizer,
		accCacher,
		forkController,
		kAppController,
	)
	require.NoError(t, err)

	attackerPermissions, err := builtInFunctions.EncodeAccountPermissionData([]*transaction.AccPermission{
		{
			Type:      transaction.AccPermission_Owner,
			Threshold: 1,
			Signers: []*transaction.AccKey{
				{Address: attackerNewOwner, Weight: 1},
			},
		},
	})
	require.NoError(t, err)

	vmInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr:         attackerContract,
			OriginalCallerAddr: attackerContract,
			Arguments:          [][]byte{victim, attackerPermissions},
			GasProvided:        1000,
		},
		RecipientAddr: victim,
		Function:      "KleverUpdateAccountPermission",
	}

	vmOutput, err := updatePermissionFunc.ProcessBuiltinFunction(vmInput)

	assert.Error(t, err)
	assert.Nil(t, vmOutput)

	persistedVictim := loadUserAccount(accCacher, victim)
	require.Len(t, persistedVictim.GetPermissions(), 1)

	ownerPermission := persistedVictim.GetPermissions()[0]
	require.Equal(t, state.Permission_Owner, ownerPermission.Type)
	require.Len(t, ownerPermission.Signers, 1)
	assert.True(t, bytes.Equal(victim, ownerPermission.Signers[0].Address))
	assert.False(t, bytes.Equal(attackerNewOwner, ownerPermission.Signers[0].Address))
}
