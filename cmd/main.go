package main

import (
	"bwd/pkg/bwd"
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Interval                time.Duration `env:"INTERVAL" env-default:""`
	SlackHook               string        `env:"SLACK_HOOK" env-default:""`
	WebBindingPort          string        `env:"WEB_BINDING_PORT" env-default:""`
	StorageConnectionString string        `env:"STORAGE_CONNECTION_STRING" env-default:""`
}

func (c *Config) validate() error {
	if c.Interval < 100*time.Millisecond {
		return errors.New("[CONFIG] Interval should be greater than 100ms")
	}

	//if c.WebBindingPort == "" {
	//	return errors.New("[CONFIG] WebBindingPort can not be empty")
	//}

	if c.StorageConnectionString == "" {
		return errors.New("[CONFIG] StorageConnectionString can not be empty")
	}

	return nil
}

func main() {
	logger := logger()

	// cfg
	cfg := &Config{}
	if err := cleanenv.ReadEnv(cfg); err != nil {
		logger.WithError(err).Fatal("can not read env vars")
	}
	if err := cfg.validate(); err != nil {
		logger.WithError(err).Fatal("can not validate config")
	}

	ctx, cancel := context.WithCancel(context.Background())

	configBwd := &bwd.ConfigBwd{
		Interval:                cfg.Interval,
		SlackHook:               cfg.SlackHook,
		WebBindingPort:          cfg.WebBindingPort,
		StorageConnectionString: cfg.StorageConnectionString,
	}

	b := bwd.New(ctx, configBwd, logger)
	err := b.Start()
	if err != nil {
		logger.WithError(err).Fatal("unsuccessful start, everything stopped.")
	}

	logger.Info("successful start, press Ctrl + C to graceful shutdown")
	sigint := make(chan os.Signal)
	signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
	<-sigint

	logger.Info("bwd stopping ...")
	cancel()
	b.Wait()

	logger.Info("bwd successful stop.")
}

type UTCFormatter struct {
	logrus.Formatter
}

func logger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetFormatter(
		UTCFormatter{
			&logrus.TextFormatter{
				TimestampFormat: time.RFC3339,
				FullTimestamp:   true,
				DisableColors:   false,
				DisableSorting:  false,
			},
		},
	)

	return logger
}
