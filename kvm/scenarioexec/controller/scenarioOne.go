package scencontroller

import (
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
)

// RunScenarioOptions defines the scenario options component
type RunScenarioOptions struct {
	ForceTraceGas bool
}

func applyScenarioOptions(scenario *scenjsonmodel.Scenario, options *RunScenarioOptions) {
	if options.ForceTraceGas {
		scenario.TraceGas = true
	}
}

// DefaultRunScenarioOptions creates a new RunScenarioOptions instance
func DefaultRunScenarioOptions() *RunScenarioOptions {
	return &RunScenarioOptions{
		ForceTraceGas: false,
	}
}

// RunSingleJSONScenario parses and prepares test, then calls testCallback.
func (r *ScenarioController) RunSingleJSONScenario(contextPath string, options *RunScenarioOptions) error {
	scenario, parseErr := ParseScenariosScenario(r.Parser, contextPath)
	if parseErr != nil {
		return parseErr
	}

	if r.RunsNewTest {
		scenario.IsNewTest = true
		r.RunsNewTest = false
	}
	applyScenarioOptions(scenario, options)
	return r.Executor.RunScenario(scenario, r.Parser.ExprInterpreter.FileResolver)
}
