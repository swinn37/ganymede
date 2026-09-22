package live_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zibbp/ganymede/internal/live"
	"github.com/zibbp/ganymede/internal/utils"
	"github.com/zibbp/ganymede/tests"
)

// A capture resumed after a stream drop can run under a new stream ID: the running capture is
// found by channel so the live check neither starts a second archive nor loses the first.
func TestRunningLiveArchiveIsFoundByChannel(t *testing.T) {
	app, err := tests.SetupWithoutWorker(t)
	require.NoError(t, err)
	ctx := t.Context()
	client := app.Database.Client

	channel := client.Channel.Create().SetName("resumed").SetDisplayName("Resumed").SetImagePath("/resumed.jpg").SaveX(ctx)
	otherChannel := client.Channel.Create().SetName("other").SetDisplayName("Other").SetImagePath("/other.jpg").SaveX(ctx)
	video := client.Vod.Create().SetChannel(channel).SetExtID("first-stream").SetExtStreamID("first-stream").
		SetType(utils.Live).SetTitle("live").SetWebThumbnailPath("/thumb.jpg").SetVideoPath("/video.m3u8").SaveX(ctx)
	capture := client.Queue.Create().SetVod(video).SetLiveArchive(true).SetTaskVideoDownload(utils.Running).SaveX(ctx)

	running, err := live.RunningLiveArchive(app.LiveService, ctx, channel.ID)
	require.NoError(t, err)
	require.NotNil(t, running)
	require.Equal(t, capture.ID, running.ID)
	require.Equal(t, video.ID, running.Edges.Vod.ID)

	running, err = live.RunningLiveArchive(app.LiveService, ctx, otherChannel.ID)
	require.NoError(t, err)
	require.Nil(t, running)

	client.Queue.UpdateOneID(capture.ID).SetTaskVideoDownload(utils.Success).ExecX(ctx)
	running, err = live.RunningLiveArchive(app.LiveService, ctx, channel.ID)
	require.NoError(t, err)
	require.Nil(t, running, "a finished capture is not running")
}
