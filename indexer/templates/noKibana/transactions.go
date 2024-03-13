package noKibana

// Transactions will hold the configuration for the transactions index
var Transactions = Object{
	"index_patterns": Array{
		"transactions-*",
	},
	"settings": Object{
		"number_of_shards":   5,
		"number_of_replicas": 0,
	},

	"mappings": Object{
		"properties": Object{
			"nonce": Object{
				"type": "long",
			},
			"timestamp": Object{
				"type": "date",
			},
			"receipts": Object{
				"type": "nested",
				"properties": Object{
					"value": Object{
						"type": "long",
					},
				},
			},
		},
	},
}
