package scencontroller

import (
	"io"
	"os"
	"path/filepath"

	"github.com/klever-io/klever-go/common"
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
	jsonFile, err = os.Open(filepath.Clean(scenFilePath))
	// if we os.Open returns an error then handle it
	if err != nil {
		return nil, err
	}

	// defer the closing of our jsonFile so that we can parse it later on
	defer func() {
		_ = jsonFile.Close()
	}()

	byteValue, err := io.ReadAll(jsonFile)
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

	err := os.MkdirAll(filepath.Dir(toPath), common.DefaultDirPermission)
	if err != nil {
		return err
	}

	return os.WriteFile(toPath, []byte(jsonString), common.DefaultDirPermission)
}
