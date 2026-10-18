package utils

import (
	"math"
	"testing"
	"time"
)

func TestLiveChatOffset(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	at := func(seconds float64) time.Time { return base.Add(time.Duration(seconds * float64(time.Second))) }
	chatStart := at(-10)
	// Run 1 captured 100s of video and ffmpeg gave up at 130s; the stream came back at 300s.
	runs := []LiveCaptureRun{
		{WallStart: at(0), WallEnd: at(130), VideoOffset: 0},
		{WallStart: at(300), WallEnd: at(500), VideoOffset: 100},
	}

	tests := []struct {
		name    string
		message time.Time
		runs    []LiveCaptureRun
		want    float64
		keep    bool
	}{
		{name: "without runs the offset is from chat start", message: at(42), runs: nil, want: 52, keep: true},
		{name: "first run", message: at(42), runs: runs, want: 42, keep: true},
		{name: "sent while the stream was down lands on the resume point", message: at(200), runs: runs, want: 100, keep: true},
		{name: "resumed run skips the gap", message: at(350), runs: runs, want: 150, keep: true},
		{name: "before the first run stays negative", message: at(-5), runs: runs, want: -5, keep: true},
		{name: "after the capture ended is dropped", message: at(501), runs: runs, keep: false},
		{name: "unfinished last run keeps later messages", message: at(900), runs: runs[:1:1], want: 100, keep: true},
	}
	// A crashed worker never records WallEnd on its last run.
	tests[len(tests)-1].runs = []LiveCaptureRun{{WallStart: at(800), VideoOffset: 0}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, keep := liveChatOffset(tt.message, chatStart, tt.runs)
			if keep != tt.keep {
				t.Fatalf("keep = %v, want %v", keep, tt.keep)
			}
			if keep && math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("offset = %v, want %v", got, tt.want)
			}
		})
	}
}
