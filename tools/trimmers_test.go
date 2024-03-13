package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubslotStartSlot_GetPkToDisplayShouldTrim(t *testing.T) {
	pk := "1234567891234"
	pkToDisplay := GetTrimmedPk(pk)
	assert.Equal(t, "123456789123", pkToDisplay)
}

func TestSubslotStartSlot_GetPkToDisplayShouldNotTrim(t *testing.T) {
	pk := "123456789123"
	pkToDisplay := GetTrimmedPk(pk)
	assert.Equal(t, pk, pkToDisplay)
}
