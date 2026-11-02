import { useEffect, useState } from 'react';

/** The breakpoint of the side navigation's column (global.css, SideNav, AdminSideNav). */
const WIDE = '(width >= 768px)';

/**
 * Whether the column fits: the app's side navigation and the administration's sidebar from
 * 768 px, the drawer and the sheet below. A browser without media queries (jsdom) counts as wide.
 */
export function useWideScreen(): boolean {
  const [wide, setWide] = useState(
    () => typeof window.matchMedia !== 'function' || window.matchMedia(WIDE).matches,
  );
  useEffect(() => {
    if (typeof window.matchMedia !== 'function') return;
    const query = window.matchMedia(WIDE);
    const update = () => setWide(query.matches);
    query.addEventListener('change', update);
    return () => query.removeEventListener('change', update);
  }, []);
  return wide;
}
