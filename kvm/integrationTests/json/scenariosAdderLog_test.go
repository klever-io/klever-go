package vmjsonintegrationtest

import (
	"testing"
)

const expectedAdderLog = `starting log:
GetFunctionNames: [add add_payable getSum init upgrade]
ValidateFunctionArities: true
GetFunctionNames: [add add_payable getSum init upgrade]
HasFunction(init): true
CallFunction(init):
VM hook begin: CheckNoPayment()
GetPointsUsed: 5
SetPointsUsed: 105
VM hook end:   CheckNoPayment()
VM hook begin: GetNumArguments()
GetPointsUsed: 125
SetPointsUsed: 225
VM hook end:   GetNumArguments()
VM hook begin: BigIntGetUnsignedArgument(0, -101)
GetPointsUsed: 315
SetPointsUsed: 1315
VM hook end:   BigIntGetUnsignedArgument(0, -101)
VM hook begin: MBufferSetBytes(-102, 131072, 3)
GetPointsUsed: 1405
SetPointsUsed: 3405
GetPointsUsed: 3405
SetPointsUsed: 3555
VM hook end:   MBufferSetBytes(-102, 131072, 3)
VM hook begin: MBufferFromBigIntUnsigned(-103, -101)
GetPointsUsed: 3645
SetPointsUsed: 5645
VM hook end:   MBufferFromBigIntUnsigned(-103, -101)
VM hook begin: MBufferStorageStore(-102, -103)
GetPointsUsed: 5665
SetPointsUsed: 80665
GetPointsUsed: 80665
GetPointsUsed: 80665
SetPointsUsed: 80665
GetPointsUsed: 80665
GetPointsUsed: 80665
SetPointsUsed: 90665
VM hook end:   MBufferStorageStore(-102, -103)
GetPointsUsed: 90680
GetPointsUsed: 90680
GetPointsUsed: 90680
GetPointsUsed: 90680
Reset: true
SetPointsUsed: 0
SetGasLimit: 9223372036853633107
SetBreakpointValue: 0
HasFunction(getSum): true
CallFunction(getSum):
VM hook begin: CheckNoPayment()
GetPointsUsed: 5
SetPointsUsed: 105
VM hook end:   CheckNoPayment()
VM hook begin: GetNumArguments()
GetPointsUsed: 125
SetPointsUsed: 225
VM hook end:   GetNumArguments()
VM hook begin: MBufferSetBytes(-101, 131072, 3)
GetPointsUsed: 320
SetPointsUsed: 2320
GetPointsUsed: 2320
SetPointsUsed: 2470
VM hook end:   MBufferSetBytes(-101, 131072, 3)
VM hook begin: MBufferStorageLoad(-101, -102)
GetPointsUsed: 2555
SetPointsUsed: 2605
GetPointsUsed: 2605
GetPointsUsed: 2605
SetPointsUsed: 17892
VM hook end:   MBufferStorageLoad(-101, -102)
VM hook begin: MBufferToBigIntUnsigned(-102, -103)
GetPointsUsed: 17962
SetPointsUsed: 19962
VM hook end:   MBufferToBigIntUnsigned(-102, -103)
VM hook begin: BigIntFinishUnsigned(-103)
GetPointsUsed: 19982
SetPointsUsed: 20982
GetPointsUsed: 20982
SetPointsUsed: 21982
VM hook end:   BigIntFinishUnsigned(-103)
GetPointsUsed: 21987
GetPointsUsed: 21987
GetPointsUsed: 21987
GetPointsUsed: 21987
Reset: true
SetPointsUsed: 0
SetGasLimit: 3857300
SetBreakpointValue: 0
HasFunction(add): true
CallFunction(add):
VM hook begin: CheckNoPayment()
GetPointsUsed: 5
SetPointsUsed: 105
VM hook end:   CheckNoPayment()
VM hook begin: GetNumArguments()
GetPointsUsed: 125
SetPointsUsed: 225
VM hook end:   GetNumArguments()
VM hook begin: BigIntGetUnsignedArgument(0, -101)
GetPointsUsed: 315
SetPointsUsed: 1315
VM hook end:   BigIntGetUnsignedArgument(0, -101)
VM hook begin: MBufferSetBytes(-102, 131072, 3)
GetPointsUsed: 1405
SetPointsUsed: 3405
GetPointsUsed: 3405
SetPointsUsed: 3555
VM hook end:   MBufferSetBytes(-102, 131072, 3)
VM hook begin: MBufferStorageLoad(-102, -103)
GetPointsUsed: 3645
SetPointsUsed: 3695
GetPointsUsed: 3695
GetPointsUsed: 3695
SetPointsUsed: 18982
VM hook end:   MBufferStorageLoad(-102, -103)
VM hook begin: MBufferToBigIntUnsigned(-103, -104)
GetPointsUsed: 19052
SetPointsUsed: 21052
VM hook end:   MBufferToBigIntUnsigned(-103, -104)
VM hook begin: BigIntAdd(-104, -104, -101)
GetPointsUsed: 21102
SetPointsUsed: 23102
VM hook end:   BigIntAdd(-104, -104, -101)
VM hook begin: MBufferFromBigIntUnsigned(-105, -104)
GetPointsUsed: 23187
SetPointsUsed: 25187
VM hook end:   MBufferFromBigIntUnsigned(-105, -104)
VM hook begin: MBufferStorageStore(-102, -105)
GetPointsUsed: 25207
SetPointsUsed: 100207
GetPointsUsed: 100207
GetPointsUsed: 100207
SetPointsUsed: 100207
GetPointsUsed: 100207
GetPointsUsed: 100207
SetPointsUsed: 101207
VM hook end:   MBufferStorageStore(-102, -105)
GetPointsUsed: 101222
GetPointsUsed: 101222
GetPointsUsed: 101222
GetPointsUsed: 101222
Clean: true
`

func TestRustAdderLog(t *testing.T) {
	ScenariosTest(t).
		Folder("adder/scenarios").
		WithExecutorLogs().
		Run().
		CheckNoError().
		CheckLog(expectedAdderLog)
}
