package scencontroller

import (
	"io/ioutil"
	"os"
	"path/filepath"

	scenjsonparse "github.com/klever-io/klever-go/kvm/scenarioexec/json/parse"
	scenjsonwrite "github.com/klever-io/klever-go/kvm/scenarioexec/json/write"
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
)

// ParseScenariosScenario reads and parses a Scenarios scenario from a JSON file.
func ParseScenariosScenario(parser scenjsonparse.Parser, scenFilePath string) (*scenjsonmodel.Scenario, error) {
	var err error
	scenFilePath, err = filepath.Abs(scenFilePath)
	if err != nil {
		return nil, err
	}

	// Open our jsonFile
	var jsonFile *os.File
	jsonFile, err = os.Open(scenFilePath)
	// if we os.Open returns an error then handle it
	if err != nil {
		return nil, err
	}

	// defer the closing of our jsonFile so that we can parse it later on
	defer func() {
		_ = jsonFile.Close()
	}()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	parser.ExprInterpreter.FileResolver.SetContext(scenFilePath)
	return parser.ParseScenarioFile(byteValue)
}

// ParseScenariosScenarioDefaultParser reads and parses a Scenarios scenario from a JSON file.
func ParseScenariosScenarioDefaultParser(scenFilePath string) (*scenjsonmodel.Scenario, error) {
	parser := scenjsonparse.NewParser(NewDefaultFileResolver())
	parser.ExprInterpreter.FileResolver.SetContext(scenFilePath)
	return ParseScenariosScenario(parser, scenFilePath)
}

// WriteScenariosScenario exports a Scenarios scenario to a file, using the default formatting.
func WriteScenariosScenario(scenario *scenjsonmodel.Scenario, toPath string) error {
	jsonString := scenjsonwrite.ScenarioToJSONString(scenario)

	err := os.MkdirAll(filepath.Dir(toPath), os.ModePerm)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(toPath, []byte(jsonString), 0644)
}
