'use client';

import '../styles/base.css';
import '../styles/audio/theme.css';
import '../styles/video/captions.css';
import '../styles/video/theme.css';
import '../styles/popups.css';
import '../styles/ganymede.css';
import type { VolumeSliderProps as CoreVolumeSliderProps } from '@videojs/core';
import { selectVolume, usePlayer, VolumePopover as VolumePopoverPrimitive } from '@videojs/react';
import type { ClassValue } from 'cn';
import { useMemo, useState } from 'react';

import { useWheelValue } from '@/app/components/videos/playerWheel';
import { ButtonTooltip } from '@/components/videojs/ui/button-tooltip';
import { MuteButton } from '@/components/videojs/ui/mute-button';
import { VolumeSlider } from '@/components/videojs/ui/volume-slider';
import { cn } from '@/lib/resolve-class-name';

const VOLUME_STEP = 0.05;

// Ganymede: the volume as a percentage above the slider.
function VolumePercent() {
  const volume = usePlayer(selectVolume);
  if (!volume || volume.volumeAvailability !== 'available') return null;
  return (
    <span className={'media-volume-percent'} aria-hidden="true">
      {Math.round((volume.muted ? 0 : volume.volume) * 100)}%
    </span>
  );
}

export interface VolumePopoverProps extends Omit<VolumePopoverPrimitive.RootProps, 'children'> {
  className?: ClassValue;
  orientation?: CoreVolumeSliderProps['orientation'];
  showTooltip?: boolean;
}

export function VolumePopover({
  className,
  showTooltip = false,
  side = 'top',
  orientation = 'vertical',
  ...props
}: VolumePopoverProps = {}) {
  // Ganymede: the mouse wheel over the mute button or the popup changes the volume by 5% per notch, also when
  // the browser merges fast spins; the slider's own wheel handling moves 5% per event.
  const store = usePlayer();
  const [trigger, setTrigger] = useState<HTMLElement | null>(null);
  const [popup, setPopup] = useState<HTMLElement | null>(null);
  const wheelTargets = useMemo(() => [trigger, popup].filter((element) => element !== null), [trigger, popup]);
  const wheelVolume = useMemo(() => ({
    // Step from what the slider shows: 0 while muted
    get: () => {
      const volume = selectVolume(store.state as object);
      return volume && !volume.muted ? volume.volume : 0;
    },
    set: (next: number) => {
      const volume = selectVolume(store.state as object);
      if (!volume) return;
      if (next > 0 && volume.muted) volume.setMuted(false);
      volume.setVolume(next);
    },
    step: VOLUME_STEP,
    min: 0,
    max: 1,
  }), [store]);
  useWheelValue(wheelTargets, wheelVolume);

  return (
    <VolumePopoverPrimitive.Root openOnHover delay={200} closeDelay={100} side={side} {...props}>
      <ButtonTooltip delay={0} disabled={!showTooltip} sticky side="top">
        <VolumePopoverPrimitive.Trigger ref={setTrigger} render={<MuteButton className={cn(className)} />} />
      </ButtonTooltip>
      <VolumePopoverPrimitive.Popup
        ref={setPopup}
        className={cn(
          'media-popup',
          'media-popup-safe-area',
          'media-popup-transition',
          'media-popup-surface',
          'media-volume-popover'
        )}
      >
        <VolumePercent />
        <VolumeSlider orientation={orientation} />
      </VolumePopoverPrimitive.Popup>
    </VolumePopoverPrimitive.Root>
  );
}
