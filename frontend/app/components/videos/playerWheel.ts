import { useEffect } from 'react';

// A wheel notch scrolls 100px in Chromium on Windows and 3 lines in Firefox.
const NOTCH_PX = 100;
const LINE_PX = NOTCH_PX / 3;
// Wheel events further apart than this start a new gesture.
const GESTURE_GAP_MS = 150;

export type WheelValue = {
  get: () => number;
  set: (value: number) => void;
  step: number;
  min: number;
  max: number;
};

// Moves a value by one step per wheel notch over the elements, without scrolling the page. Events the
// browser merged count for all their notches; small deltas (trackpads, smooth wheels) add up to notches,
// but the first one of a gesture always moves one step. Within a gesture, steps build on the last value
// set, since player state only catches up asynchronously.
// The elements must have a box of their own: browsers scroll the page without waiting for listeners on
// display: contents elements. elements and value must be stable (state, useMemo).
export const useWheelValue = (elements: Element[], value: WheelValue | null) => {
  useEffect(() => {
    if (!value) return;
    let lastEvent = -Infinity;
    let lastDirection = 0;
    let pending = 0;
    let target: number | undefined;

    const onWheel = (event: Event) => {
      const { deltaY, deltaMode, timeStamp } = event as WheelEvent;
      if (deltaY === 0) return;
      event.preventDefault();
      const px = deltaMode === WheelEvent.DOM_DELTA_LINE ? deltaY * LINE_PX : deltaMode === WheelEvent.DOM_DELTA_PAGE ? deltaY * NOTCH_PX : deltaY;
      const newGesture = timeStamp - lastEvent > GESTURE_GAP_MS || Math.sign(px) !== lastDirection;
      lastEvent = timeStamp;
      lastDirection = Math.sign(px);
      if (newGesture) {
        pending = 0;
        target = undefined;
      }
      pending += px;
      let notches = Math.trunc(pending / NOTCH_PX);
      if (notches === 0 && newGesture) {
        notches = Math.sign(pending);
        pending = 0;
      } else {
        pending -= notches * NOTCH_PX;
      }
      if (notches === 0) return;

      // Scrolling up (negative delta) raises the value
      let next = target ?? value.get();
      for (let i = 0; i < Math.abs(notches); i++) next = stepOnGrid(next, value.step, notches < 0 ? 1 : -1, value.min, value.max);
      target = next;
      value.set(next);
    };

    elements.forEach((element) => element.addEventListener('wheel', onWheel, { passive: false }));
    return () => elements.forEach((element) => element.removeEventListener('wheel', onWheel));
  }, [elements, value]);
};

// Next value on a grid of `step` in the given direction, snapping values that are off the grid, within [min, max].
export const stepOnGrid = (value: number, step: number, direction: 1 | -1, min: number, max: number) => {
  const index = direction > 0 ? Math.floor(value / step + 1e-6) + 1 : Math.ceil(value / step - 1e-6) - 1;
  return Math.min(max, Math.max(min, Math.round(index * step * 100) / 100));
};
