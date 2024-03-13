package factory_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestKAppAccountCreator_CreateAccountNilAddress(t *testing.T) {
	t.Parallel()

	accF := factory.NewKAppAccountCreator()

	_, ok := accF.(*factory.KAppAccountCreator)
	assert.Equal(t, true, ok)

	acc, err := accF.CreateAccount(nil)

	assert.Nil(t, acc)
	assert.Equal(t, err, common.ErrNilAddress)
}

func TestKAppAccountCreator_CreateAccountOk(t *testing.T) {
	t.Parallel()

	accF := factory.NewKAppAccountCreator()
	assert.False(t, check.IfNil(accF))

	_, ok := accF.(*factory.KAppAccountCreator)
	assert.Equal(t, true, ok)

	acc, err := accF.CreateAccount(make([]byte, 32))

	assert.NotNil(t, acc)
	assert.Nil(t, err)
}
