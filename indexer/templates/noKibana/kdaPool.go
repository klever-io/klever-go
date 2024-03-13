package noKibana

// KDAPools will hold the configuration for the kdaPools index
var KDAPools = Object{
	"index_patterns": Array{
		"kdappols-*",
	},
	"settings": Object{
		"number_of_shards":   3,
		"number_of_replicas": 0,
	},
	"mappings": Object{
		"properties": Object{
			"active": Object{
				"type": "boolean",
			},
			"hidden": Object{
				"type": "boolean",
			},
			"verified": Object{
				"type": "boolean",
			},
			"klvBalance": Object{
				"type": "long",
			},
			"kdaBalance": Object{
				"type": "long",
			},
			"fRatioKLV": Object{
				"type": "long",
			},
			"fRatioKDA": Object{
				"type": "long",
			},
			"ownerAddress": Object{
				"type": "text",
				"fields": Object{
					"keyword": Object{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"adminAddress": Object{
				"type": "text",
				"fields": Object{
					"keyword": Object{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"kda": Object{
				"type":      "text",
				"fielddata": "true",
				"fields": Object{
					"keyword": Object{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
		},
	},
}
