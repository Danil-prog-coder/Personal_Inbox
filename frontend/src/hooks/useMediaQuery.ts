import { useEffect, useState } from 'react';

/** Граница телефон/десктоп — ровно 900px (docs/00-product-spec.md, п. 4). */
export const DESKTOP_QUERY = '(min-width: 900px)';

export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() =>
    typeof window === 'undefined' ? false : window.matchMedia(query).matches,
  );

  useEffect(() => {
    const list = window.matchMedia(query);
    const onChange = (event: MediaQueryListEvent) => setMatches(event.matches);
    setMatches(list.matches);
    list.addEventListener('change', onChange);
    return () => list.removeEventListener('change', onChange);
  }, [query]);

  return matches;
}

export function useIsDesktop(): boolean {
  return useMediaQuery(DESKTOP_QUERY);
}
