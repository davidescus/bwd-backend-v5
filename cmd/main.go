package main

import (
	"bwd/pkg/bwd"
	syslog2 "bwd/pkg/utils"
	"bwd/pkg/utils/metrics/exporter"
	"context"
	"errors"
	"log"
	"log/syslog"
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
	go exporter.GetMetricsExporter("7070")

	log := logger()

	cfg := &Config{}
	if err := cleanenv.ReadEnv(cfg); err != nil {
		log.WithError(err).Fatal("can not read env vars")
	}

	if err := cfg.validate(); err != nil {
		log.WithError(err).Fatalf("invalid config: %+v", cfg)
	}

	ctx, cancel := context.WithCancel(context.Background())

	configBwd := &bwd.ConfigBwd{
		Interval:                cfg.Interval,
		SlackHook:               cfg.SlackHook,
		WebBindingPort:          cfg.WebBindingPort,
		StorageConnectionString: cfg.StorageConnectionString,
	}

	b := bwd.New(ctx, configBwd, log)
	err := b.Start()
	if err != nil {
		log.WithError(err).Fatal("unsuccessful start, everything stopped.")
	}

	log.Info("successful start, press Ctrl + C to graceful shutdown")
	sigint := make(chan os.Signal)
	signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
	<-sigint

	log.Info("bwd stopping ...")
	cancel()
	b.Wait()

	log.Info("bwd successful stop.")
}

type UTCFormatter struct {
	logrus.Formatter
}

func logger() logrus.FieldLogger {
	l := logrus.New()
	l.SetOutput(os.Stdout)
	l.SetFormatter(
		UTCFormatter{
			&logrus.TextFormatter{
				TimestampFormat: time.RFC3339,
				FullTimestamp:   true,
				DisableColors:   true,
				DisableSorting:  true,
				ForceQuote:      true,
			},
		},
	)

	l.SetLevel(logrus.DebugLevel)

	syslogOutput, er := syslog2.NewSyslogHook("", "", syslog.LOG_INFO|syslog.LOG_DAEMON, "")
	if er != nil {
		log.Fatal("main: unable to setup syslog output")
	}
	l.AddHook(syslogOutput)

	return l.WithField("app", "bwd").WithField("module", "main")
}
