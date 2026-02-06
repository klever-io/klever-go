package mock

// KeyValueHolderStub -
type KeyValueHolderStub struct {
	KeyCalled   func() []byte
	ValueCalled func() []byte
}

// Key -
func (kvhs *KeyValueHolderStub) Key() []byte {
	if kvhs.KeyCalled != nil {
		return kvhs.KeyCalled()
	}
	return nil
}

// Value -
func (kvhs *KeyValueHolderStub) Value() []byte {
	if kvhs.ValueCalled != nil {
		return kvhs.ValueCalled()
	}
	return nil
}
