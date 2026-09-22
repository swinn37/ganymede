package hls

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type mediaSegment struct {
	uri           string
	duration      float64
	discontinuity bool
}

// readMediaSegments parses the segments of an ffmpeg-written media playlist. It has no size
// limit: a two day capture lists tens of thousands of segments.
func readMediaSegments(path string) ([]mediaSegment, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck

	var segments []mediaSegment
	var pending mediaSegment
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "#EXT-X-DISCONTINUITY":
			pending.discontinuity = true
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
			segments = append(segments, pending)
			pending = mediaSegment{}
		}
	}
	return segments, scanner.Err()
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

// PackMediaPlaylist concatenates the segments of the media playlist src into .ts part files
// in dstDir and writes a byte-range VOD playlist named name next to them. A part ends at
// each discontinuity (a resumed capture) and, when maxPart > 0, before it would exceed
// maxPart. Parts are overwritten, and the playlist is written last, so its presence means
// packing completed and an interrupted run can simply be repeated.
func PackMediaPlaylist(ctx context.Context, src, dstDir, name string, maxPart time.Duration) error {
	segments, err := readMediaSegments(src)
	if err != nil {
		return err
	}

	partPrefix := strings.TrimSuffix(name, filepath.Ext(name))
	var body strings.Builder
	var part *os.File
	var partName string
	var partNumber int
	var partOffset int64
	var partDuration, targetDuration float64
	missingBefore := false

	closePart := func() error {
		if part == nil {
			return nil
		}
		err := errors.Join(part.Sync(), part.Close())
		part = nil
		return err
	}
	defer closePart() //nolint:errcheck

	for _, segment := range segments {
		if err := ctx.Err(); err != nil {
			return err
		}

		segmentPath := segment.uri
		if !filepath.IsAbs(segmentPath) {
			segmentPath = filepath.Join(filepath.Dir(src), segmentPath)
		}
		input, err := os.Open(segmentPath)
		if errors.Is(err, os.ErrNotExist) {
			// Timestamps jump over the hole, so the next segment starts a new part.
			log.Warn().Str("segment", segmentPath).Msg("hls segment listed in playlist is missing; skipping")
			missingBefore = true
			continue
		}
		if err != nil {
			return err
		}

		discontinuity := segment.discontinuity || missingBefore
		missingBefore = false
		exceedsMaxPart := maxPart > 0 && partDuration > 0 && partDuration+segment.duration > maxPart.Seconds()
		if part == nil || discontinuity || exceedsMaxPart {
			if err := closePart(); err != nil {
				return errors.Join(err, input.Close())
			}
			partNumber++
			partName = fmt.Sprintf("%s-part%03d.ts", partPrefix, partNumber)
			part, err = os.Create(filepath.Join(dstDir, partName))
			if err != nil {
				return errors.Join(err, input.Close())
			}
			partOffset, partDuration = 0, 0
		}

		size, err := io.Copy(part, input)
		if err := errors.Join(err, input.Close()); err != nil {
			return fmt.Errorf("copy %s into %s: %w", segmentPath, partName, err)
		}

		if discontinuity && body.Len() > 0 {
			body.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		fmt.Fprintf(&body, "#EXTINF:%.6f,\n#EXT-X-BYTERANGE:%d@%d\n%s\n", segment.duration, size, partOffset, partName)
		partOffset += size
		partDuration += segment.duration
		targetDuration = math.Max(targetDuration, segment.duration)
	}

	if err := closePart(); err != nil {
		return err
	}
	if partNumber == 0 {
		return fmt.Errorf("no segments to pack in %s", src)
	}

	playlist := fmt.Sprintf("#EXTM3U\n#EXT-X-VERSION:4\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-INDEPENDENT-SEGMENTS\n%s#EXT-X-ENDLIST\n",
		int(math.Ceil(targetDuration)), body.String())
	return writeFileAtomic(filepath.Join(dstDir, name), []byte(playlist))
}
