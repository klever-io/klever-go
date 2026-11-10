package transaction_test

import (
	"bytes"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/klever-io/klever-go/crypto/mock"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	pTX "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

var OwnerAddress = []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf")
var RefAddress = []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg")
var APRIdentifier = []byte("APR")
var FPRIdentifier = []byte("FPR")

type Controller struct {
	userDB             state.AccountsAdapter
	kappDB             state.AccountsAdapter
	peersDB            state.AccountsAdapter
	accCacher          state.AccountsCacher
	kdaKapp            state.KAppAccountHandler
	stakingKapp        state.KAppAccountHandler
	validatorKapp      state.KAppAccountHandler
	marketplaceKapp    state.KAppAccountHandler
	proposalController kapps.ActiveProposalController
	kappController     kapp.KAppController
	execTx             process.TransactionProcessor
	t                  *testing.T
}

func InitKapps(kappController kapp.KAppController, accCacher state.AccountsCacher, pc kapps.ActiveProposalController) {
	if err := kappController.InitKApps(accCacher); err != nil {
		panic(err)
	}

	if err := kappController.SetProposalController(pc); err != nil {
		panic(err)
	}
}

func NewController(t *testing.T) *Controller {
	userDB, kappDB, peersDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())
	accCacher.ResetAll(true)
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	marketplaceKapp := loadKAppAccount(accCacher, kapps.MarketKAppAddress)
	validatorsKapp := loadKAppAccount(accCacher, kapps.ValidatorsKAppAddress)

	proposalKapp := loadKAppAccount(accCacher, kapps.ProposalKAppAddress)
	pc := initProposalKapp(proposalKapp)
	_ = accCacher.SaveAll()

	args := createArgsForTxProcessorWithAccounts(userDB, peersDB, kappDB, accCacher)
	args.ScProcessor = &commonMock.SCProcessorMock{}

	marshalizerMock := &commonMock.ProtoMarshalizerMock{}
	pubkeyConvMock := createMockPubkeyConverter()
	ratingsDataMock := &commonMock.RatingsInfoMock{}

	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
	}, epochNotifier)

	argsKapp := kappcontroller.ArgsNewKApp{
		Hasher:         args.Hasher,
		Marshalizer:    marshalizerMock,
		PubkeyConv:     pubkeyConvMock,
		ForkController: forkController,
		AccountsCacher: accCacher,
		RatingsData:    ratingsDataMock,
	}

	var err error
	args.KAppController, err = kappcontroller.NewKappController(argsKapp)
	require.NoError(t, err)

	err = args.KAppController.InitKApps(accCacher)
	require.NoError(t, err)

	InitKapps(args.KAppController, accCacher, pc)

	execTx, err := pTX.NewTxProcessor(args)
	require.NoError(t, err)

	c := &Controller{userDB, kappDB, peersDB, accCacher, kdaKapp, stakingKapp, validatorsKapp, marketplaceKapp, pc, args.KAppController, execTx, t}
	c.CreateMainAssets()
	c.CreateAPRAsset()
	c.CreateFPRAsset()

	return c
}

func (c *Controller) RecreateTxProcessor(cfg config.EnableEpochs) {
	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(cfg, epochNotifier)

	args := createArgsForTxProcessor()
	args.AccountsCacher = c.accCacher
	args.ForkController = forkController
	args.ScProcessor = &commonMock.SCProcessorMock{}
	SetupKappController(c.t, &args)

	execTx, _ := pTX.NewTxProcessor(args)

	c.execTx = execTx

}

func (c *Controller) UpdateForkController(cfg config.EnableEpochs) {
	forkController, _ := fork.NewForkController(cfg, &commonMock.EpochNotifierStub{})

	marshalizerMock := &commonMock.ProtoMarshalizerMock{}
	pubkeyConvMock := createMockPubkeyConverter()
	ratingsDataMock := &commonMock.RatingsInfoMock{}

	argsKapp := kappcontroller.ArgsNewKApp{
		Hasher:         mock.HasherMock{},
		Marshalizer:    marshalizerMock,
		PubkeyConv:     pubkeyConvMock,
		ForkController: forkController,
		AccountsCacher: c.accCacher,
		RatingsData:    ratingsDataMock,
	}

	args := createArgsForTxProcessor()
	args.AccountsCacher = c.accCacher
	args.ForkController = forkController

	var err error
	args.KAppController, err = kappcontroller.NewKappController(argsKapp)
	require.NoError(c.t, err)
	_ = args.KAppController.GetValidatorsKApp().SetAccountsCacher(c.accCacher)
	InitKapps(args.KAppController, c.accCacher, c.proposalController)

	execTx, err := pTX.NewTxProcessor(args)
	require.NoError(c.t, err)

	c.kappController = args.KAppController

	c.execTx = execTx
}

func (c *Controller) AddToStakingPool(id []byte, fprData *kapps.FPRData, aprData *kapps.APRData) {
	key := kdautils.ToKDAKey(id, nil)

	staking := kapps.StakingData{}
	stakingData, err := c.stakingKapp.DataTrieTracker().RetrieveValue(key)
	if err == nil {
		_ = marshalizer.Unmarshal(&staking, stakingData)
	}

	if fprData != nil {
		staking.FPR = append(staking.FPR, fprData)
	}

	if aprData != nil {
		staking.APR = append(staking.APR, aprData)
	}

	stakingData, err = marshalizer.Marshal(&staking)
	require.Nil(c.t, err)
	_ = c.stakingKapp.DataTrieTracker().SaveKeyValue(key, stakingData)

	_ = c.kappDB.SaveAccount(c.stakingKapp)
}

func (c *Controller) CreateMainAssets() {
	klvKey := kdautils.ToKDAKey(kdautils.KLVIdentifier, nil)
	kfiKey := kdautils.ToKDAKey(kdautils.KFIIdentifier, nil)

	staking := kapps.StakingData{
		InterestType:       kapps.StakingData_FPRI,
		MinEpochsToUnstake: 1,
		MinEpochsToClaim:   0,
	}
	stakingData, err := marshalizer.Marshal(&staking)
	require.Nil(c.t, err)

	_ = c.stakingKapp.DataTrieTracker().SaveKeyValue(klvKey, stakingData)
	_ = c.stakingKapp.DataTrieTracker().SaveKeyValue(kfiKey, stakingData)

	klv := kapps.KDAData{
		ID:                kdautils.KLVIdentifier,
		AssetType:         kapps.KDAData_Fungible,
		Name:              []byte("KLEVER"),
		Ticker:            kdautils.KLVIdentifier,
		OwnerAddress:      nil,
		Precision:         6,
		InitialSupply:     10000000000000000,
		CirculatingSupply: 100000000000000,
		MaxSupply:         90000000000000000,
		IssueDate:         time.Now().Unix(),
		Royalties:         &kapps.RoyaltiesData{},
		Properties: &kapps.PropertiesData{
			CanFreeze: true,
			CanMint:   true,
			CanBurn:   true,
		},
		Attributes: &kapps.AttributesData{
			IsPaused:         false,
			IsNFTMintStopped: true,
		},
	}

	kfi := kapps.KDAData{
		ID:                kdautils.KFIIdentifier,
		AssetType:         kapps.KDAData_Fungible,
		Name:              []byte("KLEVER FINANCE"),
		Ticker:            kdautils.KFIIdentifier,
		OwnerAddress:      nil,
		Precision:         6,
		InitialSupply:     10000000000000000,
		CirculatingSupply: 100000000000000,
		MaxSupply:         90000000000000000,
		IssueDate:         time.Now().Unix(),
		Royalties:         &kapps.RoyaltiesData{},
		Properties: &kapps.PropertiesData{
			CanFreeze: true,
			CanMint:   true,
			CanBurn:   true,
		},
		Attributes: &kapps.AttributesData{
			IsPaused:         false,
			IsNFTMintStopped: true,
		},
	}

	klvData, _ := marshalizer.Marshal(&klv)
	kfiData, _ := marshalizer.Marshal(&kfi)

	_ = c.kdaKapp.DataTrieTracker().SaveKeyValue(klvKey, klvData)
	_ = c.kdaKapp.DataTrieTracker().SaveKeyValue(kfiKey, kfiData)

	_ = c.kappDB.SaveAccount(c.kdaKapp)
	_ = c.kappDB.SaveAccount(c.stakingKapp)
}

func (c *Controller) CreateAPRAsset() {
	aprKey := kdautils.ToKDAKey(APRIdentifier, nil)

	staking := kapps.StakingData{
		InterestType:       kapps.StakingData_APRI,
		MinEpochsToUnstake: 1,
		MinEpochsToClaim:   0,
	}
	stakingData, err := marshalizer.Marshal(&staking)
	require.Nil(c.t, err)

	_ = c.stakingKapp.DataTrieTracker().SaveKeyValue(aprKey, stakingData)

	apr := kapps.KDAData{
		ID:                APRIdentifier,
		AssetType:         kapps.KDAData_Fungible,
		Name:              APRIdentifier,
		Ticker:            APRIdentifier,
		OwnerAddress:      OwnerAddress,
		Precision:         6,
		InitialSupply:     10000000000000000,
		CirculatingSupply: 100000000000000,
		MaxSupply:         90000000000000000,
		IssueDate:         time.Now().Unix(),
		Royalties:         &kapps.RoyaltiesData{},
		Properties: &kapps.PropertiesData{
			CanFreeze: true,
			CanMint:   true,
			CanBurn:   true,
		},
		Attributes: &kapps.AttributesData{
			IsPaused:         false,
			IsNFTMintStopped: true,
		},
	}

	aprData, _ := marshalizer.Marshal(&apr)

	_ = c.kdaKapp.DataTrieTracker().SaveKeyValue(aprKey, aprData)

	_ = c.kappDB.SaveAccount(c.kdaKapp)
	_ = c.kappDB.SaveAccount(c.stakingKapp)
}

func (c *Controller) CreateFPRAsset() {
	fprKey := kdautils.ToKDAKey(FPRIdentifier, nil)

	staking := kapps.StakingData{
		InterestType:       kapps.StakingData_FPRI,
		MinEpochsToUnstake: 1,
		MinEpochsToClaim:   0,
	}
	stakingData, err := marshalizer.Marshal(&staking)
	require.Nil(c.t, err)

	_ = c.stakingKapp.DataTrieTracker().SaveKeyValue(fprKey, stakingData)

	fpr := kapps.KDAData{
		ID:                FPRIdentifier,
		AssetType:         kapps.KDAData_Fungible,
		Name:              FPRIdentifier,
		Ticker:            FPRIdentifier,
		OwnerAddress:      OwnerAddress,
		Precision:         6,
		InitialSupply:     10000000000000000,
		CirculatingSupply: 100000000000000,
		MaxSupply:         90000000000000000,
		IssueDate:         time.Now().Unix(),
		Royalties:         &kapps.RoyaltiesData{},
		Properties: &kapps.PropertiesData{
			CanFreeze: true,
			CanMint:   true,
			CanBurn:   true,
		},
		Attributes: &kapps.AttributesData{
			IsPaused:         false,
			IsNFTMintStopped: true,
		},
		Roles: []*kapps.RolesData{
			{
				Address:             RefAddress,
				HasRoleMint:         true,
				HasRoleSetITOPrices: true,
				HasRoleDeposit:      true,
				HasRoleTransfer:     true,
			},
		},
	}

	fprData, _ := marshalizer.Marshal(&fpr)

	_ = c.kdaKapp.DataTrieTracker().SaveKeyValue(fprKey, fprData)

	_ = c.kappDB.SaveAccount(c.kdaKapp)
	_ = c.kappDB.SaveAccount(c.stakingKapp)
}

func (c *Controller) AddUser(addr []byte, amount int64, assetID []byte) state.UserAccountHandler {
	ownerAcc := loadUserAccount(c.accCacher, addr)
	err := ownerAcc.AddToBalance(amount, assetID, true)
	require.NoError(c.t, err)

	err = c.userDB.SaveAccount(ownerAcc)
	require.NoError(c.t, err)

	return ownerAcc
}

func (c *Controller) AddAllowance(acc state.UserAccountHandler, allowance int64) {
	_ = acc.AddToAllowance(allowance)
	_ = c.userDB.SaveAccount(acc)
}
func (c *Controller) CreateBlockHeader(timestamp int64, epoch uint32, nonce uint64) *block.Block {
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}
	hdr := block.Block{
		Header: &block.BlockHeader{
			Timestamp:    timestamp,
			Nonce:        nonce,
			Epoch:        epoch,
			Slot:         nonce,
			ParentHash:   []byte(""),
			TxRootHash:   []byte("txRootHash"),
			TrieRoot:     []byte("rootHash"),
			TxCount:      1,
			PrevRandSeed: make([]byte, 0),
			RandSeed:     make([]byte, 0),
		},
		ProducerSignature: []byte("signature"),
	}

	return &hdr
}

func (c *Controller) RunFreezeTX(blk *block.Block, ownerAddress, assetID []byte, value int64) []byte {
	freezeContract := transaction.FreezeContract{
		AssetID: assetID,
		Amount:  value,
	}

	tx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.Nil(c.t, err)

	return tx.Receipts[1].Data[1]
}

func (c *Controller) RunDelegateTX(blk *block.Block, ownerAddress []byte, bucketID []byte, toAddress []byte) {
	delegateContract := transaction.DelegateContract{
		BucketID:  bucketID,
		ToAddress: toAddress,
	}

	tx, _ := createTransactionMock(&delegateContract, transaction.TXContract_DelegateContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.Nil(c.t, err)

}

func (c *Controller) RunClaimTX(blk *block.Block, ty transaction.ClaimContract_EnumClaimType, ownerAddress, assetID []byte, errorType error) *transaction.Transaction {
	claimContract := transaction.ClaimContract{
		ClaimType: ty,
		ID:        assetID,
	}

	tx, _ := createTransactionMock(&claimContract, transaction.TXContract_ClaimContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.ErrorIs(c.t, err, errorType)

	return tx
}

func (c *Controller) RunUnfreezeTX(blk *block.Block, ownerAddress, assetID, bucketID []byte, errorType error) {
	claimContract := transaction.UnfreezeContract{
		AssetID:  assetID,
		BucketID: bucketID,
	}

	tx, _ := createTransactionMock(&claimContract, transaction.TXContract_UnfreezeContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)

	assert.Equal(c.t, errorType, err)
}

func (c *Controller) RunWithdrawTX(blk *block.Block, ownerAddress, assetID []byte, errorType error) {
	claimContract := transaction.WithdrawContract{
		AssetID: assetID,
	}

	tx, _ := createTransactionMock(&claimContract, transaction.TXContract_WithdrawContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)

	assert.Equal(c.t, err, errorType)

}

func (c *Controller) RunDepositTX(blk *block.Block, ownerAddress, assetID, currencyID []byte, value int64) []byte {
	depositContract := transaction.DepositContract{
		DepositType: transaction.DepositContract_FPRDeposit,
		ID:          assetID,
		CurrencyID:  currencyID,
		Amount:      value,
	}

	tx, _ := createTransactionMock(&depositContract, transaction.TXContract_DepositContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.Nil(c.t, err)

	return tx.Receipts[1].Data[1]
}

func (c *Controller) RunTriggerUpdateAPR(
	blk *block.Block,
	assetID []byte,
	newAPR, minEpochsToClaim, minEpochsToUnstake, minEpochsToWithdraw uint32,
) {
	updateStakingContract := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateStaking,
		AssetID:     assetID,
		Staking: &transaction.StakingInfo{
			Type:                transaction.StakingInfo_APRI,
			APR:                 newAPR,
			MinEpochsToClaim:    minEpochsToClaim,
			MinEpochsToUnstake:  minEpochsToUnstake,
			MinEpochsToWithdraw: minEpochsToWithdraw,
		},
	}

	tx, _ := createTransactionMock(
		&updateStakingContract,
		transaction.TXContract_AssetTriggerContractType,
		OwnerAddress,
		0,
	)

	_, hash, err := c.execTx.PreProcessTransaction(tx)
	require.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.Nil(c.t, err)
}

func (c *Controller) CheckBalance(addr []byte, assetID []byte, amount int64) state.UserAccountHandler {
	ownerAcc := loadUserAccount(c.accCacher, addr)
	assert.Equal(c.t, amount, ownerAcc.GetBalance(assetID, true))
	return ownerAcc
}

func (c *Controller) GetAllowance(addr []byte) int64 {
	ownerAcc := loadUserAccount(c.accCacher, addr)
	return ownerAcc.GetAllowance()
}

func (c *Controller) GetPendingRewards(addr []byte) int64 {
	rewards, err := c.kappController.GetValidatorsKApp().GetPendingRewards(addr)
	if err != nil {
		return 0
	}
	return rewards
}

func (c *Controller) AddFeesToPeer(addr []byte, amount int64) error {
	ownerPeer := loadPeerAccount(c.accCacher, addr)
	ownerPeer.AddToAccumulatedFees(amount)
	err := c.accCacher.UpdatePeer(ownerPeer)
	if err != nil {
		return err
	}

	_ = c.peersDB.SaveAccount(ownerPeer)

	return nil
}

func (c *Controller) GetStakingKDA(asset []byte) *kapps.StakingData {
	kdaKey := kdautils.ToKDAKey(asset, nil)

	stakingKDABytes, err := c.stakingKapp.DataTrieTracker().RetrieveValue(kdaKey)
	assert.Nil(c.t, err)

	stakingKDAFreeze := &kapps.StakingData{}
	err = marshalizer.Unmarshal(stakingKDAFreeze, stakingKDABytes)
	assert.Nil(c.t, err)

	return stakingKDAFreeze
}

func (c *Controller) CheckFPRStakingFronzen(asset []byte, epoch uint32, expectTotal, expectFrozen int64, shouldFind bool) {
	staking := c.GetStakingKDA(asset)
	require.NotNil(c.t, staking)

	assert.Equal(c.t, expectTotal, staking.TotalStaked)

	var find bool
	for _, s := range staking.FPR {
		if s.Epoch == epoch {
			find = true
			assert.Equal(c.t, expectFrozen, s.TotalStaked)
			break
		}
	}

	assert.Equal(c.t, shouldFind, find)
}

func (c *Controller) GetAsset(assetID []byte) (*kapps.KDAData, error) {
	key := kdautils.ToKDAKey(assetID, nil)

	KDABytes, err := c.kdaKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(KDABytes) == 0 {
		return nil, common.ErrAssetNotFound
	}

	kda := &kapps.KDAData{}
	err = marshalizer.Unmarshal(kda, KDABytes)
	if err != nil {
		return nil, err
	}
	return kda, nil
}

func (c *Controller) CheckFrozenBalance(addr []byte, assetID []byte, amount int64) state.UserAccountHandler {
	ownerAcc := loadUserAccount(c.accCacher, addr)
	assert.Equal(c.t, amount, ownerAcc.GetFrozenBalance(assetID, true))
	return ownerAcc
}

func computePercentage(myStake, totalStake, rewards int64) int64 {
	return int64(
		float64(rewards) * float64(myStake) / float64(totalStake),
	)
}

func computeAPRPercentage(myStake, apr, stakedTime int64) int64 {
	return int64(
		(float64(apr) * float64(myStake) / float64(core.HundredPercent) * float64(stakedTime)) / float64(core.OneYearTimestamp),
	)
}

func TestStakingTxProcessor_FPRClaim(t *testing.T) {
	initialBalance := int64(1_000_000_000_000)
	freezeAmount := int64(1_000_000_000)

	c := NewController(t)
	c.AddUser(OwnerAddress, initialBalance, nil)

	blk := c.CreateBlockHeader(0, 1, 1)
	bucketID := c.RunFreezeTX(blk, OwnerAddress, kdautils.KLVIdentifier, freezeAmount)
	c.CheckBalance(OwnerAddress, nil, initialBalance-freezeAmount)
	c.AddToStakingPool(kdautils.KLVIdentifier, &kapps.FPRData{
		TotalAmount: 1_000,
		TotalStaked: 500_000_000,
		Epoch:       1,
	}, nil)
	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, kdautils.KLVIdentifier, state.ErrClaimNotAvailable)
	c.CheckBalance(OwnerAddress, nil, initialBalance-freezeAmount)

	c.RunUnfreezeTX(blk, OwnerAddress, kdautils.KLVIdentifier, bucketID, state.ErrUnstakeNotAvailable)

	c.RunWithdrawTX(blk, OwnerAddress, kdautils.KLVIdentifier, state.ErrWithdrawNotAvailable)

	c.AddToStakingPool(kdautils.KLVIdentifier, &kapps.FPRData{
		TotalAmount: 1_000,
		TotalStaked: 5_000_000_000,
		Epoch:       2,
	}, nil)
	blk = c.CreateBlockHeader(0, 2, 1)
	expectedRewards := computePercentage(freezeAmount, 5_000_000_000, 1_000)

	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, kdautils.KLVIdentifier, nil)
	c.CheckBalance(OwnerAddress, nil, (initialBalance-freezeAmount)+expectedRewards)

	c.RunUnfreezeTX(blk, OwnerAddress, kdautils.KLVIdentifier, bucketID, nil)
	c.CheckBalance(OwnerAddress, nil, (initialBalance-freezeAmount)+expectedRewards)

	c.RunWithdrawTX(blk, OwnerAddress, kdautils.KLVIdentifier, nil)
	c.CheckBalance(OwnerAddress, nil, (initialBalance)+expectedRewards)
}

func TestStakingTxProcessor_Allowance(t *testing.T) {
	OwnerAddress := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf")

	initialBalance := int64(1_000_000_000)
	allowanceAmount := int64(10_000)

	c := NewController(t)
	acc := c.AddUser(OwnerAddress, initialBalance, nil)
	c.AddAllowance(acc, allowanceAmount)
	c.CheckBalance(OwnerAddress, nil, (initialBalance))

	blk := c.CreateBlockHeader(0, 1, 1)
	c.RunClaimTX(blk, transaction.ClaimContract_AllowanceClaim, OwnerAddress, kdautils.KLVIdentifier, nil)
	acc = c.CheckBalance(OwnerAddress, nil, (initialBalance)+allowanceAmount)
	assert.Equal(t, int64(0), acc.GetAllowance())

	c.RunClaimTX(blk, transaction.ClaimContract_AllowanceClaim, OwnerAddress, kdautils.KLVIdentifier, nil)
	acc = c.CheckBalance(OwnerAddress, nil, (initialBalance)+allowanceAmount)
	assert.Equal(t, int64(0), acc.GetAllowance())

	secondAllowace := int64(11_000)
	c.AddAllowance(acc, secondAllowace)
	acc = c.CheckBalance(OwnerAddress, nil, initialBalance+allowanceAmount)
	assert.Equal(t, secondAllowace, acc.GetAllowance())

	c.RunClaimTX(blk, transaction.ClaimContract_AllowanceClaim, OwnerAddress, kdautils.KLVIdentifier, nil)
	acc = c.CheckBalance(OwnerAddress, nil, (initialBalance)+allowanceAmount+secondAllowace)
	assert.Equal(t, int64(0), acc.GetAllowance())
}

func TestStakingTxProcessor_FPRClaim_KFI(t *testing.T) {
	initialBalance := int64(1_000_000_000)
	freezeAmount := int64(1_000_000)

	c := NewController(t)
	c.AddUser(OwnerAddress, initialBalance, kdautils.KFIIdentifier)

	blk := c.CreateBlockHeader(0, 1, 1)
	bucketID := c.RunFreezeTX(blk, OwnerAddress, kdautils.KFIIdentifier, freezeAmount)
	c.CheckBalance(OwnerAddress, kdautils.KFIIdentifier, initialBalance-freezeAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, 0)

	c.AddToStakingPool(kdautils.KLVIdentifier, &kapps.FPRData{
		TotalAmount: 1_000,
		TotalStaked: 500_000,
		Epoch:       1,
	}, nil)
	c.AddToStakingPool(kdautils.KFIIdentifier, &kapps.FPRData{
		TotalAmount: 2_000,
		TotalStaked: 500_000,
		Epoch:       1,
	}, nil)

	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, kdautils.KLVIdentifier, state.ErrClaimNotAvailable)
	c.CheckBalance(OwnerAddress, nil, 0)
	c.CheckBalance(OwnerAddress, kdautils.KFIIdentifier, initialBalance-freezeAmount)

	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, kdautils.KFIIdentifier, state.ErrClaimNotAvailable)
	c.CheckBalance(OwnerAddress, nil, 0)
	c.CheckBalance(OwnerAddress, kdautils.KFIIdentifier, initialBalance-freezeAmount)

	c.RunUnfreezeTX(blk, OwnerAddress, kdautils.KLVIdentifier, bucketID, state.ErrNotStaked)
	c.RunUnfreezeTX(blk, OwnerAddress, kdautils.KFIIdentifier, bucketID, state.ErrUnstakeNotAvailable)

	c.RunWithdrawTX(blk, OwnerAddress, kdautils.KLVIdentifier, state.ErrWithdrawNotAvailable)
	c.RunWithdrawTX(blk, OwnerAddress, kdautils.KFIIdentifier, state.ErrWithdrawNotAvailable)

	c.AddToStakingPool(kdautils.KLVIdentifier, &kapps.FPRData{
		TotalAmount: 1_000,
		TotalStaked: 5_000_000,
		Epoch:       2,
	}, nil)
	c.AddToStakingPool(kdautils.KFIIdentifier, &kapps.FPRData{
		TotalAmount: 3_000,
		TotalStaked: 5_000_000,
		Epoch:       2,
	}, nil)

	blk = c.CreateBlockHeader(0, 2, 1)
	expectedRewards := computePercentage(freezeAmount, 5_000_000, 3_000)

	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, kdautils.KLVIdentifier, state.ErrClaimNotAvailable)
	c.CheckBalance(OwnerAddress, nil, 0)
	c.CheckBalance(OwnerAddress, kdautils.KFIIdentifier, initialBalance-freezeAmount)

	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, kdautils.KFIIdentifier, nil)
	c.CheckBalance(OwnerAddress, nil, expectedRewards)
	c.CheckBalance(OwnerAddress, kdautils.KFIIdentifier, initialBalance-freezeAmount)

	c.RunUnfreezeTX(blk, OwnerAddress, kdautils.KFIIdentifier, bucketID, nil)
	c.CheckBalance(OwnerAddress, nil, expectedRewards)
	c.CheckBalance(OwnerAddress, kdautils.KFIIdentifier, initialBalance-freezeAmount)

	c.RunWithdrawTX(blk, OwnerAddress, kdautils.KFIIdentifier, nil)
	c.CheckBalance(OwnerAddress, nil, expectedRewards)
	c.CheckBalance(OwnerAddress, kdautils.KFIIdentifier, initialBalance)

}

func TestStakingTxProcessor_APRClaim(t *testing.T) {
	initialBalance := int64(1_000_000_000)
	freezeAmount := int64(1_000_000)

	c := NewController(t)
	c.AddUser(OwnerAddress, initialBalance, APRIdentifier)

	blk := c.CreateBlockHeader(0, 1, 1)

	bucketID := c.RunFreezeTX(blk, OwnerAddress, APRIdentifier, freezeAmount)
	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-freezeAmount)

	// test with timestamp prior APR start
	initialTime := blk.GetTimestamp()
	blk.Header.Timestamp -= int64(time.Hour.Seconds())
	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, state.ErrClaimNotAvailable)
	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-freezeAmount)

	c.AddToStakingPool(APRIdentifier, nil, &kapps.APRData{
		Value:     1000,
		Timestamp: initialTime,
		Epoch:     1,
	})

	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, state.ErrClaimNotAvailable)
	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-freezeAmount)

	c.RunUnfreezeTX(blk, OwnerAddress, APRIdentifier, bucketID, state.ErrUnstakeNotAvailable)
	c.RunWithdrawTX(blk, OwnerAddress, APRIdentifier, state.ErrWithdrawNotAvailable)

	blk.Header.Timestamp = initialTime
	blk = c.CreateBlockHeader(time.Unix(blk.GetTimestamp(), 0).Add(24*time.Hour).Unix(), 2, 1)
	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, nil)

	expectedRewards := computeAPRPercentage(freezeAmount, 1000, blk.GetTimestamp()-initialTime)
	balance := initialBalance - freezeAmount + expectedRewards
	c.CheckBalance(OwnerAddress, APRIdentifier, balance)

	c.RunUnfreezeTX(blk, OwnerAddress, APRIdentifier, bucketID, nil)
	c.CheckBalance(OwnerAddress, APRIdentifier, balance)

	c.RunWithdrawTX(blk, OwnerAddress, APRIdentifier, nil)
	balance += freezeAmount
	c.CheckBalance(OwnerAddress, APRIdentifier, balance)

	blk = c.CreateBlockHeader(time.Unix(blk.GetTimestamp(), 0).Add(24*time.Hour).Unix(), 3, 1)
	c.RunFreezeTX(blk, OwnerAddress, APRIdentifier, freezeAmount)
	balance -= freezeAmount
	c.CheckBalance(OwnerAddress, APRIdentifier, balance)

	c.AddToStakingPool(APRIdentifier, nil, &kapps.APRData{
		Value:     500,
		Timestamp: blk.GetTimestamp(),
		Epoch:     3,
	})
	initialTime = blk.GetTimestamp()
	blk = c.CreateBlockHeader(time.Unix(blk.GetTimestamp(), 0).Add(24*time.Hour).Unix(), 4, 1)
	expectedRewards = computeAPRPercentage(freezeAmount, 500, blk.GetTimestamp()-initialTime)
	c.AddToStakingPool(APRIdentifier, nil, &kapps.APRData{
		Value:     100,
		Timestamp: blk.GetTimestamp(),
		Epoch:     3,
	})

	initialTime = blk.GetTimestamp()
	blk = c.CreateBlockHeader(time.Unix(blk.GetTimestamp(), 0).Add(24*time.Hour).Unix(), 5, 1)
	expectedRewards += computeAPRPercentage(freezeAmount, 100, blk.GetTimestamp()-initialTime)

	balance += expectedRewards
	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, nil)
	c.CheckBalance(OwnerAddress, APRIdentifier, balance)

	initialTime = blk.GetTimestamp()
	blk = c.CreateBlockHeader(time.Unix(blk.GetTimestamp(), 0).Add(24*time.Hour).Unix(), 6, 1)
	expectedRewards = computeAPRPercentage(freezeAmount, 100, blk.GetTimestamp()-initialTime)
	c.RunUnfreezeTX(blk, OwnerAddress, APRIdentifier, bucketID, nil)
	balance += expectedRewards
	c.CheckBalance(OwnerAddress, APRIdentifier, balance)

	blk = c.CreateBlockHeader(time.Unix(blk.GetTimestamp(), 0).Add(24*time.Hour).Unix(), 7, 1)
	c.RunWithdrawTX(blk, OwnerAddress, APRIdentifier, nil)
	balance += freezeAmount
	c.CheckBalance(OwnerAddress, APRIdentifier, balance)
}

func TestStakingTxProcessor_APRFork(t *testing.T) {
	initialBalance := int64(1_000_000_000)
	freezeAmount := int64(1_000_000)

	c := NewController(t)

	c.RecreateTxProcessor(config.EnableEpochs{FixStakingBuckets: 100})

	c.AddUser(OwnerAddress, initialBalance, APRIdentifier)

	blk := c.CreateBlockHeader(0, 1, 1)
	bucketID := c.RunFreezeTX(blk, OwnerAddress, APRIdentifier, freezeAmount)
	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-freezeAmount)
	c.CheckFrozenBalance(OwnerAddress, APRIdentifier, freezeAmount)

	blk = c.CreateBlockHeader(0, 4, 1)
	c.RunUnfreezeTX(blk, OwnerAddress, APRIdentifier, bucketID, nil)
	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-freezeAmount)
	c.CheckFrozenBalance(OwnerAddress, APRIdentifier, 0)

	bucketID = c.RunFreezeTX(blk, OwnerAddress, APRIdentifier, freezeAmount)
	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-(2*freezeAmount))
	c.CheckFrozenBalance(OwnerAddress, APRIdentifier, freezeAmount)

	blk = c.CreateBlockHeader(0, 6, 1)
	c.RunUnfreezeTX(blk, OwnerAddress, APRIdentifier, bucketID, nil)
	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-(2*freezeAmount))
	c.CheckFrozenBalance(OwnerAddress, APRIdentifier, -freezeAmount)

	blk = c.CreateBlockHeader(0, 8, 1)
	bucketID = c.RunFreezeTX(blk, OwnerAddress, APRIdentifier, freezeAmount)
	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-(3*freezeAmount))
	c.CheckFrozenBalance(OwnerAddress, APRIdentifier, 0)

	c.RecreateTxProcessor(config.EnableEpochs{FixStakingBuckets: 0})

	blk = c.CreateBlockHeader(0, 10, 1)
	c.RunUnfreezeTX(blk, OwnerAddress, APRIdentifier, bucketID, nil)
	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-(3*freezeAmount))
	c.CheckFrozenBalance(OwnerAddress, APRIdentifier, 0)

	blk = c.CreateBlockHeader(0, 12, 1)
	bucketID = c.RunFreezeTX(blk, OwnerAddress, APRIdentifier, freezeAmount)
	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-(4*freezeAmount))
	c.CheckFrozenBalance(OwnerAddress, APRIdentifier, 4*freezeAmount)

	blk = c.CreateBlockHeader(0, 14, 1)
	c.RunUnfreezeTX(blk, OwnerAddress, APRIdentifier, bucketID, nil)
	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-(4*freezeAmount))
	c.CheckFrozenBalance(OwnerAddress, APRIdentifier, 0)
}

func (c *Controller) RunAPRUpdateNTimes(
	withFork bool,
	nTimes int,
	freezeAmount int64,
	blk *block.Block,
) *block.Block {
	if !withFork {
		c.UpdateForkController(config.EnableEpochs{SmartContracts: 200})
	}

	initApr := 100
	oneDaySecs := int64((24 * time.Hour).Seconds())
	for i := 1; i <= nTimes; i++ {
		newApr := initApr * i

		prevTimestamp := blk.GetTimestamp()
		epochCounter := uint32(i)
		blk = c.CreateBlockHeader(prevTimestamp+oneDaySecs, epochCounter, 1)

		c.RunTriggerUpdateAPR(blk, APRIdentifier, uint32(newApr), 0, 0, 0)
	}

	return blk
}

func simulateRewardsWithIncreasingAPR(
	howManyUpdates int,
	freezeAmount, timestampElapsed int64,
) int64 {
	totalRewards := int64(0)
	initApr := int64(100)

	for i := 1; i <= howManyUpdates; i++ {
		totalRewards += computeAPRPercentage(
			freezeAmount,
			initApr*int64(i)-initApr,
			timestampElapsed,
		)
	}

	return totalRewards
}

func TestStakingTxProcessor_APR_Pre_Fork_Updates_More_Than_100_Times(t *testing.T) {
	initialBalance := int64(1_000_000_000)
	freezeAmount := int64(1_000_000)

	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{SmartContracts: 0})
	c.AddUser(OwnerAddress, initialBalance, APRIdentifier)

	epochCounter := uint32(0)
	blk := c.CreateBlockHeader(0, epochCounter, 1)

	c.RunFreezeTX(blk, OwnerAddress, APRIdentifier, freezeAmount)

	oneDaySecs := int64((24 * time.Hour).Seconds())

	simulatedRewardsPreFork := simulateRewardsWithIncreasingAPR(110, freezeAmount, oneDaySecs)
	// first reward is null because initial APR is 0, thus needed run 11 times to subtract afterwards
	simulatedDiscardedRewardsPosFork := simulateRewardsWithIncreasingAPR(11, freezeAmount, oneDaySecs)
	simlutedRewardsPosFork := simulatedRewardsPreFork - simulatedDiscardedRewardsPosFork

	totalAPRUpdates := 110
	updatedBlk := c.RunAPRUpdateNTimes(false, totalAPRUpdates, freezeAmount, blk)

	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-freezeAmount)
	c.RunClaimTX(updatedBlk, transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, nil)
	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-freezeAmount+simulatedRewardsPreFork)

	aprAsset, _ := c.GetAsset(APRIdentifier)
	aprAssetMinted := aprAsset.GetMintedValue()

	assert.NotEqual(t, aprAssetMinted, simlutedRewardsPosFork)
	assert.Equal(t, simulatedRewardsPreFork, aprAssetMinted)
}

func TestStakingTxProcessor_APR_Pos_Fork_Updates_More_Than_100_Times(t *testing.T) {
	initialBalance := int64(1_000_000_000)
	freezeAmount := int64(1_000_000)

	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{SmartContracts: 0})
	c.AddUser(OwnerAddress, initialBalance, APRIdentifier)

	epochCounter := uint32(0)
	blk := c.CreateBlockHeader(0, epochCounter, 1)

	c.RunFreezeTX(blk, OwnerAddress, APRIdentifier, freezeAmount)

	oneDaySecs := int64((24 * time.Hour).Seconds())

	simulatedRewardsPreFork := simulateRewardsWithIncreasingAPR(110, freezeAmount, oneDaySecs)
	// first reward is null because initial APR is 0, thus needed run 11 times to subtract afterwards
	simulatedDiscardedRewardsPosFork := simulateRewardsWithIncreasingAPR(11, freezeAmount, oneDaySecs)
	simlutedRewardsPosFork := simulatedRewardsPreFork - simulatedDiscardedRewardsPosFork

	totalAPRUpdates := 110
	updatedBlk := c.RunAPRUpdateNTimes(true, totalAPRUpdates, freezeAmount, blk)

	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-freezeAmount)
	c.RunClaimTX(updatedBlk, transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, nil)
	c.CheckBalance(OwnerAddress, APRIdentifier, initialBalance-freezeAmount+simlutedRewardsPosFork)

	aprAsset, _ := c.GetAsset(APRIdentifier)
	aprAssetMinted := aprAsset.GetMintedValue()
	assert.Equal(t, aprAssetMinted, simlutedRewardsPosFork)
	assert.NotEqual(t, aprAssetMinted, simulatedRewardsPreFork)
}

const aprSupplyStart = int64(1_700_000_000)
const aprFixedSupply = int64(10_000_000_000_000_000)
const aprLargeSupply = int64(8_000_000_000_000_000_000)

func setAPRAssetSupply(t *testing.T, c *Controller, circulating, maxSupply int64) {
	t.Helper()

	updateAPRAsset(t, c, func(asset *kapps.KDAData) {
		asset.InitialSupply = circulating
		asset.CirculatingSupply = circulating
		asset.MintedValue = circulating
		asset.MaxSupply = maxSupply
	})
}

func updateAPRAsset(t *testing.T, c *Controller, update func(asset *kapps.KDAData)) {
	t.Helper()

	asset, err := c.GetAsset(APRIdentifier)
	require.NoError(t, err)

	update(asset)

	data, err := marshalizer.Marshal(asset)
	require.NoError(t, err)
	require.NoError(t, c.kdaKapp.DataTrieTracker().SaveKeyValue(kdautils.ToKDAKey(APRIdentifier, nil), data))
	require.NoError(t, c.kappDB.SaveAccount(c.kdaKapp))
}

func getAPRAsset(t *testing.T, c *Controller) *kapps.KDAData {
	t.Helper()

	asset, err := c.GetAsset(APRIdentifier)
	require.NoError(t, err)

	return asset
}

func stakeAPRSupplyAfterRaise(t *testing.T, c *Controller, circulating, maxSupply int64, apr uint32) int64 {
	t.Helper()

	setAPRAssetSupply(t, c, circulating, maxSupply)
	c.AddUser(OwnerAddress, circulating, APRIdentifier)

	blk := c.CreateBlockHeader(aprSupplyStart, 1, 1)
	c.RunTriggerUpdateAPR(blk, APRIdentifier, apr, 0, 0, 0)

	stakedAt := aprSupplyStart + 3600
	blk = c.CreateBlockHeader(stakedAt, 1, 2)
	c.RunFreezeTX(blk, OwnerAddress, APRIdentifier, circulating)
	c.CheckBalance(OwnerAddress, APRIdentifier, 0)

	return stakedAt
}

func TestStakingTxProcessor_APRClaim_SaturatedGain_PreFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})

	stakedAt := stakeAPRSupplyAfterRaise(t, c, aprFixedSupply, aprFixedSupply, math.MaxUint32)

	c.RunClaimTX(c.CreateBlockHeader(stakedAt+3600, 2, 3), transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, common.ErrMaxSupplyExceeded)
	c.CheckBalance(OwnerAddress, APRIdentifier, 0)

	c.RunClaimTX(c.CreateBlockHeader(stakedAt+2*86400, 2, 4), transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, nil)
	c.CheckBalance(OwnerAddress, APRIdentifier, math.MaxInt64)

	asset := getAPRAsset(t, c)
	assert.Less(t, asset.CirculatingSupply, int64(0))
	assert.Less(t, asset.MintedValue, int64(0))
}

func TestStakingTxProcessor_APRClaim_SaturatedGain_PostFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})

	stakedAt := stakeAPRSupplyAfterRaise(t, c, aprFixedSupply, aprFixedSupply, math.MaxUint32)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: 0})

	atCap := c.RunClaimTX(c.CreateBlockHeader(stakedAt+3600, 2, 3), transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, common.ErrMaxSupplyExceeded)
	assert.Equal(t, transaction.Transaction_MaxSupplyExceeded, atCap.GetResultCode())
	assert.False(t, hasAPRGainDroppedReceipt(atCap.GetReceipts()))
	c.CheckBalance(OwnerAddress, APRIdentifier, 0)

	overflow := c.RunClaimTX(c.CreateBlockHeader(stakedAt+2*86400, 2, 4), transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, common.ErrInt64Overflow)
	assert.Equal(t, transaction.Transaction_ClaimError, overflow.GetResultCode())
	assert.False(t, hasAPRGainDroppedReceipt(overflow.GetReceipts()))
	c.CheckBalance(OwnerAddress, APRIdentifier, 0)

	asset := getAPRAsset(t, c)
	assert.Equal(t, aprFixedSupply, asset.CirculatingSupply)
	assert.Equal(t, aprFixedSupply, asset.MintedValue)
}

func stakeAPRSupplyBeforeRaise(t *testing.T, c *Controller, kept int64) {
	t.Helper()

	setAPRAssetSupply(t, c, aprFixedSupply, aprFixedSupply)
	c.AddUser(OwnerAddress, aprFixedSupply, APRIdentifier)

	blk := c.CreateBlockHeader(aprSupplyStart, 1, 1)
	c.RunTriggerUpdateAPR(blk, APRIdentifier, 1000, 0, 0, 0)
	c.RunFreezeTX(c.CreateBlockHeader(aprSupplyStart+1, 1, 2), OwnerAddress, APRIdentifier, aprFixedSupply-kept)

	c.RunTriggerUpdateAPR(c.CreateBlockHeader(aprSupplyStart+3600, 1, 3), APRIdentifier, math.MaxUint32, 0, 0, 0)
}

func TestStakingTxProcessor_APRClaim_OverflowingSegmentsOnFreeze_PreFix(t *testing.T) {
	kept := int64(1_000)

	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})
	stakeAPRSupplyBeforeRaise(t, c, kept)

	freeze := &transaction.FreezeContract{AssetID: APRIdentifier, Amount: kept}
	runStakingContractTx(t, c, c.CreateBlockHeader(aprSupplyStart+2*86400, 2, 4), freeze, transaction.TXContract_FreezeContractType, OwnerAddress, nil)
	c.CheckBalance(OwnerAddress, APRIdentifier, 0)

	asset := getAPRAsset(t, c)
	assert.Less(t, asset.CirculatingSupply, int64(0))
	assert.Less(t, asset.MintedValue, int64(0))
}

func TestStakingTxProcessor_APRClaim_OverflowingSegmentsOnFreeze_PostFix(t *testing.T) {
	kept := int64(1_000)

	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})
	stakeAPRSupplyBeforeRaise(t, c, kept)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: 0})

	freeze := &transaction.FreezeContract{AssetID: APRIdentifier, Amount: kept}
	runStakingContractTx(t, c, c.CreateBlockHeader(aprSupplyStart+2*86400, 2, 4), freeze, transaction.TXContract_FreezeContractType, OwnerAddress, nil)
	c.CheckBalance(OwnerAddress, APRIdentifier, 0)

	asset := getAPRAsset(t, c)
	assert.Equal(t, aprFixedSupply, asset.CirculatingSupply)
	assert.Equal(t, aprFixedSupply, asset.MintedValue)
}

func TestStakingTxProcessor_APRClaim_NearInt64MaxSupply_PreFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})

	stakedAt := stakeAPRSupplyAfterRaise(t, c, aprLargeSupply, math.MaxInt64, core.HundredPercent)

	c.RunClaimTX(c.CreateBlockHeader(stakedAt+core.OneYearTimestamp/10, 2, 3), transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, nil)
	firstGain := loadUserAccount(c.accCacher, OwnerAddress).GetBalance(APRIdentifier, true)
	assert.Greater(t, firstGain, int64(0))
	assert.Equal(t, aprLargeSupply+firstGain, getAPRAsset(t, c).CirculatingSupply)

	c.RunClaimTX(c.CreateBlockHeader(stakedAt+core.OneYearTimestamp/5, 2, 4), transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, nil)
	assert.Less(t, getAPRAsset(t, c).CirculatingSupply, int64(0))
}

func TestStakingTxProcessor_APRClaim_NearInt64MaxSupply_PostFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: 0})

	stakedAt := stakeAPRSupplyAfterRaise(t, c, aprLargeSupply, math.MaxInt64, core.HundredPercent)

	c.RunClaimTX(c.CreateBlockHeader(stakedAt+core.OneYearTimestamp/10, 2, 3), transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, nil)
	firstGain := loadUserAccount(c.accCacher, OwnerAddress).GetBalance(APRIdentifier, true)
	assert.Greater(t, firstGain, int64(0))
	assert.Equal(t, aprLargeSupply+firstGain, getAPRAsset(t, c).CirculatingSupply)

	c.RunClaimTX(c.CreateBlockHeader(stakedAt+core.OneYearTimestamp/5, 2, 4), transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, common.ErrMaxSupplyExceeded)
	c.CheckBalance(OwnerAddress, APRIdentifier, firstGain)
	assert.Equal(t, aprLargeSupply+firstGain, getAPRAsset(t, c).CirculatingSupply)
}

func TestStakingTxProcessor_APRClaim_UncappedSupply_PreFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})

	stakedAt := stakeAPRSupplyAfterRaise(t, c, aprFixedSupply, 0, math.MaxUint32)

	c.RunClaimTX(c.CreateBlockHeader(stakedAt+2*86400, 2, 3), transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, nil)
	c.CheckBalance(OwnerAddress, APRIdentifier, math.MaxInt64)
	assert.Less(t, getAPRAsset(t, c).CirculatingSupply, int64(0))
}

func TestStakingTxProcessor_APRClaim_UncappedSupply_PostFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})

	stakedAt := stakeAPRSupplyAfterRaise(t, c, aprFixedSupply, 0, math.MaxUint32)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: 0})

	c.RunClaimTX(c.CreateBlockHeader(stakedAt+2*86400, 2, 3), transaction.ClaimContract_StakingClaim, OwnerAddress, APRIdentifier, common.ErrInt64Overflow)
	c.CheckBalance(OwnerAddress, APRIdentifier, 0)
	assert.Equal(t, aprFixedSupply, getAPRAsset(t, c).CirculatingSupply)
}

func hasAPRGainDroppedReceipt(receipts []*transaction.Transaction_Receipt) bool {
	for _, receipt := range receipts {
		data := receipt.GetData()
		if len(data) == 3 && data[0][0] == byte(pTX.Warning) && string(data[1]) == common.ErrFieldAPRGainDropped && bytes.Equal(data[2], APRIdentifier) {
			return true
		}
	}

	return false
}

func withdrawAPRStakeAfterOneYear(t *testing.T, c *Controller, stake int64, apr uint32, update func(asset *kapps.KDAData)) (int64, *kapps.KDAData, bool) {
	t.Helper()

	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: 0})
	setAPRAssetSupply(t, c, stake, 0)
	updateAPRAsset(t, c, update)
	c.AddUser(OwnerAddress, stake, APRIdentifier)

	c.RunTriggerUpdateAPR(c.CreateBlockHeader(aprSupplyStart, 1, 1), APRIdentifier, apr, 0, 0, 0)
	c.RunFreezeTX(c.CreateBlockHeader(aprSupplyStart+3600, 1, 2), OwnerAddress, APRIdentifier, stake)

	blk := c.CreateBlockHeader(aprSupplyStart+3600+core.OneYearTimestamp, 2, 3)
	unfreeze := &transaction.UnfreezeContract{AssetID: APRIdentifier, BucketID: APRIdentifier}
	unfreezeTx := runStakingContractTx(t, c, blk, unfreeze, transaction.TXContract_UnfreezeContractType, OwnerAddress, nil)
	withdraw := &transaction.WithdrawContract{AssetID: APRIdentifier, WithdrawType: transaction.WithdrawContract_Staking}
	runStakingContractTx(t, c, blk, withdraw, transaction.TXContract_WithdrawContractType, OwnerAddress, nil)

	balance := loadUserAccount(c.accCacher, OwnerAddress).GetBalance(APRIdentifier, true)

	return balance - stake, getAPRAsset(t, c), hasAPRGainDroppedReceipt(unfreezeTx.GetReceipts())
}

func TestStakingTxProcessor_APRClaim_UnfreezeWithOverflowingGain_PostFix(t *testing.T) {
	stake := int64(1_000_000_000_000_000_000)
	gain, asset, dropped := withdrawAPRStakeAfterOneYear(t, NewController(t), stake, core.MaxStakingAPR, func(asset *kapps.KDAData) {})

	assert.True(t, dropped)
	assert.Zero(t, gain)
	assert.Equal(t, stake, asset.CirculatingSupply)
	assert.Equal(t, stake, asset.MintedValue)
}

func TestStakingTxProcessor_APRClaim_UnfreezeWithFullyMintedSupply_PostFix(t *testing.T) {
	gain, asset, dropped := withdrawAPRStakeAfterOneYear(t, NewController(t), aprFixedSupply, core.HundredPercent, func(asset *kapps.KDAData) {
		asset.MaxSupply = asset.CirculatingSupply
	})

	assert.True(t, dropped)
	assert.Zero(t, gain)
	assert.Equal(t, aprFixedSupply, asset.CirculatingSupply)
	assert.Equal(t, aprFixedSupply, asset.MintedValue)
}

func TestStakingTxProcessor_APRClaim_UnfreezeWithMintedValueNearLimit_PostFix(t *testing.T) {
	gain, asset, dropped := withdrawAPRStakeAfterOneYear(t, NewController(t), aprFixedSupply, core.HundredPercent, func(asset *kapps.KDAData) {
		asset.MintedValue = math.MaxInt64 - 1
	})

	assert.False(t, dropped)
	assert.Greater(t, gain, int64(0))
	assert.Equal(t, aprFixedSupply+gain, asset.CirculatingSupply)
	assert.Equal(t, int64(math.MaxInt64), asset.MintedValue)
}

func TestStakingTxProcessor_APRClaim_UnfreezeWithNegativeMintedValue_PostFix(t *testing.T) {
	gain, asset, dropped := withdrawAPRStakeAfterOneYear(t, NewController(t), aprFixedSupply, core.HundredPercent, func(asset *kapps.KDAData) {
		asset.MintedValue = -1_000
	})

	assert.False(t, dropped)
	assert.Greater(t, gain, int64(0))
	assert.Equal(t, aprFixedSupply+gain, asset.CirculatingSupply)
	assert.Equal(t, -1_000+gain, asset.MintedValue)
}

func TestStakingTxProcessor_APRClaim_UnfreezeWithNegativeSupply_PostFix(t *testing.T) {
	gain, asset, dropped := withdrawAPRStakeAfterOneYear(t, NewController(t), aprFixedSupply, core.HundredPercent, func(asset *kapps.KDAData) {
		asset.CirculatingSupply = -1
	})

	assert.True(t, dropped)
	assert.Zero(t, gain)
	assert.Equal(t, int64(-1), asset.CirculatingSupply)
	assert.Equal(t, aprFixedSupply, asset.MintedValue)
}

func TestStakingTxProcessor_APRClaim_ClaimAndFreezeWithNegativeSupply_PostFix(t *testing.T) {
	half := aprFixedSupply / 2

	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: 0})
	setAPRAssetSupply(t, c, aprFixedSupply, 0)
	updateAPRAsset(t, c, func(asset *kapps.KDAData) {
		asset.CirculatingSupply = -1
	})
	c.AddUser(OwnerAddress, aprFixedSupply, APRIdentifier)

	c.RunTriggerUpdateAPR(c.CreateBlockHeader(aprSupplyStart, 1, 1), APRIdentifier, core.HundredPercent, 0, 0, 0)
	firstFreeze := &transaction.FreezeContract{AssetID: APRIdentifier, Amount: half}
	firstFreezeTx := runStakingContractTx(t, c, c.CreateBlockHeader(aprSupplyStart+3600, 1, 2), firstFreeze, transaction.TXContract_FreezeContractType, OwnerAddress, nil)
	assert.False(t, hasAPRGainDroppedReceipt(firstFreezeTx.GetReceipts()))

	blk := c.CreateBlockHeader(aprSupplyStart+3600+core.OneYearTimestamp, 2, 3)
	claim := &transaction.ClaimContract{ClaimType: transaction.ClaimContract_StakingClaim, ID: APRIdentifier}
	claimTx := runStakingContractTx(t, c, blk, claim, transaction.TXContract_ClaimContractType, OwnerAddress, state.ErrNegativeCirculatingSupply)
	assert.False(t, hasAPRGainDroppedReceipt(claimTx.GetReceipts()))
	topUp := &transaction.FreezeContract{AssetID: APRIdentifier, Amount: half}
	topUpTx := runStakingContractTx(t, c, blk, topUp, transaction.TXContract_FreezeContractType, OwnerAddress, nil)
	assert.True(t, hasAPRGainDroppedReceipt(topUpTx.GetReceipts()))

	c.CheckBalance(OwnerAddress, APRIdentifier, 0)
	c.CheckFrozenBalance(OwnerAddress, APRIdentifier, aprFixedSupply)
	assert.Equal(t, int64(-1), getAPRAsset(t, c).CirculatingSupply)
}

func builtinArg(value int64) []byte {
	return big.NewInt(value).Bytes()
}

func runStakingBuiltin[T vmcommon.BuiltinFunction](
	t *testing.T,
	c *Controller,
	blk *block.Block,
	newBuiltin func(uint64, vmcommon.Marshalizer, state.AccountsCacher, core.ForkController, kapp.KAppController) (T, error),
	args ...[]byte,
) error {
	t.Helper()

	builtin, err := newBuiltin(0, marshalizer, c.accCacher, commonMock.NewForkControllerStub(), c.kappController)
	require.NoError(t, err)

	c.kappController.SetCurrentKAppContext(kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: OwnerAddress,
		Block:          blk,
	}))

	_, err = builtin.ProcessBuiltinFunction(&vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr:  OwnerAddress,
			Arguments:   args,
			GasProvided: 1_000_000,
		},
	})
	if err != nil {
		return err
	}

	require.NoError(t, c.accCacher.SaveAll())

	return nil
}

func runContractStakingClaim(t *testing.T, c *Controller, blk *block.Block) error {
	t.Helper()

	// a staking claim pays no royalty receivers, so the transfer cost does not matter here
	newClaim := func(gasCost uint64, m vmcommon.Marshalizer, a state.AccountsCacher, f core.ForkController, k kapp.KAppController) (vmcommon.BuiltinFunction, error) {
		return builtInFunctions.NewKleverClaimFunc(gasCost, 0, m, a, f, k)
	}
	return runStakingBuiltin(t, c, blk, newClaim, builtinArg(int64(transaction.ClaimContract_StakingClaim)), APRIdentifier)
}

func TestStakingTxProcessor_APRClaim_ContractClaimSaturatedGain_PreFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})

	stakedAt := stakeAPRSupplyAfterRaise(t, c, aprFixedSupply, aprFixedSupply, math.MaxUint32)

	err := runContractStakingClaim(t, c, c.CreateBlockHeader(stakedAt+2*86400, 2, 3))
	assert.NoError(t, err)
	c.CheckBalance(OwnerAddress, APRIdentifier, math.MaxInt64)
	assert.Less(t, getAPRAsset(t, c).CirculatingSupply, int64(0))
}

func TestStakingTxProcessor_APRClaim_ContractClaimSaturatedGain_PostFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})

	stakedAt := stakeAPRSupplyAfterRaise(t, c, aprFixedSupply, aprFixedSupply, math.MaxUint32)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: 0})

	err := runContractStakingClaim(t, c, c.CreateBlockHeader(stakedAt+3600, 2, 3))
	assert.ErrorIs(t, err, common.ErrMaxSupplyExceeded)

	err = runContractStakingClaim(t, c, c.CreateBlockHeader(stakedAt+2*86400, 2, 4))
	assert.ErrorIs(t, err, common.ErrInt64Overflow)
	c.CheckBalance(OwnerAddress, APRIdentifier, 0)
	assert.Equal(t, aprFixedSupply, getAPRAsset(t, c).CirculatingSupply)
}

func TestStakingTxProcessor_APRClaim_ContractFreezeOverflowingSegments_PreFix(t *testing.T) {
	kept := int64(1_000)

	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})
	stakeAPRSupplyBeforeRaise(t, c, kept)

	err := runStakingBuiltin(t, c, c.CreateBlockHeader(aprSupplyStart+2*86400, 2, 4), builtInFunctions.NewKleverFreezeFunc, APRIdentifier, builtinArg(kept))
	assert.NoError(t, err)
	c.CheckBalance(OwnerAddress, APRIdentifier, 0)
	assert.Less(t, getAPRAsset(t, c).CirculatingSupply, int64(0))
}

func TestStakingTxProcessor_APRClaim_ContractFreezeOverflowingSegments_PostFix(t *testing.T) {
	kept := int64(1_000)

	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})
	stakeAPRSupplyBeforeRaise(t, c, kept)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: 0})

	err := runStakingBuiltin(t, c, c.CreateBlockHeader(aprSupplyStart+2*86400, 2, 4), builtInFunctions.NewKleverFreezeFunc, APRIdentifier, builtinArg(kept))
	assert.NoError(t, err)
	assert.True(t, hasAPRGainDroppedReceipt(c.kappController.GetCurrentKAppContext().Receipts().Get()))
	c.CheckBalance(OwnerAddress, APRIdentifier, 0)
	assert.Equal(t, aprFixedSupply, getAPRAsset(t, c).CirculatingSupply)
}

func TestStakingTxProcessor_APRClaim_ContractUnfreezeSaturatedGain_PreFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})

	stakedAt := stakeAPRSupplyAfterRaise(t, c, aprFixedSupply, aprFixedSupply, math.MaxUint32)

	err := runStakingBuiltin(t, c, c.CreateBlockHeader(stakedAt+2*86400, 2, 3), builtInFunctions.NewKleverUnfreezeFunc, APRIdentifier, APRIdentifier)
	assert.NoError(t, err)
	c.CheckBalance(OwnerAddress, APRIdentifier, math.MaxInt64)
	assert.Less(t, getAPRAsset(t, c).CirculatingSupply, int64(0))
}

func TestStakingTxProcessor_APRClaim_ContractUnfreezeSaturatedGain_PostFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})

	stakedAt := stakeAPRSupplyAfterRaise(t, c, aprFixedSupply, aprFixedSupply, math.MaxUint32)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: 0})

	blk := c.CreateBlockHeader(stakedAt+2*86400, 2, 3)
	err := runStakingBuiltin(t, c, blk, builtInFunctions.NewKleverUnfreezeFunc, APRIdentifier, APRIdentifier)
	assert.NoError(t, err)
	assert.True(t, hasAPRGainDroppedReceipt(c.kappController.GetCurrentKAppContext().Receipts().Get()))
	c.CheckBalance(OwnerAddress, APRIdentifier, 0)

	c.RunWithdrawTX(blk, OwnerAddress, APRIdentifier, nil)
	c.CheckBalance(OwnerAddress, APRIdentifier, aprFixedSupply)
	assert.Equal(t, aprFixedSupply, getAPRAsset(t, c).CirculatingSupply)
}

func runContractUpdateStaking(t *testing.T, c *Controller, apr uint32) error {
	t.Helper()

	newAssetTrigger := func(funcGasCost uint64, m vmcommon.Marshalizer, a state.AccountsCacher, f core.ForkController, k kapp.KAppController) (vmcommon.BuiltinFunction, error) {
		return builtInFunctions.NewKleverAssetTriggerFunc(funcGasCost, 0, m, a, f, k)
	}

	return runStakingBuiltin(t, c, c.CreateBlockHeader(aprSupplyStart, 1, 1), newAssetTrigger,
		builtinArg(int64(transaction.AssetTriggerContract_UpdateStaking)),
		APRIdentifier,
		builtinArg(int64(transaction.StakingInfo_APRI)),
		builtinArg(int64(apr)),
		builtinArg(0),
		builtinArg(0),
		builtinArg(0),
	)
}

func lastStoredAPR(t *testing.T, c *Controller) uint32 {
	t.Helper()

	aprs := c.GetStakingKDA(APRIdentifier).GetAPR()
	require.NotEmpty(t, aprs)

	return aprs[len(aprs)-1].GetValue()
}

func TestStakingTxProcessor_APRClaim_ContractUpdateStakingAPRLimit_PreFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})

	assert.NoError(t, runContractUpdateStaking(t, c, core.MaxStakingAPR+1))
	assert.Equal(t, core.MaxStakingAPR+1, lastStoredAPR(t, c))
}

func TestStakingTxProcessor_APRClaim_ContractUpdateStakingAPRLimit_PostFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: 0})

	assert.NoError(t, runContractUpdateStaking(t, c, core.MaxStakingAPR))
	assert.Equal(t, core.MaxStakingAPR, lastStoredAPR(t, c))

	assert.Equal(t, common.ErrAPRAboveMaximum, runContractUpdateStaking(t, c, core.MaxStakingAPR+1))
	assert.Equal(t, core.MaxStakingAPR, lastStoredAPR(t, c))
}

func TestStakingTxProcessor_KDA_FPRClaim(t *testing.T) {
	initialKLVBalance := int64(2_000_000_000)
	initialFPRBalance := int64(2_000_000_000)
	freezeFPRAmount := int64(1_000_000)
	depositKLVAmount := int64(301_000_000)

	c := NewController(t)
	c.AddUser(RefAddress, initialKLVBalance, nil)

	c.AddUser(OwnerAddress, initialFPRBalance, FPRIdentifier)
	c.AddUser(OwnerAddress, initialKLVBalance, nil)

	blk := c.CreateBlockHeader(0, 1, 1)
	bucketID := c.RunFreezeTX(blk, OwnerAddress, FPRIdentifier, freezeFPRAmount)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)

	c.CheckFPRStakingFronzen(FPRIdentifier, blk.Header.Epoch+1, freezeFPRAmount, freezeFPRAmount, false)

	blk.Header.Epoch += 1
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KLVIdentifier, depositKLVAmount)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, initialKLVBalance-depositKLVAmount)

	c.CheckFPRStakingFronzen(FPRIdentifier, blk.Header.Epoch+1, freezeFPRAmount, freezeFPRAmount, true)

	blk.Header.Epoch += 1
	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, FPRIdentifier, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance+depositKLVAmount)

	blk.Header.Epoch += 1
	c.RunUnfreezeTX(blk, OwnerAddress, FPRIdentifier, bucketID, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)

	c.CheckFPRStakingFronzen(FPRIdentifier, blk.Header.Epoch-1, 0, freezeFPRAmount, true)

	c.RunWithdrawTX(blk, OwnerAddress, FPRIdentifier, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance)
}

func TestStakingTxProcessor_KDA_FPRClaim_Unfreeze_Before_Epoch(t *testing.T) {
	initialKLVBalance := int64(2_000_000_000)
	initialFPRBalance := int64(2_000_000_000)
	freezeFPRAmount := int64(1_000_000)
	depositKLVAmount := int64(301_000_000)

	c := NewController(t)
	c.AddUser(RefAddress, initialKLVBalance, nil)

	c.AddUser(OwnerAddress, initialFPRBalance, FPRIdentifier)
	c.AddUser(OwnerAddress, initialKLVBalance, nil)

	blk := c.CreateBlockHeader(0, 1, 1)
	bucketID := c.RunFreezeTX(blk, OwnerAddress, FPRIdentifier, freezeFPRAmount)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)

	blk.Header.Epoch += 1
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KLVIdentifier, depositKLVAmount)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, initialKLVBalance-depositKLVAmount)

	c.CheckFPRStakingFronzen(FPRIdentifier, blk.Header.Epoch+1, freezeFPRAmount, freezeFPRAmount, true)

	c.RunUnfreezeTX(blk, OwnerAddress, FPRIdentifier, bucketID, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)

	c.CheckFPRStakingFronzen(FPRIdentifier, blk.Header.Epoch+1, 0, 0, true)

	blk.Header.Epoch += 1
	c.RunWithdrawTX(blk, OwnerAddress, FPRIdentifier, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)
}

func TestStakingTxProcessor_KDA_FPRClaim_Unfreeze_Before_Deposit(t *testing.T) {
	initialKLVBalance := int64(2_000_000_000)
	initialFPRBalance := int64(2_000_000_000)
	freezeFPRAmount := int64(1_000_000)
	depositKLVAmount := int64(301_000_000)

	c := NewController(t)
	c.AddUser(RefAddress, initialKLVBalance, nil)

	c.AddUser(OwnerAddress, initialFPRBalance, FPRIdentifier)
	c.AddUser(OwnerAddress, initialKLVBalance, nil)

	blk := c.CreateBlockHeader(0, 1, 1)
	bucketID := c.RunFreezeTX(blk, OwnerAddress, FPRIdentifier, freezeFPRAmount)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)

	c.CheckFPRStakingFronzen(FPRIdentifier, blk.Header.Epoch+1, freezeFPRAmount, freezeFPRAmount, false)

	blk.Header.Epoch += 1
	c.RunUnfreezeTX(blk, OwnerAddress, FPRIdentifier, bucketID, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)

	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KLVIdentifier, depositKLVAmount)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, initialKLVBalance-depositKLVAmount)

	blk.Header.Epoch += 1
	c.RunWithdrawTX(blk, OwnerAddress, FPRIdentifier, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)
}

func TestStakingTxProcessor_KDA_FPRClaim_Unfreeze_Before_Maturity_PreFix(t *testing.T) {
	// Documents the pre-fix (FixStakingBuckets only, no FixAuditChangesV3) affected behavior:
	// a staker who unfreezes before claim maturity silently forfeits all pre-unstake FPR rewards,
	// which are then recoverable by the depositor via the FPR expiry refund path (value misdirection).
	initialKLVBalance := int64(2_000_000_000)
	initialFPRBalance := int64(2_000_000_000)
	freezeFPRAmount := int64(1_000_000)
	depositKLVAmount := int64(301_000_000)

	c := NewController(t)
	// Activate FixStakingBuckets only; keep FixAuditChangesV3 inactive to reproduce the bug.
	// FixAuditChangesV6 stays inactive too: from it on the expired pool goes to the asset's owner.
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV3: math.MaxUint32, FixAuditChangesV6: math.MaxUint32})
	c.AddUser(RefAddress, initialKLVBalance, nil)
	c.AddUser(OwnerAddress, initialFPRBalance, FPRIdentifier)
	c.AddUser(OwnerAddress, initialKLVBalance, nil)

	blk := c.CreateBlockHeader(0, 1, 1)
	// Scenario precondition: MinEpochsToUnstake (1) < MinEpochsToClaim (5)
	updateStakingContract := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateStaking,
		AssetID:     FPRIdentifier,
		Staking: &transaction.StakingInfo{
			Type:                transaction.StakingInfo_FPRI,
			MinEpochsToClaim:    5,
			MinEpochsToUnstake:  1,
			MinEpochsToWithdraw: 0,
		},
	}
	tx, err := createTransactionMock(&updateStakingContract, transaction.TXContract_AssetTriggerContractType, OwnerAddress, 0)
	require.NoError(t, err)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	require.NoError(t, err)
	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.NoError(t, err)

	// Staker freezes
	bucketID := c.RunFreezeTX(blk, OwnerAddress, FPRIdentifier, freezeFPRAmount)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)

	// Depositor seeds FPR rewards while bucket is active
	blk.Header.Epoch += 2
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KLVIdentifier, depositKLVAmount)
	staking := c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	assert.Equal(t, depositKLVAmount, staking.FPR[0].TotalAmount)
	assert.Zero(t, staking.FPR[0].TotalClaimed)

	// Staker unfreezes before claim window matures — claim is swallowed, no reward paid
	blk.Header.Epoch += 2
	c.RunUnfreezeTX(blk, OwnerAddress, FPRIdentifier, bucketID, nil)
	c.CheckFrozenBalance(OwnerAddress, FPRIdentifier, 0)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance) // no reward at unfreeze

	// After claim window matures: bucket is entirely skipped — ErrClaimNotAvailable (silent loss)
	blk.Header.Epoch += 1
	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, FPRIdentifier,
		state.ErrClaimNotAvailable)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance) // staker receives nothing

	staking = c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	assert.Zero(t, staking.FPR[0].TotalClaimed) // rewards permanently orphaned in pool

	// Withdraw deletes the bucket — no recovery path remains
	c.RunWithdrawTX(blk, OwnerAddress, FPRIdentifier, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)

	ownerAcc := loadUserAccount(c.accCacher, OwnerAddress)
	userKDA, err := ownerAcc.GetUserKDA(FPRIdentifier, nil, true)
	require.NoError(t, err)
	require.Empty(t, userKDA.Buckets)

	staking = c.GetStakingKDA(FPRIdentifier)
	assert.Zero(t, staking.FPR[0].TotalClaimed) // pool still holds unclaimed rewards

	// After FPR epoch expires, depositor reclaims orphaned rewards via a new deposit.
	// Value flows from staker → depositor: the depositor recovers depositKLVAmount
	// by spending only a 1-token trigger deposit.
	blk.Header.Epoch = 105
	expiryDeposit := int64(1)
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KLVIdentifier, expiryDeposit)
	// Depositor net: paid depositKLVAmount + expiryDeposit, received depositKLVAmount refund
	// → effectively only out expiryDeposit while staker lost depositKLVAmount worth of rewards
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, initialKLVBalance-expiryDeposit)
	staking = c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	assert.Equal(t, expiryDeposit, staking.FPR[0].TotalAmount)
	assert.Zero(t, staking.FPR[0].TotalClaimed)
}

func TestStakingTxProcessor_KDA_FPRClaim_Unfreeze_Before_Claim_Maturity_PostFix(t *testing.T) {
	initialKLVBalance := int64(2_000_000_000)
	initialFPRBalance := int64(2_000_000_000)
	freezeFPRAmount := int64(1_000_000)
	depositKLVAmount := int64(301_000_000)

	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV3: 0})
	c.AddUser(RefAddress, initialKLVBalance, nil)
	c.AddUser(OwnerAddress, initialFPRBalance, FPRIdentifier)
	c.AddUser(OwnerAddress, initialKLVBalance, nil)

	blk := c.CreateBlockHeader(0, 1, 1)
	updateStakingContract := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateStaking,
		AssetID:     FPRIdentifier,
		Staking: &transaction.StakingInfo{
			Type:                transaction.StakingInfo_FPRI,
			MinEpochsToClaim:    5,
			MinEpochsToUnstake:  1,
			MinEpochsToWithdraw: 0,
		},
	}
	tx, err := createTransactionMock(&updateStakingContract, transaction.TXContract_AssetTriggerContractType, OwnerAddress, 0)
	require.NoError(t, err)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	require.NoError(t, err)
	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.NoError(t, err)

	bucketID := c.RunFreezeTX(blk, OwnerAddress, FPRIdentifier, freezeFPRAmount)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)

	blk.Header.Epoch += 2
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KLVIdentifier, depositKLVAmount)
	staking := c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	require.Equal(t, uint32(4), staking.FPR[0].Epoch)
	require.Equal(t, depositKLVAmount, staking.FPR[0].TotalAmount)
	require.Equal(t, freezeFPRAmount, staking.FPR[0].TotalStaked)
	require.Zero(t, staking.FPR[0].TotalClaimed)

	blk.Header.Epoch += 2
	c.RunUnfreezeTX(blk, OwnerAddress, FPRIdentifier, bucketID, nil)
	c.CheckFrozenBalance(OwnerAddress, FPRIdentifier, 0)
	ownerAcc := loadUserAccount(c.accCacher, OwnerAddress)
	userKDA, err := ownerAcc.GetUserKDA(FPRIdentifier, nil, true)
	require.NoError(t, err)
	require.Equal(t, uint32(5), userKDA.Buckets[string(FPRIdentifier)].UnstakedEpoch)

	blk.Header.Epoch += 1
	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, FPRIdentifier, nil)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance+depositKLVAmount)
	staking = c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	assert.Equal(t, depositKLVAmount, staking.FPR[0].TotalClaimed)

	blk.Header.Epoch += 1
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KLVIdentifier, depositKLVAmount)
	staking = c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 2)

	blk.Header.Epoch += 5
	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, FPRIdentifier, state.ErrClaimNotAvailable)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance+depositKLVAmount)
	staking = c.GetStakingKDA(FPRIdentifier)
	assert.Equal(t, depositKLVAmount, staking.FPR[0].TotalClaimed)
	assert.Zero(t, staking.FPR[1].TotalClaimed)

	c.RunWithdrawTX(blk, OwnerAddress, FPRIdentifier, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance)
	ownerAcc = loadUserAccount(c.accCacher, OwnerAddress)
	userKDA, err = ownerAcc.GetUserKDA(FPRIdentifier, nil, true)
	require.NoError(t, err)
	require.Empty(t, userKDA.Buckets)
}

func TestStakingTxProcessor_KDA_FPRClaim_Unfreeze_At_Reward_Epoch_PostFix(t *testing.T) {
	const (
		initialKLVBalance = int64(2_000_000_000)
		initialFPRBalance = int64(2_000_000_000)
		freezeFPRAmount   = int64(1_000_000)
		depositKLVAmount  = int64(301_000_000)
	)

	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV3: 0})
	c.AddUser(RefAddress, initialKLVBalance, nil)
	c.AddUser(OwnerAddress, initialFPRBalance, FPRIdentifier)
	c.AddUser(OwnerAddress, initialKLVBalance, nil)

	blk := c.CreateBlockHeader(0, 1, 1)
	updateStakingContract := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateStaking,
		AssetID:     FPRIdentifier,
		Staking: &transaction.StakingInfo{
			Type:                transaction.StakingInfo_FPRI,
			MinEpochsToClaim:    5,
			MinEpochsToUnstake:  1,
			MinEpochsToWithdraw: 0,
		},
	}
	tx, err := createTransactionMock(&updateStakingContract, transaction.TXContract_AssetTriggerContractType, OwnerAddress, 0)
	require.NoError(t, err)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	require.NoError(t, err)
	err = c.execTx.ProcessTransaction(blk, hash, tx)
	require.NoError(t, err)

	bucketID := c.RunFreezeTX(blk, OwnerAddress, FPRIdentifier, freezeFPRAmount)
	blk.Header.Epoch += 1
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KLVIdentifier, depositKLVAmount)

	staking := c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	require.Equal(t, uint32(3), staking.FPR[0].Epoch)

	// Lock in the deliberate > boundary: a reward created at the exact unstake epoch stays claimable.
	blk.Header.Epoch += 1
	c.RunUnfreezeTX(blk, OwnerAddress, FPRIdentifier, bucketID, nil)
	ownerAcc := loadUserAccount(c.accCacher, OwnerAddress)
	userKDA, err := ownerAcc.GetUserKDA(FPRIdentifier, nil, true)
	require.NoError(t, err)
	require.Equal(t, staking.FPR[0].Epoch, userKDA.Buckets[string(FPRIdentifier)].UnstakedEpoch)

	blk.Header.Epoch = userKDA.LastClaim.Epoch + 5
	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, FPRIdentifier, nil)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance+depositKLVAmount)

	staking = c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	assert.Equal(t, depositKLVAmount, staking.FPR[0].TotalClaimed)
}

const (
	topUpInitialKLV = int64(2_000_000_000)
	topUpInitialFPR = int64(2_000_000_000)
	topUpBaseStake  = int64(1_000_000)
	// a one unit top-up is the cheapest possible attempt at pulling the reward
	// forward, so it is what the claim window has to hold against
	topUpIncrement     = int64(1)
	topUpRewardPool    = int64(500_000)
	topUpExpiryDeposit = int64(1)
	topUpClaimWindow   = uint32(5)
	topUpStakeEpoch    = uint32(1)
	topUpRewardEpoch   = uint32(3)
	topUpMatureEpoch   = uint32(6)
	topUpExpiryEpoch   = uint32(104)
)

func stakeFPRAndDepositReward(t *testing.T, c *Controller) *block.Block {
	t.Helper()

	c.AddUser(OwnerAddress, topUpInitialKLV, kdautils.KLVIdentifier)
	c.AddUser(OwnerAddress, topUpInitialFPR, FPRIdentifier)
	c.AddUser(RefAddress, topUpInitialKLV, kdautils.KLVIdentifier)

	blk := c.CreateBlockHeader(0, topUpStakeEpoch, 1)
	updateFPRStakingConfig(t, c, blk, topUpClaimWindow, 1, 0)

	runStakingContractTx(t, c, blk, &transaction.FreezeContract{
		AssetID: FPRIdentifier,
		Amount:  topUpBaseStake,
	}, transaction.TXContract_FreezeContractType, OwnerAddress, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, topUpInitialFPR-topUpBaseStake)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, topUpInitialKLV)

	blk.Header.Epoch = topUpStakeEpoch + 1
	runStakingContractTx(t, c, blk, &transaction.DepositContract{
		DepositType: transaction.DepositContract_FPRDeposit,
		ID:          FPRIdentifier,
		CurrencyID:  kdautils.KLVIdentifier,
		Amount:      topUpRewardPool,
	}, transaction.TXContract_DepositContractType, RefAddress, nil)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, topUpInitialKLV-topUpRewardPool)

	staking := c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	require.Equal(t, topUpRewardEpoch, staking.FPR[0].Epoch)
	require.Equal(t, topUpRewardPool, staking.FPR[0].TotalAmount)
	require.Equal(t, topUpBaseStake, staking.FPR[0].TotalStaked)
	require.Zero(t, staking.FPR[0].TotalClaimed)

	return blk
}

func topUpWhileClaimWindowShut(t *testing.T, c *Controller, blk *block.Block, epoch uint32) {
	t.Helper()

	blk.Header.Epoch = epoch
	runStakingContractTx(t, c, blk, &transaction.FreezeContract{
		AssetID: FPRIdentifier,
		Amount:  topUpIncrement,
	}, transaction.TXContract_FreezeContractType, OwnerAddress, nil)

	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, topUpInitialKLV)
	require.Zero(t, c.GetStakingKDA(FPRIdentifier).FPR[0].TotalClaimed)
}

func topUpBucket(t *testing.T, c *Controller) *kapps.UserBucket {
	t.Helper()

	ownerAcc := loadUserAccount(c.accCacher, OwnerAddress)
	userKDA, err := ownerAcc.GetUserKDA(FPRIdentifier, nil, true)
	require.NoError(t, err)

	return userKDA.Buckets[string(FPRIdentifier)]
}

func claimTopUpReward(t *testing.T, c *Controller, blk *block.Block, expectedErr error) {
	t.Helper()

	blk.Header.Epoch = topUpMatureEpoch
	runStakingContractTx(t, c, blk, &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim,
		ID:        FPRIdentifier,
	}, transaction.TXContract_ClaimContractType, OwnerAddress, expectedErr)
}

func expireTopUpPool(t *testing.T, c *Controller, blk *block.Block) {
	t.Helper()

	blk.Header.Epoch = topUpExpiryEpoch
	runStakingContractTx(t, c, blk, &transaction.DepositContract{
		DepositType: transaction.DepositContract_FPRDeposit,
		ID:          FPRIdentifier,
		CurrencyID:  kdautils.KLVIdentifier,
		Amount:      topUpExpiryDeposit,
	}, transaction.TXContract_DepositContractType, RefAddress, nil)
}

func TestStakingTxProcessor_KDA_FPRClaim_TopUpBeforeMaturity_RawTx_PreFix(t *testing.T) {
	c := NewController(t)
	// FixAuditChangesV6 stays inactive too: from it on the expired pool goes to the asset's owner.
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV5: math.MaxUint32, FixAuditChangesV6: math.MaxUint32})

	blk := stakeFPRAndDepositReward(t, c)
	topUpWhileClaimWindowShut(t, c, blk, topUpRewardEpoch)

	bucket := topUpBucket(t, c)
	require.Equal(t, topUpRewardEpoch, bucket.StakedEpoch)
	require.Equal(t, topUpBaseStake+topUpIncrement, bucket.Value)
	require.Empty(t, bucket.History)

	claimTopUpReward(t, c, blk, state.ErrClaimNotAvailable)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, topUpInitialKLV)
	assert.Zero(t, c.GetStakingKDA(FPRIdentifier).FPR[0].TotalClaimed)

	expireTopUpPool(t, c, blk)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, topUpInitialKLV-topUpExpiryDeposit)
}

func TestStakingTxProcessor_KDA_FPRClaim_TopUpBeforeMaturity_RawTx_PostFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV5: 0})

	blk := stakeFPRAndDepositReward(t, c)
	topUpWhileClaimWindowShut(t, c, blk, topUpRewardEpoch)

	bucket := topUpBucket(t, c)
	require.Equal(t, topUpRewardEpoch, bucket.StakedEpoch)
	require.Equal(t, topUpBaseStake+topUpIncrement, bucket.Value)
	require.Len(t, bucket.History, 1)
	require.Equal(t, topUpStakeEpoch, bucket.History[0].StakedEpoch)
	require.Equal(t, topUpBaseStake, bucket.History[0].Value)

	claimTopUpReward(t, c, blk, nil)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, topUpInitialKLV+topUpRewardPool)
	assert.Equal(t, topUpRewardPool, c.GetStakingKDA(FPRIdentifier).FPR[0].TotalClaimed)

	expireTopUpPool(t, c, blk)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier,
		topUpInitialKLV-topUpRewardPool-topUpExpiryDeposit)
}

const (
	residualRefFPR  = int64(1_000_000)
	residualFPRPool = int64(200_000)
)

func depositFPRReward(t *testing.T, c *Controller, blk *block.Block, sender, currencyID []byte, amount int64) {
	t.Helper()

	runStakingContractTx(t, c, blk, &transaction.DepositContract{
		DepositType: transaction.DepositContract_FPRDeposit,
		ID:          FPRIdentifier,
		CurrencyID:  currencyID,
		Amount:      amount,
	}, transaction.TXContract_DepositContractType, sender, nil)
}

// fundUnclaimedFPRPool has RefAddress, a holder of the deposit role that does not own the asset,
// fund the reward epoch with topUpRewardPool of KLV and residualFPRPool of the staked asset.
// Nobody claims it, so the whole pool is the residual once the epoch expires.
func fundUnclaimedFPRPool(t *testing.T, c *Controller) *block.Block {
	t.Helper()

	c.AddUser(RefAddress, residualRefFPR, FPRIdentifier)
	blk := stakeFPRAndDepositReward(t, c)
	depositFPRReward(t, c, blk, RefAddress, FPRIdentifier, residualFPRPool)

	staking := c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	require.Equal(t, residualFPRPool, staking.FPR[0].KDAS[string(FPRIdentifier)].TotalAmount)

	return blk
}

func checkFPRResidualBalances(c *Controller, refKLV, refFPR, ownerKLV, ownerFPR int64) {
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, refKLV)
	c.CheckBalance(RefAddress, FPRIdentifier, refFPR)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, ownerKLV)
	c.CheckBalance(OwnerAddress, FPRIdentifier, ownerFPR)
}

// Before FixAuditChangesV6 the expired residual of every currency is paid to whoever deposits next.
func TestStakingTxProcessor_KDA_FPRExpiredResidual_PaidToTheNextDepositor_PreFork(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})

	blk := fundUnclaimedFPRPool(t, c)
	expireTopUpPool(t, c, blk)

	checkFPRResidualBalances(c,
		topUpInitialKLV-topUpExpiryDeposit, residualRefFPR,
		topUpInitialKLV, topUpInitialFPR-topUpBaseStake)

	staking := c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	assert.Equal(t, topUpExpiryEpoch+1, staking.FPR[0].Epoch)
}

// From FixAuditChangesV6 on the expired residual of every currency is paid to the staking asset's
// owner, and the depositor that triggered the expiry only pays its own deposit.
func TestStakingTxProcessor_KDA_FPRExpiredResidual_PaidToTheAssetOwner_PostFork(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: 0})

	blk := fundUnclaimedFPRPool(t, c)
	expireTopUpPool(t, c, blk)

	checkFPRResidualBalances(c,
		topUpInitialKLV-topUpRewardPool-topUpExpiryDeposit, residualRefFPR-residualFPRPool,
		topUpInitialKLV+topUpRewardPool, topUpInitialFPR-topUpBaseStake+residualFPRPool)

	staking := c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	assert.Equal(t, topUpExpiryEpoch+1, staking.FPR[0].Epoch)
}

// When the owner is the one that deposits next, the residual reaches the same account on both
// sides of the fork.
func TestStakingTxProcessor_KDA_FPRExpiredResidual_OwnerDepositsNext(t *testing.T) {
	for _, tt := range []struct {
		name string
		v6   uint32
	}{
		{name: "pre fork", v6: math.MaxUint32},
		{name: "post fork", v6: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := NewController(t)
			c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: tt.v6})

			blk := fundUnclaimedFPRPool(t, c)
			blk.Header.Epoch = topUpExpiryEpoch
			depositFPRReward(t, c, blk, OwnerAddress, kdautils.KLVIdentifier, topUpExpiryDeposit)

			checkFPRResidualBalances(c,
				topUpInitialKLV-topUpRewardPool, residualRefFPR-residualFPRPool,
				topUpInitialKLV+topUpRewardPool-topUpExpiryDeposit, topUpInitialFPR-topUpBaseStake+residualFPRPool)
		})
	}
}

// The same pool crosses the fork: the epoch that expires before FixAuditChangesV6 pays the
// depositor, the one that expires after it pays the owner.
func TestStakingTxProcessor_KDA_FPRExpiredResidual_AcrossTheFork(t *testing.T) {
	const (
		secondPool        = int64(300_000)
		secondExpiryEpoch = topUpExpiryEpoch + 102
	)

	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: math.MaxUint32})

	blk := fundUnclaimedFPRPool(t, c)

	// pre fork: the deposit that funds the second pool expires the first one, and collects it
	blk.Header.Epoch = topUpExpiryEpoch
	depositFPRReward(t, c, blk, RefAddress, kdautils.KLVIdentifier, secondPool)
	checkFPRResidualBalances(c,
		topUpInitialKLV-secondPool, residualRefFPR,
		topUpInitialKLV, topUpInitialFPR-topUpBaseStake)

	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV6: 0})

	// post fork: the second pool expires on the next deposit, and goes to the owner
	blk.Header.Epoch = secondExpiryEpoch
	depositFPRReward(t, c, blk, RefAddress, kdautils.KLVIdentifier, topUpExpiryDeposit)
	checkFPRResidualBalances(c,
		topUpInitialKLV-secondPool-topUpExpiryDeposit, residualRefFPR,
		topUpInitialKLV+secondPool, topUpInitialFPR-topUpBaseStake)

	staking := c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	assert.Equal(t, secondExpiryEpoch+1, staking.FPR[0].Epoch)
	assert.Equal(t, topUpExpiryDeposit, staking.FPR[0].TotalAmount)
}

func TestStakingTxProcessor_KDA_FPRClaim_RepeatedTopUpsKeepClaimWindow_PostFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV5: 0})

	blk := stakeFPRAndDepositReward(t, c)

	for epoch := topUpRewardEpoch; epoch < topUpMatureEpoch; epoch++ {
		topUpWhileClaimWindowShut(t, c, blk, epoch)
	}

	bucket := topUpBucket(t, c)
	require.Len(t, bucket.History, int(topUpMatureEpoch-topUpRewardEpoch))
	require.Equal(t, topUpStakeEpoch, bucket.History[0].StakedEpoch)
	require.Equal(t, topUpBaseStake, bucket.History[0].Value)

	claimTopUpReward(t, c, blk, nil)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, topUpInitialKLV+topUpRewardPool)
	assert.Equal(t, topUpRewardPool, c.GetStakingKDA(FPRIdentifier).FPR[0].TotalClaimed)
}

// The fixtures above have a single staker, who takes the whole pool whichever stake the claim is
// priced at -- so they pin that the reward is no longer forfeited, but they cannot tell correct
// historical pricing from pricing at the current stake. This one splits the pool between two
// stakers and then doubles one of them after the pool is fixed. Pricing at the current stake pays
// the owner the entire 500_000 and leaves the rival nothing.
func TestStakingTxProcessor_KDA_FPRClaim_TopUpDoesNotDiluteRival_PostFix(t *testing.T) {
	rivalAddress := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksj")

	// Doubling a 1_000_000 stake in a 2_000_000 pool: the owner's share has to stay half.
	topUpDoubleStake := int64(1_000_000)
	topUpSplitShare := int64(250_000)

	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV5: 0})

	c.AddUser(OwnerAddress, topUpInitialKLV, kdautils.KLVIdentifier)
	c.AddUser(OwnerAddress, topUpInitialFPR, FPRIdentifier)
	c.AddUser(rivalAddress, topUpInitialKLV, kdautils.KLVIdentifier)
	c.AddUser(rivalAddress, topUpInitialFPR, FPRIdentifier)
	c.AddUser(RefAddress, topUpInitialKLV, kdautils.KLVIdentifier)

	blk := c.CreateBlockHeader(0, topUpStakeEpoch, 1)
	updateFPRStakingConfig(t, c, blk, topUpClaimWindow, 1, 0)

	stakers := [][]byte{OwnerAddress, rivalAddress}
	for _, staker := range stakers {
		runStakingContractTx(t, c, blk, &transaction.FreezeContract{
			AssetID: FPRIdentifier,
			Amount:  topUpBaseStake,
		}, transaction.TXContract_FreezeContractType, staker, nil)
	}

	blk.Header.Epoch = topUpStakeEpoch + 1
	runStakingContractTx(t, c, blk, &transaction.DepositContract{
		DepositType: transaction.DepositContract_FPRDeposit,
		ID:          FPRIdentifier,
		CurrencyID:  kdautils.KLVIdentifier,
		Amount:      topUpRewardPool,
	}, transaction.TXContract_DepositContractType, RefAddress, nil)

	staking := c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	require.Equal(t, topUpRewardEpoch, staking.FPR[0].Epoch)
	require.Equal(t, topUpBaseStake*2, staking.FPR[0].TotalStaked)

	blk.Header.Epoch = topUpRewardEpoch
	runStakingContractTx(t, c, blk, &transaction.FreezeContract{
		AssetID: FPRIdentifier,
		Amount:  topUpDoubleStake,
	}, transaction.TXContract_FreezeContractType, OwnerAddress, nil)

	bucket := topUpBucket(t, c)
	require.Equal(t, topUpBaseStake+topUpDoubleStake, bucket.Value)
	require.Len(t, bucket.History, 1)
	require.Equal(t, topUpStakeEpoch, bucket.History[0].StakedEpoch)
	require.Equal(t, topUpBaseStake, bucket.History[0].Value)

	blk.Header.Epoch = topUpMatureEpoch

	// Assert the owner's share before the rival claims. Pricing at the current stake pays the
	// owner the whole pool here, and the theft then surfaces on the rival as "claim not
	// available" -- an error that says nothing about why. Checking the payout first names it.
	runStakingContractTx(t, c, blk, &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim,
		ID:        FPRIdentifier,
	}, transaction.TXContract_ClaimContractType, OwnerAddress, nil)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, topUpInitialKLV+topUpSplitShare)

	runStakingContractTx(t, c, blk, &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim,
		ID:        FPRIdentifier,
	}, transaction.TXContract_ClaimContractType, rivalAddress, nil)
	c.CheckBalance(rivalAddress, kdautils.KLVIdentifier, topUpInitialKLV+topUpSplitShare)

	assert.Equal(t, topUpRewardPool, c.GetStakingKDA(FPRIdentifier).FPR[0].TotalClaimed)
}

// Two gaps the single-currency rival test cannot see:
//
//   - historical pricing must apply to fpr.KDAS reward currencies too, not just the KLV
//     TotalAmount. Swapping stakedValue for bucket.Value in the KDAS loop leaves every KLV
//     assertion green while other currencies overpay after a top-up.
//   - retained history must not pay twice. The owner claims, waits out the cooldown and claims
//     again BEFORE the rival does, so the pool still holds the rival's half and the remaining-
//     pool cap cannot mask a replay.
func TestStakingTxProcessor_KDA_FPRClaim_TopUpMultiCurrencyAndNoReplay_PostFix(t *testing.T) {
	rivalAddress := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksk")

	topUpDoubleStake := int64(1_000_000)
	topUpSplitShare := int64(250_000)
	replayEpoch := topUpMatureEpoch + topUpClaimWindow // next opening after the first claim

	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV5: 0})

	c.AddUser(OwnerAddress, topUpInitialKLV, kdautils.KLVIdentifier)
	c.AddUser(OwnerAddress, topUpInitialFPR, FPRIdentifier)
	c.AddUser(rivalAddress, topUpInitialKLV, kdautils.KLVIdentifier)
	c.AddUser(rivalAddress, topUpInitialFPR, FPRIdentifier)
	c.AddUser(RefAddress, topUpInitialKLV, kdautils.KLVIdentifier)
	c.AddUser(RefAddress, topUpInitialKLV, kdautils.KFIIdentifier)

	blk := c.CreateBlockHeader(0, topUpStakeEpoch, 1)
	updateFPRStakingConfig(t, c, blk, topUpClaimWindow, 1, 0)

	stakers := [][]byte{OwnerAddress, rivalAddress}
	for _, staker := range stakers {
		runStakingContractTx(t, c, blk, &transaction.FreezeContract{
			AssetID: FPRIdentifier, Amount: topUpBaseStake,
		}, transaction.TXContract_FreezeContractType, staker, nil)
	}

	// one pool, two reward currencies
	blk.Header.Epoch = topUpStakeEpoch + 1
	for _, currency := range [][]byte{kdautils.KLVIdentifier, kdautils.KFIIdentifier} {
		runStakingContractTx(t, c, blk, &transaction.DepositContract{
			DepositType: transaction.DepositContract_FPRDeposit, ID: FPRIdentifier,
			CurrencyID: currency, Amount: topUpRewardPool,
		}, transaction.TXContract_DepositContractType, RefAddress, nil)
	}

	staking := c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	require.Equal(t, topUpRewardEpoch, staking.FPR[0].Epoch)
	require.Equal(t, topUpBaseStake*2, staking.FPR[0].TotalStaked)
	require.Contains(t, staking.FPR[0].KDAS, string(kdautils.KFIIdentifier))

	// owner doubles their stake after the pool is fixed, with the claim window shut
	blk.Header.Epoch = topUpRewardEpoch
	runStakingContractTx(t, c, blk, &transaction.FreezeContract{
		AssetID: FPRIdentifier, Amount: topUpDoubleStake,
	}, transaction.TXContract_FreezeContractType, OwnerAddress, nil)
	require.Len(t, topUpBucket(t, c).History, 1)

	// the owner's share must be half in BOTH currencies, priced at the pre-top-up stake
	blk.Header.Epoch = topUpMatureEpoch
	runStakingContractTx(t, c, blk, &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim, ID: FPRIdentifier,
	}, transaction.TXContract_ClaimContractType, OwnerAddress, nil)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, topUpInitialKLV+topUpSplitShare)
	c.CheckBalance(OwnerAddress, kdautils.KFIIdentifier, topUpSplitShare)

	// claiming again must pay nothing while the rival's half is still in the pool
	blk.Header.Epoch = replayEpoch
	runStakingContractTx(t, c, blk, &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim, ID: FPRIdentifier,
	}, transaction.TXContract_ClaimContractType, OwnerAddress, state.ErrClaimNotAvailable)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, topUpInitialKLV+topUpSplitShare)
	c.CheckBalance(OwnerAddress, kdautils.KFIIdentifier, topUpSplitShare)

	// and the rival still receives their full half of each currency
	runStakingContractTx(t, c, blk, &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim, ID: FPRIdentifier,
	}, transaction.TXContract_ClaimContractType, rivalAddress, nil)
	c.CheckBalance(rivalAddress, kdautils.KLVIdentifier, topUpInitialKLV+topUpSplitShare)
	c.CheckBalance(rivalAddress, kdautils.KFIIdentifier, topUpSplitShare)

	staking = c.GetStakingKDA(FPRIdentifier)
	require.Equal(t, topUpRewardPool, staking.FPR[0].TotalClaimed)
	require.Equal(t, topUpRewardPool, staking.FPR[0].KDAS[string(kdautils.KFIIdentifier)].TotalClaimed)
}

// The spec-level tests drive the pre-claim themselves, so they cannot see it being removed
// from the real freeze path. This exercises accounts.Freeze's own ClaimBalance call across the
// full lifecycle: a top-up while the window is shut creates history, a later top-up once the
// window opens is paid out immediately BY THE FREEZE, the cursor moves, and the history the
// payout covered is retired rather than left behind.
func TestStakingTxProcessor_KDA_FPRClaim_FreezePreClaimLifecycle_PostFix(t *testing.T) {
	c := NewController(t)
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV5: 0})

	blk := stakeFPRAndDepositReward(t, c)

	// window shut at the reward epoch: the top-up cannot be paid, so it must be recorded
	topUpWhileClaimWindowShut(t, c, blk, topUpRewardEpoch)
	bucket := topUpBucket(t, c)
	require.Len(t, bucket.History, 1)
	require.Equal(t, topUpStakeEpoch, bucket.History[0].StakedEpoch)
	require.Equal(t, topUpRewardEpoch, bucket.History[0].ThroughEpoch)
	require.Equal(t, topUpBaseStake, bucket.History[0].Value)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, topUpInitialKLV)

	// window open: the freeze's own pre-claim must pay the recorded interval before rewriting
	// the bucket, and must do it at the historical stake
	blk.Header.Epoch = topUpMatureEpoch
	runStakingContractTx(t, c, blk, &transaction.FreezeContract{
		AssetID: FPRIdentifier, Amount: topUpIncrement,
	}, transaction.TXContract_FreezeContractType, OwnerAddress, nil)

	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, topUpInitialKLV+topUpRewardPool)
	require.Equal(t, topUpRewardPool, c.GetStakingKDA(FPRIdentifier).FPR[0].TotalClaimed)

	// the cursor moved, so the interval it covered is retired rather than carried forward
	after := topUpBucket(t, c)
	require.Empty(t, after.History, "the paid interval must not survive the freeze that paid it")
	require.Equal(t, topUpMatureEpoch, after.StakedEpoch)

	// nothing left to claim, and the cooldown restarted from the freeze
	blk.Header.Epoch = topUpMatureEpoch + 1
	runStakingContractTx(t, c, blk, &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim, ID: FPRIdentifier,
	}, transaction.TXContract_ClaimContractType, OwnerAddress, state.ErrClaimNotAvailable)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, topUpInitialKLV+topUpRewardPool)
}

func TestStakingTxProcessor_KDA_FPRClaim_MultipleFreezes(t *testing.T) {
	toAddress1 := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksh")
	toAddress2 := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksi")

	// Balances of depositer
	initialKLVBalance := int64(2_000_000_000)
	initialKFIBalance := int64(1_000_000_000)
	initialAPRBalance := int64(8_000_000_000)

	// Balances of users
	initialFPRBalance := int64(2_000_000_000)
	initialFPRBalance1 := int64(5_000_000_000)
	initialFPRBalance2 := int64(1_000_000_000)

	// FPR freeze amounts and KLV deposit amount
	freezeFPRAmount := int64(1_000_000)
	freezeFPRAmount1 := int64(100_000_000)
	freezeFPRAmount2 := int64(29_000_000)
	depositKLVAmount := int64(301_000_000)

	c := NewController(t)
	c.AddUser(RefAddress, initialKLVBalance, nil)
	c.AddUser(RefAddress, initialKFIBalance, kdautils.KFIIdentifier)
	c.AddUser(RefAddress, initialAPRBalance, APRIdentifier)

	c.AddUser(OwnerAddress, initialKLVBalance, nil)
	c.AddUser(OwnerAddress, initialFPRBalance, FPRIdentifier)

	c.AddUser(toAddress1, initialKLVBalance, nil)
	c.AddUser(toAddress1, initialFPRBalance1, FPRIdentifier)

	c.AddUser(toAddress2, initialKLVBalance, nil)
	c.AddUser(toAddress2, initialFPRBalance2, FPRIdentifier)

	// First Freeze
	blk := c.CreateBlockHeader(0, 1, 1)
	bucketID := c.RunFreezeTX(blk, OwnerAddress, FPRIdentifier, freezeFPRAmount)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)

	// Second Freeze
	blk.Header.Epoch += 1
	bucketID1 := c.RunFreezeTX(blk, toAddress1, FPRIdentifier, freezeFPRAmount1)
	c.CheckBalance(toAddress1, FPRIdentifier, initialFPRBalance1-freezeFPRAmount1)
	c.CheckBalance(toAddress1, kdautils.KLVIdentifier, initialKLVBalance)

	// Third Freeze
	blk.Header.Epoch += 1
	bucketID2 := c.RunFreezeTX(blk, toAddress2, FPRIdentifier, freezeFPRAmount2)
	c.CheckBalance(toAddress2, FPRIdentifier, initialFPRBalance2-freezeFPRAmount2)
	c.CheckBalance(toAddress2, kdautils.KLVIdentifier, initialKLVBalance)

	// Deposit Of KLV
	blk.Header.Epoch += 1
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KLVIdentifier, depositKLVAmount)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, initialKLVBalance-depositKLVAmount)

	// KLV Distribution Calculation
	totalFrozen := float64(freezeFPRAmount + freezeFPRAmount1 + freezeFPRAmount2)
	claimAmount := (float64(freezeFPRAmount) / float64(totalFrozen)) * float64(depositKLVAmount)
	claimAmount1 := (float64(freezeFPRAmount1) / float64(totalFrozen)) * float64(depositKLVAmount)
	claimAmount2 := (float64(freezeFPRAmount2) / float64(totalFrozen)) * float64(depositKLVAmount)

	// Claimed rewards validation of each user
	blk.Header.Epoch += 1
	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, FPRIdentifier, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance+int64(claimAmount))

	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, toAddress1, FPRIdentifier, nil)
	c.CheckBalance(toAddress1, FPRIdentifier, initialFPRBalance1-freezeFPRAmount1)
	c.CheckBalance(toAddress1, kdautils.KLVIdentifier, initialKLVBalance+int64(claimAmount1))

	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, toAddress2, FPRIdentifier, nil)
	c.CheckBalance(toAddress2, FPRIdentifier, initialFPRBalance2-freezeFPRAmount2)
	c.CheckBalance(toAddress2, kdautils.KLVIdentifier, initialKLVBalance+int64(claimAmount2))

	// Ensure no more payments on next epochs
	blk.Header.Epoch += 1
	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, FPRIdentifier, state.ErrClaimNotAvailable)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance+int64(claimAmount))

	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, toAddress1, FPRIdentifier, state.ErrClaimNotAvailable)
	c.CheckBalance(toAddress1, FPRIdentifier, initialFPRBalance1-freezeFPRAmount1)
	c.CheckBalance(toAddress1, kdautils.KLVIdentifier, initialKLVBalance+int64(claimAmount1))

	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, toAddress2, FPRIdentifier, state.ErrClaimNotAvailable)
	c.CheckBalance(toAddress2, FPRIdentifier, initialFPRBalance2-freezeFPRAmount2)
	c.CheckBalance(toAddress2, kdautils.KLVIdentifier, initialKLVBalance+int64(claimAmount2))

	// Unfreeze of the FPR assets
	blk.Header.Epoch += 1
	c.RunUnfreezeTX(blk, OwnerAddress, FPRIdentifier, bucketID, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)

	c.RunUnfreezeTX(blk, toAddress1, FPRIdentifier, bucketID1, nil)
	c.CheckBalance(toAddress1, FPRIdentifier, initialFPRBalance1-freezeFPRAmount1)

	c.RunUnfreezeTX(blk, toAddress2, FPRIdentifier, bucketID2, nil)
	c.CheckBalance(toAddress2, FPRIdentifier, initialFPRBalance2-freezeFPRAmount2)

	// Withdraw of the FPR assets
	c.RunWithdrawTX(blk, OwnerAddress, FPRIdentifier, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance)

	c.RunWithdrawTX(blk, toAddress1, FPRIdentifier, nil)
	c.CheckBalance(toAddress1, FPRIdentifier, initialFPRBalance1)

	c.RunWithdrawTX(blk, toAddress2, FPRIdentifier, nil)
	c.CheckBalance(toAddress2, FPRIdentifier, initialFPRBalance2)
}

func TestStakingTxProcessor_KDA_FPRClaim_MultipleDeposits(t *testing.T) {
	toAddress1 := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksh")
	toAddress2 := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksi")

	// Balances of depositer
	initialKLVBalance := int64(2_000_000_000)
	initialKFIBalance := int64(1_000_000_000)
	initialAPRBalance := int64(8_000_000_000)

	// Balances of users
	initialFPRBalance := int64(2_000_000_000)
	initialFPRBalance1 := int64(5_000_000_000)
	initialFPRBalance2 := int64(1_000_000_000)

	// FPR freeze amounts and KLV deposit amount
	freezeFPRAmount := int64(1_000_000)
	freezeFPRAmount1 := int64(100_000_000)
	freezeFPRAmount2 := int64(29_000_000)

	// Deposit amounts throughout test
	depositKLVAmount := int64(301_000_000)
	depositKLVAmount2 := int64(144_000_000)
	depositKLVAmount3 := int64(222_000_000)
	depositKLVAmount4 := int64(138_000_000)
	depositKFIAmount := int64(17_000_000)
	depositAPRAmount := int64(656_000_000)
	depositAPRAmount2 := int64(256_000_000)

	c := NewController(t)
	c.AddUser(RefAddress, initialKLVBalance, nil)
	c.AddUser(RefAddress, initialKFIBalance, kdautils.KFIIdentifier)
	c.AddUser(RefAddress, initialAPRBalance, APRIdentifier)

	c.AddUser(OwnerAddress, initialKLVBalance, nil)
	c.AddUser(OwnerAddress, initialFPRBalance, FPRIdentifier)

	c.AddUser(toAddress1, initialKLVBalance, nil)
	c.AddUser(toAddress1, initialFPRBalance1, FPRIdentifier)

	c.AddUser(toAddress2, initialKLVBalance, nil)
	c.AddUser(toAddress2, initialFPRBalance2, FPRIdentifier)

	// Freeze 1
	blk := c.CreateBlockHeader(0, 1, 1) // EPOCH 0
	bucketID := c.RunFreezeTX(blk, OwnerAddress, FPRIdentifier, freezeFPRAmount)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)

	// Freeze 2
	blk.Header.Epoch += 1 // EPOCH 1
	bucketID1 := c.RunFreezeTX(blk, toAddress1, FPRIdentifier, freezeFPRAmount1)
	c.CheckBalance(toAddress1, FPRIdentifier, initialFPRBalance1-freezeFPRAmount1)
	c.CheckBalance(toAddress1, kdautils.KLVIdentifier, initialKLVBalance)

	// Deposit KLV
	blk.Header.Epoch += 1 // EPOCH 2
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KLVIdentifier, depositKLVAmount)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, initialKLVBalance-depositKLVAmount)

	// Deposit Asset APR
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, APRIdentifier, depositAPRAmount)
	c.CheckBalance(RefAddress, APRIdentifier, initialAPRBalance-depositAPRAmount)

	// Deposit KLV Again
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KLVIdentifier, depositKLVAmount2)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, initialKLVBalance-depositKLVAmount-depositKLVAmount2)

	// First deposit round distribution calculation
	totalFrozen := float64(freezeFPRAmount + freezeFPRAmount1)
	claimKLVAmountA := int64(float64(freezeFPRAmount) / float64(totalFrozen) * float64(depositKLVAmount+depositKLVAmount2))
	claimKLVAmountA1 := int64(float64(freezeFPRAmount1) / float64(totalFrozen) * float64(depositKLVAmount+depositKLVAmount2))
	claimAPRAmountA := int64(float64(freezeFPRAmount) / float64(totalFrozen) * float64(depositAPRAmount))
	claimAPRAmountA1 := int64(float64(freezeFPRAmount1) / float64(totalFrozen) * float64(depositAPRAmount))

	// Second Deposit of APR
	blk.Header.Epoch += 2 // EPOCH 4
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, APRIdentifier, depositAPRAmount2)
	c.CheckBalance(RefAddress, APRIdentifier, initialAPRBalance-depositAPRAmount-depositAPRAmount2)

	// Second Deposit of KLV
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KLVIdentifier, depositKLVAmount3)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, initialKLVBalance-depositKLVAmount-depositKLVAmount2-depositKLVAmount3)

	// Second deposit round distribution calculation
	claimKLVAmountB := int64(float64(freezeFPRAmount) / float64(totalFrozen) * float64(depositKLVAmount3))
	claimKLVAmountB1 := int64(float64(freezeFPRAmount1) / float64(totalFrozen) * float64(depositKLVAmount3))
	claimAPRAmountB := int64(float64(freezeFPRAmount) / float64(totalFrozen) * float64(depositAPRAmount2))
	claimAPRAmountB1 := int64(float64(freezeFPRAmount1) / float64(totalFrozen) * float64(depositAPRAmount2))

	// Third Deposit of KFI
	blk.Header.Epoch += 1 // EPOCH 5
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KFIIdentifier, depositKFIAmount)
	c.CheckBalance(RefAddress, kdautils.KFIIdentifier, initialKFIBalance-depositKFIAmount)

	// New Freeze middle process
	bucketID2 := c.RunFreezeTX(blk, toAddress2, FPRIdentifier, freezeFPRAmount2)
	c.CheckBalance(toAddress2, FPRIdentifier, initialFPRBalance2-freezeFPRAmount2)
	c.CheckBalance(toAddress2, kdautils.KLVIdentifier, initialKLVBalance)

	totalFrozen += float64(freezeFPRAmount2)

	claimKFIAmountC := int64(float64(freezeFPRAmount) / float64(totalFrozen) * float64(depositKFIAmount))
	claimKFIAmountC1 := int64(float64(freezeFPRAmount1) / float64(totalFrozen) * float64(depositKFIAmount))
	claimKFIAmountC2 := int64(float64(freezeFPRAmount2) / float64(totalFrozen) * float64(depositKFIAmount))

	// Fourth Deposit of KLV
	blk.Header.Epoch += 2 // EPOCH 7
	c.RunDepositTX(blk, RefAddress, FPRIdentifier, kdautils.KLVIdentifier, depositKLVAmount4)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, initialKLVBalance-depositKLVAmount-depositKLVAmount2-depositKLVAmount3-depositKLVAmount4)

	// Fourth deposit round distribution calculation
	claimKLVAmountD := int64(float64(freezeFPRAmount) / float64(totalFrozen) * float64(depositKLVAmount4))
	claimKLVAmountD1 := int64(float64(freezeFPRAmount1) / float64(totalFrozen) * float64(depositKLVAmount4))
	claimKLVAmountD2 := int64(float64(freezeFPRAmount2) / float64(totalFrozen) * float64(depositKLVAmount4))

	// All Claims and validations made
	blk.Header.Epoch += 1 // EPOCH 8
	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, OwnerAddress, FPRIdentifier, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance+claimKLVAmountA+claimKLVAmountB+claimKLVAmountD)
	c.CheckBalance(OwnerAddress, kdautils.KFIIdentifier, claimKFIAmountC)
	c.CheckBalance(OwnerAddress, APRIdentifier, claimAPRAmountA+claimAPRAmountB)

	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, toAddress1, FPRIdentifier, nil)
	c.CheckBalance(toAddress1, FPRIdentifier, initialFPRBalance1-freezeFPRAmount1)
	c.CheckBalance(toAddress1, kdautils.KLVIdentifier, initialKLVBalance+claimKLVAmountA1+claimKLVAmountB1+claimKLVAmountD1)
	c.CheckBalance(toAddress1, kdautils.KFIIdentifier, claimKFIAmountC1)
	c.CheckBalance(toAddress1, APRIdentifier, claimAPRAmountA1+claimAPRAmountB1)

	c.RunClaimTX(blk, transaction.ClaimContract_StakingClaim, toAddress2, FPRIdentifier, nil)
	c.CheckBalance(toAddress2, FPRIdentifier, initialFPRBalance2-freezeFPRAmount2)
	c.CheckBalance(toAddress2, kdautils.KLVIdentifier, initialKLVBalance+claimKLVAmountD2)
	c.CheckBalance(toAddress2, kdautils.KFIIdentifier, claimKFIAmountC2)
	c.CheckBalance(toAddress2, APRIdentifier, 0)

	// All Unfreezes made
	blk.Header.Epoch += 1 // EPOCH 9
	c.RunUnfreezeTX(blk, OwnerAddress, FPRIdentifier, bucketID, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)

	c.RunUnfreezeTX(blk, toAddress1, FPRIdentifier, bucketID1, nil)
	c.CheckBalance(toAddress1, FPRIdentifier, initialFPRBalance1-freezeFPRAmount1)

	c.RunUnfreezeTX(blk, toAddress2, FPRIdentifier, bucketID2, nil)
	c.CheckBalance(toAddress2, FPRIdentifier, initialFPRBalance2-freezeFPRAmount2)

	// All withdrawals made
	c.RunWithdrawTX(blk, OwnerAddress, FPRIdentifier, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance)

	c.RunWithdrawTX(blk, toAddress1, FPRIdentifier, nil)
	c.CheckBalance(toAddress1, FPRIdentifier, initialFPRBalance1)

	c.RunWithdrawTX(blk, toAddress2, FPRIdentifier, nil)
	c.CheckBalance(toAddress2, FPRIdentifier, initialFPRBalance2)
}

// TestStakingTxProcessor_KDA_FPRClaim_UnfreezeBeforeMaturity_RawTx_PreFix reproduces the KLR-08
// PRE-FIX behavior gap: if MinEpochsToClaim > MinEpochsToUnstake,
// letting a staker unfreeze before the claim window matures. Unfreeze swallows
// ErrClaimNotAvailable and marks the bucket unstaked anyway. Because FixAuditChangesV3 is
// disabled here (simulating the code before the fix), computeClaimFPR skips the ENTIRE bucket
// once it is unstaked, regardless of whether the FPR reward epoch preceded the unstake. The
// staker permanently forfeits the reward, which is later reclaimed by the depositor through the
// expired-FPR refund path. Unlike the sibling test with the "_Unfreeze_Before_Maturity_PreFix"
// suffix (which drives the flow via the Controller's Run*TX helpers), this test drives the exact
// tx.Validate/PreProcessTransaction/ProcessBandwidthFee/ProcessTransaction pipeline (via
// runStakingContractTx) for closer parity with real transaction processing.
func TestStakingTxProcessor_KDA_FPRClaim_UnfreezeBeforeMaturity_RawTx_PreFix(t *testing.T) {
	const (
		initialKLVBalance = int64(2_000_000_000)
		initialFPRBalance = int64(2_000_000_000)
		freezeFPRAmount   = int64(1_000_000)
		depositKLVAmount  = int64(500_000)
		expiryDeposit     = int64(1)
	)
	c := NewController(t)
	// Disable FixAuditChangesV3 to reproduce the pre-fix behavior. FixAuditChangesV6 stays
	// inactive too: from it on the expired pool goes to the asset's owner.
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV3: math.MaxUint32, FixAuditChangesV6: math.MaxUint32})
	c.AddUser(OwnerAddress, initialKLVBalance, kdautils.KLVIdentifier)
	c.AddUser(OwnerAddress, initialFPRBalance, FPRIdentifier)
	c.AddUser(RefAddress, initialKLVBalance, kdautils.KLVIdentifier)
	blk := c.CreateBlockHeader(0, 1, 1)
	updateFPRStakingConfig(t, c, blk, 5, 1, 0)
	freezeTx := runStakingContractTx(t, c, blk, &transaction.FreezeContract{
		AssetID: FPRIdentifier,
		Amount:  freezeFPRAmount,
	}, transaction.TXContract_FreezeContractType, OwnerAddress, nil)
	bucketID := freezeTx.Receipts[1].Data[1]
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)
	blk.Header.Epoch = 2
	runStakingContractTx(t, c, blk, &transaction.DepositContract{
		DepositType: transaction.DepositContract_FPRDeposit,
		ID:          FPRIdentifier,
		CurrencyID:  kdautils.KLVIdentifier,
		Amount:      depositKLVAmount,
	}, transaction.TXContract_DepositContractType, RefAddress, nil)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, initialKLVBalance-depositKLVAmount)

	staking := c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	require.Equal(t, uint32(3), staking.FPR[0].Epoch)
	require.Equal(t, depositKLVAmount, staking.FPR[0].TotalAmount)
	require.Equal(t, freezeFPRAmount, staking.FPR[0].TotalStaked)
	require.Zero(t, staking.FPR[0].TotalClaimed)
	blk.Header.Epoch = 4
	runStakingContractTx(t, c, blk, &transaction.UnfreezeContract{
		AssetID:  FPRIdentifier,
		BucketID: bucketID,
	}, transaction.TXContract_UnfreezeContractType, OwnerAddress, nil)
	c.CheckFrozenBalance(OwnerAddress, FPRIdentifier, 0)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)
	ownerAcc := loadUserAccount(c.accCacher, OwnerAddress)
	userKDA, err := ownerAcc.GetUserKDA(FPRIdentifier, nil, true)
	require.NoError(t, err)
	lastClaimEpoch := userKDA.LastClaim.Epoch
	require.Less(t, int(blk.Header.Epoch-lastClaimEpoch), 5)
	require.Equal(t, uint32(4),
		userKDA.Buckets[string(FPRIdentifier)].UnstakedEpoch)
	blk.Header.Epoch = lastClaimEpoch + 5
	runStakingContractTx(t, c, blk, &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim,
		ID:        FPRIdentifier,
	}, transaction.TXContract_ClaimContractType, OwnerAddress,
		state.ErrClaimNotAvailable)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)
	staking = c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	assert.Equal(t, depositKLVAmount, staking.FPR[0].TotalAmount)
	assert.Zero(t, staking.FPR[0].TotalClaimed)
	runStakingContractTx(t, c, blk, &transaction.WithdrawContract{
		WithdrawType: transaction.WithdrawContract_Staking,
		AssetID:      FPRIdentifier,
	}, transaction.TXContract_WithdrawContractType, OwnerAddress, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)
	ownerAcc = loadUserAccount(c.accCacher, OwnerAddress)
	userKDA, err = ownerAcc.GetUserKDA(FPRIdentifier, nil, true)
	require.NoError(t, err)
	require.Empty(t, userKDA.Buckets)
	staking = c.GetStakingKDA(FPRIdentifier)

	require.Len(t, staking.FPR, 1)
	assert.Equal(t, depositKLVAmount, staking.FPR[0].TotalAmount)
	assert.Zero(t, staking.FPR[0].TotalClaimed)
	blk.Header.Epoch = 104
	runStakingContractTx(t, c, blk, &transaction.DepositContract{
		DepositType: transaction.DepositContract_FPRDeposit,
		ID:          FPRIdentifier,
		CurrencyID:  kdautils.KLVIdentifier,
		Amount:      expiryDeposit,
	}, transaction.TXContract_DepositContractType, RefAddress, nil)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, initialKLVBalance-expiryDeposit)
	staking = c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	assert.Equal(t, uint32(105), staking.FPR[0].Epoch)
	assert.Equal(t, expiryDeposit, staking.FPR[0].TotalAmount)
	assert.Zero(t, staking.FPR[0].TotalClaimed)
}

// TestStakingTxProcessor_KDA_FPRClaim_UnfreezeBeforeMaturity_RawTx_PostFix is the POST-FIX counterpart of
// TestStakingTxProcessor_KDA_FPRClaim_UnfreezeBeforeMaturity_RawTx_PreFix. With FixAuditChangesV3 enabled,
// computeClaimFPR only skips FPR epochs that occurred AFTER the bucket was unstaked
// (fpr.Epoch > bucket.UnstakedEpoch), instead of skipping the entire bucket. Since the FPR
// reward epoch (3) precedes the unstake epoch (4), the staker can still claim it even though
// they unfroze before the claim window matured. No value is misdirected to the depositor.
func TestStakingTxProcessor_KDA_FPRClaim_UnfreezeBeforeMaturity_RawTx_PostFix(t *testing.T) {
	const (
		initialKLVBalance = int64(2_000_000_000)
		initialFPRBalance = int64(2_000_000_000)
		freezeFPRAmount   = int64(1_000_000)
		depositKLVAmount  = int64(500_000)
	)
	c := NewController(t)
	// Explicitly enable FixAuditChangesV3 (active from epoch 0) to validate the fix.
	c.UpdateForkController(config.EnableEpochs{FixAuditChangesV3: 0})
	c.AddUser(OwnerAddress, initialKLVBalance, kdautils.KLVIdentifier)
	c.AddUser(OwnerAddress, initialFPRBalance, FPRIdentifier)
	c.AddUser(RefAddress, initialKLVBalance, kdautils.KLVIdentifier)
	blk := c.CreateBlockHeader(0, 1, 1)
	updateFPRStakingConfig(t, c, blk, 5, 1, 0)
	freezeTx := runStakingContractTx(t, c, blk, &transaction.FreezeContract{
		AssetID: FPRIdentifier,
		Amount:  freezeFPRAmount,
	}, transaction.TXContract_FreezeContractType, OwnerAddress, nil)
	bucketID := freezeTx.Receipts[1].Data[1]
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance-freezeFPRAmount)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)
	blk.Header.Epoch = 2
	runStakingContractTx(t, c, blk, &transaction.DepositContract{
		DepositType: transaction.DepositContract_FPRDeposit,
		ID:          FPRIdentifier,
		CurrencyID:  kdautils.KLVIdentifier,
		Amount:      depositKLVAmount,
	}, transaction.TXContract_DepositContractType, RefAddress, nil)
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, initialKLVBalance-depositKLVAmount)

	staking := c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	require.Equal(t, uint32(3), staking.FPR[0].Epoch)
	require.Equal(t, depositKLVAmount, staking.FPR[0].TotalAmount)
	require.Equal(t, freezeFPRAmount, staking.FPR[0].TotalStaked)
	require.Zero(t, staking.FPR[0].TotalClaimed)

	// Unfreeze before claim maturity (same pre-fix validation window).
	blk.Header.Epoch = 4
	runStakingContractTx(t, c, blk, &transaction.UnfreezeContract{
		AssetID:  FPRIdentifier,
		BucketID: bucketID,
	}, transaction.TXContract_UnfreezeContractType, OwnerAddress, nil)
	c.CheckFrozenBalance(OwnerAddress, FPRIdentifier, 0)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance)
	ownerAcc := loadUserAccount(c.accCacher, OwnerAddress)
	userKDA, err := ownerAcc.GetUserKDA(FPRIdentifier, nil, true)
	require.NoError(t, err)
	lastClaimEpoch := userKDA.LastClaim.Epoch
	require.Less(t, int(blk.Header.Epoch-lastClaimEpoch), 5)
	require.Equal(t, uint32(4),
		userKDA.Buckets[string(FPRIdentifier)].UnstakedEpoch)

	// After the claim window matures, the pre-unstake FPR reward (epoch 3 <= UnstakedEpoch 4)
	// is now claimable: the staker receives the deposit instead of forfeiting it.
	blk.Header.Epoch = lastClaimEpoch + 5
	runStakingContractTx(t, c, blk, &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim,
		ID:        FPRIdentifier,
	}, transaction.TXContract_ClaimContractType, OwnerAddress, nil)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance+depositKLVAmount)
	staking = c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	assert.Equal(t, depositKLVAmount, staking.FPR[0].TotalAmount)
	assert.Equal(t, depositKLVAmount, staking.FPR[0].TotalClaimed)

	// Withdraw returns the full principal; no leftover reward remains to be misdirected.
	runStakingContractTx(t, c, blk, &transaction.WithdrawContract{
		WithdrawType: transaction.WithdrawContract_Staking,
		AssetID:      FPRIdentifier,
	}, transaction.TXContract_WithdrawContractType, OwnerAddress, nil)
	c.CheckBalance(OwnerAddress, FPRIdentifier, initialFPRBalance)
	c.CheckBalance(OwnerAddress, kdautils.KLVIdentifier, initialKLVBalance+depositKLVAmount)
	ownerAcc = loadUserAccount(c.accCacher, OwnerAddress)
	userKDA, err = ownerAcc.GetUserKDA(FPRIdentifier, nil, true)
	require.NoError(t, err)
	require.Empty(t, userKDA.Buckets)

	// The FPR entry is now fully claimed, so a later expiry deposit finds nothing left to
	// refund to the depositor: the depositor only recovers what they deposit going forward.
	staking = c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	assert.Equal(t, depositKLVAmount, staking.FPR[0].TotalClaimed)
	blk.Header.Epoch = 104
	const expiryDeposit = int64(1)
	runStakingContractTx(t, c, blk, &transaction.DepositContract{
		DepositType: transaction.DepositContract_FPRDeposit,
		ID:          FPRIdentifier,
		CurrencyID:  kdautils.KLVIdentifier,
		Amount:      expiryDeposit,
	}, transaction.TXContract_DepositContractType, RefAddress, nil)
	// Depositor is only out the new expiry deposit; no orphaned reward is refunded to them.
	c.CheckBalance(RefAddress, kdautils.KLVIdentifier, initialKLVBalance-depositKLVAmount-expiryDeposit)
	staking = c.GetStakingKDA(FPRIdentifier)
	require.Len(t, staking.FPR, 1)
	assert.Equal(t, uint32(105), staking.FPR[0].Epoch)
	assert.Equal(t, expiryDeposit, staking.FPR[0].TotalAmount)
	assert.Zero(t, staking.FPR[0].TotalClaimed)
}

// runStakingContractTx drives a single contract through the exact production transaction
// pipeline (Validate -> PreProcessTransaction -> ProcessBandwidthFee -> ProcessTransaction),
// asserting that ProcessTransaction returns expectedErr (nil for success).
func runStakingContractTx(
	t *testing.T,
	c *Controller,
	blk *block.Block,
	contract proto.Message,
	contractType transaction.TXContract_ContractType,
	sender []byte,
	expectedErr error,
) *transaction.Transaction {
	t.Helper()
	ownerAcc := loadUserAccount(c.accCacher, sender)
	tx, err := createTransactionMock(contract, contractType, sender,
		ownerAcc.GetNonce())
	require.NoError(t, err)
	require.NoError(t, tx.Validate(c.kappController.GetForkController()))
	ownerAcc, hash, err := c.execTx.PreProcessTransaction(tx)
	require.NoError(t, err)
	_, err = c.execTx.ProcessBandwidthFee(hash, tx, ownerAcc)
	require.NoError(t, err)
	err = c.execTx.ProcessTransaction(blk, hash, tx)
	if expectedErr != nil {
		require.ErrorIs(t, err, expectedErr)
	} else {
		require.NoError(t, err)
	}
	return tx
}

// updateFPRStakingConfig sends an AssetTriggerContract that sets the FPR staking maturity
// thresholds (MinEpochsToClaim/Unstake/Withdraw) for FPRIdentifier, then asserts they were saved.
func updateFPRStakingConfig(t *testing.T, c *Controller, blk *block.Block, claim,
	unstake, withdraw uint32) {
	t.Helper()
	updateStakingContract := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateStaking,
		AssetID:     FPRIdentifier,
		Staking: &transaction.StakingInfo{
			Type:                transaction.StakingInfo_FPRI,
			MinEpochsToClaim:    claim,
			MinEpochsToUnstake:  unstake,
			MinEpochsToWithdraw: withdraw,
		},
	}
	runStakingContractTx(
		t,
		c,
		blk,
		&updateStakingContract,
		transaction.TXContract_AssetTriggerContractType,
		OwnerAddress,
		nil,
	)
	staking := c.GetStakingKDA(FPRIdentifier)
	require.Equal(t, claim, staking.MinEpochsToClaim)
	require.Equal(t, unstake, staking.MinEpochsToUnstake)
	require.Equal(t, withdraw, staking.MinEpochsToWithdraw)
}
