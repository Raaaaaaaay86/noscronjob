# noscronjob

`noscronjob` wraps [gocron v2](https://github.com/go-co-op/gocron) and extends it with a gin-style middleware chain so you can attach multiple handler functions (e.g. logging, tracing, error recovery) to each scheduled job.

## Installation

```bash
go get github.com/raaaaaaaay86/noscronjob
```

## Quick Start

### Cron Expression Jobs

```go
package main

import (
    "context"
    "log/slog"
    "time"

    "github.com/raaaaaaaay86/noscronjob"
)

func main() {
    scheduler, err := noscronjob.NewGoCronScheduler(noscronjob.Config{
        MaximumStopTime: 30 * time.Second,
    })
    if err != nil {
        panic(err)
    }

    // Run every day at 02:00
    if err := scheduler.RegisterCronJob(
        "daily-report",
        "0 2 * * *",
        generateDailyReport,
    ); err != nil {
        panic(err)
    }

    scheduler.Start(context.Background())
    defer scheduler.Close()

    // Block until shutdown signal ...
}

func generateDailyReport(ctx *noscronjob.Context) {
    slog.Info("generating daily report...")
    // do work
    ctx.Next()
}
```

### Interval Jobs

```go
if err := scheduler.RegisterIntervalJob(
    "cache-warmer",
    5 * time.Minute,
    warmCache,
); err != nil {
    panic(err)
}
```

## Middleware Chain

Register multiple handlers per job. They are called in order; call `Abort(err)` to stop the chain early.

```go
func loggingMiddleware(ctx *noscronjob.Context) {
    start := time.Now()
    slog.Info("job started")
    ctx.Next()
    slog.Info("job finished", "duration", time.Since(start))
}

func recoveryMiddleware(ctx *noscronjob.Context) {
    defer func() {
        if r := recover(); r != nil {
            ctx.Abort(fmt.Errorf("panic: %v", r))
        }
    }()
    ctx.Next()
}

func actualJob(ctx *noscronjob.Context) {
    // do work
    ctx.Next()
}

scheduler.RegisterIntervalJob(
    "my-job",
    10 * time.Second,
    loggingMiddleware, recoveryMiddleware, actualJob,
)
```

## Graceful Stop

`Start` automatically listens for context cancellation and stops the scheduler when `ctx` is done.

```go
ctx, cancel := context.WithCancel(context.Background())
scheduler.Start(ctx)

// Stop manually:
cancel()

// Or stop with a deadline and wait:
stopDone := scheduler.Stop(context.Background())
<-stopDone
```

## Integration with nosos

```go
scheduler.Start(ctx)

nosos.GracefulShutdown(ctx, nosos.GracefulShutdownSetup{
    ListenedSignals: nosos.DefaultShutdownSignals,
    MaxWait:         30 * time.Second,
    OnShutdown: func(ctx context.Context) error {
        stopDone := scheduler.Stop(ctx)
        <-stopDone
        return scheduler.Close()
    },
})
```

## API Reference

### `NewGoCronScheduler`

```go
func NewGoCronScheduler(config Config, options ...gocron.SchedulerOption) (*GoCronScheduler, error)
```

Creates a new scheduler. Pass additional `gocron.SchedulerOption` values (e.g. timezone) as needed.

```go
import "github.com/go-co-op/gocron/v2"

scheduler, err := noscronjob.NewGoCronScheduler(
    noscronjob.Config{MaximumStopTime: 15 * time.Second},
    gocron.WithLocation(time.UTC),
)
```

### `Config`

| Field             | Type            | Description                                                             |
|-------------------|-----------------|-------------------------------------------------------------------------|
| `MaximumStopTime` | `time.Duration` | Max time to wait for running jobs to finish on shutdown (default: 10s)  |

### `IScheduler` Methods

| Method                                             | Description                                                               |
|----------------------------------------------------|---------------------------------------------------------------------------|
| `RegisterCronJob(name, expression, handlers...)`   | Register a job with a cron expression (seconds-included format)           |
| `RegisterIntervalJob(name, interval, handlers...)` | Register a job that runs every `interval`                                 |
| `Start(ctx)`                                       | Start the scheduler; auto-stops when `ctx` is cancelled                   |
| `Stop(ctx) <-chan struct{}`                        | Stop running jobs; returns a channel that closes when all jobs finish     |
| `Close()`                                          | Shut down the scheduler entirely                                          |

### `Context` Methods

| Method       | Description                       |
|--------------|-----------------------------------|
| `Next()`     | Execute the next handler in chain |
| `Abort(err)` | Stop the chain with an error      |
| `Err()`      | Return the current error (if any) |
