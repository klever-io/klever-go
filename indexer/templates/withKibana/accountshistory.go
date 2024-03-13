package withKibana

// AccountsHistory will hold the configuration for the accountshistory index
var AccountsHistory = Object{
	"index_patterns": Array{
		"accountshistory-*",
	},
	"settings": Object{
		"number_of_shards":   5,
		"number_of_replicas": 0,
		//"opendistro.index_state_management.rollover_alias": "accountshistory",
	},
	"mappings": Object{
		"properties": Object{
			"timestamp": Object{
				"type": "date",
			},
		},
	},
}
