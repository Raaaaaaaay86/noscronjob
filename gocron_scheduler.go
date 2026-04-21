package noscronjob

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-co-op/gocron/v2"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/xerrors"
)

var _ IScheduler = (*GoCronScheduler)(nil)

type GoCronScheduler struct {
	scheduler gocron.Scheduler
	closeOnce sync.Once
	closed    atomic.Bool
	running   atomic.Bool
	config    Config
}

func NewGoCronScheduler(config Config, options ...gocron.SchedulerOption) (*GoCronScheduler, error) {
	scheduler, err := gocron.NewScheduler(options...)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return &GoCronScheduler{
		config:    config,
		scheduler: scheduler,
	}, nil
}

func (s *GoCronScheduler) RegisterCronJob(name string, expression string, handler ...HandlerFunc) error {
	_, err := s.scheduler.NewJob(
		gocron.CronJob(expression, true),
		gocron.NewTask(func(ctx context.Context) {
			c := NewContext(ctx, handler)
			c.Next()
			if err := c.Err(); err != nil {
				slog.Error("job failed", "name", name, "expression", expression, "error", err)
			}
		}),
		gocron.WithName(name),
	)
	if err != nil {
		return xerrors.Errorf("failed to register cron(%s) job: %w", expression, err)
	}
	return nil
}

func (s *GoCronScheduler) RegisterIntervalJob(name string, interval time.Duration, handler ...HandlerFunc) error {
	_, err := s.scheduler.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(func(ctx context.Context) {
			if s.config.TracerProvider != nil {
				tctx, span := s.withTracedContext(ctx, name)
				defer span.End()

				ctx = tctx
			}

			c := NewContext(ctx, handler)

			c.Next()

			if err := c.Err(); err != nil {
				slog.Error("job failed", "name", name, "interval", interval.String(), "error", err)
			}
		}),
		gocron.WithName(name),
	)
	if err != nil {
		return xerrors.Errorf("failed to register interval(%s) job: %w", interval.String(), err)
	}
	return nil
}

func (s *GoCronScheduler) withTracedContext(ctx context.Context, jobName string) (context.Context, trace.Span) {
	tctx, span := s.config.TracerProvider.Tracer(TRACER_NAME).Start(ctx, fmt.Sprintf("cronjob.gocron.%s", jobName))

	return tctx, span
}

func (s *GoCronScheduler) Start(ctx context.Context) {
	if s.closed.Load() {
		slog.Warn("attempted to start a closed scheduler")
		return
	}

	if s.running.Swap(true) {
		return
	}

	s.scheduler.Start()
	go func() {
		select {
		case <-ctx.Done():
			sctx, cancel := context.WithTimeout(context.Background(), s.config.GetMaximumStopTime())
			defer cancel()
			_ = s.Stop(sctx)
		}
	}()
}

func (s *GoCronScheduler) Stop(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.running.Store(false)

		stopDone := make(chan struct{})
		go func() {
			_ = s.scheduler.StopJobs()
			close(stopDone)
		}()

		select {
		case <-stopDone:
		case <-ctx.Done():
		}
	}()
	return done
}

func (s *GoCronScheduler) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		s.running.Store(false)
		err = s.scheduler.Shutdown()
	})
	return err
}
