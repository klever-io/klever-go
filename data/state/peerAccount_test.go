package state_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestNewEmptyPeerAccount(t *testing.T) {
	t.Parallel()

	acc := state.NewEmptyPeerAccount()

	assert.NotNil(t, acc)
	// TODO: assert.Equal(t, big.NewInt(0), acc.AccumulatedFees)
}

func TestNewPeerAccount_NilAddressContainerShouldErr(t *testing.T) {
	t.Parallel()

	acc, err := state.NewPeerAccount(nil)
	assert.True(t, check.IfNil(acc))
	assert.Equal(t, common.ErrNilAddress, err)
}

func TestNewPeerAccount_OkParamsShouldWork(t *testing.T) {
	t.Parallel()

	acc, err := state.NewPeerAccount(make([]byte, 32))
	assert.Nil(t, err)
	assert.False(t, check.IfNil(acc))
}

func TestPeerAccount_SetInvalidBLSPublicKey(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewPeerAccount(make([]byte, 32))
	pubKey := []byte("")

	err := acc.SetBLSPublicKey(pubKey)
	assert.Equal(t, common.ErrNilBLSPublicKey, err)
}

func TestPeerAccount_SetAndGetBLSPublicKey(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewPeerAccount(make([]byte, 32))
	pubKey := []byte("BLSpubKey")

	err := acc.SetBLSPublicKey(pubKey)
	assert.Nil(t, err)
	assert.Equal(t, pubKey, acc.GetBLSPublicKey())
}
