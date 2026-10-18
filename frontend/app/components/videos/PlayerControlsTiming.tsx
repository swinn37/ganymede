import { selectControls, usePlayer } from '@videojs/react';
import { useEffect, useRef } from 'react';

// Video.js hides the controls after a fixed 2 s without activity: hide them sooner.
const HIDE_DELAY_MS = 1200;
// The control bar and its menus: controls stay visible while the pointer rests on them.
const CONTROLS_SELECTOR = '.video-controls-content, .video-controls-primary, .video-controls-secondary, .media-popup';

// Renders inside the player container, which it finds from its own hidden element.
const VideoPlayerControlsTiming = () => {
  const store = usePlayer();
  const anchor = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    const container = anchor.current?.closest('.media-container');
    if (!container) return;
    const controls = () => selectControls(store.state as object);
    const isControl = (target: EventTarget | null) => target instanceof Element && target.closest(CONTROLS_SELECTOR) !== null;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let release: (() => void) | undefined;

    // Marks the viewer inactive: open menus, the hover lock below and a paused video still keep the controls up
    const restartTimer = () => {
      clearTimeout(timer);
      timer = setTimeout(() => controls()?.toggleControls(false), HIDE_DELAY_MS);
    };
    // Touch toggles the controls with taps, which Video.js handles
    const onPointerMove = (event: Event) => {
      if ((event as PointerEvent).pointerType !== 'touch') restartTimer();
    };
    const onPointerOver = (event: Event) => {
      if ((event as PointerEvent).pointerType === 'touch' || release || !isControl(event.target)) return;
      release = controls()?.requestControlsLock();
    };
    const onPointerOut = (event: Event) => {
      if (!release || isControl((event as PointerEvent).relatedTarget)) return;
      release();
      release = undefined;
    };

    const listeners: [string, (event: Event) => void][] = [
      ['pointermove', onPointerMove],
      ['keydown', restartTimer],
      ['keyup', restartTimer],
      ['pointerover', onPointerOver],
      ['pointerout', onPointerOut],
    ];
    listeners.forEach(([type, listener]) => container.addEventListener(type, listener));
    return () => {
      listeners.forEach(([type, listener]) => container.removeEventListener(type, listener));
      clearTimeout(timer);
      release?.();
    };
  }, [store]);

  return <span ref={anchor} hidden />;
};

export default VideoPlayerControlsTiming;
