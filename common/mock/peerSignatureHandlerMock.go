package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
)

// PeerSignatureHandler -
type PeerSignatureHandler struct {
	Signer crypto.SingleSigner
	KeyGen crypto.KeyGenerator
}

// VerifyPeerSignature -
func (p *PeerSignatureHandler) VerifyPeerSignature(pk []byte, pid core.PeerID, sig []byte) error {
	var senderPubKey crypto.PublicKey
	var err error
	senderPubKey = &cryptoMock.PublicKeyMock{}

	if p.KeyGen != nil {
		senderPubKey, err = p.KeyGen.PublicKeyFromByteArray(pk)
		if err != nil {
			return err
		}
	}

	return p.Signer.Verify(senderPubKey, pid.Bytes(), sig)
}

// GetPeerSignature -
func (p *PeerSignatureHandler) GetPeerSignature(privateKey crypto.PrivateKey, msg []byte) ([]byte, error) {
	if p.Signer != nil {
		return p.Signer.Sign(privateKey, msg)
	}

	return nil, nil
}

// IsInterfaceNil -
func (p *PeerSignatureHandler) IsInterfaceNil() bool {
	return p == nil
}
