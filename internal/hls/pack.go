package hls

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// PackMediaPlaylist concatenates the fMP4 segments of the media playlist src into .mp4 part
// files in dstDir and writes a byte-range VOD playlist named name next to them. Each part
// starts with its init segment, so it plays on its own. A part ends at each discontinuity (a
// resumed capture), at a change of init segment, before a missing segment and, when
// maxPart > 0, before it would exceed maxPart. Parts are overwritten, and the playlist is
// written last, so its presence means packing completed and an interrupted run can simply
// be repeated.
func PackMediaPlaylist(ctx context.Context, src, dstDir, name string, maxPart time.Duration) error {
	segments, err := readMediaSegments(src)
	if err != nil {
		return err
	}
	resolve := func(uri string) string {
		if path, err := url.PathUnescape(uri); err == nil {
			uri = path
		}
		if filepath.IsAbs(uri) {
			return uri
		}
		return filepath.Join(filepath.Dir(src), filepath.FromSlash(uri))
	}

	partPrefix := strings.TrimSuffix(name, filepath.Ext(name))
	var body strings.Builder
	var part *os.File
	var partName, partMap string
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
		if segment.mapURI == "" {
			return fmt.Errorf("segment %s of %s has no init segment", segment.uri, src)
		}

		segmentPath := resolve(segment.uri)
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
		if part == nil || discontinuity || segment.mapURI != partMap || exceedsMaxPart {
			if err := closePart(); err != nil {
				return errors.Join(err, input.Close())
			}
			partNumber++
			partName = fmt.Sprintf("%s-part%03d.mp4", partPrefix, partNumber)
			part, err = os.Create(filepath.Join(dstDir, partName))
			if err != nil {
				return errors.Join(err, input.Close())
			}
			initSize, err := copyFileInto(part, resolve(segment.mapURI))
			if err != nil {
				return errors.Join(fmt.Errorf("copy init segment into %s: %w", partName, err), input.Close())
			}
			if discontinuity && body.Len() > 0 {
				body.WriteString("#EXT-X-DISCONTINUITY\n")
			}
			fmt.Fprintf(&body, "#EXT-X-MAP:URI=%q,BYTERANGE=\"%d@0\"\n", URI(partName), initSize)
			partMap, partOffset, partDuration = segment.mapURI, initSize, 0
		}

		size, err := io.Copy(part, input)
		if err := errors.Join(err, input.Close()); err != nil {
			return fmt.Errorf("copy %s into %s: %w", segmentPath, partName, err)
		}

		fmt.Fprintf(&body, "#EXTINF:%.6f,\n#EXT-X-BYTERANGE:%d@%d\n%s\n", segment.duration, size, partOffset, URI(partName))
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

	playlist := fmt.Sprintf("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-INDEPENDENT-SEGMENTS\n%s#EXT-X-ENDLIST\n",
		max(1, int(math.Ceil(targetDuration))), body.String())
	return writeFileAtomicCreate(filepath.Join(dstDir, name), []byte(playlist))
}

func copyFileInto(dst io.Writer, path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close() //nolint:errcheck
	return io.Copy(dst, file)
}
