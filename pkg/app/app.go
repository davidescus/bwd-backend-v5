package app

import (
	"bwd/pkg/connector"
	"bwd/pkg/storage"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	AppStatusActive   = "ACTIVE"
	AppStatusInactive = "INACTIVE"

	ConnectorBinance = "BINANCE"
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

func (c *ConfigParams) Validate() error {
	if c.MinBasePrice > c.MaxBasePrice {
		return errors.New("minBasePrice can not be greater than maxBasePrice")
	}

	// TODO add more validations for config params

	return nil
}

type App struct {
	ctx             context.Context
	logger          *logrus.Logger
	storer          storage.AppStorer
	connector       connector.Connector
	pairInfo        PairInfo
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
	done               chan struct{}
}

type PairInfo struct {
	BasePricePrecision  int
	QuotePricePrecision int
	BaseMinVolume       float64
}

func New(ctx context.Context, cfg *ConfigApp, logger *logrus.Logger) (*App, error) {
	a := &App{
		ctx:      ctx,
		logger:   logger,
		storer:   cfg.Storer,
		id:       cfg.ID,
		exchange: cfg.Exchange,
		base:     cfg.Base,
		quote:    cfg.Quote,
		done:     make(chan struct{}),
	}

	if err := a.validate(); err != nil {
		return &App{}, err
	}

	// int connector
	if err := a.initConnector(); err != nil {
		return &App{}, err
	}

	// get pair info details from exchange
	if err := a.exchangePairInfo(); err != nil {
		return &App{}, err
	}

	return a, nil
}

func (a *App) ConfigParams() ConfigParams {
	return ConfigParams{
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

func (a *App) SetConfigParams(cfg ConfigParams) {
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

func (a *App) Start() {
	go func() {
		for {
			select {
			case <-a.ctx.Done():
				a.done <- struct{}{}
				return
			case <-a.done:
				return
			default:
				a.run()
				<-time.After(a.interval)
			}
		}
	}()

	a.logger.Infof("appID: %d successfull start", a.id)
}

func (a *App) Done() {
	<-a.done
	a.logger.Infof("appID: %d successful stop", a.id)
}

func (a *App) initConnector() error {
	switch a.exchange {
	case ConnectorBinance:
		apyKey := os.Getenv("BINANCE_API_KEY")
		secretKey := os.Getenv("BINANCE_SECRET_KEY")
		a.connector = connector.NewBinance(&connector.BinanceConfig{
			ApiKey:    apyKey,
			SecretKey: secretKey,
		})

		// TODO test if everything is ok here, make a request to exchange
	default:
		return fmt.Errorf("unknown exchange: %s", a.exchange)
	}

	return nil
}

func (a *App) exchangePairInfo() error {
	pi, err := a.connector.PairInfo(a.base, a.quote)
	if err != nil {
		return fmt.Errorf("could not get pairInfo for pair: %s %s, err: %s", a.base, a.quote, err.Error())
	}

	a.pairInfo = PairInfo{
		BasePricePrecision:  pi.BasePricePrecision,
		QuotePricePrecision: pi.QuotePricePrecision,
		BaseMinVolume:       pi.BaseMinVolume,
	}

	return nil
}

func (a *App) validate() error {
	if a.base == "" {
		return errors.New("base can not be empty")
	}

	// TODO add more validations for basic params

	return nil
}

func (a *App) run() {
	fmt.Printf("--- run appID: %d\n", a.id)
	time.Sleep(1 * time.Second)
}
