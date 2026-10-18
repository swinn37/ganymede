import { selectPlayback, selectPlaybackRate, selectTime, usePlayer } from '@videojs/react';
import { IconPlayerSkipForwardFilled } from '@tabler/icons-react';
import { useTranslations } from 'next-intl';
import classes from './PlayerLiveIndicator.module.css';

// Seek one HLS segment short of the recorded end so playback does not stall at the edge.
const LIVE_EDGE_OFFSET = 10;
// Closer to the recorded end than this counts as watching live.
const LIVE_EDGE_TOLERANCE = 30;

type Props = {
  // Called right before the button forces 1x, so the player does not save it as the viewer's speed
  onResetRate: () => void;
};

// Twitch-like live indicator above the end of the timeline of a recording in progress:
// a red LIVE badge at the live edge, a LIVE button jumping back to it otherwise.
const VideoPlayerLiveIndicator = ({ onResetRate }: Props) => {
  const t = useTranslations('VideoComponents');
  const time = usePlayer(selectTime);
  const rate = usePlayer(selectPlaybackRate);
  const playback = usePlayer(selectPlayback);

  if (!time || !time.duration || !Number.isFinite(time.duration)) return null;
  const end = time.duration;

  if (end - time.currentTime <= LIVE_EDGE_TOLERANCE) {
    return <span className={classes.liveBadge}>{t('liveButtonLabel')}</span>;
  }

  const goLive = () => {
    // Playing faster than real time would keep stalling at the edge
    if (rate && rate.playbackRate !== 1) {
      onResetRate();
      rate.setPlaybackRate(1);
    }
    void time.seek(Math.max(0, end - LIVE_EDGE_OFFSET));
    void playback?.play();
  };

  return (
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
  );
};

export default VideoPlayerLiveIndicator;
