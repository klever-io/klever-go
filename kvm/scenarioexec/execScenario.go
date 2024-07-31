package scenarioexec

import (
	"errors"

	"github.com/klever-io/klever-go/data/state"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	scencontroller "github.com/klever-io/klever-go/kvm/scenarioexec/controller"
	scenfileresolver "github.com/klever-io/klever-go/kvm/scenarioexec/fileresolver"
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/tools/check"
	vmi "github.com/klever-io/klever-go/vmcommon"
)

// Reset clears state/world.
// Is called in RunAllJSONScenariosInDirectory, but not in RunSingleJSONScenario.
func (ae *VMTestExecutor) Reset() {
	if !check.IfNil(ae.vmHost) {
		ae.vmHost.Reset()
	}
	ae.World.Clear()
}

// Close will simply close the VM
func (ae *VMTestExecutor) Close() {
	if !check.IfNil(ae.vmHost) {
		ae.vmHost.Reset()
	}
}

// RunScenario executes an individual test.
func (ae *VMTestExecutor) RunScenario(scenario *scenjsonmodel.Scenario, fileResolver scenfileresolver.FileResolver) error {
	ae.fileResolver = fileResolver
	ae.checkGas = scenario.CheckGas
	resetGasTracesIfNewTest(ae, scenario)

	err := ae.InitVM(scenario.GasSchedule)
	if err != nil {
		return err
	}

	txIndex := 0
	for _, generalStep := range scenario.Steps {
		setGasTraceInMetering(ae, true)
		err := ae.ExecuteStep(generalStep)
		if err != nil {
			return err
		}
		setGasTraceInMetering(ae, false)
		txIndex++
	}

	return nil
}

// ExecuteStep executes an individual step from a scenario.
func (ae *VMTestExecutor) ExecuteStep(generalStep scenjsonmodel.Step) error {
	err := error(nil)

	switch step := generalStep.(type) {
	case *scenjsonmodel.ExternalStepsStep:
		err = ae.ExecuteExternalStep(step)
		length := len(ae.scenarioTraceGas)
		ae.scenarioTraceGas = ae.scenarioTraceGas[:length-1]
		return err
	case *scenjsonmodel.SetStateStep:
		err = ae.ExecuteSetStateStep(step)
	case *scenjsonmodel.CheckStateStep:
		err = ae.ExecuteCheckStateStep(step)
	case *scenjsonmodel.TxStep:
		_, err = ae.ExecuteTxStep(step)
	case *scenjsonmodel.DumpStateStep:
		err = ae.DumpWorld()
	}

	logGasTrace(ae)

	return err
}

// ExecuteExternalStep executes an external step referenced by the scenario.
func (ae *VMTestExecutor) ExecuteExternalStep(step *scenjsonmodel.ExternalStepsStep) error {
	log.Trace("ExternalStepsStep", "path", step.Path)
	if len(step.Comment) > 0 {
		log.Trace("ExternalStepsStep", "comment", step.Comment)
	}

	fileResolverBackup := ae.fileResolver
	clonedFileResolver := ae.fileResolver.Clone()
	externalStepsRunner := scencontroller.NewScenarioController(ae, clonedFileResolver)

	extAbsPth := ae.fileResolver.ResolveAbsolutePath(step.Path)
	setExternalStepGasTracing(ae, step)

	err := externalStepsRunner.RunSingleJSONScenario(extAbsPth, scencontroller.DefaultRunScenarioOptions())
	if err != nil {
		return err
	}

	ae.fileResolver = fileResolverBackup

	return nil
}

// ExecuteSetStateStep executes a SetStateStep.
func (ae *VMTestExecutor) ExecuteSetStateStep(step *scenjsonmodel.SetStateStep) error {
	if len(step.Comment) > 0 {
		log.Trace("SetStateStep", "comment", step.Comment)
	}

	for _, scenAccount := range step.Accounts {
		if scenAccount.Update {
			err := ae.UpdateAccount(scenAccount)
			if err != nil {
				log.Debug("could not update account", err)
				return err
			}
		} else {
			err := ae.PutNewAccount(scenAccount)
			if err != nil {
				log.Debug("could not put new account", err)
				return err
			}
		}
	}

	// replace block info
	ae.World.PreviousBlockInfo = convertBlockInfo(step.PreviousBlockInfo, ae.World.PreviousBlockInfo)
	ae.World.CurrentBlockInfo = convertBlockInfo(step.CurrentBlockInfo, ae.World.CurrentBlockInfo)
	ae.World.Blockhashes = step.BlockHashes.ToValues()

	// append NewAddressMocks
	err := validateNewAddressMocks(step.NewAddressMocks)
	if err != nil {
		return err
	}
	addressMocksToAdd := convertNewAddressMocks(step.NewAddressMocks)
	ae.World.NewAddressMocks = append(ae.World.NewAddressMocks, addressMocksToAdd...)
	return ae.World.CommitChanges()
}

// ExecuteTxStep executes a TxStep.
func (ae *VMTestExecutor) ExecuteTxStep(step *scenjsonmodel.TxStep) (*vmi.VMOutput, error) {
	log.Trace("ExecuteTxStep", "id", step.TxIdent)
	if len(step.Comment) > 0 {
		log.Trace("ExecuteTxStep", "comment", step.Comment)
	}

	if step.DisplayLogs {
		vmhost.SetLoggingForTests()
	}

	output, err := ae.executeTx(step.TxIdent, step.Tx)
	if err != nil {
		return nil, err
	}

	if step.DisplayLogs {
		vmhost.DisableLoggingForTests()
	}

	// check results
	if step.ExpectedResult != nil {
		err = ae.checkTxResults(step.TxIdent, step.ExpectedResult, ae.checkGas, output)
		if err != nil {
			return nil, err
		}
	}

	return output, nil
}

// PutNewAccount Puts a new account in world account map. Overwrites.
func (ae *VMTestExecutor) PutNewAccount(scenAccount *scenjsonmodel.Account) error {
	worldAccount, err := convertAccount(scenAccount, ae.World)
	if err != nil {
		return err
	}
	err = validateSetStateAccount(scenAccount, worldAccount)
	if err != nil {
		return err
	}

	for _, stu := range scenAccount.Storage {
		ae.World.AccountsCacher.(*worldmock.WorldAccountsCacher).RegisterStorageKeyUse(
			scenAccount.Address.Value, stu.Key.Value,
		)
	}

	return ae.World.AccountsCacher.SaveUser(worldAccount)
}

// UpdateAccount Updates an account in world account map.
func (ae *VMTestExecutor) UpdateAccount(scenAccount *scenjsonmodel.Account) error {
	worldAccount, err := convertAccount(scenAccount, ae.World)
	if err != nil {
		return err
	}
	err = validateSetStateAccount(scenAccount, worldAccount)
	if err != nil {
		return err
	}

	existingAccount2, err := ae.World.AccountsAdapter.LoadAccount(scenAccount.Address.Value)
	if existingAccount2 == nil {
		return errors.New("account not found. could not update")
	}

	existingAccount := existingAccount2.(state.UserAccountHandler)

	if !scenAccount.Nonce.Unspecified {
		existingAccount.IncreaseNonce(worldAccount.GetNonce() - existingAccount.GetNonce())
	}
	if !scenAccount.Balance.Unspecified {
		err = existingAccount.AddToBalance(worldAccount.GetBalance(nil, true)-existingAccount.GetBalance(nil, true), nil, true)
		if err != nil {
			return err
		}
	}
	if !scenAccount.Username.Unspecified {
		existingAccount.SetName(worldAccount.GetName())
	}
	if !scenAccount.Owner.Unspecified {
		existingAccount.SetOwnerAddress(worldAccount.GetOwnerAddress())
	}
	if !scenAccount.Code.Unspecified {
		existingAccount.SetCode(ae.World.GetCode(worldAccount))
	}

	for _, stu := range scenAccount.Storage {
		ae.World.AccountsCacher.(*worldmock.WorldAccountsCacher).RegisterStorageKeyUse(
			scenAccount.Address.Value, stu.Key.Value,
		)
		_ = existingAccount.SaveKeyValue(stu.Key.Value, stu.Value.Value)
	}

	// set to cache
	return ae.World.AccountsCacher.SaveUser(existingAccount)
}
