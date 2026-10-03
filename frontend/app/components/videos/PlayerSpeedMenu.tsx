import { Menu, useMediaPlayer, useMediaRemote, useMediaState, usePlaybackRateOptions } from '@vidstack/react';
import { IconCheck } from '@tabler/icons-react';
import { useTranslations } from 'next-intl';
import { useCallback, useMemo, useState } from 'react';
import { stepOnGrid, useWheelStep } from './playerWheel';
import classes from './PlayerSpeedMenu.module.css';

// Speeds offered by this menu and by the Settings > Playback slider (Vidstack stops at 2x by default).
export const PLAYBACK_RATES = [0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.25, 2.5];
// Finer step for the mouse wheel over the button, within the same range.
const WHEEL_RATE_STEP = 0.1;

// Control bar shortcut to the playback rate, which otherwise sits under Settings > Playback.
// Uses the default layout's vds-* menu classes so it matches the built-in menus and works in fullscreen.
const VideoPlayerSpeedMenu = () => {
  const t = useTranslations('VideoComponents');
  const player = useMediaPlayer();
  const remote = useMediaRemote();
  const playbackRate = useMediaState('playbackRate');
  const options = usePlaybackRateOptions({ rates: PLAYBACK_RATES });
  const [button, setButton] = useState<HTMLButtonElement | null>(null);
  const wheelTargets = useMemo(() => (button ? [button] : []), [button]);

  const onWheelStep = useCallback((direction: 1 | -1) => {
    if (!player) return;
    const rate = stepOnGrid(player.state.playbackRate, WHEEL_RATE_STEP, direction, PLAYBACK_RATES[0], PLAYBACK_RATES[PLAYBACK_RATES.length - 1]);
    remote.changePlaybackRate(rate);
  }, [player, remote]);
  useWheelStep(wheelTargets, onWheelStep);

  return (
    <Menu.Root className="vds-menu">
      <Menu.Button
        ref={setButton}
        className={`vds-menu-button vds-button ${classes.speedButton}`}
        aria-label={t('playbackSpeedLabel')}
      >
        {playbackRate}x
      </Menu.Button>
      <Menu.Items className="vds-menu-items" placement="top end" offset={0}>
        <Menu.RadioGroup className="vds-radio-group" value={options.selectedValue}>
          {options.map(({ label, value, select }) => (
            <Menu.Radio className="vds-radio" value={value} onSelect={select} key={value}>
              <IconCheck className="vds-icon" />
              <span className="vds-radio-label">{label}</span>
            </Menu.Radio>
          ))}
        </Menu.RadioGroup>
      </Menu.Items>
    </Menu.Root>
  );
};

export default VideoPlayerSpeedMenu;
