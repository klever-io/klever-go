package data

// MarketOperations
type MarketOperations struct {
	IsNew          bool
	IsClaim        bool
	IsCanceled     bool
	Status         string
	BuyOrder       *BuyOrder
	Executed       bool
	OrderTxHash    string
	BuyOrderTxHash string
	Timestamp      int64
}

// MarketOperationsHandler
type MarketOperationsHandler interface {
	Add(key string, order *MarketOperations)
	Get(key string) (*MarketOperations, bool)
	GetAll() map[string]*MarketOperations
	Len() int
	IsInterfaceNil() bool
}

type marketOperations struct {
	altered map[string]*MarketOperations
}

// NewMarketOperations will create a new instance of marketOperations
func NewMarketOperations() *marketOperations {
	return &marketOperations{
		altered: make(map[string]*MarketOperations),
	}
}

func (ao *marketOperations) Add(key string, order *MarketOperations) {
	ao.altered[key] = order
}

func (ao *marketOperations) Get(key string) (*MarketOperations, bool) {
	altered, ok := ao.altered[key]

	return altered, ok
}

func (ao *marketOperations) Len() int {
	return len(ao.altered)
}

func (ao *marketOperations) GetAll() map[string]*MarketOperations {
	if ao == nil || ao.altered == nil {
		return map[string]*MarketOperations{}
	}

	return ao.altered
}

// IsInterfaceNil returns true if underlying object is nil
func (ao *marketOperations) IsInterfaceNil() bool {
	return ao == nil
}
