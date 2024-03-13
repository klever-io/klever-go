package noKibana

// Rating will hold the configuration for the rating index
var Rating = Object{
	"index_patterns": Array{
		"rating-*",
	},
	"settings": Object{
		"number_of_shards":   1,
		"number_of_replicas": 0,
	},

	"mappings": Object{
		"properties": Object{
			"validatorsRating": Object{
				"properties": Object{
					"rating": Object{
						"type": "float",
					},
				},
			},
		},
	},
}
