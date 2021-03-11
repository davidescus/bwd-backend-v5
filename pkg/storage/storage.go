package storage

import "time"

// Bwd interface
type Storer interface {
	// Bwd
	Apps() ([]App, error)
	// Trader
	ActiveTrades(appID int) ([]Trade, error)
	AddTrade(trade Trade) (int, error)
	UpdateTrade(trade Trade) error
	// Compounder
	LatestBalanceHistory(appID int) (BalanceHistory, error)
	LatestTradeBalanceHistory(appID int, tradeID int) (BalanceHistory, error)
	AddBalanceHistory(appID int, balance BalanceHistory) error
	LatestAppClosedTradeByOpenPrice(appID int, openPrice float64) (Trade, error)
}

type App struct {
	ID                 int
	Interval           time.Duration
	Exchange           string
	MarketOrderFees    float64
	LimitOrderFees     float64
	Base               string
	Quote              string
	QuotePercentUse    float64
	MinBasePrice       float64
	MaxBasePrice       float64
	StepQuoteVolume    float64
	StepsType          string
	StepsDetails       string
	CompoundType       string
	CompoundDetails    string
	PublishOrderNumber int
	Status             string
	IsDone             bool
}

type Trade struct {
	ID                   int
	AppID                int
	OpenBasePrice        float64
	CloseBasePrice       float64
	OpenType             string
	CloseType            string
	BaseVolume           float64
	BuyOrderID           string
	SellOrderID          string
	Status               string
	ConvertedSellLimitAt time.Time
	ClosedAt             time.Time
	UpdatedAt            time.Time
	CreatedAt            time.Time
}

type BalanceHistory struct {
	AppID           int
	Action          string
	QuoteVolume     float64
	TotalNetIncome  float64
	TotalReinvested float64
	InternalTradeID int
	CreatedAt       time.Time
}
