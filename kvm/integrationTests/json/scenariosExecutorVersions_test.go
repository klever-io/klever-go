package vmjsonintegrationtest

import (
	"testing"

	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/kvm/wasmer2"
)

func TestCErc20Executors_TwiceW2(t *testing.T) {
	testCERC20WithExecutorFactory(t, wasmer2.ExecutorFactory())
	testCERC20WithExecutorFactory(t, wasmer2.ExecutorFactory())
}

func testCERC20WithExecutorFactory(t *testing.T, factory executor.ExecutorAbstractFactory) {
	ScenariosTest(t).
		Folder("erc20-c").
		WithExecutorFactory(factory).
		WithExecutorLogs().
		Run().
		CheckNoError()
}
