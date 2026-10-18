package hls

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// ffmpeg's playlist of a run, as written with append_list (leading discontinuity included).
const firstRunPlaylist = `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:EVENT
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-MAP:URI="123_init.mp4"
#EXT-X-DISCONTINUITY
#EXTINF:10.000000,
123_segment000000.m4s
#EXTINF:4.500000,
123_segment000001.m4s
`

const secondRunPlaylist = `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:EVENT
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-MAP:URI="123_init_run1.mp4"
#EXT-X-DISCONTINUITY
#EXTINF:10.000000,
123_run1_segment000000.m4s
`

func TestCaptureRunFiles(t *testing.T) {
	t.Parallel()

	playlist, init, segments := CaptureRunFiles("123", 0)
	require.Equal(t, []string{"123-video.m3u8", "123_init.mp4", "123_segment%06d.m4s"}, []string{playlist, init, segments})
	playlist, init, segments = CaptureRunFiles("123", 2)
	require.Equal(t, []string{"123-run2.m3u8", "123_init_run2.mp4", "123_run2_segment%06d.m4s"}, []string{playlist, init, segments})
}

func TestStitchCaptureRuns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "123-video.m3u8"), []byte(firstRunPlaylist), 0o644))
	require.NoError(t, KeepFirstCaptureRun(dir, "123"))

	// The second run has not written its playlist yet: the capture lists the first run alone.
	duration, err := StitchCaptureRuns(dir, "123", 2)
	require.NoError(t, err)
	require.InDelta(t, 14.5, duration, 1e-9)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "123-run1.m3u8"), []byte(secondRunPlaylist), 0o644))
	duration, err = StitchCaptureRuns(dir, "123", 2)
	require.NoError(t, err)
	require.InDelta(t, 24.5, duration, 1e-9)

	stitched, err := os.ReadFile(filepath.Join(dir, "123-video.m3u8"))
	require.NoError(t, err)
	require.Equal(t, `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:EVENT
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-MAP:URI="123_init.mp4"
#EXTINF:10.000000,
123_segment000000.m4s
#EXTINF:4.500000,
123_segment000001.m4s
#EXT-X-DISCONTINUITY
#EXT-X-MAP:URI="123_init_run1.mp4"
#EXTINF:10.000000,
123_run1_segment000000.m4s
`, string(stitched))

	// The first run's copy is kept as it was, even though the capture playlist changed.
	require.NoError(t, KeepFirstCaptureRun(dir, "123"))
	kept, err := os.ReadFile(filepath.Join(dir, "123-run0.m3u8"))
	require.NoError(t, err)
	require.Equal(t, firstRunPlaylist, string(kept))

	require.NoError(t, RemoveCaptureRunPlaylists(dir, "123"))
	matches, err := filepath.Glob(filepath.Join(dir, "*.m3u8"))
	require.NoError(t, err)
	require.Equal(t, []string{filepath.Join(dir, "123-video.m3u8")}, matches)
}
