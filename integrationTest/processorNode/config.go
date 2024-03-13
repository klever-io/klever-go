package processorNode

var (
	elastcEnabled = false
)

// WithElastic to active elastic you must run a docker-compose with Elastic & Kibana
func WithElastic() {
	elastcEnabled = true
}
