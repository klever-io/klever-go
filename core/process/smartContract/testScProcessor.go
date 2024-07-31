package smartContract

import (
	"fmt"
	"strings"

	"github.com/klever-io/klever-go/core"
)

// TestScProcessor extends scProcessor and is used in tests as it exposes some functions
// that are not supposed to be used in production code
// Exported functions simplify the reproduction of edge cases
type TestScProcessor struct {
	*scProcessor
}

// NewTestScProcessor -
func NewTestScProcessor(internalData *scProcessor) *TestScProcessor {
	return &TestScProcessor{internalData}
}

// GetCompositeTestError composes all errors found in the logs or by parsing the scr forwarder's contents
func (tsp *TestScProcessor) GetCompositeTestError() error {
	var returnError error

	allLogs := tsp.txLogsProcessor.GetAllCurrentLogs()
	for _, logs := range allLogs {
		for _, event := range logs.GetLogEvents() {
			if string(event.GetIdentifier()) == core.SignalErrorOperation {
				returnError = wrapErrorIfNotContains(returnError, string(event.GetTopics()[1]))
			}
		}
	}

	tsp.txLogsProcessor.Clean()

	return returnError
}

func wrapErrorIfNotContains(originalError error, msg string) error {
	if originalError == nil {
		return fmt.Errorf(msg)
	}

	alreadyContainsMessage := strings.Contains(originalError.Error(), msg)
	if alreadyContainsMessage {
		return originalError
	}

	return fmt.Errorf("%s: %s", originalError.Error(), msg)
}
