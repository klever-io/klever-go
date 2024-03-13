package process

import (
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/genesis"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

// ArgsGenesisBlockCreator holds the arguments which are needed to create a genesis block
type ArgsGenesisBlockCreator struct {
	GenesisTime              int64
	StartEpochNum            uint32
	Accounts                 state.AccountsAdapter
	PeerAccounts             state.AccountsAdapter
	KAppAccounts             state.AccountsAdapter
	KAppController           kapp.KAppController
	PubkeyConv               core.PubkeyConverter
	InitialNodesSetup        genesis.InitialNodesHandler
	Economics                process.EconomicsDataHandler
	Store                    retriever.StorageService
	Blkc                     data.ChainHandler
	Marshalizer              marshal.Marshalizer
	SignMarshalizer          marshal.Marshalizer
	Hasher                   hashing.Hasher
	Uint64ByteSliceConverter typeConverters.Uint64ByteSliceConverter
	DataPool                 retriever.PoolsHolder
	AccountsParser           genesis.AccountsParser
	Indexer                  process.Indexer
	TxLogsProcessor          process.TransactionLogProcessor
	//HardForkConfig           config.HardforkConfig
	TrieStorageManagers map[string]data.StorageManager
	ChainID             string
	BlockSignKeyGen     crypto.KeyGenerator
	//ImportStartHandler  update.ImportStartHandler
	WorkingDir    string
	GenesisString string
	Preferences   *config.PreferencesConfig
	// created components
	//importHandler update.ImportHandler
}

type InitialSupply struct {
	KLV struct {
		Initial int64
		Max     int64
	}
	KFI struct {
		Initial int64
		Max     int64
	}
}
