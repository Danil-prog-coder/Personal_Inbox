/** Иконки — инлайновый SVG. Пути скопированы из референса дословно,
 * иконочные библиотеки не подключаются (docs/01-design-system.md, п. 7). */

const PATHS = {
  feed: [
    'M4 6.5h16M4 12h16M4 17.5h10',
  ],
  summary: [
    'M5 19V10M12 19V5M19 19v-6',
  ],
  connections: [
    'M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71',
    'M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71',
  ],
  settings: [
    'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z',
    'M12.22 2.6h-.44a1.9 1.9 0 0 0-1.9 1.9v.13a1.9 1.9 0 0 1-.95 1.64l-.4.24a1.9 1.9 0 0 1-1.9 0l-.11-.07a1.9 1.9 0 0 0-2.6.7l-.22.38a1.9 1.9 0 0 0 .7 2.6l.11.06a1.9 1.9 0 0 1 .95 1.64v.46a1.9 1.9 0 0 1-.95 1.65l-.11.06a1.9 1.9 0 0 0-.7 2.6l.22.38a1.9 1.9 0 0 0 2.6.7l.11-.07a1.9 1.9 0 0 1 1.9 0l.4.24a1.9 1.9 0 0 1 .95 1.64v.13a1.9 1.9 0 0 0 1.9 1.9h.44a1.9 1.9 0 0 0 1.9-1.9v-.13a1.9 1.9 0 0 1 .95-1.64l.4-.24a1.9 1.9 0 0 1 1.9 0l.11.07a1.9 1.9 0 0 0 2.6-.7l.22-.38a1.9 1.9 0 0 0-.7-2.6l-.11-.06a1.9 1.9 0 0 1-.95-1.65v-.46a1.9 1.9 0 0 1 .95-1.64l.11-.06a1.9 1.9 0 0 0 .7-2.6l-.22-.38a1.9 1.9 0 0 0-2.6-.7l-.11.07a1.9 1.9 0 0 1-1.9 0l-.4-.24a1.9 1.9 0 0 1-.95-1.64V4.5a1.9 1.9 0 0 0-1.9-1.9Z',
  ],
  search: [
    'M11 18a7 7 0 1 0 0-14 7 7 0 0 0 0 14Z',
    'm20 20-3.9-3.9',
  ],
  filter: [
    'M4 6h16M7 12h10M10 18h4',
  ],
  back: [
    'm14.5 5-7 7 7 7',
  ],
  close: [
    'M6.5 6.5l11 11M17.5 6.5l-11 11',
  ],
  bolt: [
    'm13 2-9 12h7l-1 8 9-12h-7l1-8Z',
  ],
  reply: [
    'M9 7 4 12l5 5',
    'M4 12h9a7 7 0 0 1 7 7v1',
  ],
  clock: [
    'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Z',
    'M12 7.4V12l3 1.8',
  ],
  ext: [
    'M14 4h6v6',
    'M20 4 11 13',
    'M18 14.5V19a1.6 1.6 0 0 1-1.6 1.6H5A1.6 1.6 0 0 1 3.4 19V7.6A1.6 1.6 0 0 1 5 6h4.5',
  ],
  chev: [
    'm9 5 7 7-7 7',
  ],
  warn: [
    'M12 3.6 2.6 20h18.8L12 3.6Z',
    'M12 10v4',
    'M12 17.3v.1',
  ],
  check: [
    'm4.5 12.5 5 5 10-11',
  ],
  plus: [
    'M12 5v14M5 12h14',
  ],
  dot: [
    'M12 12.6a.6.6 0 1 0 0-1.2.6.6 0 0 0 0 1.2Z',
  ],
} as const;

export type IconName = keyof typeof PATHS;

interface Props {
  name: IconName;
  size?: number;
  className?: string;
}

export function Icon({ name, size = 17, className }: Props) {
  return (
    <svg
      className={className}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.7}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {PATHS[name].map((d) => (
        <path key={d} d={d} />
      ))}
    </svg>
  );
}

/** Конверт из шапки входа и логотипа — единственная иконка вне набора. */
export function LogoIcon({ size = 19 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M3 7.5 12 13l9-5.5"
        stroke="currentColor"
        strokeWidth={1.7}
        strokeLinecap="round"
        strokeLinejoin="round"
        opacity=".55"
      />
      <rect x="3" y="5" width="18" height="14" rx="3.6" stroke="currentColor" strokeWidth={1.7} />
    </svg>
  );
}
