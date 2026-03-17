package retry

import (
	"context"
	"math"
	"time"
)

type Config struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

func DefaultConfig() Config {
	return Config{
		MaxAttempts: 10,
		InitialWait: 1 * time.Second,
		MaxWait:     512 * time.Second,
		Multiplier:  2.0,
	}
}

func Do(ctx context.Context, cfg Config, fn func(ctx context.Context) error) error {
	if cfg.MaxAttempts <= 0 {
		return nil
	}

	var lastErr error
	wait := cfg.InitialWait

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return lastErr
			}
			return err
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		if attempt < cfg.MaxAttempts-1 {
			select {
			case <-ctx.Done():
				return lastErr
			case <-time.After(wait):
			}

			next := time.Duration(float64(wait) * cfg.Multiplier)
			if cfg.MaxWait > 0 && next > cfg.MaxWait {
				next = cfg.MaxWait
			}
			wait = next
		}
	}

	return lastErr
}

func DoWithBackoff(ctx context.Context, maxAttempts int, fn func(ctx context.Context) error) error {
	return Do(ctx, Config{
		MaxAttempts: maxAttempts,
		InitialWait: 1 * time.Second,
		MaxWait:     time.Duration(math.Pow(2, float64(maxAttempts))) * time.Second,
		Multiplier:  2.0,
	}, fn)
}
