package factory

import (
	"fmt"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	"github.com/klever-io/klever-go/crypto/peerSignatureHandler"
	"github.com/klever-io/klever-go/crypto/signing"
	disabledMultiSig "github.com/klever-io/klever-go/crypto/signing/disabled/multisig"
	disabledSig "github.com/klever-io/klever-go/crypto/signing/disabled/singlesig"
	"github.com/klever-io/klever-go/crypto/signing/ed25519"
	"github.com/klever-io/klever-go/crypto/signing/ed25519/singlesig"
	mclMultiSig "github.com/klever-io/klever-go/crypto/signing/mcl/multisig"
	mclSig "github.com/klever-io/klever-go/crypto/signing/mcl/singlesig"
	"github.com/klever-io/klever-go/crypto/signing/multisig"
	storageFactory "github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/check"
)

const disabledSigChecking = "disabled"

// CryptoComponentsFactoryArgs holds the arguments needed for creating crypto components
type CryptoComponentsFactoryArgs struct {
	Config                               config.Config
	NodesConfig                          NodesSetupHandler
	KeyGen                               crypto.KeyGenerator
	PrivKey                              crypto.PrivateKey
	ActivateBLSPubKeyMessageVerification bool
}

type cryptoComponentsFactory struct {
	consensusType                        string
	config                               config.Config
	nodesConfig                          NodesSetupHandler
	keyGen                               crypto.KeyGenerator
	privKey                              crypto.PrivateKey
	activateBLSPubKeyMessageVerification bool
	importDbNoSigCheckFlag               bool
}

// NewCryptoComponentsFactory returns a new crypto components factory
func NewCryptoComponentsFactory(args CryptoComponentsFactoryArgs, importDbNoSigCheckFlag bool) (*cryptoComponentsFactory, error) {
	if args.NodesConfig == nil {
		return nil, ErrNilNodesConfig
	}
	if check.IfNil(args.KeyGen) {
		return nil, crypto.ErrNilKeyGenerator
	}
	if check.IfNil(args.PrivKey) {
		return nil, crypto.ErrNilPrivateKey
	}

	ccf := &cryptoComponentsFactory{
		consensusType:                        "bls",
		config:                               args.Config,
		nodesConfig:                          args.NodesConfig,
		keyGen:                               args.KeyGen,
		privKey:                              args.PrivKey,
		activateBLSPubKeyMessageVerification: args.ActivateBLSPubKeyMessageVerification,
		importDbNoSigCheckFlag:               importDbNoSigCheckFlag,
	}

	return ccf, nil
}

// Create will create and return crypto components
func (ccf *cryptoComponentsFactory) Create() (*CryptoComponents, error) {
	initialPubKeys := ccf.nodesConfig.InitialNodesPubKeys()
	txSingleSigner := &singlesig.Ed25519Signer{}

	interceptSingleSigner, err := ccf.createSingleSigner(ccf.importDbNoSigCheckFlag)
	if err != nil {
		return nil, err
	}

	multisigHasher, err := ccf.getMultisigHasherFromConfig()
	if err != nil {
		return nil, err
	}

	currentNodesPubKeys, err := ccf.nodesConfig.InitialElectedNodesPubKeys()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrMultiSigCreation, err.Error())
	}

	multiSigner, err := ccf.createMultiSigner(multisigHasher, currentNodesPubKeys, ccf.importDbNoSigCheckFlag)
	if err != nil {
		return nil, err
	}

	txSignKeyGen := signing.NewKeyGenerator(ed25519.NewEd25519())

	cacheConfig := ccf.config.PublicKeyPIDSignature
	cachePkPIDSignature, err := storageUnit.NewCache(storageFactory.GetCacherFromConfig(cacheConfig))
	if err != nil {
		return nil, err
	}

	peerSigHandler, err := peerSignatureHandler.NewPeerSignatureHandler(cachePkPIDSignature, interceptSingleSigner, ccf.keyGen)
	if err != nil {
		return nil, err
	}

	return &CryptoComponents{
		TxSingleSigner:       txSingleSigner,
		SingleSigner:         interceptSingleSigner,
		MultiSigner:          multiSigner,
		BlockSignKeyGen:      ccf.keyGen,
		TxSignKeyGen:         txSignKeyGen,
		InitialPubKeys:       initialPubKeys,
		PeerSignatureHandler: peerSigHandler,
	}, nil
}

func (ccf *cryptoComponentsFactory) createSingleSigner(importDbNoSigCheckFlag bool) (crypto.SingleSigner, error) {
	if importDbNoSigCheckFlag {
		log.Warn("using disabled single signer because the node is running in import-db 'turbo mode'")
		return &disabledSig.DisabledSingleSig{}, nil
	}

	switch ccf.consensusType {
	case consensus.BlsConsensusType:
		return &mclSig.BlsSingleSigner{}, nil
	case disabledSigChecking:
		log.Warn("using disabled single signer")
		return &disabledSig.DisabledSingleSig{}, nil
	default:
		return nil, ErrMissingConsensusConfig
	}
}

func (ccf *cryptoComponentsFactory) getMultisigHasherFromConfig() (hashing.Hasher, error) {
	if ccf.consensusType == consensus.BlsConsensusType {
		return &blake2b.Blake2b{HashSize: multisig.BlsHashSize}, nil
	}
	return nil, ErrMultiSigHasherMissmatch
}

func (ccf *cryptoComponentsFactory) createMultiSigner(
	hasher hashing.Hasher,
	pubKeys []string,
	importDbNoSigCheckFlag bool,
) (crypto.MultiSigner, error) {
	if importDbNoSigCheckFlag {
		log.Warn("using disabled multi signer because the node is running in import-db 'turbo mode'")
		return &disabledMultiSig.DisabledMultiSig{}, nil
	}

	//TODO: the instantiation of BLS multi signer can be done with own public key instead of all public keys
	// e.g []string{ownPubKey}.
	// The parameter pubKeys for multi-signer is relevant when we want to create a multisig and in the multisig bitmap
	// we care about the order of the initial public keys that signed, but we never use the entire set of initial
	// public keys in their initial order.

	switch ccf.consensusType {
	case consensus.BlsConsensusType:
		blsSigner := &mclMultiSig.BlsMultiSigner{Hasher: hasher}
		return multisig.NewBLSMultisig(blsSigner, pubKeys, ccf.privKey, ccf.keyGen, uint16(0))
	case disabledSigChecking:
		log.Warn("using disabled multi signer")
		return &disabledMultiSig.DisabledMultiSig{}, nil
	default:
		return nil, ErrMissingConsensusConfig
	}
}
