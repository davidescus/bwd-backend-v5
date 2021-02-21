package storage

import "time"

// Bwd interface
type BwdStorer interface {
	Apps() ([]App, error)
}

// App, Trader interface
type AppStorer interface{}

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
