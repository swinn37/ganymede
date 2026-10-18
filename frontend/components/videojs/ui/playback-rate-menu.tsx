'use client';

import '../styles/base.css';
import '../styles/audio/theme.css';
import '../styles/video/captions.css';
import '../styles/video/theme.css';
import '../styles/buttons.css';
import '../styles/menus.css';
import '../styles/popups.css';
import '../styles/ganymede.css';
import { playbackRateText, speedText } from '@videojs/core/i18n/text/menu';
import { Menu, selectPlaybackRate, Text as TextPrimitive, useHotkey, usePlayer } from '@videojs/react';
import { translateText, useTranslator } from '@videojs/react/i18n';
import { useMemo, useState } from 'react';

import { useWheelValue } from '@/app/components/videos/playerWheel';
import { Button } from '@/components/videojs/ui/button';
import { ButtonTooltip } from '@/components/videojs/ui/button-tooltip';
import { RadioItem } from '@/components/videojs/ui/radio-item';
import { cn } from '@/lib/resolve-class-name';

// Speeds offered by the speed button, Settings > Speed and the > / < shortcuts. Video.js's own list stops at 2x
// and cannot be configured yet.
export const PLAYBACK_RATES = [0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.25, 2.5];
// Finer step for the mouse wheel over the speed button, within the same range.
const WHEEL_RATE_STEP = 0.1;

export const formatPlaybackRate = (rate: number) => `${rate}×`;

// The store from a bare usePlayer() is untyped.
const currentRate = (store: { state: unknown }) => selectPlaybackRate(store.state as object);

// Next listed rate above or below the current one, staying at the ends instead of wrapping around.
const nextPlaybackRate = (current: number, direction: 1 | -1) => {
  const next = direction > 0
    ? PLAYBACK_RATES.find((rate) => rate > current + 1e-6)
    : PLAYBACK_RATES.findLast((rate) => rate < current - 1e-6);
  return next ?? current;
};

/** Radio items for PLAYBACK_RATES, used by the speed button and the settings submenu. */
export function PlaybackRateOptions() {
  const rate = usePlayer(selectPlaybackRate);
  const translator = useTranslator();
  if (!rate) return null;

  return (
    <Menu.RadioGroup
      className={'media-menu-radio-group'}
      value={String(rate.playbackRate)}
      onValueChange={(value) => rate.setPlaybackRate(Number(value))}
      aria-label={translateText(playbackRateText, translator)}
    >
      {PLAYBACK_RATES.map((option) => (
        <RadioItem key={option} value={String(option)}>
          <span>{formatPlaybackRate(option)}</span>
        </RadioItem>
      ))}
    </Menu.RadioGroup>
  );
}

export interface PlaybackRateMenuProps extends Omit<Menu.RootProps, 'children'> {
  className?: string;
}

/** Control bar button showing the current speed, with a menu of PLAYBACK_RATES and 0.1x steps on the mouse wheel. */
export function PlaybackRateMenu({ className, ...props }: PlaybackRateMenuProps = {}) {
  const rate = usePlayer(selectPlaybackRate);
  const store = usePlayer();
  const translator = useTranslator();
  const [button, setButton] = useState<HTMLElement | null>(null);
  const wheelTargets = useMemo(() => (button ? [button] : []), [button]);
  const wheelRate = useMemo(() => ({
    get: () => currentRate(store)?.playbackRate ?? 1,
    set: (next: number) => currentRate(store)?.setPlaybackRate(next),
    step: WHEEL_RATE_STEP,
    min: PLAYBACK_RATES[0],
    max: PLAYBACK_RATES[PLAYBACK_RATES.length - 1],
  }), [store]);
  useWheelValue(wheelTargets, wheelRate);

  if (!rate) return null;

  return (
    <Menu.Root side="top" align="center" boundary="viewport" {...props}>
      <ButtonTooltip label={<TextPrimitive token={speedText.key}>{speedText.text}</TextPrimitive>} side="top">
        <Menu.Trigger
          ref={setButton}
          render={<Button />}
          className={cn('media-playback-rate-menu-trigger', className)}
          aria-label={translateText(playbackRateText, translator)}
        >
          {formatPlaybackRate(rate.playbackRate)}
        </Menu.Trigger>
      </ButtonTooltip>
      <Menu.Popup className={cn('media-popup', 'media-popup-surface', 'media-menu-popup')}>
        <Menu.Content className={'media-menu-content'}>
          <PlaybackRateOptions />
        </Menu.Content>
      </Menu.Popup>
    </Menu.Root>
  );
}

/** > and < step through PLAYBACK_RATES; the built-in shortcuts only know Video.js's own list. */
export function PlaybackRateHotkeys({ disabled = false }: { disabled?: boolean }) {
  const store = usePlayer();
  const step = (direction: 1 | -1) => () => {
    const rate = currentRate(store);
    rate?.setPlaybackRate(nextPlaybackRate(rate.playbackRate, direction));
  };
  useHotkey({ keys: '>', action: 'speedUp', disabled, onActivate: step(1) });
  useHotkey({ keys: '<', action: 'speedDown', disabled, onActivate: step(-1) });
  return null;
}
