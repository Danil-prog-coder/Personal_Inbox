import { Icon } from './Icon';
import { messagesCount, plural } from '../lib/format';
import { LEVEL_COLOR, LEVEL_ORDER, SOURCE_GRADIENT, SOURCE_LABEL, SOURCE_LETTER } from '../lib/levels';
import type { SourceCard } from '../lib/types';

interface Props {
  card: SourceCard;
  onOpen: (kind: SourceCard['kind']) => void;
}

export function SourceCardItem({ card, onOpen }: Props) {
  const urgentColor = LEVEL_COLOR[card.urgent ? card.urgent.level : 'LOW'];

  return (
    <button type="button" className="source-card" onClick={() => onOpen(card.kind)}>
      <div className="source-card__row">
        <span className="source-mark" style={{ background: SOURCE_GRADIENT[card.kind] }}>
          {SOURCE_LETTER[card.kind]}
        </span>
        <span className="source-card__titles">
          <span className="source-card__name">{SOURCE_LABEL[card.kind]}</span>
          <span className="source-card__account ellipsis">
            {card.account} · {card.total} {plural(card.total, 'сообщение', 'сообщения', 'сообщений')}
          </span>
        </span>
        {card.unread > 0 && <span className="source-card__unread">{card.unread}</span>}
        <span style={{ display: 'flex', color: 'var(--ink3)' }}>
          <Icon name="chev" size={17} />
        </span>
      </div>

      {/* Полоса распределения: max(count, 0.3), иначе пустой уровень схлопывается. */}
      <div className="bars" aria-hidden="true">
        {LEVEL_ORDER.map((level) => (
          <div
            key={level}
            className="bars__segment"
            style={{
              flex: Math.max(card.distribution[level] ?? 0, 0.3),
              background: LEVEL_COLOR[level],
              boxShadow: `0 0 8px color-mix(in srgb, ${LEVEL_COLOR[level]} 40%, transparent)`,
            }}
          />
        ))}
      </div>

      <div className="source-card__urgent">
        <span
          className="dot"
          aria-hidden="true"
          style={{
            background: urgentColor,
            boxShadow: `0 0 8px color-mix(in srgb, ${urgentColor} 67%, transparent)`,
          }}
        />
        <span className="source-card__urgent-text ellipsis">
          {card.urgent ? `${card.urgent.sender_name} — ${card.urgent.subject}` : 'Ничего срочного'}
        </span>
      </div>

      {card.state === 'reauth' && (
        <span className="warn-note">
          <span className="warn-note__icon">
            <Icon name="warn" size={14} />
          </span>
          Нужна повторная авторизация — новые сообщения не поступают
        </span>
      )}

      <span className="visually-hidden">{messagesCount(card.total, card.unread)}</span>
    </button>
  );
}
