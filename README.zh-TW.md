# noscronjob

`noscronjob` 封裝了 [gocron v2](https://github.com/go-co-op/gocron)，並擴充了類似 gin 的 middleware chain，讓你可以為每個排程任務附加多個 handler 函式（例如日誌、追蹤、錯誤恢復）。

## 安裝

```bash
go get github.com/raaaaaaaay86/noscronjob
```

## 快速開始

### Cron 表達式任務

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

    // 每天凌晨 02:00 執行
    if err := scheduler.RegisterCronJob(
        "daily-report",
        "0 2 * * *",
        generateDailyReport,
    ); err != nil {
        panic(err)
    }

    scheduler.Start(context.Background())
    defer scheduler.Close()

    // 阻塞至關機信號 ...
}

func generateDailyReport(ctx *noscronjob.Context) {
    slog.Info("正在產生每日報表...")
    // 執行工作
    ctx.Next()
}
```

### 間隔任務

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

每個任務可以註冊多個 handler，依序執行。呼叫 `Abort(err)` 可提前中止 chain。

```go
func loggingMiddleware(ctx *noscronjob.Context) {
    start := time.Now()
    slog.Info("任務開始")
    ctx.Next()
    slog.Info("任務完成", "duration", time.Since(start))
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
    // 執行工作
    ctx.Next()
}

scheduler.RegisterIntervalJob(
    "my-job",
    10 * time.Second,
    loggingMiddleware, recoveryMiddleware, actualJob,
)
```

## 優雅停止

`Start` 會自動監聽 context 取消，並在 `ctx` 結束時停止 scheduler。

```go
ctx, cancel := context.WithCancel(context.Background())
scheduler.Start(ctx)

// 手動停止：
cancel()

// 或帶 deadline 停止並等待完成：
stopDone := scheduler.Stop(context.Background())
<-stopDone
```

## 與 nosos 整合

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

## API 說明

### `NewGoCronScheduler`

```go
func NewGoCronScheduler(config Config, options ...gocron.SchedulerOption) (*GoCronScheduler, error)
```

建立新的 scheduler。可傳入額外的 `gocron.SchedulerOption`（例如時區）。

```go
import "github.com/go-co-op/gocron/v2"

scheduler, err := noscronjob.NewGoCronScheduler(
    noscronjob.Config{MaximumStopTime: 15 * time.Second},
    gocron.WithLocation(time.UTC),
)
```

### `Config`

| 欄位              | 型別            | 說明                                              |
|-------------------|-----------------|---------------------------------------------------|
| `MaximumStopTime` | `time.Duration` | 關機時等待執行中任務完成的最長時間（預設：10s）   |

### `IScheduler` 方法

| 方法                                               | 說明                                                       |
|----------------------------------------------------|------------------------------------------------------------|
| `RegisterCronJob(name, expression, handlers...)`   | 以 cron 表達式（含秒格式）註冊任務                         |
| `RegisterIntervalJob(name, interval, handlers...)` | 以固定間隔執行的任務                                       |
| `Start(ctx)`                                       | 啟動 scheduler；context 取消時自動停止                     |
| `Stop(ctx) <-chan struct{}`                        | 停止執行中的任務；回傳一個 channel，所有任務完成時關閉     |
| `Close()`                                          | 完全關閉 scheduler                                         |

### `Context` 方法

| 方法         | 說明                         |
|--------------|------------------------------|
| `Next()`     | 執行 chain 中的下一個 handler |
| `Abort(err)` | 帶錯誤中止 chain             |
| `Err()`      | 回傳目前的錯誤（若有）        |
