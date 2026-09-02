import { avatarGradient, initials } from '../lib/format';
import { SOURCE_GRADIENT, SOURCE_LETTER } from '../lib/levels';
import type { SourceKind } from '../lib/types';

interface Props {
  name: string;
  source?: SourceKind;
}

/** Аватар с инициалами и бейджем источника. Цвет детерминирован именем. */
export function Avatar({ name, source }: Props) {
  return (
    <span className="avatar" style={{ background: avatarGradient(name) }} aria-hidden="true">
      {initials(name)}
      {source && (
        <span className="avatar__source" style={{ background: SOURCE_GRADIENT[source] }}>
          {SOURCE_LETTER[source]}
        </span>
      )}
    </span>
  );
}
