package exec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	osExec "os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zibbp/ganymede/ent"
	"github.com/zibbp/ganymede/internal/hls"
)

func runMediaCommand(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := osExec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

// A capture resumed after a drop appends to the same HLS playlist; packing it must give
// parts that play back as one continuous video.
func TestLiveHLSCaptureResumesAndPacksIntoPlayableParts(t *testing.T) {
	if _, err := osExec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	runMediaCommand(t, "ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=30", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
		"-t", "25", "-map", "0:v", "-map", "1:a", "-c:v", "libx265", "-g", "60", "-c:a", "aac",
		"-f", "mpegts", filepath.Join(dir, "source.ts"))
	server := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer server.Close()

	hlsDir := filepath.Join(dir, "123_u-video_hls0")
	if err := os.Mkdir(hlsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	video := ent.Vod{ExtID: "123", TmpVideoHlsPath: hlsDir, TmpVideoDownloadPath: filepath.Join(hlsDir, "123-video.m3u8"), VideoHlsPath: "/videos/123-video_hls"}
	for range 2 {
		resumeAt := 0.0
		if _, err := os.Stat(video.TmpVideoDownloadPath); err == nil {
			if resumeAt, err = hls.MediaPlaylistDuration(video.TmpVideoDownloadPath); err != nil {
				t.Fatal(err)
			}
		}
		runMediaCommand(t, "ffmpeg", liveCaptureArgs(server.URL+"/source.ts", false, nil, false, resumeAt, video)...)
	}

	captured, err := os.ReadFile(video.TmpVideoDownloadPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(captured), "#EXT-X-DISCONTINUITY"); got != 2 {
		t.Fatalf("want one discontinuity per run, got %d:\n%s", got, captured)
	}
	if strings.Contains(string(captured), "#EXT-X-ENDLIST") {
		t.Fatalf("capture playlist must stay open for resumed runs:\n%s", captured)
	}

	finalDir := filepath.Join(dir, "final")
	if err := os.Mkdir(finalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Each 25s run is captured as 10s+10s+5s segments; 12s parts hold one segment each.
	if err := hls.PackMediaPlaylist(context.Background(), video.TmpVideoDownloadPath, finalDir, "123-video.m3u8", 12*time.Second); err != nil {
		t.Fatal(err)
	}
	parts, _ := filepath.Glob(filepath.Join(finalDir, "123-video-part*.ts"))
	if len(parts) != 6 {
		t.Fatalf("want 3 parts per run, got %v", parts)
	}

	packed := filepath.Join(finalDir, "123-video.m3u8")
	durationText := runMediaCommand(t, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", packed)
	duration, err := strconv.ParseFloat(strings.TrimSpace(durationText), 64)
	if err != nil || duration < 49.5 || duration > 50.5 {
		t.Fatalf("packed playlist duration = %q, want about 50s", durationText)
	}
	if decodeErrors := runMediaCommand(t, "ffmpeg", "-v", "error", "-i", packed, "-f", "null", "-"); decodeErrors != "" {
		t.Fatalf("decoding the packed playlist reported errors:\n%s", decodeErrors)
	}
	for _, part := range parts {
		runMediaCommand(t, "ffprobe", "-v", "error", part) // each part plays on its own
	}
}
