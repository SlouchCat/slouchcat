import { useEffect, useMemo, useState } from 'react';

/** End time of a pause of `seconds` that starts now. `run` only tells a new pause from the last. */
function pauseEnd(seconds: number, run: number): number {
  void run;
  return Date.now() + seconds * 1000;
}

/**
 * Whole seconds left of a pause that starts when `seconds` (or `runId`) changes and lasts that
 * long; 0 when nothing runs. The end time is fixed in the same render, so the very first render
 * of a pause already shows it (a button never flashes enabled), and the clock keeps it right in a
 * throttled background tab.
 */
export function useCountdown(seconds: number | null, runId = 0): number {
  const running = seconds !== null && seconds > 0;
  // `runId` restarts the pause for the same number of seconds.
  const end = useMemo(() => (running ? pauseEnd(seconds, runId) : 0), [running, seconds, runId]);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!end) return;
    const timer = window.setInterval(() => {
      setNow(Date.now());
      if (Date.now() >= end) window.clearInterval(timer);
    }, 250);
    return () => window.clearInterval(timer);
  }, [end]);

  // `now` may still date from before this pause began: never report more than the pause itself.
  return running ? Math.min(seconds, Math.max(0, Math.ceil((end - now) / 1000))) : 0;
}

/** Minutes and zero-padded seconds of a countdown, for the `m:ss` text. */
export function formatCountdown(seconds: number): { minutes: string; seconds: string } {
  return {
    minutes: String(Math.floor(seconds / 60)),
    seconds: String(seconds % 60).padStart(2, '0'),
  };
}
