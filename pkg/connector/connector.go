package connector

type Connector interface {
	PairInfo(base, quote string) (PairInfo, error)
}

type PairInfo struct {
	PairName            string
	BasePricePrecision  int
	QuotePricePrecision int
	BaseMinVolume       float64
}
