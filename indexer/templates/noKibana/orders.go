package noKibana

// MarketplaceOrders will hold the configuration for the orders index
var MarketplaceOrders = Object{
	"index_patterns": Array{
		"marketplaceorders-*",
	},
	"settings": Object{
		"number_of_shards":   3,
		"number_of_replicas": 0,
	},
	"mappings": Object{
		"properties": Object{
			"orderId": Object{
				"type": "text",
				"fields": Object{
					"keyword": Object{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"marketType": Object{
				"type": "text",
				"fields": Object{
					"keyword": Object{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"marketplaceId": Object{
				"type": "text",
				"fields": Object{
					"keyword": Object{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"collectionId": Object{
				"type": "text",
				"fields": Object{
					"keyword": Object{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"assetId": Object{
				"type": "text",
				"fields": Object{
					"keyword": Object{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"currencyId": Object{
				"type": "text",
			},
			"price": Object{
				"type": "long",
			},
			"reservePrice": Object{
				"type": "long",
			},
			"endTime": Object{
				"type": "date",
			},
			"status": Object{
				"type": "text",
				"fields": Object{
					"keyword": Object{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"buyOrders": Object{
				"type": "nested",
				"properties": Object{
					"currencyId": Object{
						"type": "text",
					},
					"amount": Object{
						"type": "long",
					},
					"buyer": Object{
						"type": "text",
						"fields": Object{
							"keyword": Object{
								"type":         "keyword",
								"ignore_above": 256,
							},
						},
					},
					"status": Object{
						"type": "text",
						"fields": Object{
							"keyword": Object{
								"type":         "keyword",
								"ignore_above": 256,
							},
						},
					},
				},
			},
		},
	},
}
