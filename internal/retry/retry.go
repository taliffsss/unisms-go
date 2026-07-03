// Package retry implements a small, dependency-free exponential backoff
// helper used by the UniSMS transport layer to retry failed requests.
package retry

import (
	"context"
	"time"
)

// Policy describes how retries should be attempted.
type Policy struct {
	// MaxRetries is the number of retries after the initial attempt.
	// A value of 0 disables retries (a single attempt is made).
	MaxRetries int

	// BaseDelay is the initial backoff delay. It doubles after each
	// failed attempt, capped at MaxDelay.
	BaseDelay time.Duration

	// MaxDelay caps the exponential backoff delay.
	MaxDelay time.Duration
}

// Delay returns the backoff delay to use before retry attempt number
// attempt (1-indexed: the delay before the first retry is Delay(1)).
func (p Policy) Delay(attempt int) time.Duration {
	if attempt < 1 {
		return 0
	}

	delay := p.BaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= p.MaxDelay {
			delay = p.MaxDelay
			break
		}
	}
	if delay > p.MaxDelay {
		delay = p.MaxDelay
	}
	return delay
}

// Sleep waits for the given duration or until ctx is done, whichever
// comes first. It returns ctx.Err() if the context was cancelled or its
// deadline was exceeded before the duration elapsed.
func Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
