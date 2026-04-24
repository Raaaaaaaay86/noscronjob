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

type IScheduler interface {
	// RegisterCronJob registers job by using 6-digits cron expression which support to seconds.
	//
	// cron expression parser: https://crontab.cronhub.io/
	RegisterCronJob(expression string, opts []gocron.JobOption, handler ...HandlerFunc) error
	// RegisterIntervalJob register job by using time.Duration.
	RegisterIntervalJob(interval time.Duration, opts []gocron.JobOption, handler ...HandlerFunc) error
	// Start runs all registered jobs.
	Start(ctx context.Context)
	// Start stops all registered jobs.
	Stop(ctx context.Context) <-chan struct{}
	Close() error
}

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
		return nil, xerrors.Errorf("failed to create gocron.Scheduler: %w", err)
	}

	return &GoCronScheduler{
		config:    config,
		scheduler: scheduler,
	}, nil
}

func (s *GoCronScheduler) RegisterCronJob(expression string, opts []gocron.JobOption, handlers ...HandlerFunc) error {
	if len(handlers) == 0 {
		return nil
	}

	name, ok := getHandlerName(handlers[len(handlers)-1])
	if !ok {
		name = fmt.Sprintf("noname:%d", time.Now().UnixMicro())
	}

	opts = append(opts, gocron.WithName(name))

	task := gocron.NewTask(func(ctx context.Context) {
		if s.config.TracerProvider != nil {
			tctx, span := s.withTracedContext(ctx, name)
			defer span.End()

			ctx = tctx
		}

		c := NewContext(ctx, handlers)

		c.Next()

		if err := c.Err(); err != nil {
			slog.Error("job failed", "name", name, "expression", expression, "error", err)
		}
	})

	_, err := s.scheduler.NewJob(gocron.CronJob(expression, true), task, opts...)
	if err != nil {
		return xerrors.Errorf("failed to register cron(%s) job: %w", expression, err)
	}

	return nil
}

func (s *GoCronScheduler) RegisterIntervalJob(
	interval time.Duration,
	opts []gocron.JobOption,
	handlers ...HandlerFunc,
) error {
	if len(handlers) == 0 {
		return nil
	}

	name, ok := getHandlerName(handlers[len(handlers)-1])
	if !ok {
		name = fmt.Sprintf("noname:%d", time.Now().UnixMicro())
	}

	opts = append(opts, gocron.WithName(name))

	task := gocron.NewTask(func(ctx context.Context) {
		if s.config.TracerProvider != nil {
			tctx, span := s.withTracedContext(ctx, name)
			defer span.End()

			ctx = tctx
		}

		c := NewContext(ctx, handlers)

		c.Next()

		if err := c.Err(); err != nil {
			slog.Error("job failed", "name", name, "interval", interval.String(), "error", err)
		}
	})

	_, err := s.scheduler.NewJob(gocron.DurationJob(interval), task, opts...)
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
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), s.config.GetMaximumStopTime())
		defer cancel()
		_ = s.Stop(sctx)
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
