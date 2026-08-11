package wasmbytes

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func codeSection(body ...byte) section {
	function := append([]byte{0x00}, body...)

	payload := []byte{0x01}
	payload = binary.AppendUvarint(payload, uint64(len(function)))
	payload = append(payload, function...)

	return section{id: SectionCode, payload: payload}
}

func prefixedOpcode(subOpcode uint32, paddingGroups int) []byte {
	encoded := []byte{opPrefixFC}
	for group := 0; group < paddingGroups; group++ {
		encoded = append(encoded, byte(subOpcode&0x7F)|0x80)
		subOpcode >>= 7
	}

	return append(encoded, byte(subOpcode))
}

func TestMutatesTables_ReportsModuleWritingItsOwnTable(t *testing.T) {
	t.Parallel()

	module := moduleWithSections(codeSection(0x41, 0x00, opRefNull, 0x70, opTableSet, 0x00, opEnd))

	require.True(t, MutatesTables(module))
}

func TestMutatesTables_ReportsEachPrefixedMutationOpcode(t *testing.T) {
	t.Parallel()

	for name, subOpcode := range map[string]uint32{
		"table.init": subTableInit,
		"table.copy": subTableCopy,
		"table.grow": subTableGrow,
		"table.fill": subTableFill,
	} {
		t.Run(name, func(t *testing.T) {
			body := append([]byte{0x41, 0x00}, prefixedOpcode(subOpcode, 0)...)
			module := moduleWithSections(codeSection(append(body, 0x00, 0x00, opEnd)...))

			require.True(t, MutatesTables(module))
		})
	}
}

func TestMutatesTables_ReportsMutationOpcodesEncodedWithPadding(t *testing.T) {
	t.Parallel()

	for name, paddingGroups := range map[string]int{
		"two groups":  1,
		"five groups": 4,
	} {
		t.Run(name, func(t *testing.T) {
			body := append([]byte{0x41, 0x00}, prefixedOpcode(subTableGrow, paddingGroups)...)
			module := moduleWithSections(codeSection(append(body, 0x00, opEnd)...))

			require.True(t, MutatesTables(module))
		})
	}
}

func TestMutatesTables_IgnoresTableThatIsOnlyRead(t *testing.T) {
	t.Parallel()

	readOnly := moduleWithSections(codeSection(0x41, 0x00, opTableGet, 0x00, opEnd))
	require.False(t, MutatesTables(readOnly))

	sized := moduleWithSections(codeSection(opPrefixFC, byte(subTableSize), 0x00, opEnd))
	require.False(t, MutatesTables(sized))
}

func TestMutatesTables_IgnoresNonMutatingPrefixedOpcodes(t *testing.T) {
	t.Parallel()

	module := moduleWithSections(codeSection(opPrefixFC, byte(subMemoryCopy), 0x00, 0x00, opEnd))

	require.False(t, MutatesTables(module))
}

func TestMutatesTables_IgnoresMutationBytesOutsideTheCodeSection(t *testing.T) {
	t.Parallel()

	module := moduleWithSections(
		section{id: sectionCustom, payload: []byte{opTableSet, opPrefixFC, byte(subTableGrow)}},
	)

	require.False(t, MutatesTables(module))
}

func TestMutatesTables_IgnoresMutationBytesAppearingAsOperands(t *testing.T) {
	t.Parallel()

	constant := moduleWithSections(codeSection(0x41, opTableSet, 0x1A, opEnd))
	require.False(t, MutatesTables(constant))

	localIndex := moduleWithSections(codeSection(0x20, opTableSet, 0x1A, opEnd))
	require.False(t, MutatesTables(localIndex))

	branchTarget := moduleWithSections(codeSection(
		opBlock, 0x40, 0x41, 0x00, opBrTable, 0x01, opTableSet, opTableSet, opEnd, opEnd))
	require.False(t, MutatesTables(branchTarget))
}

func TestMutatesTables_IgnoresMutationBytesUsedAsFunctionBodySize(t *testing.T) {
	t.Parallel()

	body := append([]byte{0x00}, make([]byte, int(opTableSet)-2)...)
	body = append(body, opEnd)

	payload := []byte{0x01}
	payload = binary.AppendUvarint(payload, uint64(len(body)))
	payload = append(payload, body...)

	require.Equal(t, opTableSet, payload[1])
	require.False(t, MutatesTables(moduleWithSections(section{id: SectionCode, payload: payload})))
}

func TestMutatesTables_ReportsUndecodableModules(t *testing.T) {
	t.Parallel()

	require.True(t, MutatesTables([]byte("not a wasm module")))
	require.True(t, MutatesTables(nil))
}

func TestMutatesTables_ReportsUndecodableCodeSections(t *testing.T) {
	t.Parallel()

	truncatedBody := moduleWithSections(section{id: SectionCode, payload: []byte{0x01, 0x08, 0x00}})
	require.True(t, MutatesTables(truncatedBody))

	unknownOpcode := moduleWithSections(codeSection(0x06, opEnd))
	require.True(t, MutatesTables(unknownOpcode))
}

func TestMutatesTables_IgnoresDeployedContractsThatDoNotTouchTables(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"./../test/features/basic-features/output/basic-features.wasm",
		"./../test/features/basic-features-no-small-int-api/output/features-no-small-int-api.wasm",
		"./../test/features/composability/proxy-test-first/output/proxy-test-first.wasm",
		"./../test/contracts/counter/output/counter.wasm",
	} {
		t.Run(path, func(t *testing.T) {
			code, err := os.ReadFile(path)
			require.NoError(t, err)

			require.False(t, MutatesTables(code))
		})
	}
}
