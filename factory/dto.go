package factory

import (
	"math/big"
	"sync"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

// NetworkComponents struct holds the network components
type NetworkComponents struct {
	NetMessenger           p2p.Messenger
	InputAntifloodHandler  P2PAntifloodHandler
	OutputAntifloodHandler P2PAntifloodHandler
	PeerBlackListHandler   process.PeerBlackListCacher
	PkTimeCache            process.TimeCacher
}

// CryptoParams is a DTO for holding block signing parameters
type CryptoParams struct {
	KeyGenerator    crypto.KeyGenerator
	PrivateKey      crypto.PrivateKey
	PublicKey       crypto.PublicKey
	PublicKeyBytes  []byte
	PublicKeyString string
}

// CoreComponents is the DTO used for core components
type CoreComponents struct {
	Hasher                   hashing.Hasher
	InternalMarshalizer      marshal.Marshalizer
	TxSignMarshalizer        marshal.Marshalizer
	Uint64ByteSliceConverter typeConverters.Uint64ByteSliceConverter
	StatusHandler            core.AppStatusHandler
	ChainID                  []byte
	MinTransactionVersion    uint32
	TxSignHasher             hashing.Hasher
	WasmVMChangeLocker       *sync.RWMutex
}

// DataComponents struct holds the data components
type DataComponents struct {
	Blkc     data.ChainHandler
	Store    retriever.StorageService
	Datapool retriever.PoolsHolder
}

// TriesComponents holds the tries components
type TriesComponents struct {
	TriesContainer      state.TriesHolder
	TrieStorageManagers map[string]data.StorageManager
}

// StateComponents struct holds the state components of the Klever protocol
type StateComponents struct {
	AddressPubkeyConverter   core.PubkeyConverter
	ValidatorPubkeyConverter core.PubkeyConverter
	PeersAdapter             state.AccountsAdapter
	AccountsAdapter          state.AccountsAdapter
	KAppsAdapter             state.AccountsAdapter
	KAppController           kapp.KAppController
	KAppControllerSimulator  kapp.KAppController
	// KAppArgs are the assembled arguments the controllers above were built from,
	// so callers needing their own controller (e.g. the VM query elements in
	// cmd/node/sc.go) can reuse them instead of re-threading each dependency.
	KAppArgs          kappcontroller.ArgsNewKApp
	InBalanceForShard map[string]*big.Int
}
