package app

import (
	"context"

	"github.com/sirupsen/logrus"
)

type ConfigApp struct {
	PublishOrderNumber int
	Status             string
}

type App struct {
	ctx    context.Context
	logger *logrus.Logger

	publishOrderNumber int
	status             string
	// TODO other fields here
}

func New(ctx context.Context, cfg *ConfigApp, logger *logrus.Logger) *App {
	return &App{
		ctx:    ctx,
		logger: logger,

		publishOrderNumber: cfg.PublishOrderNumber,
		status:             cfg.Status,
	}
}

func (a *App) Start() {
	// TODO implement me
}

func (a *App) Stop() {
	// TODO implement me
}

func (a *App) SetPublishOrderNumber(publishOrderNumber int) {
	a.publishOrderNumber = publishOrderNumber
}

func (a *App) SetStatus(status string) {
	a.status = status
}

// TODO and so on create setters for all fields that can change during run
