package tasks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type captureAttempt struct {
	progressed bool
	err        error
}

// runCaptureLoop drives captureWithReconnect with a fake clock; attempts past the scripted
// ones find the stream offline.
func runCaptureLoop(ctx context.Context, attempts []captureAttempt, grace time.Duration) (int, error) {
	errOffline := errors.New("stream offline")
	clock := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	calls := 0
	now := func() time.Time { return clock }
	sleep := func(ctx context.Context, d time.Duration) error {
		clock = clock.Add(d)
		return ctx.Err()
	}
	capture := func(ctx context.Context) (bool, error) {
		attempt := captureAttempt{err: errOffline}
		if calls < len(attempts) {
			attempt = attempts[calls]
		}
		calls++
		if attempt.progressed {
			clock = clock.Add(time.Hour)
		}
		return attempt.progressed, attempt.err
	}
	err := captureWithReconnect(ctx, grace, 20*time.Second, now, sleep, capture)
	return calls, err
}

func TestCaptureWithReconnectResumesUntilStreamStaysDown(t *testing.T) {
	t.Parallel()

	calls, err := runCaptureLoop(context.Background(), []captureAttempt{
		{progressed: true, err: errors.New("ffmpeg exited")}, // stream dropped
		{},                 // immediate retry: still down
		{},                 // 20s later: still down
		{progressed: true}, // back 40s later, runs until it ends
	}, 5*time.Minute)

	require.NoError(t, err)
	// After the last run: an immediate retry, then one every 20s until 5 minutes passed.
	require.Equal(t, 4+16, calls)
}

func TestCaptureWithReconnectKeepsRetryingWhenTheFirstAttemptFails(t *testing.T) {
	t.Parallel()

	// The stream is announced before its playlist is available.
	calls, err := runCaptureLoop(context.Background(), []captureAttempt{{}, {progressed: true}}, time.Minute)
	require.NoError(t, err)
	require.Equal(t, 2+4, calls)

	calls, err = runCaptureLoop(context.Background(), nil, time.Minute)
	require.EqualError(t, err, "stream offline")
	require.Equal(t, 4, calls)
}

func TestCaptureWithReconnectStopsOnCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls, err := runCaptureLoop(ctx, []captureAttempt{{progressed: true}}, time.Hour)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls)
}

func TestCancelWhenPlaylistStalls(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "123-video.m3u8")
	require.NoError(t, os.WriteFile(path, []byte("#EXTM3U\n"), 0o644))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		cancelWhenPlaylistStalls(ctx, cancel, path, 300*time.Millisecond)
		close(done)
	}()

	// A playlist that keeps growing keeps the capture running.
	for range 10 {
		time.Sleep(60 * time.Millisecond)
		modified := time.Now()
		require.NoError(t, os.Chtimes(path, modified, modified))
	}
	require.NoError(t, ctx.Err())

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stalled capture was not cancelled")
	}
	require.ErrorIs(t, ctx.Err(), context.Canceled)
}
