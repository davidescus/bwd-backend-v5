package connector

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/adshao/go-binance/v2"
)

type BinanceConfig struct {
	Interval  time.Duration
	ApiKey    string
	SecretKey string
}

type Binance struct {
	ctx        context.Context
	cancelFunc func()
	logger     *logrus.Logger
	connection *binance.Client
	interval   time.Duration
	doneSig    chan struct{}
	m          sync.Mutex
	orders     map[int][]Order
}

func NewBinance(cfg *BinanceConfig, logger *logrus.Logger) *Binance {
	connectorCtx, cancel := context.WithCancel(context.Background())

	return &Binance{
		ctx:        connectorCtx,
		cancelFunc: cancel,
		logger:     logger,
		connection: binance.NewClient(cfg.ApiKey, cfg.SecretKey),
		interval:   cfg.Interval,
		doneSig:    make(chan struct{}),
		orders:     make(map[int][]Order),
	}
}

func (b *Binance) Start() {
	go func() {
		for {
			select {
			case <-b.ctx.Done():
				b.doneSig <- struct{}{}
				return
			default:
				b.run()
				<-time.After(b.interval)
			}
		}
	}()

	b.logger.Infof("connector Binance successful start")
}

func (b *Binance) Stop() {
	b.cancelFunc()
	b.logger.Infof("connector Binance stopping ...")

	<-b.doneSig
	b.logger.Infof("connector Binance successful stop")
}

func (b *Binance) PairInfo(base, quote string) (PairInfo, error) {
	resp, err := b.connection.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return PairInfo{}, err
	}

	for _, r := range resp.Symbols {
		if r.Symbol == fmt.Sprintf("%s%s", base, quote) {
			pairInfo := PairInfo{
				BasePricePrecision:  r.BaseAssetPrecision,
				QuotePricePrecision: r.QuotePrecision,
			}
			for k, val := range r.Filters[2] {
				if k != "minQty" {
					continue
				}
				v, ok := val.(string)
				if !ok {
					return PairInfo{}, fmt.Errorf("could not get minQty for pair: %s", r.Symbol)
				}

				floatVal, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return PairInfo{}, fmt.Errorf("could not convert to float minQty for pair: %s, err: %w", r.Symbol, err)
				}
				pairInfo.BaseMinVolume = floatVal
			}

			return pairInfo, nil
		}
	}

	return PairInfo{}, fmt.Errorf("cound not find info on exchange for pair: %s%s", base, quote)
}

// TODO implement me
func (b *Binance) AddOrder(appID int, order Order) (string, error) {
	return "", nil
}

// TODO implement me
func (b *Binance) CancelOrder(order Order) error {
	return nil
}

func (b *Binance) OrdersDetails(appID int, ordersIds []string) []Order {
	b.m.Lock()
	defer b.m.Unlock()

	var data []Order

	for _, id := range ordersIds {
		var hasOrder bool

		for _, order := range b.orders[appID] {
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

// TODO implement me
func (b *Binance) run() {
	b.logger.Info("--- binance connector run")
	// Here should happen the magic
}
