package vm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCallType_ToString(t *testing.T) {
	callType := DirectCall
	require.Equal(t, DirectCallStr, callType.ToString())

	callType = KDATransferAndExecute
	require.Equal(t, KDATransferAndExecuteStr, callType.ToString())

	callType = ExecOnDestByCaller
	require.Equal(t, ExecOnDestByCallerStr, callType.ToString())

	callType = CallType(9999)
	require.Equal(t, UnknownStr, callType.ToString())
}
