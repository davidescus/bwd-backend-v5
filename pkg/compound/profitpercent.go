package compound

import (
	"bwd/pkg/storage"
	"fmt"
	"math"
)

type ConfigProfitPercent struct {
	AppID                  int
	Storer                 storage.Storer
	InitialStepQuoteVolume float64
	MinBaseLotAllowed      float64
	MaxBaseLotAllowed      float64
	BaseLotTick            float64
}

type ProfitPercent struct {
	appID                  int
	storer                 storage.Storer
	initialStepQuoteVolume float64
	minBaseLotAllowed      float64
	maxBaseLotAllowed      float64
	baseLotTick            float64
}

func NewProfitPercent(cfg *ConfigProfitPercent) *ProfitPercent {
	return &ProfitPercent{
		appID:                  cfg.AppID,
		storer:                 cfg.Storer,
		initialStepQuoteVolume: cfg.InitialStepQuoteVolume,
		minBaseLotAllowed:      cfg.MinBaseLotAllowed,
		maxBaseLotAllowed:      cfg.MaxBaseLotAllowed,
		baseLotTick:            cfg.BaseLotTick,
	}
}

// TODO for moment this compounder will try to compound all profit
// return:
// total base volume for new trade
// quote compounded volume
func (c *ProfitPercent) Volume(step float64) (float64, float64, error) {
	// totalVolume source: initialStepVolume
	totalVolume := c.initialStepQuoteVolume / step

	// totalVolume source: latest closed trade
	latestClosedTrade, err := c.storer.LatestAppClosedTradeByOpenPrice(c.appID, step)
	if err != nil {
		return 0, 0, err
	}

	totalVolume = math.Max(totalVolume, latestClosedTrade.BaseVolume)

	// get balance and see if we can add compound value
	latestBalance, err := c.storer.LatestBalanceHistory(c.appID)
	if err != nil {
		return 0, 0, err
	}
	availableQuoteVolume := latestBalance.TotalNetIncome - latestBalance.TotalReinvested

	precision := floatPrecision(c.baseLotTick)

	var quoteCompoundedVolume float64
	if availableQuoteVolume > 0 {
		baseToCompound := availableQuoteVolume / step
		if toFixed(baseToCompound, precision) >= c.baseLotTick {
			totalVolume = totalVolume + baseToCompound
			quoteCompoundedVolume = availableQuoteVolume
		}
	}

	if totalVolume > c.maxBaseLotAllowed || totalVolume < c.minBaseLotAllowed {
		return 0, 0, fmt.Errorf("totalVolume: %v not in range: %v - %v",
			totalVolume,
			c.minBaseLotAllowed,
			c.maxBaseLotAllowed,
		)
	}

	return toFixed(totalVolume, precision), quoteCompoundedVolume, nil
}
