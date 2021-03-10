package trader

import (
	"bwd/pkg/connector"
	"bwd/pkg/storage"
)

func castStorageTrade(st storage.Trade) trade {
	return trade{
		id:                   st.ID,
		appID:                st.AppID,
		openBasePrice:        st.OpenBasePrice,
		closeBasePrice:       st.CloseBasePrice,
		openType:             st.OpenType,
		closeType:            st.CloseType,
		baseVolume:           st.BaseVolume,
		buyOrderID:           st.BuyOrderID,
		sellOrderID:          st.SellOrderID,
		status:               st.Status,
		convertedSellLimitAt: st.ConvertedSellLimitAt,
		closedAt:             st.ClosedAt,
		updatedAt:            st.UpdatedAt,
		createdAt:            st.CreatedAt,
	}
}

func castToStorageTrade(trade trade) storage.Trade {
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

func castOrder(o connector.Order) order {
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

func castStorageBalanceHistory(history storage.BalanceHistory) balanceHistory {
	return balanceHistory{
		appID:           history.AppID,
		action:          history.Action,
		baseVolume:      history.QuoteVolume,
		totalNetIncome:  history.TotalNetIncome,
		totalReinvested: history.TotalReinvested,
		internalTradeID: history.InternalTradeID,
		createdAt:       history.CreatedAt,
	}
}

func castToStorageBalanceHistory(history balanceHistory) storage.BalanceHistory {
	return storage.BalanceHistory{
		AppID:           history.appID,
		Action:          history.action,
		QuoteVolume:     history.baseVolume,
		TotalNetIncome:  history.totalNetIncome,
		TotalReinvested: history.totalReinvested,
		InternalTradeID: history.internalTradeID,
		CreatedAt:       history.createdAt,
	}
}
