package connector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/adshao/go-binance/v2"
)

type BinanceConfig struct {
	ApiKey    string
	SecretKey string
}

type Binance struct {
	connection *binance.Client
}

func NewBinance(cfg *BinanceConfig) *Binance {
	return &Binance{
		connection: binance.NewClient(cfg.ApiKey, cfg.SecretKey),
	}
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
