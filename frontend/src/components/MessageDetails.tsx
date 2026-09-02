import { useEffect, useState } from 'react';
import { Avatar } from './Avatar';
import { Icon } from './Icon';
import { Portal } from './Portal';
import { Segmented } from './Segmented';
import { formatTime, visibleCategory } from '../lib/format';
import { LEVEL_LABEL, LEVEL_ORDER, SOURCE_LABEL } from '../lib/levels';
import type { Level, Message } from '../lib/types';

interface Props {
  message: Message;
  onClose: () => void;
  onLevelChange: (level: Level) => void;
}

const LEVEL_OPTIONS = LEVEL_ORDER.map((level) => ({ value: level, label: LEVEL_LABEL[level] }));

/**
 * Телефон — лист снизу, десктоп — панель справа (разводится в CSS).
 * Единственная точка ручного исправления уровня во всём приложении.
 */
export function MessageDetails({ message, onClose, onLevelChange }: Props) {
  const [saved, setSaved] = useState(false);
  const category = visibleCategory(message.category);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [onClose]);

  useEffect(() => {
    if (!saved) return;
    const timer = window.setTimeout(() => setSaved(false), 2200);
    return () => window.clearTimeout(timer);
  }, [saved]);

  const changeLevel = (level: Level) => {
    if (level === message.level) return;
    onLevelChange(level);
    setSaved(true);
  };

  return (
    <Portal>
      <button type="button" className="overlay" aria-label="Закрыть детали" onClick={onClose} />
      <section className="sheet" role="dialog" aria-modal="true" aria-label={message.subject}>
        <div className="sheet__head">
          <button type="button" className="btn-close" onClick={onClose} aria-label="Закрыть">
            <Icon name="close" size={15} />
          </button>
          <span className="sheet__source ellipsis">{SOURCE_LABEL[message.source]}</span>
          {message.external_url && (
            <a
              className="sheet__open"
              href={message.external_url}
              target="_blank"
              rel="noreferrer noopener"
            >
              <Icon name="ext" size={14} />
              Открыть в {SOURCE_LABEL[message.source]}
            </a>
          )}
        </div>

        <div className="sheet__body">
          <div className="sheet__sender">
            <Avatar name={message.sender_name} source={message.source} />
            <div className="head-titles">
              <span className="msg-card__from ellipsis">{message.sender_name}</span>
              <span className="msg-card__addr ellipsis">
                {message.sender_addr} · {formatTime(message.received_at)}
              </span>
            </div>
          </div>

          <h2 className="sheet__subject">{message.subject}</h2>

          {message.summary && (
            <div className="summary-block">
              <span className="summary-block__label">Краткое содержание</span>
              <span className="summary-block__text">{message.summary}</span>
            </div>
          )}

          <div className="sheet__meta">
            {category && (
              <span className="pill">
                Категория: <span className="pill__value">{category}</span>
              </span>
            )}
            {message.deadline_text && (
              <span className="pill">
                <Icon name="clock" size={13} />
                {message.deadline_text}
              </span>
            )}
            {message.needs_action && (
              <span className="pill pill--action">
                <Icon name="bolt" size={13} />
                Требует действия
              </span>
            )}
            {message.needs_reply && (
              <span className="pill pill--reply">
                <Icon name="reply" size={13} />
                Требует ответа
              </span>
            )}
          </div>

          <p className="sheet__text">{message.body}</p>

          <div className="level-picker">
            <div className="level-picker__head">
              <span className="level-picker__title">Уровень важности</span>
              {saved && (
                <span className="level-picker__saved">
                  <Icon name="check" size={13} />
                  Сохранено
                </span>
              )}
            </div>
            <Segmented
              options={LEVEL_OPTIONS}
              value={message.level}
              onChange={changeLevel}
              variant="inset"
              ariaLabel="Уровень важности"
            />
            <span className="level-picker__hint">
              Исправление уходит в модель как обратная связь и влияет на оценку похожих сообщений.
            </span>
          </div>
        </div>
      </section>
    </Portal>
  );
}
