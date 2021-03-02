package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	symbols    map[string]symbol
	m          sync.Mutex
	orders     map[int][]Order
}

type symbol struct {
	base, quote string
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
		symbols:    make(map[string]symbol),
		orders:     make(map[int][]Order),
	}
}

func (b *Binance) Start() error {
	// get all symbols from exchange
	res, err := b.connection.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return err
	}

	for _, s := range res.Symbols {
		// do not know what means break
		if s.Status == "TRADING" {
			b.symbols[s.Symbol] = symbol{
				base:  s.BaseAsset,
				quote: s.QuoteAsset,
			}
		}
	}

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

	b.logger.Info("connector Binance successful start")

	return nil
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

			if err := loadPairInfo(r.Filters, &pairInfo); err != nil {
				return PairInfo{}, err
			}

			return pairInfo, nil
		}
	}

	return PairInfo{}, fmt.Errorf("cound not find info on exchange for pair: %s%s", base, quote)
}

// TODO - DONE
func (b *Binance) AddOrder(appID int, order Order) (string, error) {
	var price string
	var side binance.SideType
	var orderType binance.OrderType

	switch order.Side {
	case OrderSideBuy:
		side = binance.SideTypeBuy
	case OrderSideSell:
		side = binance.SideTypeSell
	default:
		return "", fmt.Errorf("unknown order side: %s", order.Side)
	}

	switch order.OrderType {
	case OrderTypeMarket:
		orderType = binance.OrderTypeMarket
	case OrderTypeLimit:
		orderType = binance.OrderTypeLimit
	default:
		return "", fmt.Errorf("unknown order type: %s", order.OrderType)
	}

	orderIdentifier := fmt.Sprintf("%d_%v", appID, time.Now().UTC().UnixNano())
	// id is composed by: {APP_ID}_{current_time_nano}
	// binance not accept duplicates ClientOrderId,
	symbol := fmt.Sprintf("%s%s", order.Base, order.Quote)
	volume := strconv.FormatFloat(order.Volume, 'f', -1, 64)
	price = strconv.FormatFloat(order.Price, 'f', -1, 64)

	resp, err := b.connection.NewCreateOrderService().
		Symbol(symbol).
		Side(side).
		Type(orderType).
		TimeInForce(binance.TimeInForceTypeGTC).
		Quantity(volume).
		Price(price).
		NewClientOrderID(orderIdentifier).
		Do(context.Background())

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%v", resp.OrderID), nil
}

// TODO implement me (not used at this moment)
func (b *Binance) CancelOrder(order Order) error {
	return nil
}

// first search on new orders, after this search on exchange for it
func (b *Binance) OrderDetails(appID int, order Order) (Order, error) {
	b.m.Lock()
	defer b.m.Unlock()

	for _, o := range b.orders[appID] {
		if o.ID == order.ID {
			return o, nil
		}
	}

	// search order on exchange, and add it to cache
	int64ID, err := strconv.ParseInt(order.ID, 10, 64)
	if err != nil {
		return Order{}, fmt.Errorf("failed to parse int64 order id: %s, err: %w", order.ID, err)
	}

	symbol := fmt.Sprintf("%s%s", order.Base, order.Quote)
	exhOrder, err := b.connection.NewGetOrderService().Symbol(symbol).OrderID(int64ID).Do(context.Background())
	if err != nil {
		return Order{}, fmt.Errorf("failed to fetch order id: %s, err : %w", order.ID, err)
	}

	ord, err := b.castExchangeOrder(exhOrder)
	if err != nil {
		return Order{}, err
	}

	b.orders[appID] = append(b.orders[appID], ord)

	return ord, nil
}

func (b *Binance) OrdersDetails(appID int) []Order {
	b.m.Lock()
	defer b.m.Unlock()

	return b.orders[appID]
}

func (b *Binance) run() {
	exhOpenOrders, err := b.openOrders()
	if err != nil {
		b.logger.WithError(err).Warn("could not fetch open exhOrders from exchange")
		return
	}

	b.m.Lock()
	defer b.m.Unlock()

	// add exhOrders to cache if not exists
	for appID, exhOrders := range exhOpenOrders {
		for _, exhOrder := range exhOrders {
			var orderExistsInCache bool
			for _, cacheOrder := range b.orders[appID] {
				if exhOrder.ID == cacheOrder.ID {
					orderExistsInCache = true
					break
				}
			}

			if !orderExistsInCache {
				b.orders[appID] = append(b.orders[appID], exhOrder)
			}
		}
	}

	// search for cache orders that not exists anymore in exchange open orders
	for appID, orders := range b.orders {
		for i, order := range orders {
			var isOpenOrder bool
			for _, exhOrder := range exhOpenOrders[appID] {
				if order.ID == exhOrder.ID {
					isOpenOrder = true
					break
				}
			}

			if !isOpenOrder {
				// get order details from exchange
				int64ID, err := strconv.ParseInt(order.ID, 10, 64)
				if err != nil {
					b.logger.WithError(err).Errorf("failed to parse int64 order id: %s,", order.ID)
					continue
				}

				symbol := fmt.Sprintf("%s%s", order.Base, order.Quote)
				exhOrder, err := b.connection.NewGetOrderService().Symbol(symbol).OrderID(int64ID).Do(context.Background())
				if err != nil {
					b.logger.WithError(err).Errorf("failed to fetch order id: %s", order.ID)
					continue
				}

				ord, err := b.castExchangeOrder(exhOrder)
				if err != nil {
					b.logger.WithError(err).Errorf("failed to cast exchange order id: %s", order.ID)
					continue
				}

				b.orders[appID][i] = ord
			}
		}
	}
}

func (b *Binance) openOrders() (map[int][]Order, error) {
	orders := make(map[int][]Order)

	openOrders, err := b.connection.NewListOpenOrdersService().Do(context.Background())
	if err != nil {
		return orders, err
	}

	for _, exhOrder := range openOrders {
		orderIdentifier := strings.Split(exhOrder.ClientOrderID, "_")[0]
		if len(orderIdentifier) < 1 || orderIdentifier == "web" {
			continue
		}

		appID, err := strconv.Atoi(orderIdentifier)
		if err != nil {
			jsonOrder, _ := json.Marshal(exhOrder)
			return orders, fmt.Errorf("faild to parse orderIdentifier on: %s, err: %w", jsonOrder, err)
		}

		order, err := b.castExchangeOrder(exhOrder)
		if err != nil {
			jsonOrder, _ := json.Marshal(exhOrder)
			return orders, fmt.Errorf("faild to cast exchange order to connector order on: %s, err: %w", jsonOrder, err)
		}

		orders[appID] = append(orders[appID], order)
	}

	return orders, nil
}

func (b *Binance) castExchangeOrder(order *binance.Order) (Order, error) {
	price, err := strconv.ParseFloat(order.Price, 64)
	if err != nil {
		return Order{}, err
	}

	var orderType string
	switch order.Type {
	case OrderTypeMarket:
		orderType = OrderTypeMarket
	case OrderTypeLimit:
		orderType = OrderTypeLimit
	default:
		return Order{}, errors.New(fmt.Sprintf("unknown order type: %pair", order.Type))
	}

	var side string
	switch order.Side {
	case binance.SideTypeSell:
		side = OrderSideSell
	case binance.SideTypeBuy:
		side = OrderSideBuy
	default:
		return Order{}, errors.New(fmt.Sprintf("unknown order side: %pair", order.Side))
	}

	var status string
	switch order.Status {
	case binance.OrderStatusTypeNew:
		status = OrderStatusNew
	case binance.OrderStatusTypeFilled:
		status = OrderStatusExecuted
	default:
		return Order{}, errors.New(fmt.Sprintf("unknown order side: %pair", order.Status))
	}

	volume, err := strconv.ParseFloat(order.OrigQuantity, 64)
	if err != nil {
		return Order{}, err
	}

	pair, ok := b.symbols[order.Symbol]
	if !ok {
		return Order{}, fmt.Errorf("pair: %s not founded on binance connector symbols", order.Symbol)
	}

	return Order{
		ID:        fmt.Sprintf("%v", order.OrderID),
		Base:      pair.base,
		Quote:     pair.quote,
		OrderType: orderType,
		Side:      side,
		Price:     price,
		Volume:    volume,
		Status:    status,
	}, nil
}

func loadPairInfo(filters []map[string]interface{}, pairInfo *PairInfo) error {
	// base price
	minBasePrice, err := filterValue(filters[0], "minPrice")
	if err != nil {
		return err
	}
	pairInfo.BasePrice.Min = minBasePrice

	maxBasePrice, err := filterValue(filters[0], "maxPrice")
	if err != nil {
		return err
	}
	pairInfo.BasePrice.Max = maxBasePrice

	basePriceTick, err := filterValue(filters[0], "tickSize")
	if err != nil {
		return err
	}
	pairInfo.BasePrice.Tick = basePriceTick

	// baseLot
	minBaseLotSize, err := filterValue(filters[2], "minQty")
	if err != nil {
		return err
	}
	pairInfo.BaseLot.Min = minBaseLotSize

	maxBaseLotSize, err := filterValue(filters[2], "maxQty")
	if err != nil {
		return err
	}
	pairInfo.BaseLot.Max = maxBaseLotSize

	baseLotTick, err := filterValue(filters[2], "stepSize")
	if err != nil {
		return err
	}
	pairInfo.BaseLot.Tick = baseLotTick

	quoteMinVolume, err := filterValue(filters[3], "minNotional")
	if err != nil {
		return err
	}
	pairInfo.QuoteMinVolume = quoteMinVolume

	return nil
}

func filterValue(filter map[string]interface{}, key string) (float64, error) {
	for k, val := range filter {
		if k == key {
			v, ok := val.(string)
			if !ok {
				return 0, fmt.Errorf("could not cast to string value for %s from filters", key)
			}

			return strconv.ParseFloat(v, 64)
		}
	}

	return 0, fmt.Errorf("key: %s not found on binane filter", key)
}
