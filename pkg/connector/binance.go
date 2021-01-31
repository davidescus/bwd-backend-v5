package connector

import "github.com/adshao/go-binance/v2"

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