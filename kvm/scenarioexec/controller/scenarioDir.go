package scencontroller

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	logger "github.com/klever-io/klever-go-logger"
)

var log = logger.GetOrCreate("operator")

// RunAllJSONScenariosInDirectory walks directory, parses and prepares all json scenarios,
// then calls ScenarioRunner for each of them.
func (r *ScenarioController) RunAllJSONScenariosInDirectory(
	generalTestPath string,
	specificTestPath string,
	allowedSuffix string,
	excludedFilePatterns []string,
	options *RunScenarioOptions) error {

	mainDirPath := path.Join(generalTestPath, specificTestPath)
	var nrPassed, nrFailed, nrSkipped int

	err := filepath.Walk(mainDirPath, func(testFilePath string, info os.FileInfo, err error) error {
		if strings.HasSuffix(testFilePath, allowedSuffix) {
			scenarioDisplayName := shortenTestPath(testFilePath, generalTestPath)
			log.Info(fmt.Sprintf("Scenario: %s ... ", scenarioDisplayName))
			if isExcluded(excludedFilePatterns, testFilePath, generalTestPath) {
				nrSkipped++
				log.Info(fmt.Sprintf("%s  %s\n", scenarioDisplayName, "skip"))
			} else {
				r.Executor.Reset()
				r.RunsNewTest = true
				testErr := r.RunSingleJSONScenario(testFilePath, options)
				if testErr == nil {
					nrPassed++
					log.Info(fmt.Sprintf("%s  %s\n", scenarioDisplayName, "ok"))
				} else {
					nrFailed++
					log.Info(fmt.Sprintf("%s  %s %s\n", scenarioDisplayName, "FAIL:", testErr.Error()))
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("Done. Passed: %d. Failed: %d. Skipped: %d.\n", nrPassed, nrFailed, nrSkipped))
	if nrFailed > 0 {
		return errors.New("some tests failed")
	}

	return nil
}
