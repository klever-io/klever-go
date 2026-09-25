package noKibana

// Logs will hold the configuration for the logs index
var Logs = Object{
	"index_patterns": Array{
		"logs-*",
	},
	"settings": Object{
		"number_of_shards":   3,
		"number_of_replicas": 0,
	},
	"mappings": Object{
		"properties": Object{
			"address": Object{
				"type": "keyword",
			},
			"caller": Object{
				"type": "keyword",
			},
			"contractId": Object{
				"type": "integer",
			},
			"status": Object{
				"type": "keyword",
			},
			"resultCode": Object{
				"type": "keyword",
			},
			"events": Object{
				"type": "nested",
				"properties": Object{
					"address": Object{
						"type": "keyword",
					},
					"data": Object{
						"index": "false",
						"type":  "text",
					},
					"identifier": Object{
						"type": "keyword",
					},
					"topics": Object{
						"type": "text",
					},
					"order": Object{
						"type": "integer",
					},
					"isSystemLog": Object{
						"type": "boolean",
					},
				},
			},
			"timestamp": Object{
				"type":   "date",
				"format": "epoch_second",
			},
		},
	},
}
