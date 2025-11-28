package indexer

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/klever-io/klever-go/indexer/templates"
)

func encode(obj templates.Object) (bytes.Buffer, error) {
	var buff bytes.Buffer
	if err := json.NewEncoder(&buff).Encode(obj); err != nil {
		return bytes.Buffer{}, fmt.Errorf("error encoding : %w", err)
	}

	return buff, nil
}

func prepareHashesForBulkRemove(hashes []string) templates.Object {
	return templates.Object{
		"query": templates.Object{
			"ids": templates.Object{
				"values": hashes,
			},
		},
	}
}

func prepareTimestampForBulkRemove(timestamp int64) templates.Object {
	return templates.Object{
		"query": templates.Object{
			"term": templates.Object{
				"timestamp": timestamp,
			},
		},
	}
}
