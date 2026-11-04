import { useCallback, useEffect, useState } from 'react';

/**
 * Full screen of the whole document, not of the player: a switch or auto-next replaces the
 * player, and removing the full-screen element would end full screen (W-2). The player covers the
 * viewport and its panels render inside it, so nothing else shows. With `exitOnUnmount` leaving
 * the player ends full screen (the offline player, which no other player follows).
 */
export function useFullscreen({ exitOnUnmount = false }: { exitOnUnmount?: boolean } = {}) {
  const [fullscreen, setFullscreen] = useState(() => Boolean(document.fullscreenElement));

  const toggleFullscreen = useCallback(() => {
    const root = document.documentElement;
    if (document.fullscreenElement) void document.exitFullscreen();
    else if (typeof root.requestFullscreen === 'function') void root.requestFullscreen();
  }, []);

  useEffect(() => {
    const onChange = () => setFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener('fullscreenchange', onChange);
    return () => document.removeEventListener('fullscreenchange', onChange);
  }, []);

  useEffect(
    () => () => {
      if (exitOnUnmount && document.fullscreenElement) {
        void document.exitFullscreen().catch(() => undefined);
      }
    },
    [exitOnUnmount],
  );

  return { fullscreen, toggleFullscreen };
}
