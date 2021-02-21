package compound

type None struct{}

func NewCompoundNone() *None {
	return &None{}
}

func (s *None) Volume(volume float64) float64 {
	return volume
}
