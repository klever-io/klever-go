package core

const ProtoTypeURLPrefix = "github.com/klever-io/klever-go/"

// HeaderType defines the type to be used for the header that is sent
type HeaderType string

const (
	// MetaHeader defines the type of *block.MetaBlock
	MetaHeader HeaderType = "MetaBlock"
	// ShardHeaderV1 defines the type of *block.Header
	ShardHeaderV1 HeaderType = "Header"
	// ShardHeaderV2 defines the type of *block.HeaderV2
	ShardHeaderV2 HeaderType = "HeaderV2"
)

// MegabyteSize represents the size in bytes of a megabyte
const MegabyteSize = 1024 * 1024

// KDAType defines the possible types in case of KDA tokens
type KDAType uint32

const (
	// Fungible defines the token type for KDA fungible tokens
	Fungible KDAType = iota
	// NonFungible defines the token type for KDA non fungible tokens
	NonFungible
)

// FungibleKDA defines the string for the token type of fungible KDA
const FungibleKDA = "FungibleKDA"

// NonFungibleKDA defines the string for the token type of non fungible KDA
const NonFungibleKDA = "NonFungibleKDA"

// SemiFungibleKDA defines the string for the token type of semi fungible KDA
const SemiFungibleKDA = "SemiFungibleKDA"

// MaxRoyalty defines 100% as uint32
const MaxRoyalty = uint32(10000)

// SCDeployInitFunctionName is the key for the function which is called at smart contract deploy time
const SCDeployInitFunctionName = "_init"

// DelegationSystemSCKey is the key under which there is data in case of system delegation smart contracts
const DelegationSystemSCKey = "delegation"

// KDAKeyIdentifier is the key prefix for kda tokens
const KDAKeyIdentifier = "kda"

// KDARoleIdentifier is the key prefix for kda role identifier
const KDARoleIdentifier = "role"

// KDANFTLatestNonceIdentifier is the key prefix for kda latest nonce identifier
const KDANFTLatestNonceIdentifier = "nonce"

// MinMetaTxExtraGasCost is the constant defined for minimum gas value to be sent in meta transaction
const MinMetaTxExtraGasCost = uint64(1_000_000)

// MaxLeafSize represents maximum amount of data which can be saved under one leaf
const MaxLeafSize = uint64(1 << 26) //64MB

// MaxBufferSizeToSendTrieNodes represents max buffer size to send in bytes used when resolving trie nodes
// Every trie node that has a greater size than this constant is considered a large trie node and should be split in
// smaller chunks
const MaxBufferSizeToSendTrieNodes = 1 << 18 //256KB

// MinLenArgumentsKDATransfer defines the min length of arguments for the KDA transfer
const MinLenArgumentsKDATransfer = 2

// MinLenArgumentsAssetTrigger defines the min length of arguments for the KDA asset trigger
const MinLenArgumentsAssetTrigger = 2

// MinLenArgumentsCreateAsset defines the min length of arguments for the KDA create asset
const MinLenArgumentsCreateAsset = 11

// MinLenArgumentsFreeze defines the min length of arguments for the KDA freeze
const MinLenArgumentsFreeze = 2

// MinLenArgumentsUnfreeze defines the min length of arguments for the KDA Unfreeze
const MinLenArgumentsUnfreeze = 2

// MinLenArgumentsDelegate defines the min length of arguments for the KDA Delegate
const MinLenArgumentsDelegate = 2

// MinLenArgumentsUndelegate defines the min length of arguments for the KDA Undelegate
const MinLenArgumentsUndelegate = 1

// MinLenArgumentsWithdraw defines the min length of arguments for the KDA Withdraw
const MinLenArgumentsWithdraw = 3

// MinLenArgumentsClaim defines the min length of arguments for the KDA Claim
const MinLenArgumentsClaim = 2

// MinLenArgumentsVote defines the min length of arguments for the KDA Vote
const MinLenArgumentsVote = 3

// MinLenArgumentsValidatorConfig defines the min length of arguments for the KDA ValidatorConfig
const MinLenArgumentsValidatorConfig = 6

// MinLenArgumentsCreateValidator defines the min length of arguments for the KDA CreateValidator
const MinLenArgumentsCreateValidator = 7

// MinLenArgumentsUpdateAccountPermission defines the min length of arguments for the KDA UpdateAccountPermission
const MinLenArgumentsUpdateAccountPermission = 1

// MinLenArgumentsUnjail defines the min length of arguments for the KDA Unjail
const MinLenArgumentsUnjail = 0

// MinLenArgumentsSetAccountName defines the min length of arguments for the KDA SetAccountName
const MinLenArgumentsSetAccountName = 1

// MinLenArgumentsSell defines the min length of arguments for the KDA Sell
const MinLenArgumentsSell = 7

// MinLenArgumentsBuy defines the min length of arguments for the KDA Buy
const MinLenArgumentsBuy = 4

// MinLenArgumentsProposal defines the min length of arguments for the KDA Proposal
const MinLenArgumentsProposal = 3

// MinLenArgumentsITOTrigger defines the min length of arguments for the KDA ITOTrigger
const MinLenArgumentsITOTrigger = 3

// MinLenArgumentsITOConfig defines the min length of arguments for the KDA ITOConfig
const MinLenArgumentsITOConfig = 9

// MinLenArgumentsDeposit defines the min length of arguments for the KDA Deposit
const MinLenArgumentsDeposit = 4

// MinLenArgumentsCreateMarketplace defines the min length of arguments for the KDA CreateMarketplace
const MinLenArgumentsCreateMarketplace = 3

// MinLenArgumentsConfigMarketplace defines the min length of arguments for the KDA ConfigMarketplace
const MinLenArgumentsConfigMarketplace = 4

// MinLenArgumentsCancelMarketOrder defines the min length of arguments for the KDA CancelMarketOrder
const MinLenArgumentsCancelMarketOrder = 1

// MaxLenForKDAIssueMint defines the maximum length in bytes for the issued/minted balance
const MaxLenForKDAIssueMint = 100

// ReturnDataString represents the field name for total consumed gas
const ReturnDataString = "ReturnData"

// TotalConsumedGasString represents the field name for total consumed gas
const TotalConsumedGasString = "totalConsumedGas"

// BaseOperationCostString represents the field name for base operation costs
const BaseOperationCostString = "BaseOperationCost"

// BuiltInCostString represents the field name for built in operation costs
const BuiltInCostString = "BuiltInCost"

// SCDeployIdentifier is the identifier for a smart contract deploy
const SCDeployIdentifier = "SCDeploy"

// SCUpgradeIdentifier is the identifier for a smart contract upgrade
const SCUpgradeIdentifier = "SCUpgrade"

// WriteLogIdentifier is the identifier for the information log that is generated by a smart contract call/kda transfer
const WriteLogIdentifier = "writeLog"

// SignalErrorOperation is the identifier for the log that is generated when a smart contract is executed but return an error
const SignalErrorOperation = "signalError"

// CompletedTxEventIdentifier is the identifier for the log that is generated when the execution of a smart contract call is done
const CompletedTxEventIdentifier = "completedTxEvent"

// BuiltInFunctionTransfer is the key for the Transfer built in function
const BuiltInFunctionTransfer = "KleverTransfer"

// BuiltInFunctionCreateAsset is the key for the CreateAsset built in function
const BuiltInFunctionCreateAsset = "KleverCreateAsset"

// BuiltInFunctionCreateValidator is the key for the CreateValidator built in function
const BuiltInFunctionCreateValidator = "KleverCreateValidator"

// BuiltInFunctionValidatorConfig is the key for the ValidatorConfig built in function
const BuiltInFunctionValidatorConfig = "KleverValidatorConfig"

// BuiltInFunctionFreeze is the key for the Freeze built in function
const BuiltInFunctionFreeze = "KleverFreeze"

// BuiltInFunctionUnfreeze is the key for the Unfreeze built in function
const BuiltInFunctionUnfreeze = "KleverUnfreeze"

// BuiltInFunctionDelegate is the key for the Delegate built in function
const BuiltInFunctionDelegate = "KleverDelegate"

// BuiltInFunctionUndelegate is the key for the Undelegate built in function
const BuiltInFunctionUndelegate = "KleverUndelegate"

// BuiltInFunctionWithdraw is the key for the Withdraw built in function
const BuiltInFunctionWithdraw = "KleverWithdraw"

// BuiltInFunctionClaim is the key for the Claim built in function
const BuiltInFunctionClaim = "KleverClaim"

// BuiltInFunctionUnjail is the key for the Unjail built in function
const BuiltInFunctionUnjail = "KleverUnjail"

// BuiltInFunctionAssetTrigger is the key for the AssetTrigger built in function
const BuiltInFunctionAssetTrigger = "KleverAssetTrigger"

// BuiltInFunctionSetAccountName is the key for the SetAccountName built in function
const BuiltInFunctionSetAccountName = "KleverSetAccountName"

// BuiltInFunctionProposal is the key for the Proposal built in function
const BuiltInFunctionProposal = "KleverProposal"

// BuiltInFunctionVote is the key for the Vote built in function
const BuiltInFunctionVote = "KleverVote"

// BuiltInFunctionConfigITO is the key for the Config built in function
const BuiltInFunctionConfigITO = "KleverConfigITO"

// BuiltInFunctionBuy is the key for the Buy built in function
const BuiltInFunctionBuy = "KleverBuy"

// BuiltInFunctionSell is the key for the Sell built in function
const BuiltInFunctionSell = "KleverSell"

// BuiltInFunctionCancelMarketOrder is the key for the CancelMarketOrder built in function
const BuiltInFunctionCancelMarketOrder = "KleverCancelMarketOrder"

// BuiltInFunctionCreateMarketplace is the key for the CreateMarketplace built in function
const BuiltInFunctionCreateMarketplace = "KleverCreateMarketplace"

// BuiltInFunctionConfigMarketplace is the key for the ConfigMarketplace built in function
const BuiltInFunctionConfigMarketplace = "KleverConfigMarketplace"

// BuiltInFunctionUpdateAccountPermission is the key for the UpdateAccountPermission built in function
const BuiltInFunctionUpdateAccountPermission = "KleverUpdateAccountPermission"

// BuiltInFunctionDeposit is the key for the Deposit built in function
const BuiltInFunctionDeposit = "KleverDeposit"

// BuiltInFunctionITOTrigger is the key for the ITOTrigger built in function
const BuiltInFunctionITOTrigger = "KleverITOTrigger"

// BuiltInFunctionSaveKeyValue is the key for the save key value built-in function
const BuiltInFunctionSaveKeyValue = "KleverSaveKeyValue"

// TODO: change these ones to the new ones /\

// BuiltInFunctionKDABurn is the key for the electronic standard digital token burn built-in function
const BuiltInFunctionKDABurn = "KDABurn"

// BuiltInFunctionKDALocalMint is the key for the electronic standard digital token local mint built-in function
const BuiltInFunctionKDALocalMint = "KDALocalMint"
