package vm

// ReturnDataKind specifies how to interpret VMOutputs's return data.
// More specifically, how to interpret returned data's first item.
type ReturnDataKind int

const (
	// AsBigInt to interpret as big int
	AsBigInt ReturnDataKind = 1 << iota
	// AsBigIntString to interpret as big int string
	AsBigIntString
	// AsString to interpret as string
	AsString
	// AsHex to interpret as hex
	AsHex
)
