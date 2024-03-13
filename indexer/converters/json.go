package converters

import (
	"bytes"
	"encoding/json"
	"fmt"

	logger "github.com/klever-io/klever-go-logger"
)

const defaultStr = "default"

var log = logger.GetOrCreate("indexer/converters")

// JsonEscape will format the provided string in a json compatible string
func JsonEscape(i string) string {
	b, err := json.Marshal(i)
	if err != nil {
		log.Warn("converters.JsonEscape something went wrong",
			"input", i,
			"error", err,
		)
		return defaultStr
	}

	// Trim the beginning and trailing " character
	return string(b[1 : len(b)-1])
}

// PrepareHashesForQueryRemove will prepare the provided hashes for query remove
func PrepareHashesForQueryRemove(hashes []string) *bytes.Buffer {
	if len(hashes) == 0 {
		hashes = []string{}
	}

	serializedHashes, _ := json.Marshal(hashes)
	query := `{"query": {"ids": {"values": %s}}}`
	deleteQuery := fmt.Sprintf(query, serializedHashes)

	return bytes.NewBuffer([]byte(deleteQuery))
}
