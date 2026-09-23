import { useMediaRemote, useMediaState } from '@vidstack/react';
import { IconPlayerSkipForwardFilled } from '@tabler/icons-react';
import { useTranslations } from 'next-intl';
import classes from './PlayerLiveButton.module.css';

// Seek one HLS segment short of the recorded end so playback does not stall at the edge.
const LIVE_EDGE_OFFSET = 10;
// Closer to the recorded end than this counts as watching live.
const LIVE_EDGE_TOLERANCE = 30;

// Twitch-like live indicator above the end of the timeline of a recording in progress:
// a red LIVE badge at the live edge, a LIVE button jumping back to it otherwise.
const VideoPlayerLiveButton = () => {
  const t = useTranslations('VideoComponents');
  const remote = useMediaRemote();
  const seekableEnd = useMediaState('seekableEnd');
  const currentTime = useMediaState('currentTime');

  if (!seekableEnd) return null;
  const atLiveEdge = seekableEnd - currentTime <= LIVE_EDGE_TOLERANCE;

  const goLive = () => {
    // Playing faster than real time would keep stalling at the edge
    remote.changePlaybackRate(1);
    remote.seek(Math.max(0, seekableEnd - LIVE_EDGE_OFFSET));
    remote.play();
  };

  return (
    <div className={classes.anchor}>
      {atLiveEdge ? (
        <span className={classes.liveBadge}>{t('liveButtonLabel')}</span>
      ) : (
        <button
          type="button"
          className={classes.liveButton}
          onClick={goLive}
          aria-label={t('liveButtonTooltip')}
          title={t('liveButtonTooltip')}
        >
          {t('liveButtonLabel')}
          <IconPlayerSkipForwardFilled size={12} />
        </button>
      )}
    </div>
  );
};

export default VideoPlayerLiveButton;
