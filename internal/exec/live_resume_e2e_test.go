package exec

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	osExec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zibbp/ganymede/ent"
	"github.com/zibbp/ganymede/internal/config"
	"github.com/zibbp/ganymede/internal/hls"
)

// TestLiveFmp4ResumedCaptureE2E records a stream in two runs, as when it drops and comes back,
// with real ffmpeg/ffprobe, and checks the stitched capture finalizes, probes and exports as one
// video.
func TestLiveFmp4ResumedCaptureE2E(t *testing.T) {
	// Not parallel: sets process environment for config.
	tmpDir := t.TempDir()
	for _, dir := range []string{"logs", "config", "cap_hls0"} {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0o755); err != nil {
			t.Fatalf("create %s dir: %v", dir, err)
		}
	}
	t.Setenv("TWITCH_CLIENT_ID", "e2e-test")
	t.Setenv("TWITCH_CLIENT_SECRET", "e2e-test")
	t.Setenv("LOGS_DIR", filepath.Join(tmpDir, "logs"))
	t.Setenv("CONFIG_DIR", filepath.Join(tmpDir, "config"))
	if _, err := config.Init(); err != nil {
		t.Fatalf("init config: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	run := func(name string, args ...string) {
		t.Helper()
		if out, err := osExec.CommandContext(ctx, name, args...).CombinedOutput(); err != nil {
			t.Fatalf("run %s %v: %v\n%s", name, args, err, out)
		}
	}

	// Two 8 s H.264/AAC HLS sources with their own encoders, like a stream before and after a drop.
	for i, source := range []string{"first", "second"} {
		mp4 := filepath.Join(tmpDir, source+".mp4")
		run("ffmpeg", "-y", "-hide_banner",
			"-f", "lavfi", "-i", "testsrc=size=320x240:rate=15:duration=8",
			"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=%d:duration=8", 440*(i+1)),
			"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-shortest", mp4)
		dir := filepath.Join(tmpDir, source)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create source dir: %v", err)
		}
		run("ffmpeg", "-y", "-hide_banner", "-i", mp4, "-c", "copy",
			"-start_number", "0", "-hls_time", "4", "-hls_list_size", "0", "-hls_playlist_type", "vod",
			"-hls_segment_filename", filepath.Join(dir, "seg%03d.ts"), "-f", "hls", filepath.Join(dir, "src.m3u8"))
	}
	server := httptest.NewServer(http.FileServer(http.Dir(tmpDir)))
	defer server.Close()

	capDir := filepath.Join(tmpDir, "cap_hls0")
	const id = "e2e456"
	capture := func(source string, resume LiveCaptureResume) {
		t.Helper()
		playlist, init, segments := hls.CaptureRunFiles(id, resume.Run)
		run("ffmpeg", buildLiveHlsCaptureFFmpegArgs(server.URL+"/"+source+"/src.m3u8",
			filepath.Join(capDir, playlist), filepath.Join(capDir, segments), init, false, "", false, resume)...)
	}
	playlistPath := filepath.Join(capDir, id+"-video.m3u8")

	capture("first", LiveCaptureResume{KeepOpen: true})
	first, err := os.ReadFile(playlistPath)
	if err != nil {
		t.Fatalf("read first run playlist: %v", err)
	}
	if strings.Contains(string(first), "#EXT-X-ENDLIST") {
		t.Fatalf("a run that may be resumed must leave its playlist open:\n%s", first)
	}

	// The capture task's sequence: keep run 0, stitch, record run 1 after it, stitch again.
	if err := hls.KeepFirstCaptureRun(capDir, id); err != nil {
		t.Fatalf("keep first run: %v", err)
	}
	captured, err := hls.StitchCaptureRuns(capDir, id, 1)
	if err != nil {
		t.Fatalf("stitch first run: %v", err)
	}
	capture("second", LiveCaptureResume{Run: 1, TSOffset: captured + 0.5, KeepOpen: true})
	total, err := hls.StitchCaptureRuns(capDir, id, 2)
	if err != nil {
		t.Fatalf("stitch runs: %v", err)
	}
	if total < 15 || total > 17 {
		t.Fatalf("stitched duration %.1fs, want about 16s", total)
	}
	if err := hls.RemoveCaptureRunPlaylists(capDir, id); err != nil {
		t.Fatalf("remove run playlists: %v", err)
	}
	stitched, err := os.ReadFile(playlistPath)
	if err != nil {
		t.Fatalf("read stitched playlist: %v", err)
	}
	if strings.Count(string(stitched), "#EXT-X-MAP") != 2 || strings.Count(string(stitched), "#EXT-X-DISCONTINUITY") != 1 {
		t.Fatalf("stitched playlist should map each run after a discontinuity:\n%s", stitched)
	}

	// Post-process: finalize, probe and validate the capture as one video.
	if _, err := EnsureLiveHlsPlaylist(ctx, capDir, id); err != nil {
		t.Fatalf("finalize stitched playlist: %v", err)
	}
	if err := hls.ValidateMediaPlaylistFiles(playlistPath); err != nil {
		t.Fatalf("validate stitched playlist: %v", err)
	}
	duration, err := GetVideoDuration(ctx, playlistPath)
	if err != nil {
		t.Fatalf("probe stitched playlist: %v", err)
	}
	if duration < 16 || duration > 17 {
		t.Fatalf("probed duration %ds, want about 16.5s", duration)
	}

	// An MP4 archive is exported from the stitched capture.
	exportPath := filepath.Join(tmpDir, "export.mp4")
	if err := ExportHlsToMp4(ctx, ent.Vod{ID: uuid.New(), ExtID: id, ExtStreamID: id, Title: "e2e resumed", TmpVideoHlsPath: capDir}, exportPath); err != nil {
		t.Fatalf("export stitched capture to MP4: %v", err)
	}
	probe, err := ProbeMediaDuration(ctx, exportPath)
	if err != nil {
		t.Fatalf("probe exported MP4: %v", err)
	}
	// Both runs, with the half second gap between them
	if probe.HasTimestampAnomaly() || probe.FormatDuration < 16 || probe.FormatDuration > 17 {
		t.Fatalf("exported MP4: format=%f stream=%f, want about 16.5s without anomaly", probe.FormatDuration, probe.LongestStreamDuration)
	}
	run("ffmpeg", "-v", "error", "-i", exportPath, "-f", "null", "-")
}
