import { useMediaPlayer, useMediaRemote, useMediaState } from '@vidstack/react';
import { useCallback, useState } from 'react';
import { stepOnGrid, useWheelStep } from './playerWheel';
import classes from './PlayerVolumePercent.module.css';

const VOLUME_STEP = 0.05;

// Volume percentage shown next to the volume slider. Also lets the mouse wheel change the volume
// over the whole volume control (mute button and slider) in steps of 5%.
const VideoPlayerVolumePercent = () => {
  const player = useMediaPlayer();
  const remote = useMediaRemote();
  const volume = useMediaState('volume');
  const muted = useMediaState('muted');
  const [volumeControl, setVolumeControl] = useState<Element | null>(null);

  const attach = useCallback((el: HTMLSpanElement | null) => setVolumeControl(el?.closest('.vds-volume') ?? null), []);

  const onStep = useCallback((direction: 1 | -1) => {
    if (!player) return;
    // Step from what the slider shows: 0 while muted
    const shown = player.state.muted ? 0 : player.state.volume;
    if (player.state.muted && direction > 0) remote.unmute();
    remote.changeVolume(stepOnGrid(shown, VOLUME_STEP, direction, 0, 1));
  }, [player, remote]);
  useWheelStep(volumeControl, onStep);

  return (
    <span ref={attach} className={classes.volumePercent} aria-hidden="true">
      {Math.round((muted ? 0 : volume) * 100)}%
    </span>
  );
};

export default VideoPlayerVolumePercent;
