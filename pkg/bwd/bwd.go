package bwd

import (
	"bwd/pkg/app"
	"bwd/pkg/storage"
	"context"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
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
	storer                  storage.Storer
	interval                time.Duration
	slackHook               string
	webBindingPort          string
	storageConnectionString string
	// map[AppID]app.App
	apps map[int]*app.App
	done chan struct{}
}

func New(ctx context.Context, cfg *ConfigBwd, logger *logrus.Logger) *Bwd {
	return &Bwd{
		ctx:                     ctx,
		logger:                  logger,
		interval:                cfg.Interval,
		slackHook:               cfg.SlackHook,
		webBindingPort:          cfg.WebBindingPort,
		storageConnectionString: cfg.StorageConnectionString,
		apps:                    make(map[int]*app.App),
		done:                    make(chan struct{}),
	}
}

func (b *Bwd) Start() error {
	// create storage instance
	sql, err := storage.NewMysql(b.storageConnectionString)
	if err != nil {
		return err
	}

	b.storer = sql

	// start goroutine that will create apps instances and update them
	// with new parameters periodically
	go func() {
		for {
			select {
			case <-b.ctx.Done():
				b.done <- struct{}{}
				return
			default:
				b.run()
				<-time.After(b.interval)
			}
		}
	}()

	return nil
}

func (b *Bwd) Done() {
	<-b.done
}

// run will read all apps from storage,
// init them or update params
func (b *Bwd) run() {
	apps, err := b.storer.Apps()
	if err != nil {
		b.logger.WithError(err).Error("storage Apps() error")
		return
	}

	for _, application := range apps {
		if application.Status == app.AppStatusInactive {
			// continue if app is already stopped
			if _, ok := b.apps[application.ID]; !ok {
				continue
			}

			// stop app and remove it from running apps
			b.apps[application.ID].Stop()
			delete(b.apps, application.ID)
		}

		// init app if not exists
		if _, ok := b.apps[application.ID]; !ok {
			configApp := &app.ConfigApp{
				Storer:          b.storer,
				ID:              application.ID,
				Exchange:        application.Exchange,
				MarketOrderFees: application.MarketOrderFees,
				LimitOrderFees:  application.LimitOrderFees,
				Base:            application.Base,
				Quote:           application.Quote,
				StepsType:       application.StepsType,
				StepsDetails:    application.StepsDetails,
			}

			b.apps[application.ID] = app.New(b.ctx, configApp, b.logger)
		}

		// update params if need and restart app
		var shouldRestart bool
		cp := b.apps[application.ID].ConfigParams()

		interval, err := time.ParseDuration(application.Interval)
		if err != nil {
			appJson, _ := json.Marshal(application)
			b.logger.WithError(err).Errorf("could not parse duration, app: %s", appJson)
		}
		if cp.Interval != interval {
			cp.Interval = interval
			shouldRestart = true
		}
		if cp.QuotePercentUse != application.QuotePercentUse {
			cp.QuotePercentUse = application.QuotePercentUse
			shouldRestart = true
		}
		if cp.MinBasePrice != application.MinBasePrice {
			cp.MinBasePrice = application.MinBasePrice
			shouldRestart = true
		}
		if cp.MaxBasePrice != application.MaxBasePrice {
			cp.MaxBasePrice = application.MaxBasePrice
			shouldRestart = true
		}
		if cp.StepQuoteVolume != application.StepQuoteVolume {
			cp.StepQuoteVolume = application.StepQuoteVolume
			shouldRestart = true
		}
		if cp.CompoundType != application.CompoundType {
			cp.CompoundType = application.CompoundType
			shouldRestart = true
		}
		if cp.CompoundDetails != application.CompoundDetails {
			cp.CompoundDetails = application.CompoundDetails
			shouldRestart = true
		}
		if cp.PublishOrderNumber != application.PublishOrderNumber {
			cp.PublishOrderNumber = application.PublishOrderNumber
			shouldRestart = true
		}
		if cp.Status != application.Status {
			cp.Status = application.Status
			shouldRestart = true
		}

		if shouldRestart {
			b.logger.Info("restart appID: %v for changing config params", application.ID)
			b.apps[application.ID].Stop()
			b.apps[application.ID].SetConfigParams(cp)
			if err := b.apps[application.ID].Start(); err != nil {
				appJson, _ := json.Marshal(application)
				b.logger.WithError(err).Errorf("app could not be started, app: %s", appJson)
				continue
			}
		}
	}
}
