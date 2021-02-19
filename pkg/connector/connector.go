package connector

type Connector interface {
	PairInfo(base, quote string) (PairInfo, error)
	AddOrder(appID int, order Order) (string, error)
	CancelOrder(order Order) error
	OrdersDetails(appID int, ordersIds []string) []Order
}

type PairInfo struct {
	PairName            string
	BasePricePrecision  int
	QuotePricePrecision int
	BaseMinVolume       float64
}

type Order struct {
	ID     string
	Pair   string
	Status string
}
