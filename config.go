package noscronjob

import "time"

type Config struct {
	MaximumStopTime time.Duration
}

func (c Config) GetMaximumStopTime() time.Duration {
	if c.MaximumStopTime.Seconds() == 0 {
		return 10 * time.Second
	}
	return c.MaximumStopTime
}
