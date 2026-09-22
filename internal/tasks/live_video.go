package tasks

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/rs/zerolog/log"
	"github.com/zibbp/ganymede/ent"
	"github.com/zibbp/ganymede/internal/config"
	"github.com/zibbp/ganymede/internal/exec"
	"github.com/zibbp/ganymede/internal/hls"
	"github.com/zibbp/ganymede/internal/utils"
)

const (
	liveCaptureRetryInterval = 20 * time.Second
	// Restart ffmpeg when it keeps waiting on a frozen stream instead of exiting.
	liveCaptureStallTimeout = 90 * time.Second
)

// //////////////////////
// Download Live Video //
// //////////////////////
// This task is special as it will create it's own context if the task is cancelled so the rest of the task can be completed.
type DownloadLiveVideoArgs struct {
	Continue bool              `json:"continue"`
	Input    ArchiveVideoInput `json:"input"`
}

func (DownloadLiveVideoArgs) Kind() string { return string(utils.TaskDownloadLiveVideo) }

func (args DownloadLiveVideoArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: 1,
		Tags:        []string{"archive"},
		UniqueOpts:  archiveUniqueOpts(),
	}
}

func (w *DownloadLiveVideoWorker) Timeout(job *river.Job[DownloadLiveVideoArgs]) time.Duration {
	return 49 * time.Hour
}

type DownloadLiveVideoWorker struct {
	river.WorkerDefaults[DownloadLiveVideoArgs]
}

func (w DownloadLiveVideoWorker) Work(ctx context.Context, job *river.Job[DownloadLiveVideoArgs]) error {
	// get store from context
	store, err := StoreFromContext(ctx)
	if err != nil {
		return err
	}

	err = setQueueStatus(ctx, store.Client, QueueStatusInput{
		Status:  utils.Running,
		QueueId: job.Args.Input.QueueId,
		Task:    utils.TaskDownloadVideo,
	})
	if err != nil {
		return err
	}
	client := river.ClientFromContext[pgx.Tx](ctx)

	dbItems, err := getDatabaseItems(ctx, store.Client, job.Args.Input.QueueId)
	if err != nil {
		return err
	}

	// The chat download starts once, right before ffmpeg first starts.
	var chatOnce sync.Once
	startChatDownload := func() {
		if !dbItems.Queue.ArchiveChat {
			return
		}
		log.Debug().Str("channel", dbItems.Channel.Name).Msgf("starting chat download for %s", dbItems.Video.ExtID)
		_, insertErr := client.Insert(ctx, &DownloadLiveChatArgs{
			Continue: true,
			Input:    nextArchiveInput(job.Args.Input),
		}, nil)
		if insertErr != nil {
			log.Error().Err(insertErr).Msg("failed to start chat download")
		}
	}

	// download live video
	// Note: even when download fails unexpectedly, continue with finalization steps
	// (cancel live chat, mark channel not live, enqueue post-process) so partial archive
	// can still be completed/moved instead of being left in a stuck state.
	var downloadErr error
	grace := time.Duration(config.Get().Livestream.ReconnectGraceMinutes) * time.Minute
	if dbItems.Video.VideoHlsPath != "" && grace > 0 {
		downloadErr = captureLiveVideoWithReconnect(ctx, store.Client, dbItems, grace, func() { chatOnce.Do(startChatDownload) })
	} else {
		downloadErr = exec.DownloadTwitchLiveVideo(ctx, dbItems.Video, dbItems.Channel, func() { chatOnce.Do(startChatDownload) })
	}
	remotelyCancelled := false
	if downloadErr != nil {
		if errors.Is(downloadErr, context.Canceled) {
			if !errors.Is(context.Cause(ctx), rivertype.ErrJobCancelledRemotely) {
				// Process shutdown is recovered from the partial media by the
				// watchdog after restart; don't hide it as a successful job.
				return downloadErr
			}
			remotelyCancelled = true
			finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), liveArchiveFinalizationTimeout)
			defer cancel()
			ctx = finalizeCtx
		} else {
			finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), liveArchiveFinalizationTimeout)
			defer cancel()
			ctx = finalizeCtx
			log.Error().Err(downloadErr).Str("queue_id", job.Args.Input.QueueId.String()).Msg("live video download failed; continuing with archive finalization")
		}
	}

	// cancel chat download when video download is done
	// get chat download job id
	params := river.NewJobListParams().States(
		rivertype.JobStateAvailable,
		rivertype.JobStatePending,
		rivertype.JobStateScheduled,
		rivertype.JobStateRunning,
		rivertype.JobStateRetryable,
	).First(500)
	chatDownloadJobId, err := getTaskId(ctx, client, GetTaskFilter{
		Kind:    string(utils.TaskDownloadLiveChat),
		QueueId: job.Args.Input.QueueId,
		Tags:    []string{"archive"},
	}, params)
	if err != nil {
		return err
	}
	// cancel chat download if it exists
	if chatDownloadJobId != 0 {
		_, err = client.JobCancel(ctx, chatDownloadJobId)
		if err != nil {
			return err
		}
	}

	// mark channel as not live
	if err := setWatchChannelAsNotLive(ctx, store, dbItems.Channel.ID); err != nil {
		return err
	}

	next := []transactionalJob{}
	if job.Args.Continue {
		next = append(next,
			transactionalJob{Args: &PostProcessVideoArgs{Continue: true, Input: nextArchiveInput(job.Args.Input)}},
			transactionalJob{
				Args: &UpdateStreamVideoIdArgs{Input: nextArchiveInput(job.Args.Input)},
				Opts: &river.InsertOpts{ScheduledAt: time.Now().Add(10 * time.Minute)},
			},
		)
	}
	err = setQueueStatusAndEnqueue(ctx, store, QueueStatusInput{
		Status:  utils.Success,
		QueueId: job.Args.Input.QueueId,
		Task:    utils.TaskDownloadVideo,
	}, next...)
	if err != nil {
		return err
	}

	// check if tasks are done
	if err := checkIfTasksAreDone(ctx, store.Client, job.Args.Input); err != nil {
		return err
	}
	if remotelyCancelled {
		return downloadErr
	}

	return nil
}

// captureLiveVideoWithReconnect captures the stream into its HLS playlist and resumes the
// capture while the stream comes back within grace. Each run is recorded on the queue so the
// live chat can be lined up with the video (see utils.LiveCaptureRun).
func captureLiveVideoWithReconnect(ctx context.Context, client *ent.Client, dbItems *GetDatabaseItemsResponse, grace time.Duration, startChat func()) error {
	playlistPath := tmpHLSPlaylistPath(&dbItems.Video)
	logger := log.With().Str("queue_id", dbItems.Queue.ID.String()).Str("channel", dbItems.Channel.Name).Logger()
	var runs []utils.LiveCaptureRun
	saveRuns := func() {
		// Saved even when the capture was just stopped, as finalization still converts the chat.
		if err := client.Queue.UpdateOneID(dbItems.Queue.ID).SetLiveCaptureRuns(runs).Exec(context.WithoutCancel(ctx)); err != nil {
			logger.Error().Err(err).Msg("failed to save live capture runs")
		}
	}

	capture := func(ctx context.Context) (bool, error) {
		before, _ := hls.MediaPlaylistDuration(playlistPath) // no playlist before the first run
		started := false

		runCtx, cancelRun := context.WithCancel(ctx)
		defer cancelRun()
		go cancelWhenPlaylistStalls(runCtx, cancelRun, playlistPath, liveCaptureStallTimeout)

		err := exec.DownloadTwitchLiveVideo(runCtx, dbItems.Video, dbItems.Channel, func() {
			started = true
			startChat()
			runs = append(runs, utils.LiveCaptureRun{WallStart: time.Now(), VideoOffset: before})
			saveRuns()
		})

		after, _ := hls.MediaPlaylistDuration(playlistPath)
		progressed := after > before
		if started {
			if progressed {
				runs[len(runs)-1].WallEnd = time.Now()
			} else {
				runs = runs[:len(runs)-1]
			}
			saveRuns()
		}
		if err != nil && ctx.Err() == nil {
			logger.Warn().Err(err).Bool("captured", progressed).Msg("live capture run ended")
		}
		return progressed, err
	}

	return captureWithReconnect(ctx, grace, liveCaptureRetryInterval, time.Now, sleepContext, capture)
}

// captureWithReconnect runs capture until nothing has been captured for grace since the end
// of the last run that captured something. It retries right after such a run and every
// retryInterval otherwise. It returns ctx.Err() once cancelled, and the last error when
// nothing was ever captured.
func captureWithReconnect(ctx context.Context, grace, retryInterval time.Duration, now func() time.Time, sleep func(context.Context, time.Duration) error, capture func(context.Context) (bool, error)) error {
	captured := false
	var lastErr error
	waitingSince := now()
	for {
		progressed, err := capture(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		lastErr = err
		if progressed {
			captured = true
			waitingSince = now()
			continue
		}
		if now().Sub(waitingSince) >= grace {
			break
		}
		if err := sleep(ctx, retryInterval); err != nil {
			return err
		}
	}
	if captured {
		return nil
	}
	return lastErr
}

// cancelWhenPlaylistStalls cancels a capture whose playlist has not changed for stallAfter.
func cancelWhenPlaylistStalls(ctx context.Context, cancel context.CancelFunc, path string, stallAfter time.Duration) {
	ticker := time.NewTicker(stallAfter / 9)
	defer ticker.Stop()
	var lastModified time.Time
	lastChange := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if info, err := os.Stat(path); err == nil && !info.ModTime().Equal(lastModified) {
				lastModified, lastChange = info.ModTime(), time.Now()
			} else if time.Since(lastChange) >= stallAfter {
				log.Warn().Str("playlist", path).Msg("live capture stalled; restarting ffmpeg")
				cancel()
				return
			}
		}
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
