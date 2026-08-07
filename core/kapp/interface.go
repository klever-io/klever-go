package kapp

import (
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/sharding"
)

type RateChange struct {
	Leader    uint32
	Validator uint32
}

type ValidatorsKapp interface {
	SetKAppController(controller KAppController) error
	Register(tc *transaction.CreateValidatorContract) (transaction.Transaction_TXResultCode, error)
	UpdateValidator(sender []byte, tc *transaction.ValidatorConfigContract) (transaction.Transaction_TXResultCode, error)
	Delegate(sender []byte, blockTime int64, blockEpoch uint32, tc *transaction.DelegateContract) (transaction.Transaction_TXResultCode, [][]byte, error)
	Undelegate(blockEpoch uint32, validator []byte, sender []byte, tc *transaction.UndelegateContract) (transaction.Transaction_TXResultCode, error)
	Unjail(sender []byte, tc *transaction.UnjailContract) (transaction.Transaction_TXResultCode, error)
	ProcessEconomicsEndOfEpoch(currentEpoch uint32, validatorInfos []*state.ValidatorInfo) error
	SaveUpdatesForNodesMap(nodeOwners [][]byte, peerType string) (bool, error)
	GetValidatorsInfo(validators [][]byte) ([]ValidatorAccountInfoHandler, error)
	DecreaseTempRating(validator []byte, isProposer bool) error
	SetRater(rater sharding.PeerAccountListAndRatingHandler)
	SetAccountsCacher(cacher state.AccountsCacher) error
	GetAccountsCacher() state.AccountsCacher

	UpdateMissedBlocksCounters(mb map[string]RateChange) error
	ResetValidatorStatisticsAtNewEpoch(vInfos []*state.ValidatorInfo) ([]*state.ValidatorInfo, error)
	ProcessRatingsEndOfEpoch(validatorInfos []*state.ValidatorInfo) error
	UpdateValidatorInfoOnSuccessfulBlock(validatorList []sharding.Validator, signingBitmap []byte, accumulatedFees int64) error
	DecreaseAll(validators [][]byte, missedSlots uint64, consensusGroupSize int) error
	DisplayRating(owners [][]byte)

	// V2 Epoch Rewards methods - for scalable rewards distribution
	GetPendingRewards(address []byte) (int64, error)
	ClaimPendingRewards(address []byte) (int64, error)
	// GetPendingRewardsTotal returns the sum of all unclaimed PREW in the Validators KApp trie.
	GetPendingRewardsTotal() (int64, error)

	IsInterfaceNil() bool
}

type ValidatorAccountInfoHandler interface {
	GetOwnerAddress() []byte
	GetRewardsAddress() []byte
	GetRegisterNonce() uint64
	GetSelfStaked() bool
	GetSelfStake() int64
	GetTotalStake() int64
	GetBlsPubKey() []byte
	GetJailedEpoch() uint32
	GetJailed() bool
	GetWaiting() bool
	GetNumJailed() uint32
	GetTotalSlash() int64

	GetCanDelegate() bool
	GetMaxDelegation() int64
	GetCommission() uint32
	GetTotalRewards() int64
	GetName() string
	GetLogo() string
	GetURIs() map[string]string
	GetStats() state.PeerAccountHandler
}

type KDAFeesPoolKapp interface {
	SetKAppController(controller KAppController) error
	SetAccountsCacher(cacher state.AccountsCacher) error
	Compute(klvAmount int64, info data.KDAFeeHandler) (int64, error)
	ComputeUncached(klvAmount int64, info data.KDAFeeHandler) (int64, error)
	Swap(sender state.UserAccountHandler, klvAmount int64, info data.KDAFeeHandler) error
	Validate(klvFee int64, info data.KDAFeeHandler) error
	ChangePoolOwner(poolID []byte, sender []byte, newOwner []byte) (transaction.Transaction_TXResultCode, error)
	GetPoolOwner(assetID []byte) ([]byte, error)
	UpdatePool(poolID []byte, assetOwner []byte, sender []byte, info *transaction.KDAPoolInfo) (transaction.Transaction_TXResultCode, error)
	Deposit(sender []byte, tc *transaction.DepositContract) (transaction.Transaction_TXResultCode, error)
	Withdraw(sender []byte, tc *transaction.WithdrawContract) (transaction.Transaction_TXResultCode, error)
	IsInterfaceNil() bool
}

type SystemAccountKapp interface {
	SetKAppController(controller KAppController) error
	SetAccountsCacher(cacher state.AccountsCacher) error
	SFTSetMetadata(asset, nonce []byte, args [][]byte) error
	SFTAddCirculation(asset, nonce []byte, amount int64) error
	SFTCreateMeta(asset, nonce []byte, supply int64, hash []byte) error
	SFTGetMeta(asset, nonce []byte) (*kapps.MetaV2, error)
	SFTGetMetaUncached(asset, nonce []byte) (*kapps.MetaV2, error)
	IsInterfaceNil() bool
}

type AccountsKapp interface {
	SetKAppController(controller KAppController) error
	SetAccountsCacher(cacher state.AccountsCacher) error
	GetAccountsCacher() state.AccountsCacher
	Transfer(cType transaction.TXContract_ContractType, sender []byte, tc *transaction.TransferContract) (transaction.Transaction_TXResultCode, error)
	Freeze(sender []byte, tc *transaction.FreezeContract) (transaction.Transaction_TXResultCode, error)
	Unfreeze(sender []byte, tc *transaction.UnfreezeContract) (transaction.Transaction_TXResultCode, error)
	Delegate(sender []byte, tc *transaction.DelegateContract) (transaction.Transaction_TXResultCode, error)
	Undelegate(sender []byte, tc *transaction.UndelegateContract) (transaction.Transaction_TXResultCode, error)
	Withdraw(sender []byte, tc *transaction.WithdrawContract) (transaction.Transaction_TXResultCode, error)
	ClaimStaking(sender []byte, tc *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error)
	ClaimAllowance(sender []byte, tc *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error)
	SetAccountName(sender []byte, tc *transaction.SetAccountNameContract) (transaction.Transaction_TXResultCode, error)
	UpdatePermission(authorizer []byte, target []byte, tc *transaction.UpdateAccountPermissionContract) (transaction.Transaction_TXResultCode, error)
	IsInterfaceNil() bool
}

type ITOKapp interface {
	SetKAppController(controller KAppController) error
	SetAccountsCacher(cacher state.AccountsCacher) error
	GetAccountsCacher() state.AccountsCacher
	Buy(sender []byte, tc *transaction.BuyContract) (transaction.Transaction_TXResultCode, error)
	Config(sender []byte, tc *transaction.ConfigITOContract) (transaction.Transaction_TXResultCode, error)
	Trigger(sender []byte, tc *transaction.ITOTriggerContract) (transaction.Transaction_TXResultCode, error)
	SetPrices(sender []byte, tc *transaction.SetITOPricesContract) (transaction.Transaction_TXResultCode, error)
	IsInterfaceNil() bool
}

type MarketKapp interface {
	SetKAppController(controller KAppController) error
	SetAccountsCacher(cacher state.AccountsCacher) error
	GetAccountsCacher() state.AccountsCacher
	GetMarketplace(marketplaceID []byte) (state.KAppAccountHandler, *kapps.Marketplace, error)
	GetMarketOrder(orderID []byte) (state.KAppAccountHandler, *kapps.MarketOrderData, error)
	// GetMarketEscrowTotal returns the total KLV escrowed across open market orders.
	GetMarketEscrowTotal() (int64, error)
	Buy(sender []byte, tc *transaction.BuyContract) (transaction.Transaction_TXResultCode, error)
	Claim(sender []byte, tc *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error)
	Sell(sender []byte, tc *transaction.SellContract) (transaction.Transaction_TXResultCode, error)
	CancelOrder(sender []byte, tc *transaction.CancelMarketOrderContract) (transaction.Transaction_TXResultCode, error)
	CreateMarketplace(sender []byte, tc *transaction.CreateMarketplaceContract) (transaction.Transaction_TXResultCode, error)
	ConfigMarketplace(sender []byte, tc *transaction.ConfigMarketplaceContract) (transaction.Transaction_TXResultCode, error)
	IsInterfaceNil() bool
}

type KDAKapp interface {
	SetKAppController(controller KAppController) error
	SetAccountsCacher(cacher state.AccountsCacher) error
	GetAccountsCacher() state.AccountsCacher
	GetKDA(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error)
	SetKDA(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error
	GetStaking(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error)
	SetStaking(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error
	Mint(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error)
	Burn(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error)
	Create(sender []byte, tc *transaction.CreateAssetContract) (transaction.Transaction_TXResultCode, error)
	Trigger(sender []byte, tc *transaction.AssetTriggerContract, txData [][]byte) (transaction.Transaction_TXResultCode, error)
	Deposit(sender []byte, tc *transaction.DepositContract) (transaction.Transaction_TXResultCode, error)
	IsInterfaceNil() bool
}

type ProposalKapp interface {
	SetKAppController(controller KAppController) error
	SetAccountsCacher(cacher state.AccountsCacher) error
	GetAccountsCacher() state.AccountsCacher
	GetProposal(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error)
	SetProposal(proposalKapp state.KAppAccountHandler, proposalID uint64, proposal *kapps.ProposalData, controller *kapps.ProposalController) error
	Create(sender []byte, tc *transaction.ProposalContract) (transaction.Transaction_TXResultCode, error)
	Vote(tsender []byte, c *transaction.VoteContract) (transaction.Transaction_TXResultCode, error)
	IsInterfaceNil() bool
}

type KAppController interface {
	InitKApps(state.AccountsCacher) error
	GetValidatorsKApp() ValidatorsKapp
	GetKDAFeesPoolKApp() KDAFeesPoolKapp
	GetSystemAccountKApp() SystemAccountKapp
	GetAccountsKApp() AccountsKapp
	GetITOKApp() ITOKapp
	GetMarketKApp() MarketKapp
	GetKDAKApp() KDAKapp
	GetProposalKApp() ProposalKapp
	SetCurrentKAppContext(kappContext KappContext)
	GetCurrentKAppContext() KappContext

	GetProposalController() kapps.ActiveProposalController
	SetProposalController(proposalController kapps.ActiveProposalController) error

	GetForkController() core.ForkController

	// IsReadOnly reports whether this controller is in read-only mode. Set once at
	// construction via ArgsNewKApp.ReadOnly by the VM query path (see
	// cmd/node/sc.go); there is deliberately no setter, so the safety cannot be
	// switched off on a live controller. Enforcement
	// lives in BlockChainHookImpl.ProcessBuiltInFunction, which refuses every
	// built-in when this is true, and in accountsKapp.checkReadOnly for accounts
	// operations reached outside the built-in dispatch. The individual kda, market,
	// ito, proposal and validators KApps do not check this flag themselves.
	IsReadOnly() bool

	IsInterfaceNil() bool
}

type BlockchainHook interface {
	IsPayable(sndAddress []byte, recvAddress []byte) (bool, error)
}

type KappContext interface {
	OriginalSender() []byte
	Receipts() ReceiptsContext
	ContractID() int
	Block() *block.Block
	TxHash() []byte
	TxNonce() uint64
	SetContractID(id int)
	SetReturnData(data [][]byte)
	AddReturnData(data []byte)
	GetAndClearReturnData() [][]byte
	SubGasUsed(gasUsed uint64) error
	GetGasLimit() uint64
	GetExecData() []byte
	IsScSimulation() bool
	SetExecutionTime(duration time.Duration)
	GetExecutionTime() time.Duration
}

// SystemReceiptTypeStart marks the beginning of system receipt types (Debug, Warning, Error)
const SystemReceiptTypeStart = 120

// System receipt type offsets from SystemReceiptTypeStart
const (
	ReceiptTypeDebug   = SystemReceiptTypeStart + iota // 120
	ReceiptTypeWarning                                 // 121
	ReceiptTypeError                                   // 122
)

type ReceiptsContext interface {
	Get() []*transaction.Transaction_Receipt
	GetByType(receiptType int8) []*transaction.Transaction_Receipt
	GetPreserved() []*transaction.Transaction_Receipt
	Add(receipt *transaction.Transaction_Receipt)
	AddError(contractID int, params ...string)
}
