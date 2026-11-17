package withKibana

// AccountsKDAHistory will hold the configuration for the accountskdahistory index
var AccountsKDAHistory = Object{
	"index_patterns": Array{
		"accountskdahistory-*",
	},
	"settings": Object{
		"number_of_shards":   5,
		"number_of_replicas": 0,
	},
	"mappings": Object{
		"properties": Object{
			"address": Object{
				"type": "keyword",
			},
			"token": Object{
				"type": "keyword",
			},
			"timestamp": Object{
				"type": "date",
			},
			"balance": Object{
				"type": "long",
			},
			"frozenBalance": Object{
				"type": "long",
			},
			"unfrozenBalance": Object{
				"type": "long",
			},
		},
	},
}
