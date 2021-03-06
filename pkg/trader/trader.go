package trader

import (
	"bwd/pkg/compound"
	"bwd/pkg/connector"
	"bwd/pkg/step"
	"bwd/pkg/storage"
	"bwd/pkg/utils/metrics/exporter"
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

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

var (
	metricRecStorageLatency    = exporter.GetHistogram("bwd", "trader_reconcile_storage_ms_latency", []string{"appid"})
	metricMoveExecLatency      = exporter.GetHistogram("bwd", "trader_move_exec_to_next_status_ms_latency", []string{"appid"})
	metricAddMissingLatency    = exporter.GetHistogram("bwd", "trader_add_missing_trades_ms_latency", []string{"appid"})
	metricMarkPublishLatency   = exporter.GetHistogram("bwd", "trader_mark_publish_unpublish_ms_latency", []string{"appid"})
	metricPublishTradesLatency = exporter.GetHistogram("bwd", "trader_publish_trades_ms_latency", []string{"appid"})
	metricRunTotalLatency      = exporter.GetHistogram("bwd", "trader_run_total_ms_latency", []string{"appid"})
)

type ConfigTrader struct {
	AppID           int
	Base            string
	Quote           string
	MarketOrderFees float64
	LimitOrderFees  float64
	Storer          storage.Storer
	Connector       connector.Connector
	Stepper         step.Stepper
	Compounder      compound.Compounder
}

type Trader struct {
	logger          logrus.FieldLogger
	appID           int
	base            string
	quote           string
	marketOrderFees float64
	limitOrderFees  float64
	storer          storage.Storer
	connector       connector.Connector
	stepper         step.Stepper
	compounder      compound.Compounder
}

func New(cfg *ConfigTrader, logger logrus.FieldLogger) *Trader {
	return &Trader{
		logger:          logger.WithField("module", "trader"),
		appID:           cfg.AppID,
		base:            cfg.Base,
		quote:           cfg.Quote,
		marketOrderFees: cfg.MarketOrderFees,
		limitOrderFees:  cfg.LimitOrderFees,
		storer:          cfg.Storer,
		connector:       cfg.Connector,
		stepper:         cfg.Stepper,
		compounder:      cfg.Compounder,
	}
}

func (t *Trader) Run() {
	startTimeMs := time.Now().UnixNano() / int64(time.Millisecond)
	t.logger.Debug("run trader")

	// reconciliation level 1
	// apply exchange modifications on storage by changing storage trades statuses
	// according to current exchange orders statuses
	if ok := t.reconcileStorageTrades(); !ok {
		return
	}

	// TODO reconciliation level 2
	// detect and cancel exchange orders which are not associated with a published trade

	// change trade side/status: buy to sell / sell to close when order is executed
	// calculate, store data when intermediate step is done
	if ok := t.moveTradesFromExecutedOnNextStatus(); !ok {
		return
	}

	// create missing trades
	if ok := t.addMissingTrades(); !ok {
		return
	}

	// decide who need to publish / unPublish
	if ok := t.markForPublishUnPublish(); !ok {
		return
	}

	// publish / unPublish orders for trades on exchange
	if ok := t.reconcileFromStorageToExchange(); !ok {
		return
	}

	labels := prometheus.Labels{"appid": strconv.Itoa(t.appID)}
	endTimeMs := time.Now().UnixNano() / int64(time.Millisecond)
	metricRunTotalLatency.With(labels).Observe(float64(endTimeMs - startTimeMs))
}

// only trades with statuses buyLimitPublished/sellLimitPublished will be reconciled
func (t *Trader) reconcileStorageTrades() bool {
	startTimeMs := time.Now().UnixNano() / int64(time.Millisecond)

	trades, err := t.activeTrades()
	if err != nil {
		t.logger.WithError(err).Error("reconcileStorageTrades: fail fetch active trades")
		return false
	}

	var isOk = true

	// refactor this mess
	for _, trd := range trades {
		var orderID string

		logger := t.logger.
			WithField("datatrade", fmt.Sprintf("%+v", trd)).
			WithField("datatradeid", trd.id)

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
			logger.WithError(err).Error("reconcileStorageTrades: fail connector OrderDetails")
			isOk = false
			continue
		}

		ord := castOrder(o)
		logger = logger.WithField("dataorder", fmt.Sprintf("%+v", ord))

		switch ord.status {
		case connector.OrderStatusNew:
			continue
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

			logger = logger.WithField("dataupdatedtrade", fmt.Sprintf("%+v", trd))

			err := t.storer.UpdateTrade(castToStorageTrade(trd))
			if err != nil {
				logger.WithError(err).Error("reconcileStorageTrades: fail update trade")
				isOk = false
				continue
			}

			logger.Debug("success reconcile trade")

		default:
			isOk = false
			logger.Error("unknown order status")
		}
	}

	labels := prometheus.Labels{"appid": strconv.Itoa(t.appID)}
	endTimeMs := time.Now().UnixNano() / int64(time.Millisecond)
	metricRecStorageLatency.With(labels).Observe(float64(endTimeMs - startTimeMs))

	return isOk
}

// convert buy trades to sell or close sell trades when order is executed
func (t *Trader) moveTradesFromExecutedOnNextStatus() bool {
	startTimeMs := time.Now().UnixNano() / int64(time.Millisecond)

	trades, err := t.activeTrades()
	if err != nil {
		t.logger.WithError(err).Error("moveTradesFromExecutedOnNextStatus: fail fetch active trades")
		return false
	}

	var isOk = true
	for _, trd := range trades {
		switch trd.status {
		case statusBuyLimitExecuted:
			if ok := t.changeTradeBuySell(trd); !ok {
				isOk = false
			}
		case statusSellLimitExecuted:
			if ok := t.changeTradeSellClose(trd); !ok {
				isOk = false
			}
		default:
			continue
		}
	}

	labels := prometheus.Labels{"appid": strconv.Itoa(t.appID)}
	endTimeMs := time.Now().UnixNano() / int64(time.Millisecond)
	metricMoveExecLatency.With(labels).Observe(float64(endTimeMs - startTimeMs))

	return isOk
}

func (t *Trader) changeTradeBuySell(trd trade) bool {
	logger := t.logger.
		WithField("datatrade", fmt.Sprintf("%+v", trd)).
		WithField("datatradeid", trd.id)

	trd.convertedSellLimitAt = time.Now().UTC()
	trd.status = statusSellLimit

	logger.WithField("updatedtrade", fmt.Sprintf("%+v", trd))

	if err := t.storer.UpdateTrade(castToStorageTrade(trd)); err != nil {
		logger.WithError(err).Error("changeTradeBuySell: fail update trade")
		return false
	}

	return true
}

func (t *Trader) changeTradeSellClose(trd trade) bool {
	logger := t.logger.
		WithField("datatrade", fmt.Sprintf("%+v", trd)).
		WithField("datatradeid", trd.id)

	// idempotent add trade balance history
	// TODO use const for action, or make specific method
	action := "CASHED_IN"
	if err := t.addBalanceHistoryIfNotExists(trd, action); err != nil {
		t.logger.Errorf("changeTradeSellClose: fail to addBalanceHistoryIfNotExists, err: %w", err)
		return false
	}

	trd.closedAt = time.Now().UTC()
	trd.status = statusClosed

	logger.WithField("updatedtrade", fmt.Sprintf("%+v", trd))

	if err := t.storer.UpdateTrade(castToStorageTrade(trd)); err != nil {
		logger.WithError(err).Error("changeTradeSellClose: fail update trade")
		return false
	}

	return true
}

// action is idempotent
func (t *Trader) addBalanceHistoryIfNotExists(trd trade, action string) error {
	tradeLatestBalanceHistory, err := t.storer.LatestTradeBalanceHistory(t.appID, trd.id)
	if err != nil {
		return fmt.Errorf("addBalanceHistoryIfNotExists: fail fetch LatestTradeBalanceHistory, err: %w", err)
	}

	// stop here if already have entry in balance history
	if tradeLatestBalanceHistory.InternalTradeID == trd.id {
		if tradeLatestBalanceHistory.Action == action {
			return nil
		}
	}

	prevBalance, err := t.latestBalanceHistory()
	if err != nil {
		return fmt.Errorf("addBalanceHistoryIfNotExists: fail fetch latestBalance, err: %w", err)
	}

	netProfit := t.tradeNetProfit(trd)

	balance := balanceHistory{
		appID:           t.appID,
		action:          "CASHED_IN",
		quoteVolume:     netProfit,
		totalNetIncome:  prevBalance.totalNetIncome + netProfit,
		totalReinvested: prevBalance.totalReinvested,
		internalTradeID: trd.id,
		createdAt:       time.Now().UTC(),
	}

	if err := t.storer.AddBalanceHistory(t.appID, castToStorageBalanceHistory(balance)); err != nil {
		return fmt.Errorf("addBalanceHistoryIfNotExists: fail to store balanceHistory, err: %w", err)
	}

	return nil
}

func (t *Trader) tradeNetProfit(trd trade) float64 {
	openVolume := trd.openBasePrice * trd.baseVolume
	closeVolume := trd.closeBasePrice * trd.baseVolume

	openFees := (t.marketOrderFees / 100) * openVolume
	if trd.openType == "LIMIT" {
		openFees = (t.limitOrderFees / 100) * openVolume
	}

	closeFees := (t.marketOrderFees / 100) * closeVolume
	if trd.closeType == "LIMIT" {
		closeFees = (t.limitOrderFees / 100) * openVolume
	}

	return closeVolume - openVolume - openFees - closeFees
}

func (t *Trader) addMissingTrades() bool {
	startTimeMs := time.Now().UnixNano() / int64(time.Millisecond)

	steps := t.stepper.Steps()
	trades, err := t.activeTrades()
	if err != nil {
		t.logger.WithError(err).Error("addMissingTrades: fail fetch active trades")
		return false
	}

	isOk := true

	// TODO refactor this, split in methods
	for _, s := range steps {
		var hasTrade bool
		for _, trd := range trades {
			if s == trd.openBasePrice {
				hasTrade = true
			}
		}

		if hasTrade {
			continue
		}

		logger := t.logger.WithField("step", s)

		volume, quoteCompounded, err := t.compounder.Volume(s)
		if err != nil {
			logger.WithError(err).Error("addMissingTrades: fail calculate compound volume")
			isOk = false
			continue
		}

		trd := storage.Trade{
			AppID:          t.appID,
			OpenBasePrice:  s,
			CloseBasePrice: t.stepper.ClosePrice(s),
			BaseVolume:     volume,
			Status:         statusBuyLimit,
			CreatedAt:      time.Now().UTC(),
		}

		logger.WithField("datatrade", fmt.Sprintf("%+v", trd))

		id, err := t.storer.AddTrade(trd)
		if err != nil {
			logger.WithError(err).Error("addMissingTrades: fail insert trade")
			isOk = false
			continue
		}

		// only if exists compound
		if quoteCompounded <= 0 {
			continue
		}

		// TODO major issue
		// if this fail, volume will be added again on next trade
		latestBh, err := t.storer.LatestBalanceHistory(t.appID)
		if err != nil {
			logger.WithError(err).Error("addMissingTrades: fail get LatestBalanceHistory")
			isOk = false
			continue
		}

		bh := balanceHistory{
			appID:           t.appID,
			action:          "REINVEST",
			quoteVolume:     quoteCompounded,
			totalNetIncome:  latestBh.TotalNetIncome,
			totalReinvested: latestBh.TotalReinvested + quoteCompounded,
			internalTradeID: id,
			createdAt:       time.Now().UTC(),
		}
		err = t.storer.AddBalanceHistory(t.appID, castToStorageBalanceHistory(bh))
		if err != nil {
			logger.WithError(err).Error("addMissingTrades: fail insert reinvest balance history")
			isOk = false
			continue
		}
	}

	labels := prometheus.Labels{"appid": strconv.Itoa(t.appID)}
	endTimeMs := time.Now().UnixNano() / int64(time.Millisecond)
	metricAddMissingLatency.With(labels).Observe(float64(endTimeMs - startTimeMs))

	return isOk
}

// this should decide which orders should publish / unPublish on exchange
func (t *Trader) markForPublishUnPublish() bool {
	startTimeMs := time.Now().UnixNano() / int64(time.Millisecond)

	trades, err := t.activeTrades()
	if err != nil {
		t.logger.WithError(err).Error("markForPublishUnPublish: fail fetch active trades")
		return false
	}

	isOk := true

	// simple version, will publish all orders
	for _, trd := range trades {
		logger := t.logger.WithField("datatrade", fmt.Sprintf("%+v", trd))

		switch trd.status {
		case statusBuyLimit:
			trd.status = statusBuyLimitWantsPublish
		case statusSellLimit:
			trd.status = statusSellLimitWantsPublish
		default:
			continue
		}

		if err := t.storer.UpdateTrade(castToStorageTrade(trd)); err != nil {
			logger.WithError(err).Error("markForPublishUnPublish: fail update trade")
			isOk = false
		}
	}

	labels := prometheus.Labels{"appid": strconv.Itoa(t.appID)}
	endTimeMs := time.Now().UnixNano() / int64(time.Millisecond)
	metricMarkPublishLatency.With(labels).Observe(float64(endTimeMs - startTimeMs))

	return isOk
}

func (t *Trader) reconcileFromStorageToExchange() bool {
	startTimeMs := time.Now().UnixNano() / int64(time.Millisecond)

	trades, err := t.activeTrades()
	if err != nil {
		t.logger.WithError(err).Error("reconcileFromStorageToExchange: fail fetch active trades")
		return false
	}

	isOk := true

	for _, trd := range trades {
		switch trd.status {
		case statusBuyLimitWantsPublish:
			if ok := t.publishBuyLimitOrder(trd); !ok {
				isOk = false
			}
		case statusSellLimitWantsPublish:
			if ok := t.publishSellLimitOrder(trd); !ok {
				isOk = false
			}
		default:
			continue
		}
	}

	labels := prometheus.Labels{"appid": strconv.Itoa(t.appID)}
	endTimeMs := time.Now().UnixNano() / int64(time.Millisecond)
	metricPublishTradesLatency.With(labels).Observe(float64(endTimeMs - startTimeMs))

	return isOk
}

func (t *Trader) publishBuyLimitOrder(trd trade) bool {
	logger := t.logger.WithField("datatrade", fmt.Sprintf("%+v", trd))

	ord := connector.Order{
		Base:      t.base,
		Quote:     t.quote,
		OrderType: connector.OrderTypeLimit,
		Side:      connector.OrderSideBuy,
		Price:     trd.openBasePrice,
		Volume:    trd.baseVolume,
	}

	logger.WithField("dataorder", fmt.Sprintf("%+v", ord))

	orderID, err := t.connector.AddOrder(t.appID, ord)
	if err != nil {
		logger.WithError(err).Error("publishBuyLimitOrder: fail add exchange order")
		return false
	}

	trd.buyOrderID = orderID
	trd.status = statusBuyLimitPublished
	logger.WithField("dataupdatedtrade", fmt.Sprintf("%+v", trd))

	if err := t.storer.UpdateTrade(castToStorageTrade(trd)); err != nil {
		logger.WithError(err).Error("publishBuyLimitOrder: fail update trade")
		return false
	}

	return true
}

func (t *Trader) publishSellLimitOrder(trd trade) bool {
	logger := t.logger.WithField("datatrade", fmt.Sprintf("%+v", trd))

	ord := connector.Order{
		Base:      t.base,
		Quote:     t.quote,
		OrderType: connector.OrderTypeLimit,
		Side:      connector.OrderSideSell,
		Price:     trd.closeBasePrice,
		Volume:    trd.baseVolume,
	}

	logger.WithField("dataorder", fmt.Sprintf("%+v", ord))

	orderID, err := t.connector.AddOrder(t.appID, ord)
	if err != nil {
		logger.WithError(err).Error("publishSellLimitOrder: fail add exchange order")
		return false
	}

	trd.sellOrderID = orderID
	trd.status = statusSellLimitPublished
	logger.WithField("dataupdatedtrade", fmt.Sprintf("%+v", trd))

	if err := t.storer.UpdateTrade(castToStorageTrade(trd)); err != nil {
		logger.WithError(err).Error("publishSellLimitOrder: fail update trade")
		return false
	}

	return true
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
		return []trade{}, fmt.Errorf("activeTrades: fail fetch active trades, err: %w", err)
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

type balanceHistory struct {
	appID           int
	action          string
	quoteVolume     float64
	totalNetIncome  float64
	totalReinvested float64
	internalTradeID int
	createdAt       time.Time
}

func (t *Trader) latestBalanceHistory() (balanceHistory, error) {
	prevBalance, err := t.storer.LatestBalanceHistory(t.appID)
	if err != nil {
		return balanceHistory{}, err
	}

	return castStorageBalanceHistory(prevBalance), nil
}
