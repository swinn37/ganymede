package tasks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

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
	// How often a dropped stream is looked for while the reconnect window is open.
	liveCaptureRetryInterval = 20 * time.Second
	// How often the capture playlist is restitched while a resumed run records.
	liveCaptureStitchInterval = 2 * time.Second
	// Timestamp gap before a resumed run, clear of the previous run's audio tail. Players follow
	// the playlist durations; only an MP4 export keeps the gap.
	liveCaptureRunGap = 500 * time.Millisecond
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

	// Buffered so the ffmpeg-start signal in exec never blocks this worker if
	// cancellation lands first and the receiver below has already exited.
	startChatDownload := make(chan bool, 1)

	go func(workCtx context.Context) {
		for {
			select {
			case <-startChatDownload:
				// start chat download if requested
				if dbItems.Queue.ArchiveChat {
					log.Debug().Str("channel", dbItems.Channel.Name).Msgf("starting chat download for %s", dbItems.Video.ExtID)
					client := river.ClientFromContext[pgx.Tx](workCtx)
					_, insertErr := client.Insert(workCtx, &DownloadLiveChatArgs{
						Continue: true,
						Input:    nextArchiveInput(job.Args.Input),
					}, nil)
					if insertErr != nil {
						log.Error().Err(insertErr).Msg("failed to start chat download")
					}
				}
			case <-workCtx.Done():
				return
			}
		}
	}(ctx)

	// download live video
	// Note: even when download fails unexpectedly, continue with finalization steps
	// (cancel live chat, mark channel not live, enqueue post-process) so partial archive
	// can still be completed/moved instead of being left in a stuck state.
	var downloadErr error
	if grace := time.Duration(config.Get().Livestream.ReconnectGraceMinutes) * time.Minute; grace > 0 {
		downloadErr = captureLiveVideoWithReconnect(ctx, store.Client, dbItems, grace, startChatDownload)
	} else {
		downloadErr = exec.DownloadTwitchLiveVideo(ctx, dbItems.Video, dbItems.Channel, startChatDownload, exec.LiveCaptureResume{})
	}
	logLiveCaptureOutcome(dbItems, job.Args.Input.QueueId, downloadErr)
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

// logLiveCaptureOutcome logs playlist size and segment count at capture stop.
func logLiveCaptureOutcome(dbItems *GetDatabaseItemsResponse, queueID uuid.UUID, downloadErr error) {
	var playlistBytes int64 = -1
	segments := 0
	if dbItems.Video.TmpVideoHlsPath != "" {
		if info, err := os.Stat(liveHlsPlaylistPath(&dbItems.Video)); err == nil {
			playlistBytes = info.Size()
		}
		if matches, err := filepath.Glob(filepath.Join(dbItems.Video.TmpVideoHlsPath, "*_segment*.m4s")); err == nil {
			for _, match := range matches {
				if info, err := os.Stat(match); err == nil && info.Size() > 0 {
					segments++
				}
			}
		}
	}

	logger := log.With().
		Str("queue_id", queueID.String()).
		Str("video_id", dbItems.Video.ID.String()).
		Int64("playlist_bytes", playlistBytes).
		Int("segments", segments).
		Logger()
	switch {
	case downloadErr == nil:
		logger.Info().Msg("live video download ended; capture output on disk")
	case errors.Is(downloadErr, context.Canceled):
		logger.Info().Err(downloadErr).Msg("live video download stopped; capture output on disk")
	default:
		logger.Error().Err(downloadErr).Msg("live video download failed; capture output on disk")
	}
}

// captureLiveVideoWithReconnect captures the stream and resumes the capture in the same video
// while the stream comes back within grace. Each run is recorded on the queue so the live
// chat can be lined up with the video (see utils.LiveCaptureRun).
func captureLiveVideoWithReconnect(ctx context.Context, client *ent.Client, dbItems *GetDatabaseItemsResponse, grace time.Duration, startChat chan bool) error {
	dir := dbItems.Video.TmpVideoHlsPath
	id := liveCaptureID(&dbItems.Video)
	logger := log.With().Str("queue_id", dbItems.Queue.ID.String()).Str("channel", dbItems.Channel.Name).Logger()
	var runs []utils.LiveCaptureRun
	saveRuns := func() {
		// Saved even when the capture was just stopped, as finalization still converts the chat.
		if err := client.Queue.UpdateOneID(dbItems.Queue.ID).SetLiveCaptureRuns(runs).Exec(context.WithoutCancel(ctx)); err != nil {
			logger.Error().Err(err).Msg("failed to save live capture runs")
		}
	}
	var chatOnce sync.Once

	capture := func(ctx context.Context) (bool, error) {
		run := len(runs)
		playlist, _, _ := hls.CaptureRunFiles(id, run)
		var captured float64 // seconds the video holds before this run
		if run > 0 {
			if err := hls.KeepFirstCaptureRun(dir, id); err != nil {
				return false, err
			}
			var err error
			if captured, err = hls.StitchCaptureRuns(dir, id, run); err != nil {
				return false, err
			}
		}
		before, _ := hls.MediaPlaylistDuration(filepath.Join(dir, playlist)) // no playlist before the run
		started := make(chan bool, 1)
		runCtx, cancelRun := context.WithCancel(ctx)
		defer cancelRun()
		recorded := make(chan struct{})
		go func() {
			defer close(recorded)
			select {
			case <-started:
			case <-runCtx.Done():
				return
			}
			chatOnce.Do(func() { startChat <- true })
			runs = append(runs, utils.LiveCaptureRun{WallStart: time.Now(), VideoOffset: captured})
			saveRuns()
			if run > 0 {
				// Viewers follow the capture playlist, which lists this run as ffmpeg writes it
				keepStitching(runCtx, dir, id, run+1)
			}
		}()

		resume := exec.LiveCaptureResume{Run: run, KeepOpen: true}
		if run > 0 {
			resume.TSOffset = captured + liveCaptureRunGap.Seconds()
		}
		err := exec.DownloadTwitchLiveVideo(runCtx, dbItems.Video, dbItems.Channel, started, resume)
		cancelRun()
		<-recorded

		after, _ := hls.MediaPlaylistDuration(filepath.Join(dir, playlist))
		progressed := after > before
		if len(runs) > run {
			if progressed {
				runs[run].WallEnd = time.Now()
			} else {
				runs = runs[:run]
			}
			saveRuns()
		}
		if progressed && run > 0 {
			if _, stitchErr := hls.StitchCaptureRuns(dir, id, run+1); stitchErr != nil {
				return progressed, stitchErr
			}
		}
		if err != nil && ctx.Err() == nil {
			logger.Debug().Err(err).Bool("captured", progressed).Int("run", run).Msg("live capture run ended")
		}
		return progressed, err
	}

	err := captureWithReconnect(ctx, grace, liveCaptureRetryInterval, time.Now, sleepContext, capture)
	if len(runs) > 1 {
		if removeErr := hls.RemoveCaptureRunPlaylists(dir, id); removeErr != nil {
			logger.Warn().Err(removeErr).Msg("failed to remove live capture run playlists")
		}
	}
	return err
}

// keepStitching restitches the capture playlist of runs until ctx is done.
func keepStitching(ctx context.Context, dir, id string, runs int) {
	ticker := time.NewTicker(liveCaptureStitchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := hls.StitchCaptureRuns(dir, id, runs); err != nil {
				log.Warn().Err(err).Str("dir", dir).Msg("failed to stitch live capture runs")
			}
		}
	}
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
