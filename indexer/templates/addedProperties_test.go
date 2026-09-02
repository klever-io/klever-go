package templates_test

import (
	"testing"

	"github.com/klever-io/klever-go/indexer/templates"
	"github.com/klever-io/klever-go/indexer/templates/noKibana"
	"github.com/klever-io/klever-go/indexer/templates/withKibana"
	"github.com/stretchr/testify/require"
)

// TestTransactionsTemplatesCarryTheAddedProperties holds both transactions templates to
// templates.TransactionsAddedProperties, the same definition elasticProcessor puts onto a
// live index at start-up. A new index built from a template that lacks the property, or
// that types it differently, would disagree with every existing index on the field's type,
// and a filter written for one would miss the other.
func TestTransactionsTemplatesCarryTheAddedProperties(t *testing.T) {
	t.Parallel()

	require.Equal(t, templates.Object{"type": "keyword"}, templates.TransactionsAddedProperties["scAddresses"],
		"scAddresses is filtered with an exact term, which needs the keyword type")

	for name, template := range map[string]templates.Object{
		"noKibana":   noKibana.Transactions,
		"withKibana": withKibana.Transactions,
	} {
		properties := template["mappings"].(templates.Object)["properties"].(templates.Object)
		for field, mapping := range templates.TransactionsAddedProperties {
			require.Equal(t, mapping, properties[field],
				"%s transactions template must map %s exactly as the live index gets it", name, field)
		}
	}
}
