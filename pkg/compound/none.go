package compound

import "fmt"

type ConfigNone struct {
	InitialStepQuoteVolume float64
	MinBaseLotAllowed      float64
	MaxBaseLotAllowed      float64
	BaseLotTick            float64
}

type None struct {
	initialStepQuoteVolume float64
	minBaseLotAllowed      float64
	maxBaseLotAllowed      float64
	baseLotTick            float64
}

func NewCompoundNone(cfg *ConfigNone) *None {
	return &None{
		initialStepQuoteVolume: cfg.InitialStepQuoteVolume,
		minBaseLotAllowed:      cfg.MinBaseLotAllowed,
		maxBaseLotAllowed:      cfg.MaxBaseLotAllowed,
		baseLotTick:            cfg.BaseLotTick,
	}
}

func (c *None) Volume(step float64) (float64, error) {
	precision := floatPrecision(c.baseLotTick)
	volume := toFixed(c.initialStepQuoteVolume/step, precision)

	if volume > c.maxBaseLotAllowed || volume < c.minBaseLotAllowed {
		return 0, fmt.Errorf("volume: %v not in range: %v - %v",
			volume,
			c.minBaseLotAllowed,
			c.maxBaseLotAllowed,
		)
	}

	return toFixed(volume, precision), nil
}
