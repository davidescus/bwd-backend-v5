package app

import (
	"bwd/pkg/storage"
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	AppStatusActive   = "ACTIVE"
	AppStatusInactive = "INACTIVE"
)

type ConfigApp struct {
	Storer          storage.AppStorer
	ID              int
	Exchange        string
	MarketOrderFees float64
	LimitOrderFees  float64
	Base            string
	Quote           string
	StepsType       string
	StepsDetails    string
}

type ConfigParams struct {
	Interval           time.Duration
	QuotePercentUse    float64
	MinBasePrice       float64
	MaxBasePrice       float64
	StepQuoteVolume    float64
	CompoundType       string
	CompoundDetails    string
	PublishOrderNumber int
	Status             string
}

type App struct {
	ctx             context.Context
	logger          *logrus.Logger
	storer          storage.AppStorer
	id              int
	exchange        string
	base            string
	quote           string
	marketOrderFees float64
	limitOrderFees  float64
	stepsType       string
	stepsDetails    string
	// params that can be changed
	interval           time.Duration
	quotePercentUse    float64
	minBasePrice       float64
	maxBasePrice       float64
	stepQuoteVolume    float64
	compoundType       string
	compoundDetails    string
	publishOrderNumber int
	status             string
}

func New(ctx context.Context, cfg *ConfigApp, logger *logrus.Logger) *App {
	return &App{
		ctx:      ctx,
		logger:   logger,
		storer:   cfg.Storer,
		id:       cfg.ID,
		exchange: cfg.Exchange,
		base:     cfg.Base,
		quote:    cfg.Quote,
	}
}

func (a *App) ConfigParams() *ConfigParams {
	return &ConfigParams{
		Interval:           a.interval,
		QuotePercentUse:    a.quotePercentUse,
		MinBasePrice:       a.minBasePrice,
		MaxBasePrice:       a.maxBasePrice,
		StepQuoteVolume:    a.stepQuoteVolume,
		CompoundType:       a.compoundType,
		CompoundDetails:    a.compoundDetails,
		PublishOrderNumber: a.publishOrderNumber,
		Status:             a.status,
	}
}

func (a *App) SetConfigParams(cfg *ConfigParams) {
	a.interval = cfg.Interval
	a.quotePercentUse = cfg.QuotePercentUse
	a.minBasePrice = cfg.MinBasePrice
	a.maxBasePrice = cfg.MaxBasePrice
	a.stepQuoteVolume = cfg.StepQuoteVolume
	a.compoundType = cfg.CompoundType
	a.compoundDetails = cfg.CompoundDetails
	a.publishOrderNumber = cfg.PublishOrderNumber
	a.status = cfg.Status
}

func (a *App) Start() error {
	// TODO implement me

	return nil
}

func (a *App) Stop() {
	// TODO implement me
}
