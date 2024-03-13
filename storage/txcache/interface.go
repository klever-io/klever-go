package txcache

type scoreComputer interface {
	computeScore(scoreParams senderScoreParams) uint32
}

// TxFeesHandler handles a transaction gas and gas cost
type TxFeesHandler interface {
	// TODO:
	IsInterfaceNil() bool
}

// ForEachTransaction is an iterator callback
type ForEachTransaction func(txHash []byte, value *WrappedTransaction)
