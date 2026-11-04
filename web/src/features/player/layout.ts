import { useEffect, useState } from 'react';
import type { PlayerChromeLayout } from '@/ui/aria';

/**
 * A phone (11.2): narrower than 768 px, or a touch screen held sideways. The same query makes the
 * panels a sheet from the bottom (PlayerPanel.module.css).
 */
export const PHONE_QUERY = '(width < 768px), (pointer: coarse) and (height <= 500px)';

/** The layout of the player's controls; follows the query when the window turns or resizes. */
export function usePlayerLayout(): PlayerChromeLayout {
  const matches = () =>
    typeof window.matchMedia === 'function' && window.matchMedia(PHONE_QUERY).matches;
  const [phone, setPhone] = useState(matches);
  useEffect(() => {
    if (typeof window.matchMedia !== 'function') return;
    const query = window.matchMedia(PHONE_QUERY);
    const update = () => setPhone(query.matches);
    query.addEventListener('change', update);
    return () => query.removeEventListener('change', update);
  }, []);
  return phone ? 'phone' : 'desktop';
}

/**
 * Whether the page cannot set the volume: Safari on the iPhone keeps `volume` read-only, so the
 * player shows only the mute button there (11.2). An iPad can set it.
 */
export function volumeIsReadOnly(userAgent: string = navigator.userAgent): boolean {
  return /iPhone|iPod/.test(userAgent);
}
