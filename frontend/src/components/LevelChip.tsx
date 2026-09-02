import { Icon } from './Icon';
import { LEVEL_BG, LEVEL_ICON, LEVEL_INK, LEVEL_LABEL, LEVEL_COLOR } from '../lib/levels';
import type { Level } from '../lib/types';

/** Уровень важности = цвет + иконка + подпись. Только цветом — никогда. */
export function LevelChip({ level }: { level: Level }) {
  return (
    <span
      className="level-chip"
      style={{
        color: LEVEL_INK[level],
        background: LEVEL_BG[level],
        border: `1px solid color-mix(in srgb, ${LEVEL_COLOR[level]} 27%, transparent)`,
      }}
    >
      <Icon name={LEVEL_ICON[level]} size={12} />
      {LEVEL_LABEL[level]}
    </span>
  );
}
