package vod

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/zibbp/ganymede/ent"
	"github.com/zibbp/ganymede/internal/hls"
)

// LivePreviewAvailable checks the capture output, independent of settings changed mid-recording.
func LivePreviewAvailable(video *ent.Vod) bool {
	captureID := hls.LiveCaptureID(video.ExtID, video.ExtStreamID, video.TmpVideoDownloadPath)
	if !video.Status.Active() || video.TmpVideoHlsPath == "" || captureID == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(video.TmpVideoHlsPath, captureID+"-video.m3u8"))
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

// ArchivedVideoAvailable reports whether an archive still processing its chat already has its
// video in place, complete. Viewers of the live preview switch to it before the move removes
// the temporary HLS.
func ArchivedVideoAvailable(video *ent.Vod) bool {
	if !video.Status.Active() || !nonEmptyFile(video.VideoPath) {
		return false
	}
	switch {
	case video.VideoHlsPath == "":
		// MP4: complete once moved out of the temporary export
		return video.TmpVideoConvertPath == "" || !exists(video.TmpVideoConvertPath)
	case nonEmptyFile(strings.TrimSuffix(video.VideoPath, filepath.Ext(video.VideoPath)) + "-part001.mp4"):
		// Parts: the playlist is written once all the parts are
		return true
	default:
		// HLS moved as a whole directory: complete once the temporary HLS is gone
		return video.TmpVideoHlsPath == "" || !exists(video.TmpVideoHlsPath)
	}
}

func nonEmptyFile(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
