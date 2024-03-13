package noKibana

// ITOs will hold the configuration for the itos index
var ITOs = Object{
	"index_patterns": Array{
		"itos-*",
	},
	"settings": Object{
		"number_of_shards":   3,
		"number_of_replicas": 0,
	},
	"mappings": Object{
		"properties": Object{
			"isActive": Object{
				"type": "boolean",
			},
			"maxAmount": Object{
				"type": "long",
			},
			"packData": Object{
				"properties": Object{
					"key": Object{
						"type": "text",
						"fields": Object{
							"keyword": Object{
								"type":         "keyword",
								"ignore_above": 256,
							},
						},
					},
					"packs": Object{
						"properties": Object{
							"amount": Object{
								"type": "long",
							},
							"price": Object{
								"type": "long",
							},
						},
					},
				},
			},
			"receiverAddress": Object{
				"type": "text",
				"fields": Object{
					"keyword": Object{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"whitelistInfo": Object{
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
					"limit": Object{
						"type": "long",
					},
				},
			},
		},
	},
}
