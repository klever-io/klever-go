package factory_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestPeerAccountCreator_CreateAccountNilAddress(t *testing.T) {
	t.Parallel()

	accF := factory.NewPeerAccountCreator()

	_, ok := accF.(*factory.PeerAccountCreator)
	assert.Equal(t, true, ok)

	acc, err := accF.CreateAccount(nil)

	assert.Nil(t, acc)
	assert.Equal(t, err, common.ErrNilAddress)
}

func TestPeerAccountCreator_CreateAccountOk(t *testing.T) {
	t.Parallel()

	accF := factory.NewPeerAccountCreator()
	assert.False(t, check.IfNil(accF))

	_, ok := accF.(*factory.PeerAccountCreator)
	assert.Equal(t, true, ok)

	acc, err := accF.CreateAccount(make([]byte, 32))

	assert.NotNil(t, acc)
	assert.Nil(t, err)
}
