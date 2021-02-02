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
	apps                    map[int]bwdApp
	done                    chan struct{}
}

type bwdApp struct {
	cancelFunc func()
	app        *app.App
}

func New(ctx context.Context, cfg *ConfigBwd, logger *logrus.Logger) *Bwd {
	return &Bwd{
		ctx:                     ctx,
		logger:                  logger,
		interval:                cfg.Interval,
		slackHook:               cfg.SlackHook,
		webBindingPort:          cfg.WebBindingPort,
		storageConnectionString: cfg.StorageConnectionString,
		apps:                    make(map[int]bwdApp),
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
				// TODO make async stop process
				for _, a := range b.apps {
					a.app.Done()
				}
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
	b.logger.Info("bwd successful stop")
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
			b.apps[application.ID].cancelFunc()
			b.apps[application.ID].app.Done()
			b.logger.Infof("appID: %d stopped, user action", application.ID)
			delete(b.apps, application.ID)
		}

		// TODO ask @Adrian how to do
		// TODO refactor here, it works, but does not smell good

		// init app if not exists
		if _, ok := b.apps[application.ID]; !ok {
			appNewInstance, err := b.newApp(application)
			if err != nil {
				appJson, _ := json.Marshal(application)
				b.logger.WithError(err).Errorf("app could not be started, app: %s", appJson)

				continue
			}

			b.apps[application.ID] = appNewInstance

			currentParams := b.apps[application.ID].app.ConfigParams()
			newParams, _ := b.updateParams(currentParams, application)
			// validate ConfigApp
			if err := newParams.Validate(); err != nil {
				appJson, _ := json.Marshal(application)
				configAppJson, _ := json.Marshal(newParams)
				b.logger.WithError(err).Errorf("invalid config app: %s, config: %s", appJson, configAppJson)

				continue
			}

			b.apps[application.ID].app.SetConfigParams(newParams)
			b.apps[application.ID].app.Start()

			continue
		}

		// update newParams if need and restart app
		currentParams := b.apps[application.ID].app.ConfigParams()
		newParams, shouldRestart := b.updateParams(currentParams, application)

		if shouldRestart {
			// validate ConfigApp
			if err := newParams.Validate(); err != nil {
				appJson, _ := json.Marshal(application)
				configAppJson, _ := json.Marshal(currentParams)
				b.logger.WithError(err).Errorf("invalid config app: %s, config: %s", appJson, configAppJson)
				continue
			}

			b.logger.Infof("restart appID: %d for changing config newParams", application.ID)
			b.apps[application.ID].cancelFunc()
			b.apps[application.ID].app.Done()

			appNewInstance, err := b.newApp(application)
			if err != nil {
				appJson, _ := json.Marshal(application)
				b.logger.WithError(err).Errorf("app could not be started, app: %s, err: %s", appJson, err.Error())

				continue
			}

			b.apps[application.ID] = appNewInstance
			b.apps[application.ID].app.SetConfigParams(newParams)
			b.apps[application.ID].app.Start()
		}
	}
}

func (b *Bwd) newApp(a storage.App) (bwdApp, error) {
	configApp := &app.ConfigApp{
		Storer:          b.storer,
		ID:              a.ID,
		Exchange:        a.Exchange,
		MarketOrderFees: a.MarketOrderFees,
		LimitOrderFees:  a.LimitOrderFees,
		Base:            a.Base,
		Quote:           a.Quote,
		StepsType:       a.StepsType,
		StepsDetails:    a.StepsDetails,
	}

	ctx, cancel := context.WithCancel(b.ctx)
	application, err := app.New(ctx, configApp, b.logger)
	if err != nil {
		cancel()
		return bwdApp{}, err
	}

	return bwdApp{
		cancelFunc: cancel,
		app:        application,
	}, nil
}

func (b *Bwd) updateParams(p app.ConfigParams, a storage.App) (app.ConfigParams, bool) {
	var shouldRestart bool

	if p.Interval != a.Interval {
		p.Interval = a.Interval
		shouldRestart = true
	}
	if p.QuotePercentUse != a.QuotePercentUse {
		p.QuotePercentUse = a.QuotePercentUse
		shouldRestart = true
	}
	if p.MinBasePrice != a.MinBasePrice {
		p.MinBasePrice = a.MinBasePrice
		shouldRestart = true
	}
	if p.MaxBasePrice != a.MaxBasePrice {
		p.MaxBasePrice = a.MaxBasePrice
		shouldRestart = true
	}
	if p.StepQuoteVolume != a.StepQuoteVolume {
		p.StepQuoteVolume = a.StepQuoteVolume
		shouldRestart = true
	}
	if p.CompoundType != a.CompoundType {
		p.CompoundType = a.CompoundType
		shouldRestart = true
	}
	if p.CompoundDetails != a.CompoundDetails {
		p.CompoundDetails = a.CompoundDetails
		shouldRestart = true
	}
	if p.PublishOrderNumber != a.PublishOrderNumber {
		p.PublishOrderNumber = a.PublishOrderNumber
		shouldRestart = true
	}
	if p.Status != a.Status {
		p.Status = a.Status
		shouldRestart = true
	}

	return p, shouldRestart
}
