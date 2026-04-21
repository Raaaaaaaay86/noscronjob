package noscronjob

import (
	"context"
	"time"

	"github.com/go-co-op/gocron/v2"
)

type IScheduler interface {
	RegisterCronJob(expression string, opts []gocron.JobOption, handler ...HandlerFunc) error
	RegisterIntervalJob(interval time.Duration, opts []gocron.JobOption, handler ...HandlerFunc) error
	Start(ctx context.Context)
	Stop(ctx context.Context) <-chan struct{}
	Close() error
}
