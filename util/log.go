package util

import (
	"fmt"
	"log"

	"github.com/getsentry/sentry-go"
)

// SentryConfig config
type SentryConfig struct {
	DSN         string `yaml:"dsn"         mapstructure:"dsn"`
	Environment string `yaml:"environment" mapstructure:"environment"`
}

// Logger wraps log infrastructure
type Logger struct {
}

// NewLogger Constructor
func NewLogger(config SentryConfig) *Logger {

	err := sentry.Init(sentry.ClientOptions{
		Dsn:         config.DSN,
		Environment: config.Environment,
	})
	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}

	return &Logger{}
}

// Fatal error
func (l *Logger) Fatal(err error) {
	sentry.CaptureException(err)
	log.Fatal(err)
}

// Fatalf error
func (l *Logger) Fatalf(format string, v ...interface{}) {
	err := fmt.Errorf(format, v...)
	l.Fatal(err)
}

// Warning error
func (l *Logger) Warning(err error) {
	sentry.CaptureException(err)
	log.Print(err)
}

// Warningf error
func (l *Logger) Warningf(format string, v ...interface{}) {
	err := fmt.Errorf(format, v...)
	l.Warning(err)
}
