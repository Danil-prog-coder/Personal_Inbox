import { useEffect, useState } from 'react';
import { Icon } from '../components/Icon';
import { Portal } from '../components/Portal';
import { api, ApiError } from '../lib/api';
import { formatSyncedAgo } from '../lib/format';
import { SOURCE_GRADIENT, SOURCE_LABEL, SOURCE_LETTER } from '../lib/levels';
import type { Connection, SourceKind } from '../lib/types';

/** Сервисы из планов развития показываются неактивными (решение №23). */
const SERVICES: {
  kind: SourceKind | 'github' | 'slack' | 'notion';
  name: string;
  hint: string;
  letter: string;
  gradient: string;
  soon: boolean;
}[] = [
  {
    kind: 'gmail',
    name: 'Gmail',
    hint: 'Авторизация через Google',
    letter: 'M',
    gradient: 'var(--src-gmail)',
    soon: false,
  },
  {
    kind: 'telegram',
    name: 'Telegram',
    hint: 'Нужно написать боту @inbox_bot',
    letter: 'T',
    gradient: 'var(--src-tg)',
    soon: false,
  },
  {
    kind: 'github',
    name: 'GitHub',
    hint: 'Issues, PR, упоминания',
    letter: 'G',
    gradient: 'var(--fill2)',
    soon: true,
  },
  {
    kind: 'slack',
    name: 'Slack',
    hint: 'Треды и личные сообщения',
    letter: 'S',
    gradient: 'var(--fill2)',
    soon: true,
  },
  {
    kind: 'notion',
    name: 'Notion',
    hint: 'Комментарии и упоминания',
    letter: 'N',
    gradient: 'var(--fill2)',
    soon: true,
  },
];

interface Props {
  connections: Connection[];
  onChanged: () => void;
  onToast: (text: string) => void;
}

export function ConnectionsScreen({ connections, onChanged, onToast }: Props) {
  const [addOpen, setAddOpen] = useState(false);
  const [telegramOpen, setTelegramOpen] = useState(false);
  const [botToken, setBotToken] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const reauthCount = connections.filter((item) => item.state === 'reauth').length;

  // Модальные окна закрываются с клавиатуры так же, как детали сообщения.
  useEffect(() => {
    if (!addOpen && !telegramOpen) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      setAddOpen(false);
      setTelegramOpen(false);
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [addOpen, telegramOpen]);

  const connectGmail = async () => {
    setError('');
    try {
      const { auth_url: url } = await api.gmailAuthUrl();
      window.location.href = url;
    } catch (caught) {
      setError(caught instanceof ApiError ? caught.message : 'Не удалось начать авторизацию');
    }
  };

  const connectTelegram = async () => {
    setBusy(true);
    setError('');
    try {
      const connection = await api.connectTelegram(botToken.trim());
      setTelegramOpen(false);
      setBotToken('');
      onChanged();
      onToast(`Telegram подключён — ${connection.account}`);
    } catch (caught) {
      setError(caught instanceof ApiError ? caught.message : 'Не удалось подключить бота');
    } finally {
      setBusy(false);
    }
  };

  const disconnect = async (kind: SourceKind) => {
    await api.disconnect(kind);
    onChanged();
    onToast(`${SOURCE_LABEL[kind]} отключён`);
  };

  const startConnect = (kind: SourceKind) => {
    setAddOpen(false);
    setError('');
    if (kind === 'gmail') {
      void connectGmail();
    } else {
      setTelegramOpen(true);
    }
  };

  return (
    <div className="screen">
      <div className="screen-head">
        <div className="head-titles">
          <h1 className="screen-title">Источники</h1>
          <span className="screen-subtitle">
            {reauthCount > 0 ? 'Один источник требует внимания' : 'Все источники синхронизированы'}
          </span>
        </div>
      </div>

      <div className="screen-body screen-body--narrow">
        {connections.map((connection) => (
          <div
            key={connection.kind}
            className={`conn-card${connection.state === 'reauth' ? ' conn-card--reauth' : ''}`}
          >
            <div className="conn-card__row">
              <span
                className="conn-card__mark"
                style={{ background: SOURCE_GRADIENT[connection.kind] }}
              >
                {SOURCE_LETTER[connection.kind]}
              </span>
              <div className="conn-card__titles">
                <span className="conn-card__name">{SOURCE_LABEL[connection.kind]}</span>
                <span
                  className={
                    connection.state === 'reauth'
                      ? 'conn-card__status conn-card__status--warn'
                      : connection.state === 'off'
                        ? 'conn-card__status conn-card__status--off'
                        : 'conn-card__status'
                  }
                >
                  {connection.state === 'active' &&
                    `${connection.account} · ${formatSyncedAgo(connection.last_sync_at)}`}
                  {connection.state === 'reauth' && 'Требуется повторная авторизация'}
                  {connection.state === 'off' && 'Не подключён'}
                </span>
              </div>
              {connection.state === 'active' ? (
                <button
                  type="button"
                  className="btn-small btn-small--secondary"
                  onClick={() => disconnect(connection.kind)}
                >
                  Отключить
                </button>
              ) : (
                <button
                  type="button"
                  className="btn-small"
                  onClick={() => startConnect(connection.kind)}
                >
                  {connection.state === 'reauth' ? 'Переподключить' : 'Подключить'}
                </button>
              )}
            </div>
            {connection.state === 'reauth' && (
              <div className="warn-note" style={{ margin: '0 10px 10px', width: 'auto' }}>
                <span className="warn-note__icon">
                  <Icon name="warn" size={14} />
                </span>
                Токен истёк — новые сообщения не поступают. Ранее полученные остаются в ленте:
                переподключение их не затронет.
              </div>
            )}
          </div>
        ))}

        {error && <span className="auth__error">{error}</span>}

        <button type="button" className="add-source" onClick={() => setAddOpen(true)}>
          <span style={{ display: 'flex' }}>
            <Icon name="plus" size={17} />
          </span>
          Добавить источник
        </button>
      </div>

      {addOpen && (
        <Portal>
          <button
            type="button"
            className="overlay"
            aria-label="Закрыть выбор сервиса"
            onClick={() => setAddOpen(false)}
          />
          <div className="modal" role="dialog" aria-modal="true" aria-label="Выберите сервис">
            <div className="modal__head">
              <span className="modal__title">Выберите сервис</span>
              <button
                type="button"
                className="btn-close"
                aria-label="Закрыть"
                onClick={() => setAddOpen(false)}
              >
                <Icon name="close" size={15} />
              </button>
            </div>
            {SERVICES.map((service) => (
              <button
                key={service.kind}
                type="button"
                className="service"
                disabled={service.soon}
                onClick={() => startConnect(service.kind as SourceKind)}
              >
                <span className="conn-card__mark" style={{ background: service.gradient }}>
                  {service.letter}
                </span>
                <span className="service__body">
                  <span className="service__name">{service.name}</span>
                  <span className="service__hint">{service.hint}</span>
                </span>
                {service.soon && <span className="service__tag">Скоро</span>}
              </button>
            ))}
          </div>
        </Portal>
      )}

      {telegramOpen && (
        <Portal>
          <button
            type="button"
            className="overlay"
            aria-label="Закрыть подключение Telegram"
            onClick={() => setTelegramOpen(false)}
          />
          <div className="modal" role="dialog" aria-modal="true" aria-label="Подключение Telegram">
            <div className="modal__head">
              <span className="modal__title">Подключение Telegram</span>
              <button
                type="button"
                className="btn-close"
                aria-label="Закрыть"
                onClick={() => setTelegramOpen(false)}
              >
                <Icon name="close" size={15} />
              </button>
            </div>
            <p className="panel__hint" style={{ padding: '0 4px' }}>
              1. Создайте бота через @BotFather и скопируйте токен.
              <br />
              2. Вставьте токен сюда — проверим его и покажем имя бота.
              <br />
              3. Добавьте бота в чаты, которые хотите видеть в ленте.
            </p>
            <div className="warn-note" style={{ width: 'auto' }}>
              <span className="warn-note__icon">
                <Icon name="warn" size={14} />
              </span>
              Бот видит только те чаты, куда его добавили, и не видит вашу личную переписку.
              История до добавления бота недоступна.
            </div>
            <label className="field">
              <span className="field__label">Токен бота</span>
              <input
                className="field__input"
                value={botToken}
                onChange={(event) => setBotToken(event.target.value)}
                placeholder="123456789:AA..."
              />
            </label>
            {error && <span className="auth__error">{error}</span>}
            <button
              type="button"
              className="btn-primary"
              disabled={busy || botToken.trim().length < 10}
              onClick={connectTelegram}
            >
              Проверить и подключить
            </button>
          </div>
        </Portal>
      )}
    </div>
  );
}
