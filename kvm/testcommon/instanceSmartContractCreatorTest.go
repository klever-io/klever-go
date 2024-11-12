// Package testcommon contains utility definitions used in unit and integration tests
package testcommon

import (
	"testing"

	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kvm/executor"
	executorwrapper "github.com/klever-io/klever-go/kvm/executor/wrapper"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	mockWorld "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

// InstanceCreatorTestTemplate holds the data to build a contract creation test
type InstanceCreatorTestTemplate struct {
	tb                      testing.TB
	address                 []byte
	input                   *vmcommon.ContractCreateInput
	setup                   func(vmhost.VMHost, *contextmock.BlockchainHookStub)
	assertResults           func(*contextmock.BlockchainHookStub, *VMOutputVerifier)
	host                    vmhost.VMHost
	hostBuilder             *TestHostBuilder
	stubAccountInitialNonce uint64
}

// BuildInstanceCreatorTest starts the building process for a contract creation test
func BuildInstanceCreatorTest(tb testing.TB) *InstanceCreatorTestTemplate {
	return &InstanceCreatorTestTemplate{
		tb:                      tb,
		setup:                   func(vmhost.VMHost, *contextmock.BlockchainHookStub) {},
		hostBuilder:             NewTestHostBuilder(tb),
		stubAccountInitialNonce: 24,
	}
}

// WithExecutorFactory allows caller to choose the Executor type.
func (template *InstanceCreatorTestTemplate) WithExecutorFactory(factory executor.ExecutorAbstractFactory) *InstanceCreatorTestTemplate {
	template.hostBuilder.WithExecutorFactory(factory)
	return template
}

// WithExecutorLogs sets an ExecutorLogger
func (template *InstanceCreatorTestTemplate) WithExecutorLogs(executorLogger executorwrapper.ExecutorLogger) *InstanceCreatorTestTemplate {
	template.hostBuilder.WithExecutorLogs(executorLogger)
	return template
}

// WithInput provides the ContractCreateInput for a TestCreateTemplateConfig
func (template *InstanceCreatorTestTemplate) WithInput(input *vmcommon.ContractCreateInput) *InstanceCreatorTestTemplate {
	template.input = input
	return template
}

// WithWasmerSIGSEGVPassthrough sets the wasmerSIGSEGVPassthrough flag
func (template *InstanceCreatorTestTemplate) WithWasmerSIGSEGVPassthrough(passthrough bool) *InstanceCreatorTestTemplate {
	template.hostBuilder.WithWasmerSIGSEGVPassthrough(passthrough)
	return template
}

// WithAddress provides the address for a TestCreateTemplateConfig
func (template *InstanceCreatorTestTemplate) WithAddress(address []byte) *InstanceCreatorTestTemplate {
	template.address = address
	return template
}

// WithSetup provides the setup function for a TestCreateTemplateConfig
func (template *InstanceCreatorTestTemplate) WithSetup(setup func(vmhost.VMHost, *contextmock.BlockchainHookStub)) *InstanceCreatorTestTemplate {
	template.setup = setup
	return template
}

// AndAssertResults provides the function that will aserts the results
func (template *InstanceCreatorTestTemplate) AndAssertResults(assertResults func(*contextmock.BlockchainHookStub, *VMOutputVerifier)) {
	template.assertResults = assertResults
	template.runTest(true)
}

// AndAssertResultsWithoutReset provides the function that will aserts the results
func (template *InstanceCreatorTestTemplate) AndAssertResultsWithoutReset(assertResults func(*contextmock.BlockchainHookStub, *VMOutputVerifier)) {
	template.assertResults = assertResults
	template.runTest(false)
}

func (template *InstanceCreatorTestTemplate) runTest(reset bool) {
	var blhookStub *contextmock.BlockchainHookStub
	if template.host == nil {
		blhookStub = template.createBlockchainStub()
		template.hostBuilder.WithBlockchainHook(blhookStub)
		template.host = template.hostBuilder.Build()
		template.setup(template.host, blhookStub)
	}
	defer func() {
		if reset {
			template.host.Reset()
		}

		// Extra verification for instance leaks
		err := template.host.Runtime().ValidateInstances()
		require.Nil(template.tb, err)
	}()

	vmOutput, err := template.host.RunSmartContractCreate(template.input)

	verify := NewVMOutputVerifier(template.tb, vmOutput, err)
	template.assertResults(blhookStub, verify)
}

func (template *InstanceCreatorTestTemplate) createBlockchainStub() *contextmock.BlockchainHookStub {
	stubBlockchainHook := &contextmock.BlockchainHookStub{}
	stubBlockchainHook.GetUserAccountCalled = func(address []byte) (state.UserAccountHandler, error) {
		return &mockWorld.Account{
			Nonce: 24,
		}, nil
	}
	stubBlockchainHook.NewAddressCalled = func(creatorAddress []byte, nonce uint64, vmType []byte, randomSeed []byte) ([]byte, error) {
		return template.address, nil
	}

	stubBlockchainHook.GetKAppControllerCalled = func() kapp.KAppController {
		return &stub.KAppControllerStub{
			GetAccountsKAppCalled: func() kapp.AccountsKapp {
				return &stub.KAppAccountsStub{}
			},
		}
	}

	return stubBlockchainHook
}
