package txfeesmocks

// TxFeesHandler -
type TxFeesHandler interface {
	// TODO:
	IsInterfaceNil() bool
}

// TxFeesHandlerMock -
type TxFeesHandlerMock struct{}

// IsInterfaceNil -
func (ghm *TxFeesHandlerMock) IsInterfaceNil() bool {
	return ghm == nil
}
