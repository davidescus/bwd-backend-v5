package connector

import (
	"context"
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

func (f *FakeConnector) Start() error {
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

	return nil
}

func (f *FakeConnector) Stop() {
	f.cancelFunc()
	f.logger.Infof("connector FakeConnector stopping ...")

	<-f.doneSig
	f.logger.Infof("connector FakeConnector successful stop")
}

func (f *FakeConnector) PairInfo(base, quote string) (PairInfo, error) {
	return PairInfo{
		BasePricePrecision:  8,
		QuotePricePrecision: 8,
	}, nil
}

func (f *FakeConnector) AddOrder(appID int, order Order) (string, error) {
	order.ID = strconv.Itoa(rand.Intn(10000000))

	f.m.Lock()
	defer f.m.Unlock()

	order.Status = OrderStatusNew
	f.orders[appID] = append(f.orders[appID], order)

	return order.ID, nil
}

// TODO implement me
func (f *FakeConnector) CancelOrder(order Order) error {
	return nil
}

// first search on new orders, after this search on exchange for it
func (f *FakeConnector) OrderDetails(appID int, order Order) (Order, error) {
	f.m.Lock()
	defer f.m.Unlock()

	for _, order := range f.orders[appID] {
		if order.ID == order.ID {
			return order, nil
		}
	}

	return Order{
		ID:     order.ID,
		Status: OrderStatusNotFound,
	}, nil

}

func (f *FakeConnector) OrdersDetails(appID int) []Order {
	f.m.Lock()
	defer f.m.Unlock()

	return f.orders[appID]
}

func (f *FakeConnector) run() {
	f.m.Lock()
	defer f.m.Unlock()

	// random orders executed
	for appID, _ := range f.orders {
		for idx, order := range f.orders[appID] {
			if rand.Intn(100000)%2 == 0 {
				order.Status = OrderStatusExecuted
				f.orders[appID][idx] = order
			}
		}
	}
}
