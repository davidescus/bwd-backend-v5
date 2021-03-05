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
	logger     logrus.FieldLogger
	appID      int
	base       string
	quote      string
	storer     storage.Storer
	connector  connector.Connector
	stepper    step.Stepper
	compounder compound.Compounder
}

func New(cfg *ConfigTrader, logger logrus.FieldLogger) *Trader {
	return &Trader{
		logger:     logger.WithField("module", "trader"),
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
	t.logger.Debug("run trader")

	// reconciliation level 1
	// apply exchange modifications on storage by changing storage trades statuses
	// according to current exchange orders statuses
	if err := t.reconcileStorageTrades(); err != nil {
		t.logger.WithError(err).Error("reconcileFromExchangeToStorage with error")
		return
	}

	// TODO reconciliation level 2
	// detect and cancel exchange orders which are not associated with a published trade

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

func (t *Trader) reconcileStorageTrades() error {
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

		ord := castOrder(o)

		switch ord.status {
		case connector.OrderStatusNew:
		case connector.OrderStatusPartiallyFilled:
			continue
		case connector.OrderStatusExecuted:
			if trd.status == statusBuyLimitPublished {
				trd.openType = ord.orderType
				trd.status = statusBuyLimitExecuted
			} else {
				trd.closeType = ord.orderType
				trd.status = statusSellLimitExecuted
			}
			err := t.storer.UpdateTrade(castToStorageTrade(trd))
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

		if err := t.storer.UpdateTrade(castToStorageTrade(trd)); err != nil {
			return err
		}
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

		if err := t.storer.UpdateTrade(castToStorageTrade(trd)); err != nil {
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
			if err := t.publishBuyLimitOrder(trd); err != nil {
				return err
			}
		case statusSellLimitWantsPublish:
			if err := t.publishSellLimitOrder(trd); err != nil {
				return err
			}
		default:
			continue
		}
	}

	return nil
}

func (t *Trader) publishBuyLimitOrder(trd trade) error {
	ord := connector.Order{
		Base:      t.base,
		Quote:     t.quote,
		OrderType: connector.OrderTypeLimit,
		Side:      connector.OrderSideBuy,
		Price:     trd.openBasePrice,
		Volume:    trd.baseVolume,
	}

	orderID, err := t.connector.AddOrder(t.appID, ord)
	if err != nil {
		return err
	}

	trd.buyOrderID = orderID
	trd.status = statusBuyLimitPublished
	if err := t.storer.UpdateTrade(castToStorageTrade(trd)); err != nil {
		return err
	}

	return nil
}

func (t *Trader) publishSellLimitOrder(trd trade) error {
	ord := connector.Order{
		Base:      t.base,
		Quote:     t.quote,
		OrderType: connector.OrderTypeLimit,
		Side:      connector.OrderSideSell,
		Price:     trd.closeBasePrice,
		Volume:    trd.baseVolume,
	}

	orderID, err := t.connector.AddOrder(t.appID, ord)
	if err != nil {
		return err
	}

	trd.sellOrderID = orderID
	trd.status = statusSellLimitPublished
	if err := t.storer.UpdateTrade(castToStorageTrade(trd)); err != nil {
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
		trades = append(trades, castStorageTrade(v))
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
