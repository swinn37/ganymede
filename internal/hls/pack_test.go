package hls

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A capture stitched from two runs, as resumed after a drop, with fake init and segment bytes.
const stitchedCapture = `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-MAP:URI="123_init.mp4"
#EXTINF:10.000000,
123_segment000000.m4s
#EXTINF:10.000000,
123_segment000001.m4s
#EXTINF:9.500000,
123_segment000002.m4s
#EXT-X-DISCONTINUITY
#EXT-X-MAP:URI="123_init_run1.mp4"
#EXTINF:10.000000,
123_run1_segment000000.m4s
#EXTINF:4.250000,
123_run1_segment000001.m4s
#EXT-X-ENDLIST
`

var stitchedFiles = map[string]string{
	"123_init.mp4":               "I",
	"123_segment000000.m4s":      "aaaa",
	"123_segment000001.m4s":      "bbbbbb",
	"123_segment000002.m4s":      "cc",
	"123_init_run1.mp4":          "JJ",
	"123_run1_segment000000.m4s": "ddd",
	"123_run1_segment000001.m4s": "e",
}

// writeCapture writes the files and the playlist listing them, and returns the playlist path.
func writeCapture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	path := filepath.Join(dir, "123-video.m3u8")
	require.NoError(t, os.WriteFile(path, []byte(stitchedCapture), 0o644))
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	byts, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(byts)
}

func TestPackMediaPlaylistSplitsAtMaxPartAndRuns(t *testing.T) {
	t.Parallel()
	src := writeCapture(t, stitchedFiles)
	dst := t.TempDir()

	require.NoError(t, PackMediaPlaylist(context.Background(), src, dst, "123-video.m3u8", 25*time.Second))

	// 10+10 fits in 25s, the 9.5s segment starts part 2, the resumed run starts part 3; each
	// part starts with its run's init segment.
	require.Equal(t, "Iaaaabbbbbb", readFile(t, filepath.Join(dst, "123-video-part001.mp4")))
	require.Equal(t, "Icc", readFile(t, filepath.Join(dst, "123-video-part002.mp4")))
	require.Equal(t, "JJddde", readFile(t, filepath.Join(dst, "123-video-part003.mp4")))
	require.Equal(t, `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-MAP:URI="123-video-part001.mp4",BYTERANGE="1@0"
#EXTINF:10.000000,
#EXT-X-BYTERANGE:4@1
123-video-part001.mp4
#EXTINF:10.000000,
#EXT-X-BYTERANGE:6@5
123-video-part001.mp4
#EXT-X-MAP:URI="123-video-part002.mp4",BYTERANGE="1@0"
#EXTINF:9.500000,
#EXT-X-BYTERANGE:2@1
123-video-part002.mp4
#EXT-X-DISCONTINUITY
#EXT-X-MAP:URI="123-video-part003.mp4",BYTERANGE="2@0"
#EXTINF:10.000000,
#EXT-X-BYTERANGE:3@2
123-video-part003.mp4
#EXTINF:4.250000,
#EXT-X-BYTERANGE:1@5
123-video-part003.mp4
#EXT-X-ENDLIST
`, readFile(t, filepath.Join(dst, "123-video.m3u8")))
	require.NoError(t, ValidateMediaPlaylistFiles(filepath.Join(dst, "123-video.m3u8")))
}

func TestPackMediaPlaylistWithoutMaxPartWritesOnePartPerRun(t *testing.T) {
	t.Parallel()
	src := writeCapture(t, stitchedFiles)
	dst := t.TempDir()

	require.NoError(t, PackMediaPlaylist(context.Background(), src, dst, "123-video.m3u8", 0))

	require.Equal(t, "Iaaaabbbbbbcc", readFile(t, filepath.Join(dst, "123-video-part001.mp4")))
	require.Equal(t, "JJddde", readFile(t, filepath.Join(dst, "123-video-part002.mp4")))
	require.NoFileExists(t, filepath.Join(dst, "123-video-part003.mp4"))
}

func TestPackMediaPlaylistSkipsMissingSegmentWithDiscontinuity(t *testing.T) {
	t.Parallel()
	files := map[string]string{}
	for name, content := range stitchedFiles {
		files[name] = content
	}
	delete(files, "123_segment000001.m4s")
	src := writeCapture(t, files)
	dst := t.TempDir()

	require.NoError(t, PackMediaPlaylist(context.Background(), src, dst, "123-video.m3u8", 0))

	require.Equal(t, "Iaaaa", readFile(t, filepath.Join(dst, "123-video-part001.mp4")))
	require.Equal(t, "Icc", readFile(t, filepath.Join(dst, "123-video-part002.mp4")))
	require.Contains(t, readFile(t, filepath.Join(dst, "123-video.m3u8")), `123-video-part001.mp4
#EXT-X-DISCONTINUITY
#EXT-X-MAP:URI="123-video-part002.mp4",BYTERANGE="1@0"
#EXTINF:9.500000,
`)
}

func TestPackMediaPlaylistRerunOverwritesPartialParts(t *testing.T) {
	t.Parallel()
	src := writeCapture(t, stitchedFiles)
	dst := t.TempDir()
	// Leftover from a pack interrupted before the playlist was written.
	require.NoError(t, os.WriteFile(filepath.Join(dst, "123-video-part001.mp4"), []byte("Iaaaabbbbbbccstale"), 0o644))

	require.NoError(t, PackMediaPlaylist(context.Background(), src, dst, "123-video.m3u8", 0))

	require.Equal(t, "Iaaaabbbbbbcc", readFile(t, filepath.Join(dst, "123-video-part001.mp4")))
}

func TestPackMediaPlaylistFailsWithoutSegments(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "123-video.m3u8")
	require.NoError(t, os.WriteFile(src, []byte("#EXTM3U\n#EXT-X-TARGETDURATION:10\n"), 0o644))
	dst := t.TempDir()

	require.Error(t, PackMediaPlaylist(context.Background(), src, dst, "123-video.m3u8", 0))
	require.NoFileExists(t, filepath.Join(dst, "123-video.m3u8"))
}
