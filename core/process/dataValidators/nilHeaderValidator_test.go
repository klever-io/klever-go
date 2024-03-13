package dataValidators_test

import (
	"testing"

	"github.com/klever-io/klever-go/core/process/dataValidators"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestNilHeaderValidator(t *testing.T) {
	t.Parallel()

	nhhv, err := dataValidators.NewNilHeaderValidator()

	assert.False(t, check.IfNil(nhhv))
	assert.Nil(t, err)
}

func TestNilHeaderValidator_IsHeaderValidForProcessing(t *testing.T) {
	t.Parallel()

	nhv, _ := dataValidators.NewNilHeaderValidator()

	assert.Nil(t, nhv.HeaderValidForProcessing(nil))
}

//------- IsInterfaceNil

func TestNilHeaderValidator_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	hdrValidator, _ := dataValidators.NewNilHeaderValidator()
	_ = hdrValidator
	hdrValidator = nil

	assert.True(t, check.IfNil(hdrValidator))
}
