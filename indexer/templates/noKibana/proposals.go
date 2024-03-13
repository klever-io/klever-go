package noKibana

// Proposals will hold the configuration for the proposals index
var Proposals = Object{
	"index_patterns": Array{
		"proposals-*",
	},
	"settings": Object{
		"number_of_shards":   3,
		"number_of_replicas": 0,
	},
	"mappings": Object{
		"properties": Object{
			"proposalId": Object{
				"type": "long",
			},
			"proposalStatus": Object{
				"type": "text",
				"fields": Object{
					"keyword": Object{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"proposer": Object{
				"type": "text",
				"fields": Object{
					"keyword": Object{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"timestamp": Object{
				"type": "date",
			},
			"epochEnd": Object{
				"type": "long",
			},
			"epochStart": Object{
				"type": "long",
			},
			"voters": Object{
				"type": "nested",
				"properties": Object{
					"address": Object{
						"type": "text",
						"fields": Object{
							"keyword": Object{
								"type":         "keyword",
								"ignore_above": 256,
							},
						},
					},
					"amount": Object{
						"type": "long",
					},
					"timestamp": Object{
						"type": "long",
					},
					"type": Object{
						"type": "long",
					},
				},
			},
			"votes": Object{
				"properties": Object{
					"0": Object{
						"type": "long",
					},
					"1": Object{
						"type": "long",
					},
				},
			},
		},
	},
}
