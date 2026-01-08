package data

// AlteredSmartContract
type AlteredSmartContract struct {
	IsNew bool
}

// AlteredSmartContractsHandler defines the actions that an altered smart contracts handler should do
type AlteredSmartContractsHandler interface {
	Add(key string, smartContract *AlteredSmartContract)
	Get(key string) ([]*AlteredSmartContract, bool)
	GetAll() map[string][]*AlteredSmartContract
	Len() int
	IsInterfaceNil() bool
}

type alteredSmartContracts struct {
	altered map[string][]*AlteredSmartContract
}

// NewAlteredSmartContracts will create a new instance of alteredSmartContracts
func NewAlteredSmartContracts() *alteredSmartContracts {
	return &alteredSmartContracts{
		altered: make(map[string][]*AlteredSmartContract),
	}
}

func (asc *alteredSmartContracts) Add(key string, smartContract *AlteredSmartContract) {
	_, ok := asc.altered[key]
	if !ok {
		asc.altered[key] = make([]*AlteredSmartContract, 0)
	}

	// Always append to track each transaction interaction with the smart contract
	// This ensures accurate transaction counting for totalTransactions
	asc.altered[key] = append(asc.altered[key], smartContract)
}

func (asc *alteredSmartContracts) Get(key string) ([]*AlteredSmartContract, bool) {
	altered, ok := asc.altered[key]

	return altered, ok
}

func (asc *alteredSmartContracts) Len() int {
	return len(asc.altered)
}

func (asc *alteredSmartContracts) GetAll() map[string][]*AlteredSmartContract {
	if asc == nil || asc.altered == nil {
		return map[string][]*AlteredSmartContract{}
	}

	return asc.altered
}

// IsInterfaceNil returns true if underlying object is nil
func (asc *alteredSmartContracts) IsInterfaceNil() bool {
	return asc == nil
}
