package noscronjob

import (
	"time"

	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	MaximumStopTime time.Duration
	TracerProvider  trace.TracerProvider
}

func (c Config) GetMaximumStopTime() time.Duration {
	if c.MaximumStopTime.Seconds() == 0 {
		return 10 * time.Second
	}
	return c.MaximumStopTime
}
