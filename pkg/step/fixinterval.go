package step

import (
	"encoding/json"
	"math"
	"strconv"
)

type StepsFixInterval struct {
	max, min, interval float64
	precision          int
}

func NewStepsFixInterval(settings string) (*StepsFixInterval, error) {
	s := &StepsFixInterval{}

	if err := s.parseSettings(settings); err != nil {
		return nil, err
	}

	// TODO validate settings

	return s, nil
}

func (s *StepsFixInterval) Steps() []float64 {
	var steps []float64

	m := s.max
	for m >= s.min {
		steps = append(steps, m)
		m = toFixed(m-s.interval, s.precision)
	}

	return steps
}

func (s *StepsFixInterval) parseSettings(settings string) error {
	tmp := struct {
		Min       string `json:"min"`
		Max       string `json:"max"`
		Interval  string `json:"interval"`
		Precision int    `json:"precision"`
	}{}

	if err := json.Unmarshal([]byte(settings), &tmp); err != nil {
		return err
	}

	minFloat, err := strconv.ParseFloat(tmp.Min, 64)
	if err != nil {
		return err
	}
	s.min = minFloat

	maxFloat, err := strconv.ParseFloat(tmp.Max, 64)
	if err != nil {
		return err
	}
	s.max = maxFloat

	intervalFloat, err := strconv.ParseFloat(tmp.Interval, 64)
	if err != nil {
		return err
	}
	s.interval = intervalFloat

	s.precision = tmp.Precision

	return nil
}

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}
