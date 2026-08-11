package wasmbytes

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

const sectionCustom byte = 0

type section struct {
	id      byte
	payload []byte
}

func preamble() []byte {
	return []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}
}

func moduleWithSections(sections ...section) []byte {
	module := preamble()
	for _, current := range sections {
		module = append(module, current.id)
		module = binary.AppendUvarint(module, uint64(len(current.payload)))
		module = append(module, current.payload...)
	}
	return module
}

func TestHasStartSection_RejectsMalformedModules(t *testing.T) {
	t.Parallel()

	truncatedPayload := append(preamble(), 0x01, 0x08, 0x00)
	truncatedLength := append(preamble(), 0x01)
	overflowingLEB128 := append(preamble(), 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)

	for name, module := range map[string][]byte{
		"empty":                     {},
		"shorter than the preamble": {0x00, 0x61, 0x73},
		"wrong magic":               {0x01, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00},
		"wrong version":             {0x00, 0x61, 0x73, 0x6D, 0x02, 0x00, 0x00, 0x00},
		"payload runs past the end": truncatedPayload,
		"section length missing":    truncatedLength,
		"length overflows uint32":   overflowingLEB128,
	} {
		t.Run(name, func(t *testing.T) {
			hasStart, err := HasStartSection(module)

			require.ErrorIs(t, err, ErrInvalidWasmModule)
			require.False(t, hasStart)
		})
	}
}

func TestHasStartSection_DetectsDeclaredStartSection(t *testing.T) {
	t.Parallel()

	module := moduleWithSections(
		section{id: 1, payload: []byte{0x01}},
		section{id: SectionStart, payload: []byte{0x00}},
	)

	hasStart, err := HasStartSection(module)

	require.NoError(t, err)
	require.True(t, hasStart)
}

func TestHasStartSection_IgnoresModulesWithoutStartSection(t *testing.T) {
	t.Parallel()

	module := moduleWithSections(
		section{id: 1, payload: []byte{0x01}},
		section{id: sectionCustom, payload: []byte("start")},
		section{id: 10, payload: []byte{0x02}},
	)

	hasStart, err := HasStartSection(module)

	require.NoError(t, err)
	require.False(t, hasStart)
}

func TestHasStartSection_IgnoresStartSectionBytesInsideAPayload(t *testing.T) {
	t.Parallel()

	module := moduleWithSections(
		section{id: sectionCustom, payload: []byte{0x08, 0x01, 0x00}},
	)

	hasStart, err := HasStartSection(module)

	require.NoError(t, err)
	require.False(t, hasStart)
}

func TestHasStartSection_DetectsStartSectionAfterAPayloadHoldingItsBytes(t *testing.T) {
	t.Parallel()

	module := moduleWithSections(
		section{id: sectionCustom, payload: []byte{0x08, 0x01, 0x00}},
		section{id: SectionStart, payload: []byte{0x00}},
	)

	hasStart, err := HasStartSection(module)

	require.NoError(t, err)
	require.True(t, hasStart)
}

func TestHasStartSection_AcceptsModuleWithOnlyPreamble(t *testing.T) {
	t.Parallel()

	hasStart, err := HasStartSection(preamble())

	require.NoError(t, err)
	require.False(t, hasStart)
}

func TestHasStartSection_DetectsStartSectionAfterMultiByteLength(t *testing.T) {
	t.Parallel()

	module := moduleWithSections(
		section{id: 1, payload: make([]byte, 200)},
		section{id: SectionStart, payload: []byte{0x00}},
	)

	hasStart, err := HasStartSection(module)

	require.NoError(t, err)
	require.True(t, hasStart)
}
