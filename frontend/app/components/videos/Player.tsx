import { ArchiveStatus, isArchiveActive } from "@/app/util/archiveStatus";
import { Video, VideoPlayer as VideoJsPlayer, usePlayer as useVodPlayer } from '@videojs/react/video';
import { selectError } from '@videojs/react';
import { HlsJsVideo } from '@videojs/react/media/hlsjs-video';
import { I18nProvider } from '@videojs/react/i18n';
import { VideoSkin } from '@/components/videojs/video/skin';
import { VideoDetails, VideoType as GanymedeVideoType } from '@/app/hooks/useVideos';
import classes from "./Player.module.css"
import type { CSSProperties } from 'react';
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { env } from 'next-runtime-env';
import dayjs from 'dayjs';
import { useLocale } from 'next-intl';
import { escapeURL } from '@/app/util/util';
import { extractVideoNoteTimestamps } from '@/app/util/videoNotes';
import { PlaybackStatus, useFetchPlaybackForVideo, useSetPlaybackProgressForVideo, useStartPlaybackForVideo, useUpdatePlaybackProgressForVideo } from '@/app/hooks/usePlayback';
import { useAxiosPrivate } from '@/app/hooks/useAxios';
import useAuthStore from '@/app/store/useAuthStore';
import { useSearchParams } from 'next/navigation';
import VideoEventBus from '@/app/util/VideoEventBus';
import VideoPlayerTheaterModeIcon from './PlayerTheaterModeIcon';
import useSettingsStore from '@/app/store/useSettingsStore';
import VideoPlayerHideChatIcon from './PlayerHideChatIcon';
import VideoPlayerAbsoluteTimeIcon from './PlayerAbsoluteTimeIcon';
import type { GanymedePlayerRef } from './ganymedePlayerRef';
import VideoPlayerLiveIndicator from './PlayerLiveIndicator';
import VideoPlayerControlsTiming from './PlayerControlsTiming';

interface Params {
  video: VideoDetails;
  ref: GanymedePlayerRef;
}

const AbsoluteTimeOverlay = ({ startedAt, currentTime }: { startedAt: string | Date; currentTime: number }) => {
  const flooredCurrentTime = Math.floor(currentTime);
  const absoluteTime = useMemo(
    () => dayjs(startedAt).add(flooredCurrentTime, 'second'),
    [startedAt, flooredCurrentTime],
  );

  return (
    <div className={classes.absoluteTimeOverlay}>
      <span className={classes.absoluteTimeText}>{absoluteTime.format('YYYY-MM-DD HH:mm:ss')}</span>
    </div>
  );
};

// Preset `usePlayer` hooks are typed, but TS resolves the selector state as
// `unknown` under the repo toolchain; narrow at the selection site.
const AbsoluteTimeDisplay = ({ startedAt }: { startedAt: string | Date }) => {
  const currentTime = useVodPlayer((s) => (s as unknown as { currentTime: number }).currentTime);
  const safeCurrentTime = typeof currentTime === 'number' && Number.isFinite(currentTime) ? currentTime : 0;
  return <AbsoluteTimeOverlay startedAt={startedAt} currentTime={safeCurrentTime} />;
};

// A recording that fails to load may have just been archived: check now rather than at the page's next refresh
const RecordingErrorRefresh = ({ videoId }: { videoId: string }) => {
  const error = useVodPlayer((s) => selectError(s as object)?.error);
  const queryClient = useQueryClient();
  useEffect(() => {
    if (error) queryClient.invalidateQueries({ queryKey: ["video", videoId] });
  }, [error, queryClient, videoId]);
  return null;
};

const PlayerCustomControls = () => {
  return (
    <>
      <VideoPlayerTheaterModeIcon />
      <VideoPlayerAbsoluteTimeIcon />
      <VideoPlayerHideChatIcon />
    </>
  );
};

const VideoPlayer = ({ video, ref }: Params) => {
  const searchParams = useSearchParams()
  const locale = useLocale();

  const isLoggedIn = useAuthStore(state => state.isLoggedIn);

  const player = ref;
  const hasStartedPlayback = useRef(false);
  const hasInitializedPlaybackTime = useRef(false);
  const pendingResumeTime = useRef<number | null>(null);
  // Last position and play state seen by the tick, carried over when the recording gives way to the archived video
  const lastPlayback = useRef<{ time: number; playing: boolean } | null>(null);
  const switchResume = useRef<{ time: number; playing: boolean } | null>(null);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const updatePlaybackProgressMutation = useUpdatePlaybackProgressForVideo()
  const setPlaybackProgressMutation = useSetPlaybackProgressForVideo()

  const videoTheaterMode = useSettingsStore((state) => state.videoTheaterMode);
  const showAbsoluteTime = useSettingsStore((state) => state.showAbsoluteTime);
  // video_start_at is the wall-clock time of video second 0 for live
  // archives, resolved to match the chat sync; falls back to created_at.
  const startedAt = video.type === GanymedeVideoType.Live
    ? (video.video_start_at || video.created_at)
    : video.streamed_at;
  const autoplayVideo = useSettingsStore((state) => state.autoplayVideo);
  const playbackRate = useSettingsStore((state) => state.playbackRate);
  const setPlaybackRate = useSettingsStore((state) => state.setPlaybackRate);
  // Set by the LIVE button right before it forces 1x, so that rate is not saved as the viewer's speed
  const keepSavedRate = useRef(false);
  const rateApplied = useRef(false);

  const axiosPrivate = useAxiosPrivate();
  // get playback data
  const { data: playbackData } = useFetchPlaybackForVideo(axiosPrivate, video.id, {
    refetchOnMount: "always",
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    retry: false,
    enabled: (isLoggedIn)
  })

  // start playback
  const startPlaybackMutation = useStartPlaybackForVideo(axiosPrivate, video.id)
  useEffect(() => {
    if (isLoggedIn && !hasStartedPlayback.current) {
      startPlaybackMutation.mutate();
      hasStartedPlayback.current = true;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Once the archive's video is in place, it plays while the chat is still processed
  const isRecording = isArchiveActive(video.status) && !video.video_available;
  const isHls = isRecording || video.video_path.endsWith("m3u8");

  // Captured during the commit, before the tick reads the new element
  const wasRecording = useRef(isRecording);
  useLayoutEffect(() => {
    if (wasRecording.current && !isRecording) switchResume.current = lastPlayback.current;
    wasRecording.current = isRecording;
  }, [isRecording]);

  const videoSrc = useMemo(() => {
    // Allow for processing videos to be played via HLS from the temp directory if enabled
    if (isRecording) {
      return `${(env('NEXT_PUBLIC_CDN_URL') ?? '')}${escapeURL(video.tmp_video_hls_path)}/${video.ext_stream_id || video.ext_id}-video.m3u8`;
    }
    return `${(env('NEXT_PUBLIC_CDN_URL') ?? '')}${escapeURL(video.video_path)}`;
  }, [isRecording, video.tmp_video_hls_path, video.ext_stream_id, video.ext_id, video.video_path]);

  // A recording in progress plays like a regular video: from the start, with a duration that grows with the
  // playlist instead of hls.js's infinite live duration.
  const recordingSource = useMemo(
    () => ({ src: videoSrc, engine: { hlsJs: { startPosition: 0, liveDurationInfinity: false } } }),
    [videoSrc],
  );

  const videoPoster = useMemo(() => {
    if (video.thumbnail_path) {
      return `${(env('NEXT_PUBLIC_CDN_URL') ?? '')}${escapeURL(video.thumbnail_path)}`;
    }
    return "";
  }, [video.thumbnail_path]);

  const chapterSrc = useMemo(() => {
    if (video.status !== ArchiveStatus.Completed) return undefined;
    return `${(env('NEXT_PUBLIC_API_URL') ?? '')}/api/v1/chapter/video/${video.id}/webvtt`;
  }, [video.status, video.id]);

  // thumbnails URL only when not processing
  const thumbnailsSrc = useMemo(() => {
    if (video.status !== ArchiveStatus.Completed) return undefined;
    return `${(env('NEXT_PUBLIC_API_URL') ?? '')}/api/v1/vod/${video.id}/thumbnails/vtt`;
  }, [video.status, video.id]);

  const noteBookmarks = useMemo(
    () => extractVideoNoteTimestamps(video.notes, video.id),
    [video.notes, video.id],
  );

  // Resolve the resume target once server playback or ?t= is known.
  useEffect(() => {
    if (hasInitializedPlaybackTime.current) return;
    if (pendingResumeTime.current !== null) return;
    if (playbackData && playbackData.time != null) {
      // Resume from server-side playback progress.
      pendingResumeTime.current = playbackData.time;
    } else {
      // Check if time is set in the url
      const time = searchParams.get("t");
      if (time !== null) {
        const parsed = parseInt(time, 10);
        if (Number.isFinite(parsed)) {
          pendingResumeTime.current = parsed;
        }
      }
    }
  }, [playbackData, searchParams]);

  const applyPendingResume = useCallback(() => {
    const el = player.current;
    if (!el) return;
    if (hasInitializedPlaybackTime.current) return;
    if (pendingResumeTime.current === null) return;
    try {
      el.currentTime = pendingResumeTime.current;
    } catch {
      return;
    }
    hasInitializedPlaybackTime.current = true;
  }, [player]);

  const applyStoredVolume = useCallback(() => {
    const el = player.current;
    if (!el) return;
    try {
      const localVolume = localStorage.getItem("ganymede-volume");
      if (localVolume !== null) {
        const parsed = parseFloat(localVolume);
        if (Number.isFinite(parsed)) {
          el.volume = Math.min(1, Math.max(0, parsed));
        }
      }
    } catch {
      // localStorage may be unavailable; keep element default.
    }
  }, [player]);

  const handleVolumeChange = useCallback(() => {
    const el = player.current;
    if (!el) return;
    const volume = el.volume;
    if (volume !== 1) {
      try {
        localStorage.setItem("ganymede-volume", volume.toString());
      } catch {
        // Ignore storage errors.
      }
    }
  }, [player]);

  // The saved speed applies once per source: seeks fire canplay again, and the LIVE button's 1x must stick.
  useEffect(() => {
    rateApplied.current = false;
  }, [videoSrc]);

  const applyStoredRate = useCallback(() => {
    const el = player.current;
    if (!el || rateApplied.current) return;
    el.defaultPlaybackRate = playbackRate;
    el.playbackRate = playbackRate;
    rateApplied.current = true;
  }, [player, playbackRate]);

  const handleRateChange = useCallback(() => {
    const el = player.current;
    if (!el || !rateApplied.current) return;
    if (keepSavedRate.current) {
      keepSavedRate.current = false;
      return;
    }
    setPlaybackRate(el.playbackRate);
  }, [player, setPlaybackRate]);

  // The archived video goes on where the recording was, playing or paused (pause also cancels autoplay)
  const applySwitchResume = useCallback(() => {
    const el = player.current;
    const resume = switchResume.current;
    if (!el || !resume) return;
    switchResume.current = null;
    el.currentTime = resume.time;
    if (resume.playing) el.play().catch(() => undefined);
    else el.pause();
  }, [player]);

  const handleMediaReady = useCallback(() => {
    applyStoredVolume();
    applyStoredRate();
    applyPendingResume();
    applySwitchResume();
  }, [applyStoredVolume, applyStoredRate, applyPendingResume, applySwitchResume]);

  // On the first play, the HLS adapter jumps to the live edge of a recording in progress. Put the playhead back
  // where it was: the start or the resume position. A later seek, or one that already was near the end, stays.
  useEffect(() => {
    const el = player.current;
    if (!mounted || !isRecording || !el) return;
    let before: number | undefined;
    let deadline = 0;
    const onPlay = (event: Event) => {
      if (event.target !== el || before !== undefined) return;
      before = el.currentTime;
      deadline = performance.now() + 10000;
    };
    const onSeeking = () => {
      if (before === undefined || Number.isNaN(before) || performance.now() > deadline) return;
      const end = el.seekable.length ? el.seekable.end(el.seekable.length - 1) : NaN;
      const restore = before;
      before = NaN;
      if (restore < end - 60 && el.currentTime > end - 60) el.currentTime = restore;
    };
    // Capture on the document runs before the adapter's own play listener on the element
    document.addEventListener('play', onPlay, true);
    el.addEventListener('seeking', onSeeking);
    return () => {
      document.removeEventListener('play', onPlay, true);
      el.removeEventListener('seeking', onSeeking);
    };
  }, [mounted, isRecording, player, videoSrc]);

  // Retry resume once the element exists and the target is known.
  useEffect(() => {
    if (!mounted) return;
    if (hasInitializedPlaybackTime.current) return;
    if (pendingResumeTime.current === null) return;
    applyPendingResume();
  }, [mounted, playbackData, videoSrc, applyPendingResume]);


  // Read the current video and mutation callbacks without restarting the timer on every render.
  const reportPlaybackProgress = useRef<(() => boolean) | null>(null);
  useEffect(() => {
    reportPlaybackProgress.current = () => {
      if (!player.current || player.current.paused) return false;
      const playerTimeInt = Math.floor(player.current.currentTime);
      if (playerTimeInt === 0) return false;

      updatePlaybackProgressMutation.mutate({
        axiosPrivate,
        videoId: video.id,
        time: playerTimeInt,
      });

      if (video.status === ArchiveStatus.Completed && playerTimeInt / video.duration >= 0.98) {
        setPlaybackProgressMutation.mutate({
          axiosPrivate,
          videoId: video.id,
          status: PlaybackStatus.Finished,
        });
        return true;
      }
      return false;
    };
  }, [player, video, axiosPrivate, updatePlaybackProgressMutation, setPlaybackProgressMutation]);

  // Playback progress reporting
  useEffect(() => {
    if (!isLoggedIn) return;
    const playbackInterval = setInterval(() => {
      if (reportPlaybackProgress.current?.()) clearInterval(playbackInterval);
    }, 10000);
    return () => clearInterval(playbackInterval);
  }, [isLoggedIn, video.id, video.status]);

  // Fast tick for chat player - set player information in bus
  useEffect(() => {
    const ticketInterval = setInterval(() => {
      if (player.current == null) return;
      lastPlayback.current = { time: player.current.currentTime, playing: !player.current.paused };

      let time = player.current.currentTime
      // Clip chats are offset with the position of the clip in the VOD
      // Append the offset to the current player time to account for this
      if (video.type == GanymedeVideoType.Clip && video.clip_vod_offset) {
        time = time + video.clip_vod_offset
      };

      VideoEventBus.setData({
        isPaused: player.current.paused,
        isPlaying: !player.current.paused,
        time: time
      })
    }, 100);
    return () => {
      clearInterval(ticketInterval);
    };
  }, [player, video.clip_vod_offset, video.type]);

  const skinStyle = {
    '--media-accent-color': 'var(--mantine-color-violet-6)',
    '--media-object-fit': 'contain',
    '--media-border-radius': '0',
    '--media-video-border-radius': '0',
  } as CSSProperties;

  // Avoid SSR mismatch: Video.js media engines need the browser.
  if (!mounted) {
    return (
      <div
        className={
          videoTheaterMode
            ? classes.mediaPlayerTheaterMode
            : classes.mediaPlayer
        }
      />
    );
  }

  const mediaProps = {
    ref: player,
    crossOrigin: "anonymous" as const,
    playsInline: true,
    autoPlay: autoplayVideo,
    preload: "auto" as const,
    onLoadedMetadata: handleMediaReady,
    onCanPlay: handleMediaReady,
    onVolumeChange: handleVolumeChange,
    onRateChange: handleRateChange,
  };

  return (
    <div className={classes.playerWrapper}>
      <VideoJsPlayer title={video.title} poster={videoPoster}>
        <I18nProvider locale={locale}>
          <VideoSkin
            className={
              videoTheaterMode
                ? classes.mediaPlayerTheaterMode
                : classes.mediaPlayer
            }
            style={skinStyle}
            bookmarks={noteBookmarks}
            customControls={<PlayerCustomControls />}
            afterTimeSlider={isRecording ? <VideoPlayerLiveIndicator onResetRate={() => { keepSavedRate.current = true }} /> : undefined}
          >
            {isRecording ? (
              // Keyed so the archived video gets a fresh element: the recording's events stay with the old one
              <HlsJsVideo key="recording" {...mediaProps} source={recordingSource} streamType="on-demand" />
            ) : isHls ? (
              <HlsJsVideo {...mediaProps} src={videoSrc}>
                {chapterSrc && (
                  <track kind="chapters" src={chapterSrc} default />
                )}
                {thumbnailsSrc && (
                  <track kind="metadata" label="thumbnails" src={thumbnailsSrc} default />
                )}
              </HlsJsVideo>
            ) : (
              <Video {...mediaProps} src={videoSrc}>
                {chapterSrc && (
                  <track kind="chapters" src={chapterSrc} default />
                )}
                {thumbnailsSrc && (
                  <track kind="metadata" label="thumbnails" src={thumbnailsSrc} default />
                )}
              </Video>
            )}
            {showAbsoluteTime && <AbsoluteTimeDisplay startedAt={startedAt} />}
            {isRecording && <RecordingErrorRefresh videoId={video.id} />}
            <VideoPlayerControlsTiming />
          </VideoSkin>
        </I18nProvider>
      </VideoJsPlayer>
    </div>
  );
}

export default VideoPlayer;
