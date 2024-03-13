package withKibana

// Accounts will hold the configuration for the accounts index
var AccountsKDA = Object{
	"index_patterns": Array{
		"accountskda-*",
	},
	"settings": Object{
		"number_of_shards":   3,
		"number_of_replicas": 0,
	},
	"mappings": Object{
		"properties": Object{
			"buckets": Object{
				"type":              "nested",
				"include_in_parent": true,
				"properties": Object{
					"id": Object{
						"type":  "text",
						"index": "false",
					},
					"stakeAt": Object{
						"type": "long",
					},
					"stakedEpoch": Object{
						"type": "long",
					},
					"unstakedEpoch": Object{
						"type": "long",
					},
					"balance": Object{
						"type": "long",
					},
					"delegation": Object{
						"type": "keyword",
					},
				},
			},
		},
	},
}
