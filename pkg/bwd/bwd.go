package bwd

import (
	"bwd/pkg/app"
	"bwd/pkg/storage"
	"context"
	"fmt"
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
	apps map[int]app.App
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
		apps:                    make(map[int]app.App),
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

func (b *Bwd) run() {
	// fetch apps from storage,

	// init/update them

	fmt.Println(" --- run")
}
