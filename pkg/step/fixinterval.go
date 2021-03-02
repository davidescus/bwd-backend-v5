package step

import (
	"encoding/json"
	"strconv"
)

type ConfigStepsFixInterval struct {
	MinPriceAllowed float64
	MaxPriceAllowed float64
	PriceTick       float64
	QuoteMinVolume  float64
	AppSettings     string
}

type StepsFixInterval struct {
	// exchange restrictions
	minPriceAllowed float64
	maxPriceAllowed float64
	priceTick       float64
	// app settings
	appSettings  string
	gridMinPrice float64
	gridMaxPrice float64
	gridInterval float64
}

func NewStepsFixInterval(cfg *ConfigStepsFixInterval) (*StepsFixInterval, error) {
	s := &StepsFixInterval{
		minPriceAllowed: cfg.MinPriceAllowed,
		maxPriceAllowed: cfg.MaxPriceAllowed,
		priceTick:       cfg.PriceTick,
		appSettings:     cfg.AppSettings,
		gridMinPrice:    0,
		gridMaxPrice:    0,
		gridInterval:    0,
	}

	if err := s.parseSettings(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *StepsFixInterval) Steps() []float64 {
	var steps []float64

	precision := floatPrecision(s.priceTick)
	m := s.gridMinPrice

	for toFixed(m, precision) >= s.gridMaxPrice {
		steps = append(steps, toFixed(m, precision))
		m = m - s.gridInterval
	}

	return steps
}

func (s *StepsFixInterval) ClosePrice(step float64) float64 {
	precision := floatPrecision(s.priceTick)
	return toFixed(step+s.gridInterval, precision)
}

func (s *StepsFixInterval) parseSettings() error {
	tmp := struct {
		Min      string `json:"min"`
		Max      string `json:"max"`
		Interval string `json:"interval"`
	}{}

	if err := json.Unmarshal([]byte(s.appSettings), &tmp); err != nil {
		return err
	}

	minFloat, err := strconv.ParseFloat(tmp.Min, 64)
	if err != nil {
		return err
	}
	s.gridMaxPrice = minFloat

	maxFloat, err := strconv.ParseFloat(tmp.Max, 64)
	if err != nil {
		return err
	}
	s.gridMinPrice = maxFloat

	intervalFloat, err := strconv.ParseFloat(tmp.Interval, 64)
	if err != nil {
		return err
	}
	s.gridInterval = intervalFloat

	return nil
}
