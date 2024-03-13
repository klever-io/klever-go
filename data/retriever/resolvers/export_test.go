package resolvers

import (
	"github.com/klever-io/klever-go/data/retriever"
)

func (hdrRes *HeaderResolver) EpochHandler() retriever.EpochHandler {
	return hdrRes.epochHandler
}
