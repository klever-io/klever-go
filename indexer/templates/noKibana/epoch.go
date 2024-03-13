package noKibana

// Epoch will hold the configuration for the epoch index
var Epoch = Object{
	"index_patterns": Array{
		"epoch-*",
	},
	"settings": Object{
		"number_of_shards":   3,
		"number_of_replicas": 0,
	},
}
