import { useCallback, useEffect, useMemo, useState } from 'react';
import { FiltersPanel } from '../components/FiltersPanel';
import { Icon } from '../components/Icon';
import { MessageCard } from '../components/MessageCard';
import { MessageDetails } from '../components/MessageDetails';
import { SourceCardItem } from '../components/SourceCardItem';
import { api } from '../lib/api';
import { messagesCount } from '../lib/format';
import { SOURCE_LABEL } from '../lib/levels';
import { EMPTY_FILTERS } from '../lib/types';
import type { Filters, Level, Message, SourceCard, SourceKind } from '../lib/types';
import type { StreamEvent } from '../hooks/useStream';

interface Props {
  sources: SourceCard[];
  openedSource: SourceKind | null;
  onOpenSource: (kind: SourceKind | null) => void;
  streamEvent: StreamEvent | null;
  onDataChanged: () => void;
  /** Сообщение, которое нужно открыть сразу — переход из сводки. */
  openMessageId: number | null;
  onMessageOpened: () => void;
}

/** Подсветка новой карточки гаснет через 9 секунд, как в референсе. */
const NEW_HIGHLIGHT_MS = 9000;

export function FeedScreen({
  sources,
  openedSource,
  onOpenSource,
  streamEvent,
  onDataChanged,
  openMessageId,
  onMessageOpened,
}: Props) {
  const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS);
  const [query, setQuery] = useState('');
  // Запрос уходит на сервер не на каждую букву.
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [showFilters, setShowFilters] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);
  const [counts, setCounts] = useState({ total: 0, unread: 0 });
  const [opened, setOpened] = useState<Message | null>(null);
  const [newIds, setNewIds] = useState<number[]>([]);

  const dirty = useMemo(
    () => Object.values(filters).some((value) => value !== 'all'),
    [filters],
  );
  const filtered = dirty || query.trim().length > 0;

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query), 250);
    return () => window.clearTimeout(timer);
  }, [query]);

  const load = useCallback(async () => {
    if (!openedSource) return;
    const list = await api.messages(openedSource, filters, debouncedQuery);
    setMessages(list.items);
    setCounts({ total: list.total, unread: list.unread });
  }, [openedSource, filters, debouncedQuery]);

  useEffect(() => {
    void load();
  }, [load]);

  // Новое сообщение вставляется сверху само, без плашки «Показать новые».
  useEffect(() => {
    if (!streamEvent || !openedSource) return;
    const { name, message } = streamEvent;
    if (message.source !== openedSource) return;

    // При активных фильтрах вставлять вслепую нельзя: сообщение может им
    // не соответствовать. Проще перезапросить выборку у бэкенда.
    if (filtered) {
      void load();
      return;
    }

    setMessages((current) => {
      const known = current.some((item) => item.id === message.id);
      if (name === 'message.created') {
        if (known) return current.map((item) => (item.id === message.id ? message : item));
        setCounts((counters) => ({
          total: counters.total + 1,
          unread: counters.unread + (message.is_read ? 0 : 1),
        }));
        return [message, ...current];
      }
      // Достраиваем карточку на месте, если она есть в текущей выборке.
      return known ? current.map((item) => (item.id === message.id ? message : item)) : current;
    });

    if (name === 'message.created') {
      setNewIds((current) => [...current, message.id]);
      window.setTimeout(
        () => setNewIds((current) => current.filter((id) => id !== message.id)),
        NEW_HIGHLIGHT_MS,
      );
    }
    setOpened((current) => (current && current.id === message.id ? message : current));
  }, [streamEvent, openedSource, filtered, load]);

  // Переход из сводки: карточка открывается сразу, даже если её нет в выборке.
  useEffect(() => {
    if (openMessageId === null) return;
    let cancelled = false;
    api.message(openMessageId).then((message) => {
      if (cancelled) return;
      setOpened(message);
      onMessageOpened();
      if (!message.is_read) {
        void api.markRead(message.id).then((updated) => {
          setOpened((current) => (current && current.id === updated.id ? updated : current));
          onDataChanged();
        });
      }
    });
    return () => {
      cancelled = true;
    };
  }, [openMessageId, onMessageOpened, onDataChanged]);

  const reset = () => {
    setFilters(EMPTY_FILTERS);
    setQuery('');
    setDebouncedQuery('');
  };

  const openMessage = async (message: Message) => {
    setOpened(message);
    if (message.is_read) return;
    // Открытие деталей помечает сообщение прочитанным (решение №12).
    const updated = await api.markRead(message.id);
    setMessages((current) => current.map((item) => (item.id === updated.id ? updated : item)));
    setCounts((current) => ({ ...current, unread: Math.max(0, current.unread - 1) }));
    setOpened(updated);
    onDataChanged();
  };

  const changeLevel = async (level: Level) => {
    if (!opened) return;
    const updated = await api.setLevel(opened.id, level);
    setOpened(updated);
    setMessages((current) => current.map((item) => (item.id === updated.id ? updated : item)));
    onDataChanged();
  };

  if (!openedSource) {
    const total = sources.reduce((sum, card) => sum + card.total, 0);
    const unread = sources.reduce((sum, card) => sum + card.unread, 0);
    return (
      <div className="screen">
        <div className="screen-head">
          <div className="head-row">
            <div className="head-titles">
              <h1 className="screen-title">Лента</h1>
              <span className="screen-subtitle">{messagesCount(total, unread)}</span>
            </div>
          </div>
        </div>
        <div className="screen-body">
          {sources.length === 0 ? (
            <div className="empty">
              <span className="empty__icon">
                <Icon name="connections" size={20} />
              </span>
              <span className="empty__title">Пока пусто</span>
              <span className="empty__text">
                Сообщения появятся здесь после первой синхронизации.
              </span>
            </div>
          ) : (
            sources.map((card) => (
              <SourceCardItem key={card.kind} card={card} onOpen={onOpenSource} />
            ))
          )}
        </div>
      </div>
    );
  }

  const empty = messages.length === 0;
  const sourceCard = sources.find((card) => card.kind === openedSource);

  return (
    <div className="screen">
      <div className="screen-head">
        <div className="head-row">
          <div className="head-left">
            <button
              type="button"
              className="btn-round"
              aria-label="Назад к источникам"
              onClick={() => {
                onOpenSource(null);
                reset();
                setShowFilters(false);
              }}
            >
              <Icon name="back" size={17} />
            </button>
            <div className="head-titles">
              <h1 className="screen-title">{SOURCE_LABEL[openedSource]}</h1>
              <span className="screen-subtitle">
                {messagesCount(counts.total, counts.unread)}
              </span>
            </div>
          </div>
          <button
            type="button"
            className={`filter-button${showFilters ? ' filter-button--on' : ''}`}
            aria-expanded={showFilters}
            onClick={() => setShowFilters((value) => !value)}
          >
            <span style={{ display: 'flex' }}>
              <Icon name="filter" size={15} />
            </span>
            <span>Фильтры</span>
            {dirty && <span className="filter-button__dot" aria-hidden="true" />}
          </button>
        </div>

        <div className="search">
          <span className="search__icon">
            <Icon name="search" size={17} />
          </span>
          <input
            className="search__input"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Поиск по сообщениям"
            aria-label="Поиск по сообщениям"
          />
          {query.trim() && <span className="search__count">{messages.length}</span>}
        </div>

        {showFilters && (
          <FiltersPanel filters={filters} onChange={setFilters} onReset={reset} />
        )}
      </div>

      <div className="screen-body">
        {empty ? (
          dirty || query.trim() ? (
            <div className="empty">
              <span className="empty__icon">
                <Icon name="search" size={20} />
              </span>
              <span className="empty__title">Ничего не найдено</span>
              <span className="empty__text">
                По этим фильтрам сообщений нет. Попробуйте сбросить их или изменить запрос.
              </span>
              <button type="button" className="empty__button" onClick={reset}>
                Сбросить фильтры
              </button>
            </div>
          ) : (
            <div className="empty">
              <span className="empty__icon">
                <Icon name="feed" size={20} />
              </span>
              <span className="empty__title">Пока пусто</span>
              <span className="empty__text">
                Сообщения появятся здесь после первой синхронизации.
              </span>
            </div>
          )
        ) : (
          messages.map((message) => (
            <MessageCard
              key={message.id}
              message={message}
              isNew={newIds.includes(message.id)}
              onOpen={openMessage}
            />
          ))
        )}
        {sourceCard?.state === 'reauth' && !empty && (
          <span className="warn-note">
            <span className="warn-note__icon">
              <Icon name="warn" size={14} />
            </span>
            Токен истёк — новые сообщения не поступают. Ранее полученные остаются в ленте:
            переподключение их не затронет.
          </span>
        )}
      </div>

      {opened && (
        <MessageDetails
          message={opened}
          onClose={() => setOpened(null)}
          onLevelChange={changeLevel}
        />
      )}
    </div>
  );
}
