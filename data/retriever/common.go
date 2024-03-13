package retriever

import "github.com/klever-io/klever-go/common"

// SetEpochHandlerToHdrResolver sets the epoch handler to the metablock hdr resolver
func SetEpochHandlerToHdrResolver(
	resolversContainer ResolversContainer,
	epochHandler EpochHandler,
) error {
	resolver, err := resolversContainer.Get(common.BlocksTopic)
	if err != nil {
		return err
	}

	hdrResolver, ok := resolver.(HeaderResolver)
	if !ok {
		return common.ErrWrongTypeInContainer
	}

	err = hdrResolver.SetEpochHandler(epochHandler)
	if err != nil {
		return err
	}

	return nil
}
