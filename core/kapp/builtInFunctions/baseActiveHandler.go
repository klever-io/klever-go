package builtInFunctions

type baseAlwaysActiveHandler struct {
}

// IsActive returns true as this built-in function is always active
func (b baseAlwaysActiveHandler) IsActive() bool {
	return true
}

// IsInterfaceNil always returns false
func (b baseAlwaysActiveHandler) IsInterfaceNil() bool {
	return false
}
