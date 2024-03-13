package scencontroller

import (
	scenfileresolver "github.com/klever-io/klever-go/kvm/scenarioexec/fileresolver"
	scenjsonparse "github.com/klever-io/klever-go/kvm/scenarioexec/json/parse"
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
)

// TestExecutor describes a component that can run a VM test.
type TestExecutor interface {
	// ExecuteTest executes the test and checks if it passed. Failure is signaled by returning an error.
	ExecuteTest(*scenjsonmodel.Test) error
}

// TestRunner is a component that can run tests, using a provided executor.
type TestRunner struct {
	Executor TestExecutor
	Parser   scenjsonparse.Parser
}

// NewTestRunner creates new TestRunner instance.
func NewTestRunner(executor TestExecutor, fileResolver scenfileresolver.FileResolver) *TestRunner {
	return &TestRunner{
		Executor: executor,
		Parser:   scenjsonparse.NewParser(fileResolver),
	}
}
