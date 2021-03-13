package bwd

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
	logger                  logrus.FieldLogger
	storer                  storage.Storer
	interval                time.Duration
	slackHook               string
	webBindingPort          string
	storageConnectionString string
	connectors              map[string]connector.Connector
	runningApps             map[int]*app.App
	isDone                  chan struct{}
}

func New(ctx context.Context, cfg *ConfigBwd, logger logrus.FieldLogger) *Bwd {
	return &Bwd{
		ctx:                     ctx,
		logger:                  logger.WithField("module", "bwd"),
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
		return fmt.Errorf("bwd create mysql instance fail, err: %w", err)
	}

	b.storer = sql

	// start goroutine that will create runningApps instances and update them
	// with new parameters periodically
	go func() {
		for {
			select {
			case <-b.ctx.Done():
				// stop apps
				for _, a := range b.runningApps {
					a.Stop()
				}

				// stop connectors
				for _, c := range b.connectors {
					c.Stop()
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
}

func (b *Bwd) run() {
	b.logger.Debug("run bwd")
	apps, err := b.storer.Apps()
	if err != nil {
		b.logger.WithError(err).Errorf("fail fetch apps from storage")
		return
	}

	for _, a := range apps {
		// create connector if not exists
		_, ok := b.connectors[a.Exchange]
		if !ok {
			b.logger.WithField("dataconnector", a.Exchange).Info("try create connector")
			c, err := b.createConnector(a.Exchange)
			if err != nil {
				b.logger.WithError(err).Errorf("fail to init connector: %s", a.Exchange)
				continue
			}
			b.logger.WithField("dataconnector", a.Exchange).Info("success connector created")

			if err = c.Start(); err != nil {
				b.logger.WithError(err).Error("fail to start connector: %s", a.Exchange)
				continue
			}
			b.logger.WithField("dataconnector", a.Exchange).Info("success connector start")
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
			// TODO set interval as config
			Interval:  3 * time.Second,
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
	appJson, _ := json.Marshal(appCfg)
	logger := b.logger.WithField("dataapp", string(appJson))

	switch appCfg.Status {
	case "ACTIVE":
		// start app only if not exists in running apps
		if _, ok := b.runningApps[appCfg.ID]; !ok {
			logger.Info("try start app")
			a := b.createApp(appCfg)
			if err := a.Start(); err != nil {
				logger.WithError(err).Error("fail start app")
				return
			}
			b.runningApps[appCfg.ID] = a
			logger.Info("success start app")
		}
	case "INACTIVE":
		if a, ok := b.runningApps[appCfg.ID]; ok {
			logger.Info("try stop app")
			a.Stop()
			delete(b.runningApps, appCfg.ID)
			logger.Info("success stop app")
		}
	default:
		logger.Errorf("unknown app status")
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
