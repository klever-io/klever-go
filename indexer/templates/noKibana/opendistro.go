package noKibana

// OpenDistro will hold the configuration for the opendistro
var OpenDistro = Object{
	"index_patterns": Array{
		".opendistro-*",
	},
	"settings": Object{
		"number_of_shards":   1,
		"number_of_replicas": 0,
	},
}
