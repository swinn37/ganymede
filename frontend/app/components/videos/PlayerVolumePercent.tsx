import { useMediaPlayer, useMediaRemote, useMediaState } from '@vidstack/react';
import { useCallback, useMemo, useState } from 'react';
import { useWheelValue } from './playerWheel';
import classes from './PlayerVolumePercent.module.css';

const VOLUME_STEP = 0.05;

// Volume percentage shown next to the volume slider. Also lets the mouse wheel change the volume
// over the whole volume control (mute button and slider) in steps of 5%.
const VideoPlayerVolumePercent = () => {
  const player = useMediaPlayer();
  const remote = useMediaRemote();
  const volume = useMediaState('volume');
  const muted = useMediaState('muted');
  const [wheelTargets, setWheelTargets] = useState<Element[]>([]);

  // The mute button, the slider and this label: the .vds-volume group around them has no box in the large layout
  const attach = useCallback((el: HTMLSpanElement | null) => {
    const group = el?.closest('.vds-volume');
    const targets = [group?.querySelector('.vds-button'), group?.querySelector('.vds-volume-slider'), el];
    setWheelTargets(targets.filter((target): target is Element => !!target));
  }, []);

  const wheelVolume = useMemo(() => player && {
    // Step from what the slider shows: 0 while muted
    get: () => (player.state.muted ? 0 : player.state.volume),
    set: (next: number) => {
      if (next > 0 && player.state.muted) remote.unmute();
      remote.changeVolume(next);
    },
    step: VOLUME_STEP,
    min: 0,
    max: 1,
  }, [player, remote]);
  useWheelValue(wheelTargets, wheelVolume);

  return (
    <span ref={attach} className={classes.volumePercent} aria-hidden="true">
      {Math.round((muted ? 0 : volume) * 100)}%
    </span>
  );
};

export default VideoPlayerVolumePercent;
