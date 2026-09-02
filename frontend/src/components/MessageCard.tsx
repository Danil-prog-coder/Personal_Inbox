import { Avatar } from './Avatar';
import { Icon } from './Icon';
import { LevelChip } from './LevelChip';
import { formatTime, visibleCategory } from '../lib/format';
import { LEVEL_COLOR } from '../lib/levels';
import type { Message } from '../lib/types';

interface Props {
  message: Message;
  isNew: boolean;
  onOpen: (message: Message) => void;
}

export function MessageCard({ message, isNew, onOpen }: Props) {
  const processing = message.status === 'PROCESSING';
  const category = visibleCategory(message.category);

  return (
    <article className={`msg-card${isNew ? ' msg-card--new' : ''}`}>
      <div
        className="msg-card__edge"
        aria-hidden="true"
        style={{
          background: LEVEL_COLOR[message.level],
          boxShadow: `0 0 12px color-mix(in srgb, ${LEVEL_COLOR[message.level]} 60%, transparent)`,
        }}
      />
      <button type="button" className="msg-card__button" onClick={() => onOpen(message)}>
        <Avatar name={message.sender_name} source={message.source} />
        <div className="msg-card__content">
          <div className="msg-card__top">
            <span className="msg-card__sender">
              <span className="msg-card__from ellipsis">{message.sender_name}</span>
              <span className="msg-card__addr ellipsis">
                {message.sender_addr} · {formatTime(message.received_at)}
              </span>
            </span>
            {!message.is_read && <span className="msg-card__unread-dot" aria-label="Не прочитано" />}
          </div>

          <span
            className={`msg-card__subject${message.is_read ? '' : ' msg-card__subject--unread'}`}
          >
            {message.subject}
          </span>
          <span className="msg-card__snippet">{message.body}</span>

          {processing ? (
            <div className="msg-card__processing">
              <span className="spinner" aria-hidden="true" />
              <span className="processing-label">Определяем важность…</span>
              <span className="shimmer" aria-hidden="true" />
            </div>
          ) : (
            <div className="msg-card__meta">
              <LevelChip level={message.level} />
              {category && <span className="meta-chip">{category}</span>}
              {message.deadline_text && (
                <span
                  className={`deadline-chip${
                    message.level === 'CRITICAL' ? ' deadline-chip--critical' : ''
                  }`}
                >
                  <Icon name="clock" size={12} />
                  {message.deadline_text}
                </span>
              )}
              {message.needs_action && (
                <span className="flag flag--action" title="Требует действия">
                  <Icon name="bolt" size={12} />
                </span>
              )}
              {message.needs_reply && (
                <span className="flag flag--reply" title="Требует ответа">
                  <Icon name="reply" size={12} />
                </span>
              )}
              {message.analysis_failed && <span className="failed-note">Оценка недоступна</span>}
            </div>
          )}
        </div>
      </button>
    </article>
  );
}
