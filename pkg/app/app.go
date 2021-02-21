package app

// package app is responsible for:
// - init compounder
// - init stepper
// - loop at specific interval and run trader

import (
	"bwd/pkg/compound"
	"bwd/pkg/connector"
	"bwd/pkg/step"
	"bwd/pkg/storage"
	"bwd/pkg/trader"
	"context"
	"errors"
	"fmt"
	"strconv"
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
	steps              stepsSettings
	basePrice          priceSettings
	compound           compoundSettings
	pairInfo           pairInfo
	publishOrderNumber int
	doneSig            chan struct{}
	stepQuoteVolume    float64
	cancelFunc         func()
	stepper            step.Stepper
	compounder         compound.Compounder
}

type pair struct {
	base, quote string
}
type fees struct {
	market, limit float64
}
type stepsSettings struct {
	kind, settings string
}
type compoundSettings struct {
	kind, details string
}
type priceSettings struct {
	min, max float64
}
type pairInfo struct {
	basePricePrecision  int
	quotePricePrecision int
	baseMinVolume       float64
}

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
		steps: stepsSettings{
			kind:     cfg.StepsType,
			settings: cfg.StepsDetails,
		},
		basePrice: priceSettings{
			min: cfg.MinBasePrice,
			max: cfg.MaxBasePrice,
		},
		compound: compoundSettings{
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

	if err := a.initStepper(); err != nil {
		return err
	}

	if err := a.initCompounder(); err != nil {
		return err
	}

	// TODO init trader

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

// TODO add more validations
func (a *App) validate() error {
	if a.pair.base == "" {
		return errors.New("base can not be empty")
	}

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

// TODO use constants instead of strings
func (a *App) initStepper() error {
	switch a.steps.kind {
	case "FIX_INTERVAL":
		settings := fmt.Sprintf(`{"min":"%s","max":"%s","interval":"%s","precision":%d}`,
			strconv.FormatFloat(a.basePrice.min, 'f', -1, 64),
			strconv.FormatFloat(a.basePrice.max, 'f', -1, 64),
			a.steps.settings,
			a.pairInfo.basePricePrecision,
		)
		s, err := step.NewStepsFixInterval(settings)
		if err != nil {
			return err
		}
		a.stepper = s
	default:
		return fmt.Errorf("unknown stepper type: %s", a.steps.kind)
	}
	return nil
}

func (a *App) initCompounder() error {
	switch a.compound.kind {
	case "NONE":
		a.compounder = compound.NewCompoundNone()
	default:
		return fmt.Errorf("unknown stepper type: %s", a.steps.kind)
	}
	return nil
}

func (a *App) run() {
	a.logger.Infof("run appID: %d", a.id)

	cfgTrader := &trader.ConfigTrader{
		Storer:     a.storer,
		Connector:  a.connector,
		Stepper:    a.stepper,
		Compounder: a.compounder,
	}
	t := trader.New(cfgTrader, a.logger)

	t.Run()
}
