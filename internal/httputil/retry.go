// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Package httputil provides HTTP helpers shared across stages.
package httputil

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// RetryBaseDelay controls the base duration for exponential backoff on
// HTTP 429 responses. Tests override this to avoid real sleeps.
var RetryBaseDelay = 10 * time.Second

const defaultMaxRetries = 5

// DoWithRetry executes an HTTP request and retries on HTTP 429 (Too Many
// Requests) with exponential backoff. The delay starts at RetryBaseDelay
// (10 s) and doubles each attempt: 10 s, 20 s, 40 s, 80 s, 160 s.
//
// When maxRetries is 0 the default (5) is used. On each 429 the response
// body is drained and closed before sleeping. If the context is cancelled
// during a backoff wait the function returns ctx.Err(). After exhausting
// retries the last 429 response is returned so the caller can inspect it.
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, maxRetries int) (*http.Response, error) {
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}

	for attempt := 0; ; attempt++ {
		resp, err := client.Do(req.Clone(ctx))
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		// Exhausted retries — return the 429 response as-is.
		if attempt >= maxRetries {
			return resp, nil
		}

		// Drain and close the body before retrying.
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		backoff := time.Duration(math.Pow(2, float64(attempt))) * RetryBaseDelay
		fmt.Fprintf(io.Discard, "rate limited, retrying in %v (attempt %d/%d)\n", backoff, attempt+1, maxRetries)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
}
