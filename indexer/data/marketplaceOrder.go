package data

type Order struct {
	OrderID               string      `json:"orderId"`
	MarketType            string      `json:"marketType"`
	MarketplaceID         string      `json:"marketplaceId"`
	CollectionID          string      `json:"collectionId"`
	AssetID               string      `json:"assetId"`
	CurrencyID            string      `json:"currencyId"`
	Price                 int64       `json:"price"`
	ReservePrice          int64       `json:"reservePrice"`
	CurrentBidder         string      `json:"currentBidder"`
	CurrentBid            int64       `json:"currentBid"`
	CurrentBuyOrderTxHash string      `json:"currentBuyOrderTxHash"`
	EndTime               int64       `json:"endTime"`
	Status                OrderStatus `json:"status"`
	BuyOrders             []*BuyOrder `json:"buyOrders"`
	OrderTxHash           string      `json:"orderTxHash"`
	OwnerAddress          string      `json:"ownerAddress"`
	Timestamp             int64       `json:"timestamp,omitempty"`
}

type BuyOrder struct {
	CurrencyID string      `json:"currencyId"`
	Amount     int64       `json:"amount"`
	Bidder     string      `json:"bidder"`
	Status     OrderStatus `json:"status"`
}

type OrderStatus string

const (
	Fulfilled OrderStatus = "fulfilled"
	Created   OrderStatus = "created"
	Canceled  OrderStatus = "canceled"
)
