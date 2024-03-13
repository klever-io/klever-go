package factory_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestAccountCreator_CreateAccountNilAddress(t *testing.T) {
	t.Parallel()

	accF := factory.NewAccountCreator()

	_, ok := accF.(*factory.AccountCreator)
	assert.Equal(t, true, ok)
	assert.False(t, check.IfNil(accF))

	acc, err := accF.CreateAccount(nil)

	assert.Nil(t, acc)
	assert.Equal(t, err, common.ErrNilAddress)
}

func TestAccountCreator_CreateAccountOk(t *testing.T) {
	t.Parallel()

	accF := factory.NewAccountCreator()

	_, ok := accF.(*factory.AccountCreator)
	assert.Equal(t, true, ok)

	acc, err := accF.CreateAccount(make([]byte, 32))

	assert.Nil(t, err)
	assert.False(t, check.IfNil(acc))
}
