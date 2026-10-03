import { useEffect } from 'react';

// Wheel events closer together than this are ignored, so a trackpad swipe does not race through the steps.
const WHEEL_STEP_INTERVAL_MS = 50;

// Calls onStep(1) when the wheel scrolls up over one of the elements and onStep(-1) when it scrolls down,
// without scrolling the page. The elements must have a box of their own: browsers scroll the page without
// waiting for listeners on display: contents elements. elements and onStep must be stable (state, useCallback).
export const useWheelStep = (elements: Element[], onStep: (direction: 1 | -1) => void) => {
  useEffect(() => {
    let lastStep = -Infinity;
    const onWheel = (event: Event) => {
      const { deltaY, timeStamp } = event as WheelEvent;
      if (deltaY === 0) return;
      event.preventDefault();
      if (timeStamp - lastStep < WHEEL_STEP_INTERVAL_MS) return;
      lastStep = timeStamp;
      onStep(deltaY < 0 ? 1 : -1);
    };
    elements.forEach((element) => element.addEventListener('wheel', onWheel, { passive: false }));
    return () => elements.forEach((element) => element.removeEventListener('wheel', onWheel));
  }, [elements, onStep]);
};

// Next value on a grid of `step` in the given direction, snapping values that are off the grid, within [min, max].
export const stepOnGrid = (value: number, step: number, direction: 1 | -1, min: number, max: number) => {
  const index = direction > 0 ? Math.floor(value / step + 1e-6) + 1 : Math.ceil(value / step - 1e-6) - 1;
  return Math.min(max, Math.max(min, Math.round(index * step * 100) / 100));
};
