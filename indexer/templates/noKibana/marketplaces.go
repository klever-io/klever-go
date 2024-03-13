package noKibana

// Marketplaces will hold the configuration for the marketplaces index
var Marketplaces = Object{
	"index_patterns": Array{
		"marketplaces-*",
	},
	"settings": Object{
		"number_of_shards":   3,
		"number_of_replicas": 0,
	},
}
