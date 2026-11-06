package vmhooksgenerate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWriteRustOpcodeCost pins the Rust OpcodeCost the generator emits. The
// executor reads klever-go's OpcodeCost through a pointer, so the struct must
// be repr(C) with the same fields: 123 u32, and none of the removed table
// instructions.
func TestWriteRustOpcodeCost(t *testing.T) {
	// The generator reads its inputs relative to the vmhooks package.
	t.Chdir("..")

	outDir := t.TempDir()
	writer := NewEIGenWriter(outDir, "opcode_cost.rs")
	WriteRustOpcodeCost(writer)
	writer.Close()

	content, err := os.ReadFile(filepath.Join(outDir, "opcode_cost.rs"))
	require.NoError(t, err)
	output := string(content)

	require.Contains(t, output, "#[repr(C)]\n#[derive(Clone, Debug, Default)]\npub struct OpcodeCost {\n")
	require.Equal(t, 123, strings.Count(output, ": u32,"))
	require.Contains(t, output, "pub opcode_select: u32,\n    pub opcode_throw: u32,\n")
	require.NotContains(t, output, "opcode_table")
}
