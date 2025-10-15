package retry

import (
	"math/rand/v2"
	"time"
)

type ExponentialBackoffWithJitter struct {
	Min         time.Duration // Minimal wait interval
	Max         time.Duration // Maximal wait interval
	Multiplier  float64       // Multiplier for the wait interval
	Jttr        float64       // Jitter for the wait interval
	MaxAttempts int           // Maximal number of attempts to run the function
}

func (e *ExponentialBackoffWithJitter) Retry(f func() error, retriable func(error) bool) error {
	backoff := e.Min

	var lastRetriableErr error
	for i := 0; i < e.MaxAttempts; i++ {
		err := f()

		if err == nil {
			return nil
		}

		if !retriable(err) {
			return err
		}

		lastRetriableErr = err

		time.Sleep(backoff)
		backoff = time.Duration(float64(backoff) * e.Multiplier)
		backoff += time.Duration(rand.Float64() * e.Jttr * float64(backoff))
		if backoff > e.Max {
			backoff = e.Max
		}
	}

	return lastRetriableErr
}
