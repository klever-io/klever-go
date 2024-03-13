package factory

import (
	"fmt"
	"sync"

	"github.com/klever-io/klever-go/config"
	factoryHasher "github.com/klever-io/klever-go/crypto/hashing/factory"
	"github.com/klever-io/klever-go/statusHandler"
	factoryMarshalizer "github.com/klever-io/klever-go/tools/marshal/factory"
	"github.com/klever-io/klever-go/tools/typeConverters/uint64ByteSlice"
)

// CoreComponentsFactoryArgs holds the arguments needed for creating a core components factory
type CoreComponentsFactoryArgs struct {
	Config                config.Config
	ChainID               []byte
	MinTransactionVersion uint32
}

// CoreComponentsFactory is responsible for creating the core components
type CoreComponentsFactory struct {
	config                config.Config
	chainID               []byte
	MinTransactionVersion uint32
}

// NewCoreComponentsFactory initializes the factory which is responsible to creating core components
func NewCoreComponentsFactory(args CoreComponentsFactoryArgs) *CoreComponentsFactory {
	return &CoreComponentsFactory{
		config:                args.Config,
		chainID:               args.ChainID,
		MinTransactionVersion: args.MinTransactionVersion,
	}
}

// Create creates the core components
func (ccf *CoreComponentsFactory) Create() (*CoreComponents, error) {
	hasher, err := factoryHasher.NewHasher(ccf.config.Hasher.Type)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrHasherCreation, err.Error())
	}

	internalMarshalizer, err := factoryMarshalizer.NewMarshalizer(ccf.config.Marshalizer.Type)
	if err != nil {
		return nil, fmt.Errorf("%w (internal): %s", ErrMarshalizerCreation, err.Error())
	}

	txSignMarshalizer, err := factoryMarshalizer.NewMarshalizer(ccf.config.TxSignMarshalizer.Type)
	if err != nil {
		return nil, fmt.Errorf("%w (tx sign): %s", ErrMarshalizerCreation, err.Error())
	}

	txSignHasher, err := factoryHasher.NewHasher(ccf.config.TxSignHasher.Type)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrHasherCreation, err.Error())
	}

	uint64ByteSliceConverter := uint64ByteSlice.NewBigEndianConverter()

	wasmVMChangeLocker := &sync.RWMutex{}

	return &CoreComponents{
		Hasher:                   hasher,
		InternalMarshalizer:      internalMarshalizer,
		TxSignMarshalizer:        txSignMarshalizer,
		Uint64ByteSliceConverter: uint64ByteSliceConverter,
		StatusHandler:            statusHandler.NewNilStatusHandler(),
		ChainID:                  ccf.chainID,
		MinTransactionVersion:    ccf.MinTransactionVersion,
		TxSignHasher:             txSignHasher,
		WasmVMChangeLocker:       wasmVMChangeLocker,
	}, nil
}
