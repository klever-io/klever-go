package noKibana

import (
	"github.com/klever-io/klever-go/indexer/templates"
)

type Object = templates.Object
type Array = templates.Array

// Accounts will hold the configuration for the accounts index
var Accounts = Object{
	"index_patterns": Array{
		"accounts-*",
	},
	"settings": Object{
		"number_of_shards":   3,
		"number_of_replicas": 0,
		"index": Object{
			"sort.field": Array{
				"timestamp", "nonce",
			},
			"sort.order": Array{
				"desc", "desc",
			},
		},
	},
	"mappings": Object{
		"properties": Object{
			"nonce": Object{
				"type": "long",
			},
			"timestamp": Object{
				"type": "date",
			},
		},
	},
}
