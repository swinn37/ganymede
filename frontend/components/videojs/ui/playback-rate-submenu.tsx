'use client';

import '../styles/base.css';
import '../styles/audio/theme.css';
import '../styles/video/captions.css';
import '../styles/video/theme.css';
import '../styles/menus.css';
import { speedText } from '@videojs/core/i18n/text/menu';
import { Menu, selectPlaybackRate, usePlayer } from '@videojs/react';
import { Text as TextPrimitive } from '@videojs/react';
import { SpeedIcon as SpeedIconPrimitive } from '@videojs/react/icons';
import { PlaybackRateRadioGroup } from '@videojs/react/ui/playback-rate-radio-group';

import { MenuChevron } from '@/components/videojs/ui/menu-chevron';
import { formatPlaybackRate, PlaybackRateOptions } from '@/components/videojs/ui/playback-rate-menu';

export type PlaybackRateSubmenuProps = Omit<Menu.RootProps, 'children'>;

// Ganymede: lists PLAYBACK_RATES instead of PlaybackRateRadioGroup.Options, which only offers Video.js's list up to 2x.
// The group's Root stays: it makes the settings menu count the speed as an available option.
export function PlaybackRateSubmenu({ ...props }: PlaybackRateSubmenuProps = {}) {
  const rate = usePlayer(selectPlaybackRate);
  if (!rate) return null;

  return (
    <Menu.Root {...props}>
      <PlaybackRateRadioGroup.Root>
        <Menu.Trigger className={'media-menu-trigger-item'}>
          <SpeedIconPrimitive className={'media-menu-trigger-item-icon'} />
          <TextPrimitive token={speedText.key}>{speedText.text}</TextPrimitive>
          <span className={'media-menu-hint'}>
            <span className={'media-menu-hint-label'}>{formatPlaybackRate(rate.playbackRate)}</span>
            <MenuChevron />
          </span>
        </Menu.Trigger>
        <Menu.Content className={'media-menu-content'}>
          <Menu.Item className={'media-menu-back-item'}>
            <MenuChevron back />
            <TextPrimitive token={speedText.key}>{speedText.text}</TextPrimitive>
          </Menu.Item>
          <Menu.Separator className={'media-menu-separator'} />
          <PlaybackRateOptions />
        </Menu.Content>
      </PlaybackRateRadioGroup.Root>
    </Menu.Root>
  );
}
