import { useEffect, useState } from 'react';

/**
 * Whether the element given to the returned ref (one after a list) lies in view or less than half
 * a screen below it: a list that grows at its end reads its next page then (concept 18 C-3,
 * instead of «Mehr laden»). The element may come and go (a list that loads, empties, fills).
 */
export function useNearEnd(): [(node: Element | null) => void, boolean] {
  const [node, setNode] = useState<Element | null>(null);
  const [near, setNear] = useState(false);
  useEffect(() => {
    if (!node || typeof IntersectionObserver === 'undefined') return;
    const observer = new IntersectionObserver(
      (entries) => setNear(entries.some((entry) => entry.isIntersecting)),
      { rootMargin: '0px 0px 50% 0px' },
    );
    observer.observe(node);
    return () => {
      observer.disconnect();
      setNear(false);
    };
  }, [node]);
  return [setNode, near];
}
