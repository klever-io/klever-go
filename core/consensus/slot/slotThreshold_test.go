package slot_test

import (
	"testing"

	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/bls"
	"github.com/stretchr/testify/assert"
)

func TestSlotThreshold_NewThresholdShouldWork(t *testing.T) {
	t.Parallel()

	rthr := slot.NewSlotThreshold()

	assert.NotNil(t, rthr)
}

func TestSlotThreshold_SetThresholdShouldWork(t *testing.T) {
	t.Parallel()

	rthr := slot.NewSlotThreshold()

	rthr.SetThreshold(bls.SrBlock, 1)
	rthr.SetThreshold(bls.SrSignature, 5)

	assert.Equal(t, 1, rthr.Threshold(bls.SrBlock))
	assert.Equal(t, 5, rthr.Threshold(bls.SrSignature))
}

func TestSlotThreshold_SetFallbackThresholdShouldWork(t *testing.T) {
	t.Parallel()

	rthr := slot.NewSlotThreshold()

	rthr.SetFallbackThreshold(bls.SrBlock, 1)
	rthr.SetFallbackThreshold(bls.SrSignature, 5)

	assert.Equal(t, 1, rthr.FallbackThreshold(bls.SrBlock))
	assert.Equal(t, 5, rthr.FallbackThreshold(bls.SrSignature))
}
