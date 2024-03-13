package slot_test

import (
	"testing"

	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/stretchr/testify/assert"
)

func TestSlotState_NewSlotStateShouldWork(t *testing.T) {
	t.Parallel()

	rstate := slot.NewSlotState()
	assert.NotNil(t, rstate)
}

func TestSlotState_SetJobDoneShouldWork(t *testing.T) {
	t.Parallel()

	rstate := slot.NewSlotState()

	rstate.SetJobDone(1, true)

	assert.True(t, rstate.JobDone(1))
}

func TestSlotState_ResetJobDoneShouldWork(t *testing.T) {
	t.Parallel()

	rstate := slot.NewSlotState()

	rstate.SetJobDone(1, true)
	rstate.SetJobDone(2, true)
	rstate.SetJobDone(3, true)
	rstate.SetJobDone(4, true)
	rstate.SetJobDone(5, true)

	rstate.ResetJobsDone()

	assert.False(t, rstate.JobDone(1))
	assert.False(t, rstate.JobDone(2))
	assert.False(t, rstate.JobDone(3))
	assert.False(t, rstate.JobDone(4))
	assert.False(t, rstate.JobDone(5))
}
