package http

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/zibbp/ganymede/ent"
	"github.com/zibbp/ganymede/internal/utils"
)

type videoDetailsService struct {
	VodService
	video *ent.Vod
}

func (s videoDetailsService) GetVod(context.Context, uuid.UUID, bool, bool, bool, bool) (*ent.Vod, error) {
	return s.video, nil
}

func TestVideoDetailsLivePreviewAvailability(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status utils.ArchiveStatus
		output string
		want   bool
	}{
		{"missing playlist", utils.ArchiveRunning, "missing", false},
		{"empty playlist", utils.ArchiveRunning, "empty", false},
		{"directory instead of playlist", utils.ArchiveRunning, "directory", false},
		{"recording with playlist", utils.ArchiveRunning, "playlist", true},
		{"finalizing with playlist", utils.ArchiveFinalizing, "playlist", true},
		{"completed with leftover playlist", utils.ArchiveCompleted, "playlist", false},
		{"failed with leftover playlist", utils.ArchiveFailed, "playlist", false},
		{"no preview directory", utils.ArchiveRunning, "no-path", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			video := &ent.Vod{ID: uuid.New(), ExtID: "123", Type: utils.Live, Status: tc.status, TmpVideoHlsPath: t.TempDir()}
			path := filepath.Join(video.TmpVideoHlsPath, "123-video.m3u8")
			switch tc.output {
			case "playlist":
				require.NoError(t, os.WriteFile(path, []byte("#EXTM3U\n#EXTINF:10,\n123_segment000001.ts\n"), 0600))
			case "empty":
				require.NoError(t, os.WriteFile(path, nil, 0600))
			case "directory":
				require.NoError(t, os.Mkdir(path, 0700))
			case "no-path":
				video.TmpVideoHlsPath = ""
			}
			require.Equal(t, tc.want, requestVideoDetails(t, video)["live_preview_available"])
		})
	}
}

// The video of a live archive is reported once it is complete in its final place, while viewers
// may still play the temporary HLS: they switch to it before the move removes the preview.
func TestVideoDetailsVideoAvailability(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status utils.ArchiveStatus
		hls    bool     // final video saved as HLS
		files  []string // tmp (temporary HLS), convert (MP4 export), video, empty-video, part
		want   bool
	}{
		{"recording", utils.ArchiveRunning, false, []string{"tmp"}, false},
		{"MP4 exported, not moved yet", utils.ArchiveFinalizing, false, []string{"tmp", "convert"}, false},
		{"MP4 being copied into place", utils.ArchiveFinalizing, false, []string{"tmp", "convert", "video"}, false},
		{"MP4 moved, preview kept for viewers", utils.ArchiveFinalizing, false, []string{"tmp", "video"}, true},
		{"MP4 moved, preview removed", utils.ArchiveFinalizing, false, []string{"video"}, true},
		{"empty MP4", utils.ArchiveFinalizing, false, []string{"empty-video"}, false},
		{"parts being packed", utils.ArchiveFinalizing, true, []string{"tmp", "part"}, false},
		{"parts packed, preview kept for viewers", utils.ArchiveFinalizing, true, []string{"tmp", "part", "video"}, true},
		{"HLS directory being copied", utils.ArchiveFinalizing, true, []string{"tmp", "video"}, false},
		{"HLS directory moved", utils.ArchiveFinalizing, true, []string{"video"}, true},
		{"completed", utils.ArchiveCompleted, false, []string{"video"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			video := &ent.Vod{ID: uuid.New(), ExtID: "123", Type: utils.Live, Status: tc.status,
				TmpVideoHlsPath: filepath.Join(dir, "tmp_hls"), TmpVideoConvertPath: filepath.Join(dir, "convert.mp4"),
				VideoPath: filepath.Join(dir, "123-video.mp4")}
			if tc.hls {
				video.VideoHlsPath = filepath.Join(dir, "123-video_hls")
				video.VideoPath = filepath.Join(video.VideoHlsPath, "123-video.m3u8")
				require.NoError(t, os.Mkdir(video.VideoHlsPath, 0700))
			}
			for _, file := range tc.files {
				switch file {
				case "tmp":
					require.NoError(t, os.Mkdir(video.TmpVideoHlsPath, 0700))
				case "convert":
					require.NoError(t, os.WriteFile(video.TmpVideoConvertPath, []byte("mp4"), 0600))
				case "video":
					require.NoError(t, os.WriteFile(video.VideoPath, []byte("video"), 0600))
				case "empty-video":
					require.NoError(t, os.WriteFile(video.VideoPath, nil, 0600))
				case "part":
					require.NoError(t, os.WriteFile(filepath.Join(video.VideoHlsPath, "123-video-part001.mp4"), []byte("part"), 0600))
				}
			}
			require.Equal(t, tc.want, requestVideoDetails(t, video)["video_available"])
		})
	}
}

func TestVideoDetailsDetectsNewLivePreview(t *testing.T) {
	t.Parallel()
	video := &ent.Vod{ID: uuid.New(), ExtID: "123", Type: utils.Live, Status: utils.ArchiveRunning, TmpVideoHlsPath: t.TempDir()}
	require.Equal(t, false, requestVideoDetails(t, video)["live_preview_available"])
	path := filepath.Join(video.TmpVideoHlsPath, "123-video.m3u8")
	require.NoError(t, os.WriteFile(path, []byte("#EXTM3U\n#EXTINF:10,\n123_segment000001.ts\n"), 0600))
	require.Equal(t, true, requestVideoDetails(t, video)["live_preview_available"])
	require.NoError(t, os.Remove(path))
	require.Equal(t, false, requestVideoDetails(t, video)["live_preview_available"])
}

func TestVideoDetailsPreviewSurvivesExternalVideoIDUpdate(t *testing.T) {
	t.Parallel()
	video := &ent.Vod{ID: uuid.New(), ExtID: "vod999", ExtStreamID: "stream123", Type: utils.Live,
		Status: utils.ArchiveRunning, TmpVideoHlsPath: t.TempDir()}
	path := filepath.Join(video.TmpVideoHlsPath, "stream123-video.m3u8")
	require.NoError(t, os.WriteFile(path, []byte("#EXTM3U\n"), 0600))
	require.Equal(t, true, requestVideoDetails(t, video)["live_preview_available"])
}

func requestVideoDetails(t *testing.T, video *ent.Vod) map[string]any {
	t.Helper()
	handler := &Handler{Service: Services{VodService: videoDetailsService{video: video}}}
	recorder := httptest.NewRecorder()
	c := echo.New().NewContext(httptest.NewRequest("GET", "/api/v1/vod/"+video.ID.String(), nil), recorder)
	c.SetParamNames("id")
	c.SetParamValues(video.ID.String())
	require.NoError(t, handler.GetVod(c))
	require.Equal(t, 200, recorder.Code)
	var response struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, video.ID.String(), response.Data["id"])
	require.Equal(t, string(video.Status), response.Data["status"])
	return response.Data
}
