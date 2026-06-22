package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePickerInput_ValidIndex(t *testing.T) {
	idx, action := parsePickerInput("0", 3, true, true)
	assert.Equal(t, 0, idx)
	assert.Equal(t, PickerActionSelect, action)

	idx, action = parsePickerInput("2", 3, true, true)
	assert.Equal(t, 2, idx)
	assert.Equal(t, PickerActionSelect, action)
}

func TestParsePickerInput_IndexOutOfRange(t *testing.T) {
	idx, action := parsePickerInput("3", 3, true, true)
	assert.Equal(t, 0, idx)
	assert.Equal(t, PickerActionInvalid, action)

	idx, action = parsePickerInput("-1", 3, true, true)
	assert.Equal(t, 0, idx)
	assert.Equal(t, PickerActionInvalid, action)
}

func TestParsePickerInput_NextPage(t *testing.T) {
	// Can go next
	idx, action := parsePickerInput("n", 3, true, true)
	assert.Equal(t, 0, idx)
	assert.Equal(t, PickerActionNext, action)

	// Can't go next
	idx, action = parsePickerInput("n", 3, true, false)
	assert.Equal(t, 0, idx)
	assert.Equal(t, PickerActionInvalid, action)
}

func TestParsePickerInput_PreviousPage(t *testing.T) {
	// Can go prev
	idx, action := parsePickerInput("p", 3, true, true)
	assert.Equal(t, 0, idx)
	assert.Equal(t, PickerActionPrev, action)

	// Can't go prev
	idx, action = parsePickerInput("p", 3, false, true)
	assert.Equal(t, 0, idx)
	assert.Equal(t, PickerActionInvalid, action)
}

func TestParsePickerInput_Quit(t *testing.T) {
	idx, action := parsePickerInput("q", 3, true, true)
	assert.Equal(t, 0, idx)
	assert.Equal(t, PickerActionQuit, action)
}

func TestParsePickerInput_InvalidInput(t *testing.T) {
	tests := []string{"abc", "x", "", "1a", "N", "P", "Q"}
	for _, input := range tests {
		idx, action := parsePickerInput(input, 3, true, true)
		assert.Equal(t, 0, idx)
		assert.Equal(t, PickerActionInvalid, action)
	}
}
