package noscronjob

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type SchedulerTestSuite struct {
	suite.Suite
	config Config
}

func (s *SchedulerTestSuite) SetupTest() {
	s.config = Config{
		MaximumStopTime: 1 * time.Second,
	}
}

func TestSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(SchedulerTestSuite))
}

func (s *SchedulerTestSuite) TestBasicLifecycle() {
	scheduler, err := NewGoCronScheduler(s.config)
	s.NoError(err)

	var count atomic.Int32
	err = scheduler.RegisterIntervalJob("test", 100*time.Millisecond, func(c *Context) {
		count.Add(1)
	})
	s.NoError(err)

	scheduler.Start(context.Background())
	time.Sleep(250 * time.Millisecond)

	s.GreaterOrEqual(count.Load(), int32(2))

	<-scheduler.Stop(context.Background())
	lastCount := count.Load()
	time.Sleep(200 * time.Millisecond)

	s.LessOrEqual(count.Load(), lastCount+1)
	_ = scheduler.Close()
}

func (s *SchedulerTestSuite) TestRestart() {
	scheduler, err := NewGoCronScheduler(s.config)
	s.NoError(err)

	var count atomic.Int32
	_ = scheduler.RegisterIntervalJob("restart-test", 100*time.Millisecond, func(c *Context) {
		count.Add(1)
	})

	scheduler.Start(context.Background())
	time.Sleep(150 * time.Millisecond)
	<-scheduler.Stop(context.Background())

	countAfterStop := count.Load()
	scheduler.Start(context.Background())
	time.Sleep(200 * time.Millisecond)

	s.Greater(count.Load(), countAfterStop, "Count should increase after restart")
	_ = scheduler.Close()
}

func (s *SchedulerTestSuite) TestGracefulShutdown_Completion() {
	scheduler, err := NewGoCronScheduler(s.config)
	s.NoError(err)

	var finished atomic.Bool
	_ = scheduler.RegisterIntervalJob("graceful", 100*time.Millisecond, func(c *Context) {
		time.Sleep(300 * time.Millisecond)
		finished.Store(true)
	})

	scheduler.Start(context.Background())
	time.Sleep(150 * time.Millisecond)

	_ = scheduler.Close()

	s.True(finished.Load(), "Job did not finish during graceful shutdown")
}

func (suite *SchedulerTestSuite) TestGracefulShutdown_Context() {
	s, err := NewGoCronScheduler(suite.config)
	suite.NoError(err)

	var cancelled atomic.Bool
	_ = s.RegisterIntervalJob("ctx-test", 100*time.Millisecond, func(c *Context) {
		select {
		case <-c.Done():
			cancelled.Store(true)
		case <-time.After(500 * time.Millisecond):
		}
	})

	s.Start(context.Background())
	time.Sleep(150 * time.Millisecond)
	_ = s.Close()

	suite.True(cancelled.Load(), "Job context was not cancelled during shutdown")
}

func (suite *SchedulerTestSuite) TestAutoStop() {
	s, err := NewGoCronScheduler(suite.config)
	suite.NoError(err)

	var count atomic.Int32
	_ = s.RegisterIntervalJob("autostop", 100*time.Millisecond, func(c *Context) {
		count.Add(1)
	})

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(150 * time.Millisecond)

	cancel()
	time.Sleep(100 * time.Millisecond)

	lastCount := count.Load()
	time.Sleep(200 * time.Millisecond)
	suite.LessOrEqual(count.Load(), lastCount+1)
	_ = s.Close()
}

func (s *SchedulerTestSuite) TestIdempotency() {
	scheduler, err := NewGoCronScheduler(s.config)
	s.NoError(err)

	scheduler.Start(context.Background())
	scheduler.Start(context.Background())

	s.True(scheduler.running.Load())
	_ = scheduler.Close()
}

func (s *SchedulerTestSuite) TestNoStartAfterClose() {
	scheduler, err := NewGoCronScheduler(s.config)
	s.NoError(err)
	_ = scheduler.Close()

	var count atomic.Int32
	_ = scheduler.RegisterIntervalJob("no-start", 10*time.Millisecond, func(c *Context) {
		count.Add(1)
	})

	scheduler.Start(context.Background())
	time.Sleep(50 * time.Millisecond)

	s.Equal(int32(0), count.Load(), "Scheduler should not start after close")
}
