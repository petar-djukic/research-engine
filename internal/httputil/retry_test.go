// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package httputil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Use a tiny base delay so tests finish quickly.
	RetryBaseDelay = 1 * time.Millisecond
}

func TestDoWithRetry_ImmediateSuccess(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	require.NoError(t, err)

	resp, err := DoWithRetry(context.Background(), ts.Client(), req, 5)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestDoWithRetry_RetriesThen200(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	require.NoError(t, err)

	resp, err := DoWithRetry(context.Background(), ts.Client(), req, 5)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

func TestDoWithRetry_ExhaustsRetries(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	require.NoError(t, err)

	maxRetries := 3
	resp, err := DoWithRetry(context.Background(), ts.Client(), req, maxRetries)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	// 1 initial + 3 retries = 4 total calls.
	assert.Equal(t, int32(4), atomic.LoadInt32(&calls))
}

func TestDoWithRetry_ContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	// Use a longer base delay so the context cancels during the wait.
	old := RetryBaseDelay
	RetryBaseDelay = 500 * time.Millisecond
	defer func() { RetryBaseDelay = old }()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	require.NoError(t, err)

	_, err = DoWithRetry(ctx, ts.Client(), req, 5)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestDoWithRetry_DefaultMaxRetries(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	require.NoError(t, err)

	resp, err := DoWithRetry(context.Background(), ts.Client(), req, 0)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	// 1 initial + 5 default retries = 6 total calls.
	assert.Equal(t, int32(6), atomic.LoadInt32(&calls))
}

func TestDoWithRetry_Non429ErrorPassesThrough(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	require.NoError(t, err)

	resp, err := DoWithRetry(context.Background(), ts.Client(), req, 5)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}
