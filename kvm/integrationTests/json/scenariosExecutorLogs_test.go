package vmjsonintegrationtest

import (
	"testing"

	"github.com/klever-io/klever-go/kvm/testcommon/testexecutor"
	"github.com/klever-io/klever-go/kvm/wasmer2"
)

func TestRustCompareAdderLog(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	ScenariosTest(t).
		Folder("adder/scenarios").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestRustFactorialLog(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}
	ScenariosTest(t).
		Folder("factorial/scenarios").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestRustErc20Log(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	ScenariosTest(t).
		Folder("erc20-rust/scenarios").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestCErc20Log(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	ScenariosTest(t).
		Folder("erc20-c").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestDigitalCashLog(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	ScenariosTest(t).
		Folder("digital-cash").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestKDAMultiTransferOnCallbackLog(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	ScenariosTest(t).
		Folder("features/composability/scenarios").
		File("forw_raw_call_async_retrieve_multi_transfer.scen.json").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestCreateAsyncCallLog(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	ScenariosTest(t).
		Folder("features/composability/scenarios-promises").
		File("promises_single_transfer.scen.json").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestKDAMultiTransferOnCallAndCallbackLog(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	ScenariosTest(t).
		Folder("features/composability/scenarios").
		File("forw_raw_async_send_and_retrieve_multi_transfer_funds.scen.json").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestMultisigLog(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	ScenariosTest(t).
		Folder("multisig/scenarios").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestDnsContractLog(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	if testing.Short() {
		t.Skip("not a short test")
	}

	ScenariosTest(t).
		Folder("dns").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestCrowdfundingKdaLog(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	ScenariosTest(t).
		Folder("crowdfunding-kda").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestKlvKdaSwapLog(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	ScenariosTest(t).
		Folder("klv-kda-swap").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestPingPongKlvLog(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	ScenariosTest(t).
		Folder("ping-pong-klv").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestRustAttestationLog(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	if testing.Short() {
		t.Skip("not a short test")
	}

	ScenariosTest(t).
		Folder("attestation-rust").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestCAttestationLog(t *testing.T) {
	if !testexecutor.IsWasmer1Allowed() {
		t.Skip("run exclusively with wasmer1")
	}

	if testing.Short() {
		t.Skip("not a short test")
	}

	ScenariosTest(t).
		Folder("attestation-c").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}
