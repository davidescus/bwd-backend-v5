package connector

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type FakeConnectorConfig struct {
	Interval time.Duration
}

type FakeConnector struct {
	ctx        context.Context
	cancelFunc func()
	logger     *logrus.Logger
	interval   time.Duration
	doneSig    chan struct{}
	m          sync.Mutex
	orders     map[int][]Order
}

func NewFakeConnector(cfg *FakeConnectorConfig, logger *logrus.Logger) *FakeConnector {
	connectorCtx, cancel := context.WithCancel(context.Background())

	return &FakeConnector{
		ctx:        connectorCtx,
		cancelFunc: cancel,
		m:          sync.Mutex{},
		logger:     logger,
		interval:   cfg.Interval,
		doneSig:    make(chan struct{}),
		orders:     make(map[int][]Order),
	}
}

func (f *FakeConnector) Start() {
	go func() {
		for {
			select {
			case <-f.ctx.Done():
				f.doneSig <- struct{}{}
				return
			default:
				f.run()
				<-time.After(f.interval)
			}
		}
	}()

	f.logger.Infof("connector FakeConnector successful start")
}

func (f *FakeConnector) Stop() {
	f.cancelFunc()
	f.logger.Infof("connector FakeConnector stopping ...")

	<-f.doneSig
	f.logger.Infof("connector FakeConnector successful stop")
}

func (f *FakeConnector) PairInfo(base, quote string) (PairInfo, error) {
	return PairInfo{
		PairName:            fmt.Sprintf("%s%s", base, quote),
		BasePricePrecision:  8,
		QuotePricePrecision: 8,
		BaseMinVolume:       0.001,
	}, nil
}

func (f *FakeConnector) AddOrder(appID int, order Order) (string, error) {
	// TODO create order details from it
	return strconv.Itoa(rand.Intn(1000000)), nil
}

func (f *FakeConnector) CancelOrder(order Order) error {
	return nil
}

func (f *FakeConnector) OrdersDetails(appID int, ordersIds []string) []Order {
	f.m.Lock()
	defer f.m.Unlock()

	var data []Order

	for _, id := range ordersIds {
		var hasOrder bool

		for _, order := range f.orders[appID] {
			if order.ID == id {
				data = append(data, order)
				hasOrder = true
				break
			}
		}

		if !hasOrder {
			data = append(data, Order{
				ID:     id,
				Pair:   "",
				Status: "NOT_FOUND",
			})
		}
	}

	return data
}

func (f *FakeConnector) run() {
	f.logger.Info("--- FakeConnector connector run")
	// Here should happen the magic
}
