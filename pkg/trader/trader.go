package trader

import (
	"bwd/pkg/compound"
	"bwd/pkg/connector"
	"bwd/pkg/step"
	"bwd/pkg/storage"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	statusBuyLimit              = "BUY_LIMIT"
	statusSellLimit             = "SELL_LIMIT"
	statusBuyLimitWantsPublish  = "BUY_LIMIT_WANTS_PUBLISH"
	statusSellLimitWantsPublish = "SELL_LIMIT_WANTS_PUBLISH"
	statusBuyLimitPublished     = "BUY_LIMIT_PUBLISHED"
	statusSellLimitPublished    = "SELL_LIMIT_PUBLISHED"
	statusBuyLimitExecuted      = "BUY_LIMIT_EXECUTED"
	statusSellLimitExecuted     = "SELL_LIMIT_EXECUTED"
	statusClosed                = "CLOSED"
)

type ConfigTrader struct {
	AppID      int
	Base       string
	Quote      string
	Storer     storage.Storer
	Connector  connector.Connector
	Stepper    step.Stepper
	Compounder compound.Compounder
}

type Trader struct {
	logger     *logrus.Logger
	appID      int
	base       string
	quote      string
	storer     storage.Storer
	connector  connector.Connector
	stepper    step.Stepper
	compounder compound.Compounder
}

func New(cfg *ConfigTrader, logger *logrus.Logger) *Trader {
	return &Trader{
		logger:     logger,
		appID:      cfg.AppID,
		base:       cfg.Base,
		quote:      cfg.Quote,
		storer:     cfg.Storer,
		connector:  cfg.Connector,
		stepper:    cfg.Stepper,
		compounder: cfg.Compounder,
	}
}

func (t *Trader) Run() {
	// do reconciliation
	if err := t.reconcileFromExchangeToStorage(); err != nil {
		t.logger.WithError(err).Error("reconcileFromExchangeToStorage with error")
		return
	}

	// ??? just change statuses
	if err := t.moveTradesFromExecutedOnNextStatus(); err != nil {
		t.logger.WithError(err).Error("moveTradesFromExecutedOnNextStatus with error")
		return
	}

	// create missing trades
	if err := t.addMissingTrades(); err != nil {
		t.logger.WithError(err).Error("addMissingTrades with error")
		return
	}

	// decide who need to publish / unPublish
	if err := t.markForPublishUnPublish(); err != nil {
		t.logger.WithError(err).Error("markForPublishUnPublish with error")
		return
	}

	// publish / unPublish orders for trades on exchange
	if err := t.reconcileFromStorageToExchange(); err != nil {
		t.logger.WithError(err).Error("reconcileFromStorageToExchange with error")
		return
	}
}

func (t *Trader) reconcileFromExchangeToStorage() error {
	// level 1 reconcile trades orders (source of true: exchange)
	err := t.reconcileTradesIndividually()
	if err != nil {
		return err
	}

	// level 2 reconcile existing orders on exchange (cancel mistakes orders)
	// TODO to implement second level of reconciliation

	return nil
}

func (t *Trader) reconcileTradesIndividually() error {
	trades, err := t.activeTrades()
	if err != nil {
		return err
	}

	for _, trd := range trades {
		var orderID string

		switch trd.status {
		case statusBuyLimitPublished:
			orderID = trd.buyOrderID
		case statusSellLimitPublished:
			orderID = trd.sellOrderID
		default:
			continue
		}

		connectorOrder := connector.Order{
			ID:    orderID,
			Base:  t.base,
			Quote: t.quote,
		}
		o, err := t.connector.OrderDetails(t.appID, connectorOrder)
		if err != nil {
			return err
		}

		ord := castConnectorOrderToTraderOrder(o)

		switch ord.status {
		case connector.OrderStatusNew:
			continue
		case connector.OrderStatusExecuted:
			if trd.status == statusBuyLimitPublished {
				trd.openType = ord.orderType
				trd.status = statusBuyLimitExecuted
			} else {
				trd.closeType = ord.orderType
				trd.status = statusSellLimitExecuted
			}
			err := t.storer.UpdateTrade(castTradeToStorageTrade(trd))
			if err != nil {
				return err
			}
			fmt.Println("orders executed, trade moved to next status")
		default:
			return fmt.Errorf("unknown connectorOrder status: %s", ord.status)
		}
	}

	return nil
}

// just move from executed on next status
func (t *Trader) moveTradesFromExecutedOnNextStatus() error {
	trades, err := t.activeTrades()
	if err != nil {
		return err
	}

	for _, trd := range trades {
		oldStatus := trd.status
		switch trd.status {
		case statusBuyLimitExecuted:
			trd.convertedSellLimitAt = time.Now().UTC()
			trd.status = statusSellLimit
		case statusSellLimitExecuted:
			trd.closedAt = time.Now().UTC()
			trd.status = statusClosed
		default:
			continue
		}

		err := t.storer.UpdateTrade(castTradeToStorageTrade(trd))
		if err != nil {
			return err
		}
		t.logger.Infof("tradeID: %d moved from: %s to: %s", trd.id, oldStatus, trd.status)
	}

	return nil
}

func (t *Trader) addMissingTrades() error {
	steps := t.stepper.Steps()
	trades, err := t.activeTrades()
	if err != nil {
		return err
	}

	for _, s := range steps {
		var hasTrade bool
		for _, trd := range trades {
			if s == trd.openBasePrice {
				hasTrade = true
			}
		}

		if !hasTrade {
			volume, err := t.compounder.Volume(s)
			if err != nil {
				return err
			}

			trd := storage.Trade{
				AppID:          t.appID,
				OpenBasePrice:  s,
				CloseBasePrice: t.stepper.ClosePrice(s),
				BaseVolume:     volume,
				Status:         statusBuyLimit,
				CreatedAt:      time.Now().UTC(),
			}
			if err := t.storer.AddTrade(trd); err != nil {
				return err
			}
		}
	}

	return nil
}

// this should decide which orders should publish / unPublish on exchange
func (t *Trader) markForPublishUnPublish() error {
	trades, err := t.activeTrades()
	if err != nil {
		return err
	}

	// simple version, will publish all orders
	for _, trd := range trades {
		switch trd.status {
		case statusBuyLimit:
			trd.status = statusBuyLimitWantsPublish
		case statusSellLimit:
			trd.status = statusSellLimitWantsPublish
		default:
			continue
		}

		if err := t.storer.UpdateTrade(castTradeToStorageTrade(trd)); err != nil {
			return err
		}
	}

	return nil
}

func (t *Trader) reconcileFromStorageToExchange() error {
	trades, err := t.activeTrades()
	if err != nil {
		return err
	}

	for _, trd := range trades {
		switch trd.status {
		case statusBuyLimitWantsPublish:
			if err := t.addBuyLimitOrder(trd); err != nil {
				return err
			}
		case statusSellLimitWantsPublish:
			if err := t.addSellLimitOrder(trd); err != nil {
				return err
			}
		default:
			continue
		}
	}

	return nil
}

func (t *Trader) addBuyLimitOrder(trade trade) error {
	order := connector.Order{
		Base:      t.base,
		Quote:     t.quote,
		OrderType: connector.OrderTypeLimit,
		Side:      connector.OrderSideBuy,
		Price:     trade.openBasePrice,
		Volume:    trade.baseVolume,
	}

	orderID, err := t.connector.AddOrder(t.appID, order)
	if err != nil {
		return err
	}

	trade.buyOrderID = orderID
	trade.status = statusBuyLimitPublished
	if err := t.storer.UpdateTrade(castTradeToStorageTrade(trade)); err != nil {
		return err
	}

	return nil
}

func (t *Trader) addSellLimitOrder(trade trade) error {
	ord := connector.Order{
		Base:      t.base,
		Quote:     t.quote,
		OrderType: connector.OrderTypeLimit,
		Side:      connector.OrderSideSell,
		Price:     trade.closeBasePrice,
		Volume:    trade.baseVolume,
	}

	orderID, err := t.connector.AddOrder(t.appID, ord)
	if err != nil {
		return err
	}

	trade.sellOrderID = orderID
	trade.status = statusSellLimitPublished
	if err := t.storer.UpdateTrade(castTradeToStorageTrade(trade)); err != nil {
		return err
	}

	return nil
}

type trade struct {
	id                   int
	appID                int
	openBasePrice        float64
	closeBasePrice       float64
	openType             string
	closeType            string
	baseVolume           float64
	buyOrderID           string
	sellOrderID          string
	status               string
	convertedSellLimitAt time.Time
	closedAt             time.Time
	updatedAt            time.Time
	createdAt            time.Time
}

func (t *Trader) activeTrades() ([]trade, error) {
	var trades []trade

	res, err := t.storer.ActiveTrades(t.appID)
	if err != nil {
		return []trade{}, err
	}

	for _, v := range res {
		trades = append(trades, trade{
			id:                   v.ID,
			appID:                v.AppID,
			openBasePrice:        v.OpenBasePrice,
			closeBasePrice:       v.CloseBasePrice,
			openType:             v.OpenType,
			closeType:            v.CloseType,
			baseVolume:           v.BaseVolume,
			buyOrderID:           v.BuyOrderID,
			sellOrderID:          v.SellOrderID,
			status:               v.Status,
			convertedSellLimitAt: v.ConvertedSellLimitAt,
			closedAt:             v.ClosedAt,
			updatedAt:            v.UpdatedAt,
			createdAt:            v.CreatedAt,
		})
	}

	return trades, nil
}

type order struct {
	id        string
	base      string
	quote     string
	orderType string
	side      string
	price     float64
	volume    float64
	status    string
}
