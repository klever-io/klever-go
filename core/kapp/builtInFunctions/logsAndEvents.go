package builtInFunctions

import (
	"math/big"

	"github.com/klever-io/klever-go/vmcommon"
)

// TODO: add logs into all built-in functions with KDA actions
func addKDAEntryInVMOutput(vmOutput *vmcommon.VMOutput, identifier []byte, tokenID []byte, nonce uint64, value *big.Int, args ...[]byte) {
	entry := newEntryForKDA(identifier, tokenID, nonce, value, args...)

	if vmOutput.Logs == nil {
		vmOutput.Logs = make([]*vmcommon.LogEntry, 0, 1)
	}

	vmOutput.Logs = append(vmOutput.Logs, entry)
}

func newEntryForKDA(identifier, tokenID []byte, nonce uint64, value *big.Int, args ...[]byte) *vmcommon.LogEntry {
	nonceBig := big.NewInt(0).SetUint64(nonce)

	logEntry := &vmcommon.LogEntry{
		Identifier: identifier,
		Topics:     [][]byte{tokenID, nonceBig.Bytes(), value.Bytes()},
	}

	if len(args) > 0 {
		logEntry.Address = args[0]
	}

	if len(args) > 1 {
		logEntry.Topics = append(logEntry.Topics, args[1:]...)
	}

	return logEntry
}
