package bwd

// bwd is responsible for:
// - init storage
// - configure and start/stop connectors
// - configure and start/stop apps

import (
	"bwd/pkg/app"
	"bwd/pkg/connector"
	"bwd/pkg/storage"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	binanceConnector = "BINANCE"
	fakeConnector    = "FAKE"
)

type ConfigBwd struct {
	Interval                time.Duration
	SlackHook               string
	WebBindingPort          string
	StorageConnectionString string
}

type Bwd struct {
	ctx                     context.Context
	logger                  *logrus.Logger
	storer                  storage.BwdStorer
	interval                time.Duration
	slackHook               string
	webBindingPort          string
	storageConnectionString string
	connectors              map[string]connector.Connector
	runningApps             map[int]*app.App
	isDone                  chan struct{}
}

func New(ctx context.Context, cfg *ConfigBwd, logger *logrus.Logger) *Bwd {
	return &Bwd{
		ctx:                     ctx,
		logger:                  logger,
		interval:                cfg.Interval,
		slackHook:               cfg.SlackHook,
		webBindingPort:          cfg.WebBindingPort,
		storageConnectionString: cfg.StorageConnectionString,
		connectors:              make(map[string]connector.Connector),
		runningApps:             make(map[int]*app.App),
		isDone:                  make(chan struct{}),
	}
}

func (b *Bwd) Start() error {
	// create storage instance
	sql, err := storage.NewMysql(b.storageConnectionString)
	if err != nil {
		return err
	}

	b.storer = sql

	// start goroutine that will create runningApps instances and update them
	// with new parameters periodically
	go func() {
		for {
			select {
			case <-b.ctx.Done():
				// TODO make async stop process
				for _, a := range b.runningApps {
					a.Stop()
				}
				close(b.isDone)
				return
			default:
				b.run()
				<-time.After(b.interval)
			}
		}
	}()

	return nil
}

func (b *Bwd) Wait() {
	<-b.isDone
	b.logger.Info("bwd successful stop")
}

func (b *Bwd) run() {
	apps, err := b.storer.Apps()
	if err != nil {
		b.logger.WithError(err).Error("storage Apps() error")
		return
	}

	for _, a := range apps {
		// create connector if not exists
		_, ok := b.connectors[a.Exchange]
		if !ok {
			c, err := b.createConnector(a.Exchange)
			if err != nil {
				b.logger.WithError(err).Error("could not init connector")
				continue
			}
			b.connectors[a.Exchange] = c
		}

		b.applyAppConfig(a)
	}
}

func (b *Bwd) createConnector(exchange string) (connector.Connector, error) {
	switch exchange {
	case binanceConnector:
		apyKey := os.Getenv("BINANCE_API_KEY")
		secretKey := os.Getenv("BINANCE_SECRET_KEY")
		binanceCfg := &connector.BinanceConfig{
			Interval:  4 * time.Second,
			ApiKey:    apyKey,
			SecretKey: secretKey,
		}
		return connector.NewBinance(binanceCfg, b.logger), nil
	case fakeConnector:
		fakeConnectorCfg := &connector.FakeConnectorConfig{
			Interval: 4 * time.Second,
		}
		return connector.NewFakeConnector(fakeConnectorCfg, b.logger), nil
	default:
		return nil, fmt.Errorf("unknown exchange: %s", exchange)
	}
}

func (b *Bwd) applyAppConfig(appCfg storage.App) {
	switch appCfg.Status {
	case "ACTIVE":
		// start app only if not exists in running apps
		if _, ok := b.runningApps[appCfg.ID]; !ok {
			a := b.createApp(appCfg)
			err := a.Start()
			if err != nil {
				appJson, _ := json.Marshal(appCfg)
				b.logger.WithError(err).Error("could not start application, appDetails: %s", string(appJson))
				return
			}
			b.runningApps[appCfg.ID] = a
		}
	case "INACTIVE":
		if a, ok := b.runningApps[appCfg.ID]; ok {
			a.Stop()
			delete(b.runningApps, appCfg.ID)
		}
	default:
		appJson, _ := json.Marshal(appCfg)
		b.logger.Errorf("unknown app status, app: %s", appJson)
	}
}

func (b *Bwd) createApp(a storage.App) *app.App {
	appCfg := &app.ConfigApp{
		Storer:             b.storer,
		Connector:          b.connectors[a.Exchange],
		Interval:           a.Interval,
		ID:                 a.ID,
		Exchange:           a.Exchange,
		MarketOrderFees:    a.MarketOrderFees,
		LimitOrderFees:     a.LimitOrderFees,
		Base:               a.Base,
		Quote:              a.Quote,
		StepsType:          a.StepsType,
		StepsDetails:       a.StepsDetails,
		MinBasePrice:       a.MinBasePrice,
		MaxBasePrice:       a.MaxBasePrice,
		StepQuoteVolume:    a.StepQuoteVolume,
		CompoundType:       a.CompoundType,
		CompoundDetails:    a.CompoundDetails,
		PublishOrderNumber: a.PublishOrderNumber,
	}

	return app.New(appCfg, b.logger)
}
