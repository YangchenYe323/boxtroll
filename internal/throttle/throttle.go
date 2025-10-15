package throttle

import (
	"math/rand/v2"
	"sync"
	"time"
)

type Throttler struct {
	sync.Mutex

	minInterval   time.Duration
	maxInterval   time.Duration
	lastExecution time.Time
}

func New(minInterval, maxInterval time.Duration) *Throttler {
	if minInterval > maxInterval {
		panic("minInterval must be less than maxInterval")
	}

	return &Throttler{
		minInterval: minInterval,
		maxInterval: maxInterval,
	}
}

func (t *Throttler) Run(f func() error) error {
	t.Lock()
	defer t.Unlock()

	// Pick a random throttle interval
	interval := t.minInterval + time.Duration(rand.Float64()*(float64(t.maxInterval-t.minInterval)))

	if time.Since(t.lastExecution) < interval {
		time.Sleep(interval)
	}

	err := f()
	t.lastExecution = time.Now()
	return err
}
