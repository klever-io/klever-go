package transaction_test

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/crypto/mock"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	pTX "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	err = execTx.SetProposalController(pc)
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

	_ = execTx.SetProposalController(c.proposalController)

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

	err = execTx.SetProposalController(c.proposalController)
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

func (c *Controller) RunClaimTX(blk *block.Block, ty transaction.ClaimContract_EnumClaimType, ownerAddress, assetID []byte, errorType error) {
	claimContract := transaction.ClaimContract{
		ClaimType: ty,
		ID:        assetID,
	}

	tx, _ := createTransactionMock(&claimContract, transaction.TXContract_ClaimContractType, ownerAddress, 0)
	_, hash, err := c.execTx.PreProcessTransaction(tx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, hash, tx)
	assert.Equal(c.t, errorType, err)
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
