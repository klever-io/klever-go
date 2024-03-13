package scencontroller

import scenfileresolver "github.com/klever-io/klever-go/kvm/scenarioexec/fileresolver"

// NewDefaultFileResolver yields a new DefaultFileResolver instance.
// Reexported here to avoid having all external packages importing the parsscenexpressionreconstructor.
// DefaultFileResolver is in parse for local tests only.
func NewDefaultFileResolver() *scenfileresolver.DefaultFileResolver {
	return scenfileresolver.NewDefaultFileResolver()
}
