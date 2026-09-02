package templates

import (
	"bytes"
	"encoding/json"
)

// Array type will rename type []interface{}
type Array []interface{}

// Object data will rename type map[string]interface{}
type Object map[string]interface{}

// ToBuffer will convert an Object to a *bytes.Buffer
func (o *Object) ToBuffer() *bytes.Buffer {
	objectBytes, _ := json.Marshal(o)

	buff := &bytes.Buffer{}
	_, _ = buff.Write(objectBytes)

	return buff
}

// TransactionsAddedProperties are mapping properties added to the transactions index after
// deployments may already carry that index. Both transactions templates include them, so a
// new index gets them at creation; elasticProcessor puts the same properties onto a live
// index at start-up, because a template only applies when an index is created and a field
// first written without a mapping is typed dynamically, as text with a keyword subfield,
// which is not the keyword type a filter on it is written for. One definition, two uses,
// so the template and the live index cannot disagree on the type.
var TransactionsAddedProperties = Object{
	"scAddresses": Object{
		"type": "keyword",
	},
}
