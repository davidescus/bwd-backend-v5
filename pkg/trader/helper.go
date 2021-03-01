package trader

import (
	"bwd/pkg/connector"
	"bwd/pkg/storage"
)

func castTradeToStorageTrade(trade trade) storage.Trade {
	return storage.Trade{
		ID:                   trade.id,
		AppID:                trade.appID,
		OpenBasePrice:        trade.openBasePrice,
		CloseBasePrice:       trade.closeBasePrice,
		OpenType:             trade.openType,
		CloseType:            trade.closeType,
		BaseVolume:           trade.baseVolume,
		BuyOrderID:           trade.buyOrderID,
		SellOrderID:          trade.sellOrderID,
		Status:               trade.status,
		ConvertedSellLimitAt: trade.convertedSellLimitAt,
		ClosedAt:             trade.closedAt,
		UpdatedAt:            trade.updatedAt,
		CreatedAt:            trade.createdAt,
	}
}

func castConnectorOrderToTraderOrder(o connector.Order) order {
	return order{
		id:        o.ID,
		base:      o.Base,
		quote:     o.Quote,
		orderType: o.OrderType,
		side:      o.Side,
		price:     o.Price,
		volume:    o.Volume,
		status:    o.Status,
	}
}
