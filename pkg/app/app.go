package app

import (
	"bwd/pkg/connector"
	"bwd/pkg/storage"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

type ConfigApp struct {
	Storer             storage.AppStorer
	Connector          connector.Connector
	Interval           time.Duration
	ID                 int
	Exchange           string
	MarketOrderFees    float64
	LimitOrderFees     float64
	Base               string
	Quote              string
	StepsType          string
	StepsDetails       string
	MinBasePrice       float64
	MaxBasePrice       float64
	StepQuoteVolume    float64
	CompoundType       string
	CompoundDetails    string
	PublishOrderNumber int
}

type App struct {
	ctx                context.Context
	logger             *logrus.Logger
	storer             storage.AppStorer
	connector          connector.Connector
	interval           time.Duration
	id                 int
	exchange           string
	pair               pair
	fees               fees
	steps              steps
	basePrice          basePrice
	compound           compound
	pairInfo           pairInfo
	publishOrderNumber int
	doneSig            chan struct{}
	stepQuoteVolume    float64
	cancelFunc         func()
}

type pair struct {
	base, quote string
}
type fees struct {
	market, limit float64
}
type steps struct {
	kind, details string
}
type compound struct {
	kind, details string
}
type basePrice struct {
	min, max float64
}
type pairInfo struct {
	basePricePrecision  int
	quotePricePrecision int
	baseMinVolume       float64
}

// TODO add all config on app
func New(cfg *ConfigApp, logger *logrus.Logger) *App {
	appCtx, cancel := context.WithCancel(context.Background())

	return &App{
		ctx:        appCtx,
		cancelFunc: cancel,
		logger:     logger,
		storer:     cfg.Storer,
		connector:  cfg.Connector,
		interval:   cfg.Interval,
		id:         cfg.ID,
		exchange:   cfg.Exchange,
		pair: pair{
			base:  cfg.Base,
			quote: cfg.Quote,
		},
		fees: fees{
			market: cfg.MarketOrderFees,
			limit:  cfg.LimitOrderFees,
		},
		steps: steps{
			kind:    cfg.StepsType,
			details: cfg.StepsDetails,
		},
		basePrice: basePrice{
			min: cfg.MinBasePrice,
			max: cfg.MaxBasePrice,
		},
		compound: compound{
			kind:    cfg.CompoundType,
			details: cfg.CompoundDetails,
		},
		stepQuoteVolume:    0,
		publishOrderNumber: 0,
		doneSig:            make(chan struct{}),
	}
}

func (a *App) Start() error {
	if err := a.validate(); err != nil {
		return err
	}

	if err := a.exchangePairInfo(); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-a.ctx.Done():
				a.doneSig <- struct{}{}
				return
			default:
				a.run()
				<-time.After(a.interval)
			}
		}
	}()

	a.logger.Infof("appID: %d starts with success", a.id)

	return nil
}

func (a *App) Stop() {
	a.logger.Infof("appID: %d stopping ...", a.id)
	a.cancelFunc()

	<-a.doneSig
	a.logger.Infof("appID: %d stops with success", a.id)
}

func (a *App) validate() error {
	if a.pair.base == "" {
		return errors.New("base can not be empty")
	}

	// TODO add more validations

	return nil
}

func (a *App) exchangePairInfo() error {
	pi, err := a.connector.PairInfo(a.pair.base, a.pair.quote)
	if err != nil {
		return fmt.Errorf("could not get pairInfo for pair: %s %s, err: %s", a.pair.base, a.pair.quote, err.Error())
	}

	a.pairInfo = pairInfo{
		basePricePrecision:  pi.BasePricePrecision,
		quotePricePrecision: pi.QuotePricePrecision,
		baseMinVolume:       pi.BaseMinVolume,
	}

	return nil
}

func (a *App) run() {
	a.logger.Infof("run appID: %d", a.id)
}
