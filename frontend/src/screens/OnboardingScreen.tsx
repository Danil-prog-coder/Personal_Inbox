import { useState } from 'react';
import { Icon } from '../components/Icon';

interface OnboardingProps {
  onDone: (criteria: string) => void;
}

/** Первый запуск: спрашиваем критерии важности. Шаг можно пропустить —
 * тогда критерии пустые и модель работает на общих основаниях.
 *
 * Раньше экран показывался после регистрации; регистрации больше нет,
 * и показывает его теперь App — один раз на установку (решение №50). */
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
