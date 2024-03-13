package factory

import (
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/core/versioning"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ process.InterceptedDataFactory = (*interceptedTxDataFactory)(nil)

var log = logger.GetOrCreate("interceptors/factory")

type interceptedTxDataFactory struct {
	protoMarshalizer            marshal.Marshalizer
	signMarshalizer             marshal.Marshalizer
	hasher                      hashing.Hasher
	keyGen                      crypto.KeyGenerator
	singleSigner                crypto.SingleSigner
	pubkeyConverter             core.PubkeyConverter
	feeHandler                  process.EconomicsDataHandler
	whiteListerVerifiedTxs      process.WhiteListHandler
	chainID                     []byte
	minTransactionVersion       uint32
	enableSignedTxWithHashEpoch uint32
	//epochStartTrigger           process.EpochStartTriggerHandler
	txSignHasher     hashing.Hasher
	txVersionChecker process.TxVersionCheckerHandler
}

// NewInterceptedTxDataFactory creates an instance of interceptedTxDataFactory
func NewInterceptedTxDataFactory(argument *ArgInterceptedDataFactory) (*interceptedTxDataFactory, error) {
	if argument == nil {
		return nil, process.ErrNilArgumentStruct
	}
	if check.IfNil(argument.ProtoMarshalizer) {
		return nil, process.ErrNilMarshalizer
	}
	if check.IfNil(argument.TxSignMarshalizer) {
		return nil, process.ErrNilMarshalizer
	}
	if check.IfNil(argument.Hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(argument.AccountKeyGen) {
		return nil, common.ErrNilKeyGen
	}
	if check.IfNil(argument.Signer) {
		return nil, common.ErrNilSingleSigner
	}
	if check.IfNil(argument.AddressPubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(argument.FeeHandler) {
		return nil, process.ErrNilEconomicsFeeHandler
	}
	// if check.IfNil(argument.EpochStartTrigger) {
	// 	return nil, common.ErrNilEpochStartTrigger
	// }
	if check.IfNil(argument.WhiteListerVerifiedTxs) {
		return nil, process.ErrNilWhiteListHandler
	}
	if len(argument.ChainID) == 0 {
		return nil, process.ErrInvalidChainID
	}
	if argument.MinTransactionVersion == 0 {
		return nil, process.ErrInvalidTransactionVersion
	}
	if check.IfNil(argument.TxSignHasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(argument.EpochNotifier) {
		return nil, common.ErrNilEpochNotifier
	}

	itdf := &interceptedTxDataFactory{
		protoMarshalizer:       argument.ProtoMarshalizer,
		signMarshalizer:        argument.TxSignMarshalizer,
		hasher:                 argument.Hasher,
		keyGen:                 argument.AccountKeyGen,
		singleSigner:           argument.Signer,
		pubkeyConverter:        argument.AddressPubkeyConv,
		whiteListerVerifiedTxs: argument.WhiteListerVerifiedTxs,
		chainID:                argument.ChainID,
		minTransactionVersion:  argument.MinTransactionVersion,
		feeHandler:             argument.FeeHandler,
		// epochStartTrigger:           argument.EpochStartTrigger,
		txVersionChecker:            versioning.NewTxVersionChecker(argument.MinTransactionVersion),
		enableSignedTxWithHashEpoch: argument.EnableSignTxWithHashEpoch,
		txSignHasher:                argument.TxSignHasher,
	}

	argument.EpochNotifier.RegisterNotifyHandler(itdf)

	return itdf, nil
}

// Create creates instances of InterceptedData by unmarshalling provided buffer
func (itdf *interceptedTxDataFactory) Create(buff []byte) (process.InterceptedData, error) {
	return transaction.NewInterceptedTransaction(
		buff,
		itdf.protoMarshalizer,
		itdf.signMarshalizer,
		itdf.hasher,
		itdf.keyGen,
		itdf.singleSigner,
		itdf.pubkeyConverter,
		itdf.whiteListerVerifiedTxs,
		itdf.chainID,
		itdf.txSignHasher,
		itdf.feeHandler,
		itdf.txVersionChecker,
	)
}

// IsInterfaceNil returns true if there is no value under the interface
func (itdf *interceptedTxDataFactory) IsInterfaceNil() bool {
	return itdf == nil
}

// EpochConfirmed is called whenever a new epoch is confirmed
func (itdf *interceptedTxDataFactory) EpochConfirmed(epoch uint32) {

}
