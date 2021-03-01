package compound

type None struct {
	initialStepQuoteVolume float64
}

func NewCompoundNone(initialStepQuoteVolume float64) *None {
	return &None{
		initialStepQuoteVolume: initialStepQuoteVolume,
	}
}

func (c *None) Volume(step float64) float64 {
	return c.initialStepQuoteVolume
}
