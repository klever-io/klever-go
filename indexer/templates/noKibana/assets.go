package noKibana

// Assets will hold the configuration for the assets index
var Assets = Object{
	"index_patterns": Array{
		"assets-*",
	},
	"settings": Object{
		"number_of_shards":   3,
		"number_of_replicas": 0,
	},
}
