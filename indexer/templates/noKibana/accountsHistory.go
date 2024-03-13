package noKibana

// AccountsHistory will hold the configuration for the accountshistory index
var AccountsHistory = Object{
	"index_patterns": Array{
		"accountshistory-*",
	},
	"settings": Object{
		"number_of_shards":   5,
		"number_of_replicas": 0,
	},
	"mappings": Object{
		"properties": Object{
			"timestamp": Object{
				"type": "date",
			},
		},
	},
}
