package hls

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// writeCapture writes fake segment files and the media playlist listing them.
func writeCapture(t *testing.T, dir string, playlist string, segments map[string]string) string {
	t.Helper()
	for name, content := range segments {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	path := filepath.Join(dir, "123-video.m3u8")
	require.NoError(t, os.WriteFile(path, []byte(playlist), 0o644))
	return path
}

const captureWithReconnect = `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:EVENT
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-DISCONTINUITY
#EXTINF:10.000000,
123_1_segment000000.ts
#EXTINF:10.000000,
123_1_segment000001.ts
#EXTINF:9.500000,
123_1_segment000002.ts
#EXT-X-DISCONTINUITY
#EXTINF:10.000000,
123_2_segment000003.ts
#EXTINF:4.250000,
123_2_segment000004.ts
`

var captureSegments = map[string]string{
	"123_1_segment000000.ts": "aaaa",
	"123_1_segment000001.ts": "bbbbbb",
	"123_1_segment000002.ts": "cc",
	"123_2_segment000003.ts": "ddd",
	"123_2_segment000004.ts": "e",
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	byts, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(byts)
}

func TestPackMediaPlaylistSplitsAtMaxPartAndDiscontinuities(t *testing.T) {
	t.Parallel()
	src := writeCapture(t, t.TempDir(), captureWithReconnect, captureSegments)
	dst := t.TempDir()

	require.NoError(t, PackMediaPlaylist(context.Background(), src, dst, "123-video.m3u8", 25*time.Second))

	// 10+10 fits in 25s, the 9.5s segment starts part 2, the reconnect starts part 3.
	require.Equal(t, "aaaabbbbbb", readFile(t, filepath.Join(dst, "123-video-part001.ts")))
	require.Equal(t, "cc", readFile(t, filepath.Join(dst, "123-video-part002.ts")))
	require.Equal(t, "ddde", readFile(t, filepath.Join(dst, "123-video-part003.ts")))
	require.Equal(t, `#EXTM3U
#EXT-X-VERSION:4
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-INDEPENDENT-SEGMENTS
#EXTINF:10.000000,
#EXT-X-BYTERANGE:4@0
123-video-part001.ts
#EXTINF:10.000000,
#EXT-X-BYTERANGE:6@4
123-video-part001.ts
#EXTINF:9.500000,
#EXT-X-BYTERANGE:2@0
123-video-part002.ts
#EXT-X-DISCONTINUITY
#EXTINF:10.000000,
#EXT-X-BYTERANGE:3@0
123-video-part003.ts
#EXTINF:4.250000,
#EXT-X-BYTERANGE:1@3
123-video-part003.ts
#EXT-X-ENDLIST
`, readFile(t, filepath.Join(dst, "123-video.m3u8")))
}

func TestPackMediaPlaylistWithoutMaxPartWritesOnePartPerCapture(t *testing.T) {
	t.Parallel()
	src := writeCapture(t, t.TempDir(), captureWithReconnect, captureSegments)
	dst := t.TempDir()

	require.NoError(t, PackMediaPlaylist(context.Background(), src, dst, "123-video.m3u8", 0))

	require.Equal(t, "aaaabbbbbbcc", readFile(t, filepath.Join(dst, "123-video-part001.ts")))
	require.Equal(t, "ddde", readFile(t, filepath.Join(dst, "123-video-part002.ts")))
	require.NoFileExists(t, filepath.Join(dst, "123-video-part003.ts"))
}

func TestPackMediaPlaylistSkipsMissingSegmentWithDiscontinuity(t *testing.T) {
	t.Parallel()
	segments := map[string]string{
		"123_1_segment000000.ts": "aaaa",
		"123_1_segment000002.ts": "cc",
		"123_2_segment000003.ts": "ddd",
		"123_2_segment000004.ts": "e",
	}
	src := writeCapture(t, t.TempDir(), captureWithReconnect, segments)
	dst := t.TempDir()

	require.NoError(t, PackMediaPlaylist(context.Background(), src, dst, "123-video.m3u8", 0))

	require.Equal(t, "aaaa", readFile(t, filepath.Join(dst, "123-video-part001.ts")))
	require.Equal(t, "cc", readFile(t, filepath.Join(dst, "123-video-part002.ts")))
	require.Equal(t, "ddde", readFile(t, filepath.Join(dst, "123-video-part003.ts")))
	require.Contains(t, readFile(t, filepath.Join(dst, "123-video.m3u8")), `#EXT-X-BYTERANGE:4@0
123-video-part001.ts
#EXT-X-DISCONTINUITY
#EXTINF:9.500000,
#EXT-X-BYTERANGE:2@0
123-video-part002.ts
`)
}

func TestPackMediaPlaylistRerunOverwritesPartialParts(t *testing.T) {
	t.Parallel()
	src := writeCapture(t, t.TempDir(), captureWithReconnect, captureSegments)
	dst := t.TempDir()
	// Leftover from a pack interrupted before the playlist was written.
	require.NoError(t, os.WriteFile(filepath.Join(dst, "123-video-part001.ts"), []byte("aaaabbbbbbccstale"), 0o644))

	require.NoError(t, PackMediaPlaylist(context.Background(), src, dst, "123-video.m3u8", 0))

	require.Equal(t, "aaaabbbbbbcc", readFile(t, filepath.Join(dst, "123-video-part001.ts")))
}

func TestPackMediaPlaylistFailsWithoutSegments(t *testing.T) {
	t.Parallel()
	src := writeCapture(t, t.TempDir(), "#EXTM3U\n#EXT-X-TARGETDURATION:10\n", nil)
	dst := t.TempDir()

	require.Error(t, PackMediaPlaylist(context.Background(), src, dst, "123-video.m3u8", 0))
	require.NoFileExists(t, filepath.Join(dst, "123-video.m3u8"))
}

func TestMediaPlaylistDuration(t *testing.T) {
	t.Parallel()
	src := writeCapture(t, t.TempDir(), captureWithReconnect, nil)

	duration, err := MediaPlaylistDuration(src)
	require.NoError(t, err)
	require.InDelta(t, 43.75, duration, 1e-9)
}
