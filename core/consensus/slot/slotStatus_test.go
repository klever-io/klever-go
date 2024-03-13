package slot_test

import (
	"testing"

	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/bls"
	"github.com/stretchr/testify/assert"
)

func TestSlotStatus_NewSlotStatusShouldWork(t *testing.T) {
	t.Parallel()

	rstatus := slot.NewSlotStatus()
	assert.NotNil(t, rstatus)
}

func TestSlotStatus_SetSlotStatusShouldWork(t *testing.T) {
	t.Parallel()

	rstatus := slot.NewSlotStatus()

	rstatus.SetStatus(bls.SrSignature, slot.SsFinished)
	assert.Equal(t, slot.SsFinished, rstatus.Status(bls.SrSignature))
}

func TestSlotStatus_ResetSlotStatusShouldWork(t *testing.T) {
	t.Parallel()

	rstatus := slot.NewSlotStatus()

	rstatus.SetStatus(bls.SrStartSlot, slot.SsFinished)
	rstatus.SetStatus(bls.SrBlock, slot.SsFinished)
	rstatus.SetStatus(bls.SrSignature, slot.SsFinished)
	rstatus.SetStatus(bls.SrEndSlot, slot.SsFinished)

	rstatus.ResetSlotStatus()

	assert.Equal(t, slot.SsNotFinished, rstatus.Status(bls.SrStartSlot))
	assert.Equal(t, slot.SsNotFinished, rstatus.Status(bls.SrBlock))
	assert.Equal(t, slot.SsNotFinished, rstatus.Status(bls.SrSignature))
	assert.Equal(t, slot.SsNotFinished, rstatus.Status(bls.SrEndSlot))
}
