package vmjsonintegrationtest

import (
	"testing"

	"github.com/klever-io/klever-go/kvm/wasmer2"
)

func TestRustCompareAdderLog(t *testing.T) {
	ScenariosTest(t).
		Folder("adder/scenarios").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestRustFactorialLog(t *testing.T) {
	ScenariosTest(t).
		Folder("factorial/scenarios").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestRustErc20Log(t *testing.T) {
	ScenariosTest(t).
		Folder("erc20-rust/scenarios").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestCErc20Log(t *testing.T) {
	ScenariosTest(t).
		Folder("erc20-c").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestDigitalCashLog(t *testing.T) {
	ScenariosTest(t).
		Folder("digital-cash").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestCrowdfundingKdaLog(t *testing.T) {
	ScenariosTest(t).
		Folder("crowdfunding-kda").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestKlvKdaSwapLog(t *testing.T) {
	ScenariosTest(t).
		Folder("klv-kda-swap").
		WithExecutorFactory(wasmer2.ExecutorFactory()).
		WithExecutorLogs().
		Run().
		CheckNoError()
}

func TestRustAttestationLog(t *testing.T) {
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
