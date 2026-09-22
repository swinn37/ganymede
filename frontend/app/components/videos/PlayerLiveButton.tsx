import { useMediaRemote, useMediaState } from '@vidstack/react';
import { IconPlayerSkipForwardFilled } from '@tabler/icons-react';
import { useTranslations } from 'next-intl';
import classes from './PlayerLiveButton.module.css';

// Seek one HLS segment short of the recorded end so playback does not stall at the edge.
const LIVE_EDGE_OFFSET = 10;
// Closer to the recorded end than this counts as watching live.
const LIVE_EDGE_TOLERANCE = 30;

// Twitch-like button to jump back to the most recent part of a recording in progress.
const VideoPlayerLiveButton = () => {
  const t = useTranslations('VideoComponents');
  const remote = useMediaRemote();
  const seekableEnd = useMediaState('seekableEnd');
  const currentTime = useMediaState('currentTime');
  const atLiveEdge = seekableEnd - currentTime <= LIVE_EDGE_TOLERANCE;

  const goLive = () => {
    // Playing faster than real time would keep stalling at the edge
    remote.changePlaybackRate(1);
    remote.seek(Math.max(0, seekableEnd - LIVE_EDGE_OFFSET));
    remote.play();
  };

  return (
    <button
      type="button"
      className={`vds-button ${classes.liveButton}`}
      data-live-edge={atLiveEdge || undefined}
      onClick={goLive}
      aria-label={t('liveButtonTooltip')}
      title={t('liveButtonTooltip')}
    >
      <span className={classes.liveDot} />
      {t('liveButtonLabel')}
      <IconPlayerSkipForwardFilled size={14} />
    </button>
  );
};

export default VideoPlayerLiveButton;
