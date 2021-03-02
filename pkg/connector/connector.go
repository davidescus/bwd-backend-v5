package connector

const (
	OrderTypeMarket = "MARKET"
	OrderTypeLimit  = "LIMIT"

	OrderSideBuy  = "BUY"
	OrderSideSell = "SELL"

	OrderStatusNew      = "NEW"
	OrderStatusExecuted = "EXECUTED"
	OrderStatusNotFound = "NOT_FOUND"
)

type Connector interface {
	Start() error
	Stop()
	PairInfo(base, quote string) (PairInfo, error)
	AddOrder(appID int, order Order) (string, error)
	CancelOrder(order Order) error
	OrderDetails(appID int, order Order) (Order, error)
	OrdersDetails(appID int) []Order
}

type PairInfo struct {
	BasePricePrecision  int
	QuotePricePrecision int
	BasePrice           struct {
		Min, Max, Tick float64
	}
	BaseLot struct {
		Min, Max, Tick float64
	}
	QuoteMinVolume float64
}

type Order struct {
	ID        string
	Base      string
	Quote     string
	OrderType string
	Side      string
	Price     float64
	Volume    float64
	Status    string
}
