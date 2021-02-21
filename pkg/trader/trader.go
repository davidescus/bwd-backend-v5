package trader

import (
	"bwd/pkg/compound"
	"bwd/pkg/connector"
	"bwd/pkg/step"
	"bwd/pkg/storage"

	"github.com/sirupsen/logrus"
)

type ConfigTrader struct {
	Storer     storage.AppStorer
	Connector  connector.Connector
	Stepper    step.Stepper
	Compounder compound.Compounder
}

type Trader struct {
	logger     *logrus.Logger
	storer     storage.AppStorer
	connector  connector.Connector
	stepper    step.Stepper
	compounder compound.Compounder
}

func New(cfg *ConfigTrader, logger *logrus.Logger) *Trader {
	return &Trader{
		logger:     logger,
		storer:     cfg.Storer,
		connector:  cfg.Connector,
		stepper:    cfg.Stepper,
		compounder: cfg.Compounder,
	}
}

func (t *Trader) Run() {
	t.logger.Info(" --- trader run -- implement me")

	steps := t.stepper.Steps()

	_ = steps
}
