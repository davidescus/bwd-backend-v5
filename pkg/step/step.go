package step

type Stepper interface {
	Steps() []float64
	NextStepUp(step float64) float64
}
