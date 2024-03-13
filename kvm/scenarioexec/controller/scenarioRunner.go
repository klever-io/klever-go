package scencontroller

import (
	scenfileresolver "github.com/klever-io/klever-go/kvm/scenarioexec/fileresolver"
	scenjsonparse "github.com/klever-io/klever-go/kvm/scenarioexec/json/parse"
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
)

// ScenarioRunner describes a component that can run a VM scenario.
type ScenarioRunner interface {
	// Reset clears state/world.
	Reset()

	// RunScenario executes the scenario and checks if it passed. Failure is signaled by returning an error.
	// The FileResolver helps with resolving external steps.
	RunScenario(*scenjsonmodel.Scenario, scenfileresolver.FileResolver) error
}

// ScenarioController is a component that can run json scenarios, using a provided executor.
type ScenarioController struct {
	Executor    ScenarioRunner
	RunsNewTest bool
	Parser      scenjsonparse.Parser
}

// NewScenarioController creates new ScenarioController instance.
func NewScenarioController(executor ScenarioRunner, fileResolver scenfileresolver.FileResolver) *ScenarioController {
	return &ScenarioController{
		Executor: executor,
		Parser:   scenjsonparse.NewParser(fileResolver),
	}
}
