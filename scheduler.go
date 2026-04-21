package noscronjob

import (
	"context"
	"time"
)

type IScheduler interface {
	RegisterCronJob(expression string, handler ...HandlerFunc) error
	RegisterIntervalJob(interval time.Duration, handler ...HandlerFunc) error
	Start(ctx context.Context)
	Stop(ctx context.Context) <-chan struct{}
	Close() error
}
