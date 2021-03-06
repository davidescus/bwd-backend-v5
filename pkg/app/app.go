package app

import (
	"bwd/pkg/compound"
	"bwd/pkg/connector"
	"bwd/pkg/step"
	"bwd/pkg/storage"
	"bwd/pkg/trader"
	"bwd/pkg/utils/metrics/exporter"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sirupsen/logrus"
)

const (
	stepperTypeFixInterval      = "FIX_INTERVAL"
	compounderTypeNone          = "NONE"
	compounderTypeProfitPercent = "PROFIT_PERCENT"
)

var (
	// TODO bwd should be an app type
	metricRun   = exporter.GetCounter("bwd_app", "run", []string{"appid"})
	metricError = exporter.GetCounter("bwd_app", "error", []string{"appid"})
)

type ConfigApp struct {
	Storer             storage.Storer
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
	logger             logrus.FieldLogger
	storer             storage.Storer
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
	trader             *trader.Trader
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
	basePrice           struct {
		min, max, tick float64
	}
	baseLot struct {
		min, max, tick float64
	}
	quoteMinVolume float64
}

// New will return a pointer to an configured app
func New(cfg *ConfigApp, logger logrus.FieldLogger) *App {
	appCtx, cancel := context.WithCancel(context.Background())

	return &App{
		ctx:        appCtx,
		cancelFunc: cancel,
		logger:     logger.WithField("module", "app").WithField("appid", cfg.ID),
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
		stepQuoteVolume:    cfg.StepQuoteVolume,
		publishOrderNumber: cfg.PublishOrderNumber,
		doneSig:            make(chan struct{}),
	}
}

// Start will should care about:
// get infos, precisions, qty for pair from exchange
// validate app if it is properly configured
// init trader dependencies (stepper, compounder)
// init trader and run it on loop (at tick interval)
func (a *App) Start() error {
	if err := a.exchangePairInfo(); err != nil {
		return fmt.Errorf("fail exchangePairInfo, err: %w", err)
	}

	if err := a.validate(); err != nil {
		return fmt.Errorf("fail validate, err: %w", err)
	}

	if err := a.initStepper(); err != nil {
		return fmt.Errorf("fail initStepper, err: %w", err)
	}

	if err := a.initCompounder(); err != nil {
		return fmt.Errorf("fail initCompounder, err: %w", err)
	}

	a.initTrader()

	go func() {
		for {
			a.logger.Debug("run app")

			select {
			case <-a.ctx.Done():
				a.doneSig <- struct{}{}
				return
			default:
				metricRun.With(prometheus.Labels{"appid": strconv.Itoa(a.id)}).Inc()
				a.trader.Run()
				<-time.After(a.interval)
			}
		}
	}()

	return nil
}

// Stop will wait for trader to finish
func (a *App) Stop() {
	a.cancelFunc()
	<-a.doneSig
}

func (a *App) validate() error {
	if a.id < 1 {
		return errors.New("appId should be unique and greater than 1")
	}

	if a.exchange == "" {
		return errors.New("exchange can not be empty")
	}

	if a.pair.base == "" {
		return errors.New("base can not be empty")
	}

	if a.pair.quote == "" {
		return errors.New("quote can not be empty")
	}

	if a.fees.market < 0 {
		return errors.New("market order fee can not be less than 0")
	}

	if a.fees.limit < 0 {
		return errors.New("limit order fee can not be less than 0")
	}

	if a.basePrice.min < a.pairInfo.basePrice.min {
		return errors.New("minBasePrice should be greater or equal than exchange minBasePrice")
	}

	if a.basePrice.max > a.pairInfo.basePrice.max {
		return errors.New("maxBasePrice should be lower or equal than exchange maxBasePrice")
	}

	if a.basePrice.max < a.basePrice.min {
		return errors.New("maxBasePrice should be greater or equal with minBasePrice")
	}

	if a.stepQuoteVolume < a.pairInfo.quoteMinVolume {
		return errors.New("stepQuoteVolume can not be less than pair quoteMinVolume")
	}

	if a.publishOrderNumber < 1 {
		return errors.New("publishOrdersNumber should be at least 1")
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
		basePrice: struct {
			min, max, tick float64
		}{
			pi.BasePrice.Min,
			pi.BasePrice.Max,
			pi.BasePrice.Tick,
		},
		baseLot: struct {
			min, max, tick float64
		}{
			pi.BaseLot.Min,
			pi.BaseLot.Max,
			pi.BaseLot.Tick,
		},
		quoteMinVolume: pi.QuoteMinVolume,
	}

	return nil
}

func (a *App) initStepper() error {
	switch a.steps.kind {
	case stepperTypeFixInterval:
		cfgStepsFixInterval := step.ConfigStepsFixInterval{
			MinPriceAllowed: a.pairInfo.basePrice.min,
			MaxPriceAllowed: a.pairInfo.basePrice.max,
			PriceTick:       a.pairInfo.basePrice.tick,
			AppSettings: fmt.Sprintf(`{"min":"%s","max":"%s","interval":"%s"}`,
				strconv.FormatFloat(a.basePrice.min, 'f', -1, 64),
				strconv.FormatFloat(a.basePrice.max, 'f', -1, 64),
				a.steps.settings,
			),
		}
		s, err := step.NewStepsFixInterval(&cfgStepsFixInterval)
		if err != nil {
			cfgJson, _ := json.Marshal(cfgStepsFixInterval)
			return fmt.Errorf("cound not init stepper: %s with config: %s, err: %s",
				stepperTypeFixInterval,
				cfgJson,
				err.Error(),
			)
		}

		a.stepper = s
	default:
		return fmt.Errorf("unknown stepper type: %s", a.steps.kind)
	}
	return nil
}

func (a *App) initCompounder() error {
	switch a.compound.kind {
	case compounderTypeNone:
		a.compounder = compound.NewCompoundNone(&compound.ConfigNone{
			InitialStepQuoteVolume: a.stepQuoteVolume,
			MinBaseLotAllowed:      a.pairInfo.baseLot.min,
			MaxBaseLotAllowed:      a.pairInfo.baseLot.max,
			BaseLotTick:            a.pairInfo.baseLot.tick,
		})
	case compounderTypeProfitPercent:
		a.compounder = compound.NewProfitPercent(&compound.ConfigProfitPercent{
			AppID:                  a.id,
			Storer:                 a.storer,
			InitialStepQuoteVolume: a.stepQuoteVolume,
			MinBaseLotAllowed:      a.pairInfo.baseLot.min,
			MaxBaseLotAllowed:      a.pairInfo.baseLot.max,
			BaseLotTick:            a.pairInfo.baseLot.tick,
		})
	default:
		return fmt.Errorf("unknown compounder type: %s", a.compound.kind)
	}
	return nil
}

func (a *App) initTrader() {
	cfgTrader := &trader.ConfigTrader{
		AppID:           a.id,
		Base:            a.pair.base,
		Quote:           a.pair.quote,
		MarketOrderFees: a.fees.market,
		LimitOrderFees:  a.fees.limit,
		Storer:          a.storer,
		Connector:       a.connector,
		Stepper:         a.stepper,
		Compounder:      a.compounder,
	}

	a.trader = trader.New(cfgTrader, a.logger)
}
