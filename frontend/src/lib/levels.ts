import type { Level } from './types';

/** Порядок уровней — и для сортировки, и для полос, и для переключателя. */
export const LEVEL_ORDER: Level[] = ['CRITICAL', 'HIGH', 'NORMAL', 'LOW'];

export const LEVEL_LABEL: Record<Level, string> = {
  CRITICAL: 'Критично',
  HIGH: 'Важно',
  NORMAL: 'Обычное',
  LOW: 'Неважно',
};

/** Чистый цвет уровня: грань карточки, точки, полосы, свечение. */
export const LEVEL_COLOR: Record<Level, string> = {
  CRITICAL: 'var(--lvl-critical)',
  HIGH: 'var(--lvl-high)',
  NORMAL: 'var(--lvl-normal)',
  LOW: 'var(--lvl-low)',
};

/** Фон чипа уровня. */
export const LEVEL_BG: Record<Level, string> = {
  CRITICAL: 'var(--lvl-critical-bg)',
  HIGH: 'var(--lvl-high-bg)',
  NORMAL: 'var(--lvl-normal-bg)',
  LOW: 'var(--lvl-low-bg)',
};

/** Читаемый оттенок для текста уровня. */
export const LEVEL_INK: Record<Level, string> = {
  CRITICAL: 'var(--crit-ink)',
  HIGH: 'var(--warn-ink)',
  NORMAL: 'var(--info-ink)',
  LOW: 'var(--mute-ink)',
};

/** Цвет никогда не идёт один: у каждого уровня своя иконка. */
export const LEVEL_ICON = {
  CRITICAL: 'warn',
  HIGH: 'bolt',
  NORMAL: 'dot',
  LOW: 'clock',
} as const;

export const SOURCE_LABEL: Record<SourceKindKey, string> = {
  gmail: 'Gmail',
  telegram: 'Telegram',
};

export const SOURCE_LETTER: Record<SourceKindKey, string> = {
  gmail: 'M',
  telegram: 'T',
};

export const SOURCE_GRADIENT: Record<SourceKindKey, string> = {
  gmail: 'var(--src-gmail)',
  telegram: 'var(--src-tg)',
};

type SourceKindKey = 'gmail' | 'telegram';
