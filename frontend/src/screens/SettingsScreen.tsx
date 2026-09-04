import { useState } from 'react';
import { Icon } from '../components/Icon';
import { Segmented } from '../components/Segmented';
import { api } from '../lib/api';
import type { Density, Theme, User } from '../lib/types';

interface Props {
  user: User;
  theme: Theme;
  density: Density;
  onThemeChange: (theme: Theme) => void;
  onDensityChange: (density: Density) => void;
  onUserChange: (user: User) => void;
  onGoConnections: () => void;
  onToast: (text: string) => void;
}

const THEMES: { value: Theme; label: string }[] = [
  { value: 'dark', label: 'Тёмное' },
  { value: 'light', label: 'Светлое' },
];

const DENSITIES: { value: Density; label: string }[] = [
  { value: 'spacious', label: 'Просторно' },
  { value: 'compact', label: 'Компактно' },
];

export function SettingsScreen({
  user,
  theme,
  density,
  onThemeChange,
  onDensityChange,
  onUserChange,
  onGoConnections,
  onToast,
}: Props) {
  const [criteria, setCriteria] = useState(user.criteria);
  const [savingCriteria, setSavingCriteria] = useState(false);

  const saveCriteria = async () => {
    setSavingCriteria(true);
    try {
      const result = await api.updateMe({ criteria });
      onUserChange(result.user);
      onToast('Критерии сохранены — переоценка запущена');
    } finally {
      setSavingCriteria(false);
    }
  };

  return (
    <div className="screen">
      <div className="screen-flow">
        <div className="screen-flow__column">
        <h1 className="screen-title">Настройки</h1>
        <div className="panel">
          <div className="head-titles">
            <span className="panel__title">Критерии важности</span>
            <span className="panel__hint">
              Свободный текст — контекст для модели, а не жёсткие правила.
            </span>
          </div>
          <div className="textarea-panel">
            <textarea
              className="textarea"
              value={criteria}
              onChange={(event) => setCriteria(event.target.value)}
              placeholder="Важны письма от клиентов Northline, всё про договоры и сроки…"
              aria-label="Критерии важности"
            />
          </div>
          <div className="warn-note" style={{ width: 'auto' }}>
            <span className="warn-note__icon">
              <Icon name="warn" size={14} />
            </span>
            После сохранения существующие сообщения будут переоценены. Это происходит фоново
            и может занять до нескольких часов.
          </div>
          <button
            type="button"
            className="btn-small"
            style={{ alignSelf: 'flex-start' }}
            disabled={savingCriteria || criteria === user.criteria}
            onClick={saveCriteria}
          >
            Сохранить критерии
          </button>
        </div>

        <div className="panel">
          <div className="head-titles">
            <span className="panel__title">Внешний вид</span>
          </div>
          <div className="appearance-row">
            <span className="settings-row__label">Тема</span>
            <div className="appearance-row__control">
              <Segmented
                options={THEMES}
                value={theme}
                onChange={onThemeChange}
                ariaLabel="Тема"
              />
            </div>
          </div>
          <div className="appearance-row">
            <span className="settings-row__label">Плотность карточек</span>
            <div className="appearance-row__control">
              <Segmented
                options={DENSITIES}
                value={density}
                onChange={onDensityChange}
                ariaLabel="Плотность карточек"
              />
            </div>
          </div>
        </div>

        <div className="panel panel--rows">
          <button type="button" className="settings-row" onClick={onGoConnections}>
            <span className="settings-row__body">
              <span className="settings-row__label">Управление подключениями</span>
              <span className="settings-row__hint">Gmail, Telegram</span>
            </span>
            <span style={{ flex: 'none', color: 'var(--ink3)', display: 'flex' }}>
              <Icon name="chev" size={16} />
            </span>
          </button>

        </div>
        </div>
      </div>
    </div>
  );
}
