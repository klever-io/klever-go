package noKibana

// Blocks will hold the configuration for the blocks index
var Blocks = Object{
	"index_patterns": Array{
		"blocks-*",
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
