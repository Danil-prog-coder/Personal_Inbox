import { useState } from 'react';
import { Icon, LogoIcon } from '../components/Icon';
import { api, ApiError } from '../lib/api';
import type { User } from '../lib/types';

const SUBTITLE =
  'Одна лента вместо десяти вкладок. Почта и мессенджеры, отсортированные по тому, ' +
  'что действительно требует вас.';

interface Props {
  onAuthenticated: (user: User, isNew: boolean) => void;
}

/** Вход и регистрация — на одном экране, без отдельной страницы. */
export function AuthScreen({ onAuthenticated }: Props) {
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const register = mode === 'register';

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setError('');
    if (register && password.length < 8) {
      setError('Пароль должен быть не короче 8 символов');
      return;
    }
    setBusy(true);
    try {
      const user = register
        ? await api.register(email.trim(), password)
        : await api.login(email.trim(), password);
      onAuthenticated(user, register);
    } catch (caught) {
      setError(caught instanceof ApiError ? caught.message : 'Что-то пошло не так');
    } finally {
      setBusy(false);
    }
  };

  return (
    <form className="auth" onSubmit={submit}>
      <div className="auth__floaty" aria-hidden="true" />
      <div className="auth__header">
        <div className="auth__brand">
          <span className="auth__brand-mark">
            <LogoIcon />
          </span>
          <span className="auth__brand-name">Personal Inbox</span>
        </div>
        <h1 className="auth__title">{register ? 'Создайте аккаунт' : 'С возвращением'}</h1>
        <p className="auth__subtitle">{SUBTITLE}</p>
      </div>

      <div className="auth__form">
        <div className="auth__fields">
          <label className="field">
            <span className="field__label">Email</span>
            <input
              className="field__input"
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
          </label>
          <label className="field">
            <span className="field__label">Пароль</span>
            <input
              className="field__input field__input--password"
              type="password"
              autoComplete={register ? 'new-password' : 'current-password'}
              required
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </label>
        </div>

        {error && <span className="auth__error">{error}</span>}

        <button type="submit" className="btn-primary auth__submit" disabled={busy}>
          {register ? 'Создать аккаунт' : 'Войти'}
        </button>
        <button
          type="button"
          className="auth__switch"
          onClick={() => {
            setMode(register ? 'login' : 'register');
            setError('');
          }}
        >
          {register ? 'Уже есть аккаунт? Войти' : 'Нет аккаунта? Зарегистрироваться'}
        </button>
      </div>
    </form>
  );
}

interface OnboardingProps {
  onDone: (criteria: string) => void;
}

/** Онбординг критериев. Шаг можно пропустить — тогда критерии пустые. */
export function OnboardingScreen({ onDone }: OnboardingProps) {
  const [criteria, setCriteria] = useState('');

  return (
    <div className="auth">
      <div className="auth__floaty" aria-hidden="true" />
      <div className="auth__header">
        <h1 className="auth__title">Что для вас важно?</h1>
        <p className="auth__subtitle">
          Опишите своими словами, какие сообщения нельзя пропустить. Это можно изменить позже
          в настройках.
        </p>
      </div>

      <div className="auth__form" style={{ maxWidth: 520 }}>
        <div className="criteria-panel">
          <textarea
            className="textarea"
            value={criteria}
            onChange={(event) => setCriteria(event.target.value)}
            placeholder="Например: важны письма от клиентов Northline, всё про договоры и сроки, сообщения от команды с блокерами. Рассылки и уведомления сервисов — неважно."
            aria-label="Критерии важности"
          />
        </div>
        <div className="criteria-note">
          <span style={{ display: 'flex', flex: 'none', marginTop: 1, opacity: 0.5 }}>
            <Icon name="dot" size={17} />
          </span>
          <p className="criteria-note__text">
            Это не жёсткие правила. Текст уходит в модель как контекст: она сама решает, что для вас
            критично, и уточняет оценку по вашим исправлениям.
          </p>
        </div>
        <div className="criteria-actions">
          <button type="button" className="btn-primary" onClick={() => onDone(criteria)}>
            Начать анализ
          </button>
          <button type="button" className="btn-secondary" onClick={() => onDone('')}>
            Пропустить
          </button>
        </div>
      </div>
    </div>
  );
}
