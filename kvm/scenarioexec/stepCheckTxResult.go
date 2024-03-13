package scenarioexec

import (
	"fmt"
	"math/big"

	scenexpressionreconstructor "github.com/klever-io/klever-go/kvm/scenarioexec/expression/reconstructor"
	scenjsonwrite "github.com/klever-io/klever-go/kvm/scenarioexec/json/write"
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
	vmi "github.com/klever-io/klever-go/vmcommon"
)

func (ae *VMTestExecutor) checkTxResults(
	txIndex string,
	blResult *scenjsonmodel.TransactionResult,
	checkGas bool,
	output *vmi.VMOutput,
) error {

	if !blResult.Status.Check(big.NewInt(int64(output.ReturnCode))) {
		return fmt.Errorf("result code mismatch. Tx '%s'. Want: %s. Have: %d (%s). Message: %s",
			txIndex, blResult.Status.Original, int(output.ReturnCode), output.ReturnCode.String(), output.ReturnMessage)
	}

	if !blResult.Message.Check([]byte(output.ReturnMessage)) {
		return fmt.Errorf("result message mismatch. Tx '%s'. Want: %s. Have: %s",
			txIndex, blResult.Message.Original, output.ReturnMessage)
	}

	// check result
	if !blResult.Out.CheckList(output.ReturnData) {
		return fmt.Errorf("result mismatch. Tx '%s'. Want: %s. Have: %s",
			txIndex,
			checkBytesListPretty(blResult.Out),
			ae.exprReconstructor.ReconstructList(output.ReturnData, scenexpressionreconstructor.NoHint))
	}

	// check gas
	// unlike other checks, if unspecified the remaining gas check is ignored
	if checkGas && !blResult.Gas.IsUnspecified() && !blResult.Gas.Check(output.GasRemaining) {
		return fmt.Errorf("result gas mismatch. Tx '%s'. Want: %s. Got: %d (0x%x)",
			txIndex,
			blResult.Gas.Original,
			output.GasRemaining,
			output.GasRemaining)
	}

	return ae.checkTxLogs(txIndex, blResult.Logs, output.Logs)
}

func (ae *VMTestExecutor) checkTxLogs(
	txIndex string,
	expectedLogs scenjsonmodel.LogList,
	actualLogs []*vmi.LogEntry,
) error {
	// "logs": "*" means any value is accepted, log check ignored
	if expectedLogs.IsStar {
		return nil
	}

	// this is the real log check
	if len(actualLogs) < len(expectedLogs.List) {
		return fmt.Errorf("too few logs. Tx '%s'. Want:%d. Got:%d",
			txIndex,
			len(expectedLogs.List),
			len(actualLogs))
	}

	for i, actualLog := range actualLogs {
		if i < len(expectedLogs.List) {
			testLog := expectedLogs.List[i]
			err := ae.checkTxLog(txIndex, i, testLog, actualLog)
			if err != nil {
				return err
			}
		} else if !expectedLogs.MoreAllowedAtEnd {
			return fmt.Errorf("unexpected log. Tx '%s'. Log index: %d. Log:\n%s",
				txIndex,
				i,
				scenjsonwrite.LogToString(ae.convertLogToTestFormat(actualLog)),
			)
		}
	}

	return nil
}

func (ae *VMTestExecutor) checkTxLog(
	txIndex string,
	logIndex int,
	expectedLog *scenjsonmodel.LogEntry,
	actualLog *vmi.LogEntry) error {
	if !expectedLog.Address.Check(actualLog.Address) {
		return fmt.Errorf("bad log address. Tx '%s'. Log index: %d. Want:\n%s\nGot:\n%s",
			txIndex,
			logIndex,
			scenjsonwrite.LogToString(expectedLog),
			scenjsonwrite.LogToString(ae.convertLogToTestFormat(actualLog)))
	}
	if !expectedLog.Endpoint.Check(actualLog.Identifier) {
		return fmt.Errorf("bad log identifiscenexpressionreconstructor. Tx '%s'. Log index: %d. Want:\n%s\nGot:\n%s",
			txIndex,
			logIndex,
			scenjsonwrite.LogToString(expectedLog),
			scenjsonwrite.LogToString(ae.convertLogToTestFormat(actualLog)))
	}
	if !expectedLog.Topics.CheckList(actualLog.Topics) {
		return fmt.Errorf("bad log topics. Tx '%s'. Log index: %d. Want: %s. Have: %s",
			txIndex,
			logIndex,
			checkBytesListPretty(expectedLog.Topics),
			ae.exprReconstructor.ReconstructList(actualLog.Topics, scenexpressionreconstructor.NoHint))
	}
	if !expectedLog.Data.CheckList(actualLog.Data) {
		return fmt.Errorf("bad log data. Tx '%s'. Log index: %d. Want:\n%s\nGot:\n%s",
			txIndex,
			logIndex,
			scenjsonwrite.LogToString(expectedLog),
			scenjsonwrite.LogToString(ae.convertLogToTestFormat(actualLog)))
	}
	return nil
}

// JSONCheckBytesString formats a list of JSONCheckBytes for printing to console.
func checkBytesListPretty(jcbl scenjsonmodel.JSONCheckValueList) string {
	str := "["
	for i, jcb := range jcbl.Values {
		if i > 0 {
			str += ", "
		}

		str += orderedjson.JSONString(jcb.Original)
	}
	return str + "]"
}
