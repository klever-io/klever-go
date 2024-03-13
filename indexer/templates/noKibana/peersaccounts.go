package noKibana

// PeersAccounts will hold the configuration for the peersaccounts index
var PeersAccounts = Object{
	"index_patterns": Array{
		"peersaccounts-*",
	},
	"settings": Object{
		"number_of_shards":   3,
		"number_of_replicas": 0,
		"index": Object{
			"sort.field": Array{
				"list", "rating",
			},
			"sort.order": Array{
				"asc", "desc",
			},
		},
	},

	"mappings": Object{
		"properties": Object{
			"list": Object{
				"type": "keyword",
			},
			"rating": Object{
				"type": "long",
			},
		},
	},
}
