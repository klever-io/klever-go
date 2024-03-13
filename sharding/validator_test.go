package sharding

import (
	"testing"

	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestValidator_NewValidatorShouldFailOnNilPublickKey(t *testing.T) {
	t.Parallel()

	v, err := NewValidator([]byte("oa1"), nil, DefaultSelectionChances, 1)

	assert.Nil(t, v)
	assert.Equal(t, ErrNilPubKey, err)
}

func TestValidator_NewValidatorShouldWork(t *testing.T) {
	t.Parallel()

	v, err := NewValidator([]byte("oa1"), []byte("pk1"), DefaultSelectionChances, 1)

	assert.NotNil(t, v)
	assert.Nil(t, err)
	assert.False(t, check.IfNil(v))
}

func TestValidator_PubKeyShouldWork(t *testing.T) {
	t.Parallel()

	v, _ := NewValidator([]byte("oa1"), []byte("pk1"), DefaultSelectionChances, 1)

	assert.Equal(t, []byte("pk1"), v.PubKey())
}
