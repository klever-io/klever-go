package factory

import (
	"bytes"
	"errors"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	shardingMock "github.com/klever-io/klever-go/sharding/mock"
)

var errSingleSignKeyGenMock = errors.New("errSingleSignKeyGenMock")
var errSignerMockVerifySigFails = errors.New("errSignerMockVerifySigFails")
var sigOk = []byte("signature")

func createMockPubkeyConverter() core.PubkeyConverter {
	return shardingMock.NewPubkeyConverterMock(32)
}

func createMockKeyGen() crypto.KeyGenerator {
	return &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, e error) {
			if string(b) == "" {
				return nil, errSingleSignKeyGenMock
			}

			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
}

func createMockSigner() crypto.SingleSigner {
	return &cryptoMock.SignerMock{
		VerifyStub: func(public crypto.PublicKey, msg []byte, sig []byte) error {
			if !bytes.Equal(sig, sigOk) {
				return errSignerMockVerifySigFails
			}
			return nil
		},
	}
}

func createMockFeeHandler() process.EconomicsDataHandler {
	return &mock.FeeHandlerStub{}
}

func createMockArgument() *ArgInterceptedDataFactory {
	return &ArgInterceptedDataFactory{
		ProtoMarshalizer:        &mock.MarshalizerMock{},
		TxSignMarshalizer:       &mock.MarshalizerMock{},
		Hasher:                  mock.HasherMock{},
		EpochStartTrigger:       &mock.EpochStartTriggerStub{},
		HeaderSigVerifier:       &consensusMock.HeaderSigVerifierStub{},
		HeaderIntegrityVerifier: &mock.HeaderIntegrityVerifierStub{},
		MultiSigVerifier:        mock.NewMultiSigner(),
		NodesCoordinator:        mock.NewNodesCoordinatorMock(),
		AccountKeyGen:           createMockKeyGen(),
		BlockKeyGen:             createMockKeyGen(),
		Signer:                  createMockSigner(),
		BlockSigner:             createMockSigner(),
		AddressPubkeyConv:       createMockPubkeyConverter(),
		FeeHandler:              createMockFeeHandler(),
		WhiteListerVerifiedTxs:  &mock.WhiteListHandlerStub{},
		ChainID:                 []byte("chainID"),
		MinTransactionVersion:   1,
		TxSignHasher:            mock.HasherMock{},
		EpochNotifier:           &mock.EpochNotifierStub{},
		ForkController:          mock.NewForkControllerStub(),
	}
}
