package hls

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// A live capture resumed after its stream dropped writes each run to its own playlist, init
// segment and segments, since ffmpeg keeps a single EXT-X-MAP when it appends to a playlist.
// Run 0 keeps the regular capture names. Once a second run starts, the capture playlist
// stitches the runs together, with a discontinuity and the run's EXT-X-MAP at each run.

// CaptureRunFiles returns the playlist, init segment and segment pattern ffmpeg writes for a
// capture run, relative to the capture directory.
func CaptureRunFiles(id string, run int) (playlist, init, segmentPattern string) {
	if run == 0 {
		return id + "-video.m3u8", id + "_init.mp4", id + "_segment%06d.m4s"
	}
	return fmt.Sprintf("%s-run%d.m3u8", id, run), fmt.Sprintf("%s_init_run%d.mp4", id, run), fmt.Sprintf("%s_run%d_segment%%06d.m4s", id, run)
}

// firstRunCopy is where run 0's playlist is kept once the capture playlist stitches runs.
func firstRunCopy(id string) string {
	return id + "-run0.m3u8"
}

type mediaSegment struct {
	uri           string
	duration      float64
	discontinuity bool
	mapURI        string
}

// readMediaSegments parses the segments of an ffmpeg-written media playlist, with the
// EXT-X-MAP each one uses. It has no size limit: a two day capture lists tens of thousands of
// segments.
func readMediaSegments(path string) ([]mediaSegment, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck

	var segments []mediaSegment
	var pending mediaSegment
	mapURI := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "#EXT-X-DISCONTINUITY":
			pending.discontinuity = true
		case strings.HasPrefix(line, "#EXT-X-MAP:"):
			mapURI = hlsAttribute(line, "URI")
		case strings.HasPrefix(line, "#EXTINF:"):
			value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
			duration, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid EXTINF %q in %s: %w", line, path, err)
			}
			pending.duration = duration
		case line == "" || strings.HasPrefix(line, "#"):
		default:
			pending.uri = line
			pending.mapURI = mapURI
			segments = append(segments, pending)
			pending = mediaSegment{}
		}
	}
	return segments, scanner.Err()
}

// hlsAttribute returns the value of a tag attribute, unquoted.
func hlsAttribute(line, name string) string {
	_, attrs, _ := strings.Cut(line, ":")
	for _, attr := range strings.Split(attrs, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(attr), "=")
		if ok && key == name {
			return strings.Trim(value, `"`)
		}
	}
	return ""
}

// MediaPlaylistDuration returns the total EXTINF duration of a media playlist in seconds.
func MediaPlaylistDuration(path string) (float64, error) {
	segments, err := readMediaSegments(path)
	if err != nil {
		return 0, err
	}
	var total float64
	for _, segment := range segments {
		total += segment.duration
	}
	return total, nil
}

// KeepFirstCaptureRun copies run 0's playlist aside before the capture playlist starts
// stitching runs over it. It does nothing once the copy exists.
func KeepFirstCaptureRun(dir, id string) error {
	copyPath := filepath.Join(dir, firstRunCopy(id))
	if fileExists(copyPath) {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(dir, id+"-video.m3u8"))
	if err != nil {
		return err
	}
	return writeFileAtomicCreate(copyPath, data)
}

// StitchCaptureRuns writes the capture playlist of runs 0 to runs-1 and returns its duration in
// seconds. A run whose playlist does not exist yet adds nothing. The playlist stays an open
// EVENT playlist: post-processing finalizes it.
func StitchCaptureRuns(dir, id string, runs int) (float64, error) {
	var body strings.Builder
	var total, targetDuration float64
	for run := 0; run < runs; run++ {
		name, init, _ := CaptureRunFiles(id, run)
		if run == 0 {
			name = firstRunCopy(id)
		}
		segments, err := readMediaSegments(filepath.Join(dir, name))
		if os.IsNotExist(err) && run > 0 {
			continue
		}
		if err != nil {
			return 0, err
		}
		for i, segment := range segments {
			if i == 0 || segment.mapURI != segments[i-1].mapURI {
				// ffmpeg lists the init next to the playlist; the runs share one directory
				mapURI := segment.mapURI
				if mapURI == "" {
					mapURI = init
				}
				if body.Len() > 0 {
					body.WriteString("#EXT-X-DISCONTINUITY\n")
				}
				fmt.Fprintf(&body, "#EXT-X-MAP:URI=%q\n", URI(mapURI))
			} else if segment.discontinuity {
				body.WriteString("#EXT-X-DISCONTINUITY\n")
			}
			fmt.Fprintf(&body, "#EXTINF:%.6f,\n%s\n", segment.duration, segment.uri)
			total += segment.duration
			targetDuration = math.Max(targetDuration, segment.duration)
		}
	}

	playlist := fmt.Sprintf("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-INDEPENDENT-SEGMENTS\n%s",
		max(1, int(math.Ceil(targetDuration))), body.String())
	return total, writeFileAtomicCreate(filepath.Join(dir, id+"-video.m3u8"), []byte(playlist))
}

// RemoveCaptureRunPlaylists deletes the per-run playlists once the capture playlist stitches
// all runs for good.
func RemoveCaptureRunPlaylists(dir, id string) error {
	matches, err := filepath.Glob(filepath.Join(dir, id+"-run*.m3u8"))
	if err != nil {
		return err
	}
	for _, match := range matches {
		if err := os.Remove(match); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
