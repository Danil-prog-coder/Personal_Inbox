import { useEffect, useState } from 'react';
import { Icon } from '../components/Icon';
import { Portal } from '../components/Portal';
import { api, ApiError } from '../lib/api';
import { formatSyncedAgo } from '../lib/format';
import { SOURCE_GRADIENT, SOURCE_LABEL, SOURCE_LETTER } from '../lib/levels';
import type { Connection, SourceKind } from '../lib/types';

/** Сервисы из планов развития показываются неактивными (решения №23, №51). */
const SERVICES: {
  kind: SourceKind | 'vk' | 'github';
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
    hint: 'Вход в ваш аккаунт',
    letter: 'T',
    gradient: 'var(--src-tg)',
    soon: false,
  },
  {
    kind: 'vk',
    name: 'VK',
    hint: 'Сообщения сообществ',
    letter: 'B',
    gradient: 'var(--fill2)',
    soon: true,
  },
  {
    kind: 'github',
    name: 'GitHub',
    hint: 'Issues, PR, упоминания',
    letter: 'G',
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
  // Шаги входа: телефон → код → пароль (последний только при двухфакторной).
  const [step, setStep] = useState<'phone' | 'code' | 'password'>('phone');
  const [phone, setPhone] = useState('');
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const reauthCount = connections.filter((item) => item.state === 'reauth').length;

  const closeTelegram = () => {
    setTelegramOpen(false);
    setStep('phone');
    setPhone('');
    setCode('');
    setPassword('');
    setError('');
  };

  // Модальные окна закрываются с клавиатуры так же, как детали сообщения.
  useEffect(() => {
    if (!addOpen && !telegramOpen) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      setAddOpen(false);
      closeTelegram();
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

  const sendCode = async () => {
    setBusy(true);
    setError('');
    try {
      await api.telegramStart(phone.trim());
      setStep('code');
    } catch (caught) {
      setError(caught instanceof ApiError ? caught.message : 'Не удалось отправить код');
    } finally {
      setBusy(false);
    }
  };

  const confirmCode = async () => {
    setBusy(true);
    setError('');
    try {
      const result = await api.telegramConfirm(code.trim(), password);
      if ('password_needed' in result) {
        setStep('password');
        return;
      }
      closeTelegram();
      onChanged();
      onToast(`Telegram подключён — ${result.account}`);
    } catch (caught) {
      setError(caught instanceof ApiError ? caught.message : 'Не удалось подключить Telegram');
      // Код одноразовый: сервер его уже забрал, повтор возможен только
      // с новым кодом.
      if (caught instanceof ApiError && caught.status === 400) setStep('phone');
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
      <div className="screen-flow">
        <div className="screen-flow__column">
        <div className="head-titles">
          <h1 className="screen-title">Источники</h1>
          <span className="screen-subtitle">
            {reauthCount > 0 ? 'Один источник требует внимания' : 'Все источники синхронизированы'}
          </span>
        </div>
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
            onClick={closeTelegram}
          />
          <div className="modal" role="dialog" aria-modal="true" aria-label="Подключение Telegram">
            <div className="modal__head">
              <span className="modal__title">Подключение Telegram</span>
              <button
                type="button"
                className="btn-close"
                aria-label="Закрыть"
                onClick={closeTelegram}
              >
                <Icon name="close" size={15} />
              </button>
            </div>
            {step === 'phone' && (
              <>
                <p className="panel__hint" style={{ padding: '0 4px' }}>
                  Вход в ваш аккаунт Telegram — тот же, что в официальном приложении.
                  Пришлём код подтверждения, как при входе на новом устройстве.
                </p>
                <div className="warn-note" style={{ width: 'auto' }}>
                  <span className="warn-note__icon">
                    <Icon name="warn" size={14} />
                  </span>
                  Приложение получит доступ ко всей переписке аккаунта и будет хранить
                  ключ сессии в своей базе. Держите его так же, как пароль.
                </div>
                <label className="field">
                  <span className="field__label">Номер телефона</span>
                  <input
                    className="field__input"
                    type="tel"
                    autoComplete="tel"
                    value={phone}
                    onChange={(event) => setPhone(event.target.value)}
                    placeholder="+7 999 000-00-00"
                  />
                </label>
              </>
            )}

            {step === 'code' && (
              <>
                <p className="panel__hint" style={{ padding: '0 4px' }}>
                  Код отправлен в Telegram на номер {phone.trim()}. Ищите его в чате
                  «Telegram», а не в SMS.
                </p>
                <label className="field">
                  <span className="field__label">Код из Telegram</span>
                  <input
                    className="field__input"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    value={code}
                    onChange={(event) => setCode(event.target.value)}
                    placeholder="12345"
                  />
                </label>
              </>
            )}

            {step === 'password' && (
              <>
                <p className="panel__hint" style={{ padding: '0 4px' }}>
                  У аккаунта включена двухфакторная защита. Введите облачный пароль —
                  тот, который Telegram спрашивает после кода.
                </p>
                <label className="field">
                  <span className="field__label">Облачный пароль</span>
                  <input
                    className="field__input"
                    type="password"
                    autoComplete="current-password"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                  />
                </label>
              </>
            )}

            {error && <span className="auth__error">{error}</span>}

            {step === 'phone' ? (
              <button
                type="button"
                className="btn-primary"
                disabled={busy || phone.trim().length < 5}
                onClick={sendCode}
              >
                Получить код
              </button>
            ) : (
              <button
                type="button"
                className="btn-primary"
                disabled={busy || (step === 'code' ? code.trim() === '' : password === '')}
                onClick={confirmCode}
              >
                {step === 'code' ? 'Подтвердить' : 'Войти'}
              </button>
            )}
          </div>
        </Portal>
      )}
    </div>
  );
}
