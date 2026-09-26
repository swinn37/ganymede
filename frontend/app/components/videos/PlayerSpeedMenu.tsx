import { Menu, useMediaState, usePlaybackRateOptions } from '@vidstack/react';
import { IconCheck } from '@tabler/icons-react';
import { useTranslations } from 'next-intl';
import classes from './PlayerSpeedMenu.module.css';

// Speeds offered by this menu and by the Settings > Playback slider (Vidstack stops at 2x by default).
export const PLAYBACK_RATES = [0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.25, 2.5];

// Control bar shortcut to the playback rate, which otherwise sits under Settings > Playback.
// Uses the default layout's vds-* menu classes so it matches the built-in menus and works in fullscreen.
const VideoPlayerSpeedMenu = () => {
  const t = useTranslations('VideoComponents');
  const playbackRate = useMediaState('playbackRate');
  const options = usePlaybackRateOptions({ rates: PLAYBACK_RATES });

  return (
    <Menu.Root className="vds-menu">
      <Menu.Button
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
